package protokube

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
	"time"
)

// The tag name we use to differentiate multiple logically independent clusters running in the same region
const TagNameKubernetesCluster = "KubernetesCluster"

// The tag name we use for specifying that something is in the master role
const TagNameRoleMaster = "k8s.io/role/master"

const DefaultAttachDevice = "/dev/xvdb"

type AWSVolumes struct {
	ec2      *ec2.EC2
	metadata *ec2metadata.EC2Metadata

	zone       string
	clusterTag string
	instanceId string
}

var _ Volumes = &AWSVolumes{}

func NewAWSVolumes() (*AWSVolumes, error) {
	a := &AWSVolumes{}

	s := session.New()
	s.Handlers.Send.PushFront(func(r *request.Request) {
		// Log requests
		glog.V(4).Infof("AWS API Request: %s/%s", r.ClientInfo.ServiceName, r.Operation)
	})

	config := aws.NewConfig()
	a.metadata = ec2metadata.New(s, config)

	region, err := a.metadata.Region()
	if err != nil {
		return nil, fmt.Errorf("error querying ec2 metadata service (for az/region): %v", err)
	}

	a.zone, err = a.metadata.GetMetadata("placement/availability-zone")
	if err != nil {
		return nil, fmt.Errorf("error querying ec2 metadata service (for az): %v", err)
	}

	a.instanceId, err = a.metadata.GetMetadata("instance-id")
	if err != nil {
		return nil, fmt.Errorf("error querying ec2 metadata service (for instance-id): %v", err)
	}

	a.ec2 = ec2.New(s, config.WithRegion(region))

	err = a.discoverTags()
	if err != nil {
		return nil, err
	}

	return a, nil
}

func (a *AWSVolumes) discoverTags() error {
	instance, err := a.describeInstance()
	if err != nil {
		return err
	}

	tagMap := make(map[string]string)
	for _, tag := range instance.Tags {
		tagMap[aws.StringValue(tag.Key)] = aws.StringValue(tag.Value)
	}

	clusterID := tagMap[TagNameKubernetesCluster]
	if clusterID == "" {
		return fmt.Errorf("Cluster tag %q not found on this instance (%q)", TagNameKubernetesCluster, a.instanceId)
	}

	a.clusterTag = clusterID

	return nil
}

func (a *AWSVolumes) describeInstance() (*ec2.Instance, error) {
	request := &ec2.DescribeInstancesInput{}
	request.InstanceIds = []*string{&a.instanceId}

	var instances []*ec2.Instance
	err := a.ec2.DescribeInstancesPages(request, func(p *ec2.DescribeInstancesOutput, lastPage bool) (shouldContinue bool) {
		for _, r := range p.Reservations {
			instances = append(instances, r.Instances...)
		}
		return true
	})

	if err != nil {
		return nil, fmt.Errorf("error querying for EC2 instance %q: %v", a.instanceId, err)
	}

	if len(instances) != 1 {
		return nil, fmt.Errorf("unexpected number of instances found with id %q: %d", a.instanceId, len(instances))
	}

	return instances[0], nil
}

func newEc2Filter(name string, value string) *ec2.Filter {
	filter := &ec2.Filter{
		Name: aws.String(name),
		Values: []*string{
			aws.String(value),
		},
	}
	return filter
}

func (a *AWSVolumes) FindMountedVolumes() ([]*Volume, error) {
	request := &ec2.DescribeVolumesInput{}
	request.Filters = []*ec2.Filter{
		newEc2Filter("tag:"+TagNameKubernetesCluster, a.clusterTag),
		newEc2Filter("tag-key", TagNameRoleMaster),
		newEc2Filter("attachment.instance-id", a.instanceId),
	}

	var volumes []*Volume
	err := a.ec2.DescribeVolumesPages(request, func(p *ec2.DescribeVolumesOutput, lastPage bool) (shouldContinue bool) {
		for _, v := range p.Volumes {
			vol := &Volume{
				Name:      aws.StringValue(v.VolumeId),
				Available: false,
			}

			var myAttachment *ec2.VolumeAttachment

			for _, attachment := range v.Attachments {
				if aws.StringValue(attachment.InstanceId) == a.instanceId {
					myAttachment = attachment
				}
			}

			if myAttachment == nil {
				glog.Warningf("Requested volumes attached to this instance, but volume %q was returned that was not attached", a.instanceId)
				continue
			}

			vol.Device = aws.StringValue(myAttachment.Device)
			volumes = append(volumes, vol)
		}
		return true
	})

	if err != nil {
		return nil, fmt.Errorf("error querying for EC2 volumes: %v", err)
	}
	return volumes, nil
}

func (a *AWSVolumes) FindMountableVolumes() ([]*Volume, error) {
	request := &ec2.DescribeVolumesInput{}
	request.Filters = []*ec2.Filter{
		newEc2Filter("tag:"+TagNameKubernetesCluster, a.clusterTag),
		newEc2Filter("tag-key", TagNameRoleMaster),
		newEc2Filter("availability-zone", a.zone),
	}

	var volumes []*Volume
	err := a.ec2.DescribeVolumesPages(request, func(p *ec2.DescribeVolumesOutput, lastPage bool) (shouldContinue bool) {
		for _, v := range p.Volumes {
			vol := &Volume{
				Name: aws.StringValue(v.VolumeId),
			}
			state := aws.StringValue(v.State)

			switch state {
			case "available":
				vol.Available = true
				break
			}

			volumes = append(volumes, vol)
		}
		return true
	})

	if err != nil {
		return nil, fmt.Errorf("error querying for EC2 volumes: %v", err)
	}
	return volumes, nil
}

// AttachVolume attaches the specified volume to this instance, returning the mountpoint & nil if successful
func (a *AWSVolumes) AttachVolume(volume *Volume) (string, error) {
	volumeID := volume.Name

	device := volume.Device
	if device == "" {
		device = DefaultAttachDevice

		request := &ec2.AttachVolumeInput{
			Device:     aws.String(device),
			InstanceId: aws.String(a.instanceId),
			VolumeId:   aws.String(volumeID),
		}

		attachResponse, err := a.ec2.AttachVolume(request)
		if err != nil {
			return "", fmt.Errorf("Error attaching EBS volume %q: %v", volumeID, err)
		}

		glog.V(2).Infof("AttachVolume request returned %v", attachResponse)
	}

	// Wait (forever) for volume to attach or reach a failure-to-attach condition
	for {
		time.Sleep(10 * time.Second)

		request := &ec2.DescribeVolumesInput{
			VolumeIds: []*string{&volumeID},
		}

		response, err := a.ec2.DescribeVolumes(request)
		if err != nil {
			return "", fmt.Errorf("Error describing EBS volume %q: %v", volumeID, err)
		}

		attachmentState := ""
		for _, v := range response.Volumes {
			for _, a := range v.Attachments {
				attachmentState = aws.StringValue(a.State)
			}
		}

		if attachmentState == "" {
			// TODO: retry?
			// Not attached
			return "", fmt.Errorf("Attach was requested, but volume %q was not seen as attaching", volumeID)
		}

		switch attachmentState {
		case "attached":
			return device, nil

		case "attaching":
			glog.V(2).Infof("Waiting for volume %q to be attached (currently %q)", volumeID, attachmentState)
		// continue looping

		default:
			return "", fmt.Errorf("Observed unexpected volume state %q", attachmentState)
		}
	}
}
