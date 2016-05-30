package awstasks

import (
	"fmt"

	"encoding/base64"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/golang/glog"
	"k8s.io/kube-deploy/upup/pkg/fi"
	"k8s.io/kube-deploy/upup/pkg/fi/cloudup/awsup"
	"strings"
	"time"
)

func buildTimestampString() string {
	now := time.Now()
	return now.UTC().Format("20060102T150405Z")
}

// This one is a little weird because we can't update a launch configuration
// So we have to create the launch configuration as part of the group
//go:generate fitask -type=AutoscalingGroup
type AutoscalingGroup struct {
	Name                    *string

	ID *string

	UserData                *fi.ResourceHolder

	MinSize                 *int64
	MaxSize                 *int64
	Subnets                 []*Subnet
	Tags                    map[string]string

	ImageID                 *string
	InstanceType            *string
	SSHKey                  *SSHKey
	SecurityGroups          []*SecurityGroup
	AssociatePublicIP       *bool
	BlockDeviceMappings     []*BlockDeviceMapping
	IAMInstanceProfile      *IAMInstanceProfile

	launchConfigurationName *string
}

func findAutoscalingGroup(cloud *awsup.AWSCloud, name string) (*autoscaling.Group, error) {
	request := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []*string{&name},
	}

	var found []*autoscaling.Group
	err := cloud.Autoscaling.DescribeAutoScalingGroupsPages(request, func(p *autoscaling.DescribeAutoScalingGroupsOutput, lastPage bool) (shouldContinue bool) {
		for _, g := range p.AutoScalingGroups {
			if aws.StringValue(g.AutoScalingGroupName) == name {
				found = append(found, g)
			} else {
				glog.Warningf("Got ASG with unexpected name")
			}
		}

		return true
	})

	if err != nil {
		return nil, fmt.Errorf("error listing AutoscalingGroups: %v", err)
	}

	if len(found) == 0 {
		return nil, nil
	}

	if len(found) != 1 {
		return nil, fmt.Errorf("Found multiple AutoscalingGroups with name %q", name)
	}

	return found[0], nil
}

func (e *AutoscalingGroup) Find(c *fi.Context) (*AutoscalingGroup, error) {
	cloud := c.Cloud.(*awsup.AWSCloud)

	g, err := findAutoscalingGroup(cloud, *e.Name)
	if err != nil {
		return nil, err
	}
	if g == nil {
		return nil, nil
	}

	actual := &AutoscalingGroup{}
	actual.Name = g.AutoScalingGroupName
	actual.MinSize = g.MinSize
	actual.MaxSize = g.MaxSize

	if g.VPCZoneIdentifier != nil {
		subnets := strings.Split(*g.VPCZoneIdentifier, ",")
		for _, subnet := range subnets {
			actual.Subnets = append(actual.Subnets, &Subnet{ID: aws.String(subnet)})
		}
	}

	if len(g.Tags) != 0 {
		actual.Tags = make(map[string]string)
		for _, tag := range g.Tags {
			actual.Tags[*tag.Key] = *tag.Value
		}
	}

	if g.LaunchConfigurationName == nil {
		return nil, fmt.Errorf("autoscaling Group %q had no LaunchConfiguration", *actual.Name)
	}
	actual.launchConfigurationName = g.LaunchConfigurationName

	found, err := e.findLaunchConfiguration(c, *g.LaunchConfigurationName, actual)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("unable to find autoscaling LaunchConfiguration %q", *g.LaunchConfigurationName)
	}

	if subnetSlicesEqualIgnoreOrder(actual.Subnets, e.Subnets) {
		actual.Subnets = e.Subnets
	}

	return actual, nil
}

func (e *AutoscalingGroup) Run(c *fi.Context) error {
	return fi.DefaultDeltaRunMethod(e, c)
}

func (s *AutoscalingGroup) CheckChanges(a, e, changes *AutoscalingGroup) error {
	if a != nil {
		if e.Name == nil {
			return fi.RequiredField("Name")
		}
	}
	return nil
}

func (e *AutoscalingGroup) buildTags(cloud fi.Cloud) map[string]string {
	tags := make(map[string]string)
	for k, v := range cloud.(*awsup.AWSCloud).BuildTags(e.Name, nil) {
		tags[k] = v
	}
	for k, v := range e.Tags {
		tags[k] = v
	}
	return tags
}

func (_ *AutoscalingGroup) RenderAWS(t *awsup.AWSAPITarget, a, e, changes *AutoscalingGroup) error {
	if a == nil {
		launchConfigurationName := *e.Name + "-" + buildTimestampString()
		glog.V(2).Infof("Creating autoscaling LaunchConfiguration with Name:%q", launchConfigurationName)

		err := renderAutoscalingLaunchConfigurationAWS(t, launchConfigurationName, e)
		if err != nil {
			return err
		}

		glog.V(2).Infof("Creating autoscaling Group with Name:%q", *e.Name)

		request := &autoscaling.CreateAutoScalingGroupInput{}
		request.AutoScalingGroupName = e.Name
		request.LaunchConfigurationName = &launchConfigurationName
		request.MinSize = e.MinSize
		request.MaxSize = e.MaxSize

		var subnetIDs []string
		for _, s := range e.Subnets {
			subnetIDs = append(subnetIDs, *s.ID)
		}
		request.VPCZoneIdentifier = aws.String(strings.Join(subnetIDs, ","))

		tags := []*autoscaling.Tag{}
		for k, v := range e.buildTags(t.Cloud) {
			tags = append(tags, &autoscaling.Tag{
				Key:          aws.String(k),
				Value:        aws.String(v),
				ResourceId:   e.Name,
				ResourceType: aws.String("auto-scaling-group"),
			})
		}
		request.Tags = tags

		_, err = t.Cloud.Autoscaling.CreateAutoScalingGroup(request)
		if err != nil {
			return fmt.Errorf("error creating AutoscalingGroup: %v", err)
		}
	} else {
		if changes.UserData != nil {
			launchConfigurationName := *e.Name + "-" + buildTimestampString()
			glog.V(2).Infof("Creating autoscaling LaunchConfiguration with Name:%q", launchConfigurationName)

			err := renderAutoscalingLaunchConfigurationAWS(t, launchConfigurationName, e)
			if err != nil {
				return err
			}

			request := &autoscaling.UpdateAutoScalingGroupInput{
				AutoScalingGroupName:    e.Name,
				LaunchConfigurationName: &launchConfigurationName,
			}
			_, err = t.Cloud.Autoscaling.UpdateAutoScalingGroup(request)
			if err != nil {
				return fmt.Errorf("error updating AutoscalingGroup: %v", err)
			}
		}
	}

	return nil //return output.AddAWSTags(cloud.Tags(), v, "vpc")
}

func (e *AutoscalingGroup) findLaunchConfiguration(c *fi.Context, name string, dest *AutoscalingGroup) (bool, error) {
	cloud := c.Cloud.(*awsup.AWSCloud)

	request := &autoscaling.DescribeLaunchConfigurationsInput{
		LaunchConfigurationNames: []*string{&name},
	}

	response, err := cloud.Autoscaling.DescribeLaunchConfigurations(request)
	if err != nil {
		return false, fmt.Errorf("error listing AutoscalingLaunchConfigurations: %v", err)
	}

	if response == nil || len(response.LaunchConfigurations) == 0 {
		return false, nil
	}

	if len(response.LaunchConfigurations) != 1 {
		return false, fmt.Errorf("found multiple AutoscalingLaunchConfigurations with name: %q", *e.Name)
	}

	glog.V(2).Info("found existing AutoscalingLaunchConfiguration")
	i := response.LaunchConfigurations[0]
	dest.Name = i.LaunchConfigurationName
	dest.ImageID = i.ImageId
	dest.InstanceType = i.InstanceType
	dest.SSHKey = &SSHKey{Name: i.KeyName}

	securityGroups := []*SecurityGroup{}
	for _, sgID := range i.SecurityGroups {
		securityGroups = append(securityGroups, &SecurityGroup{ID: sgID})
	}
	dest.SecurityGroups = securityGroups
	dest.AssociatePublicIP = i.AssociatePublicIpAddress

	dest.BlockDeviceMappings = []*BlockDeviceMapping{}
	for _, b := range i.BlockDeviceMappings {
		dest.BlockDeviceMappings = append(dest.BlockDeviceMappings, BlockDeviceMappingFromAutoscaling(b))
	}
	userData, err := base64.StdEncoding.DecodeString(*i.UserData)
	if err != nil {
		return false, fmt.Errorf("error decoding UserData: %v", err)
	}
	dest.UserData = fi.WrapResource(fi.NewStringResource(string(userData)))
	dest.IAMInstanceProfile = &IAMInstanceProfile{Name: i.IamInstanceProfile}
	dest.AssociatePublicIP = i.AssociatePublicIpAddress

	// Avoid spurious changes on ImageId
	if e.ImageID != nil && dest.ImageID != nil && *dest.ImageID != *e.ImageID {
		image, err := cloud.ResolveImage(*e.ImageID)
		if err != nil {
			glog.Warningf("unable to resolve image: %q: %v", *e.ImageID, err)
		} else if image == nil {
			glog.Warningf("unable to resolve image: %q: not found", *e.ImageID)
		} else if aws.StringValue(image.ImageId) == *dest.ImageID {
			glog.V(4).Infof("Returning matching ImageId as expected name: %q -> %q", *dest.ImageID, *e.ImageID)
			dest.ImageID = e.ImageID
		}
	}

	return true, nil
}

func renderAutoscalingLaunchConfigurationAWS(t *awsup.AWSAPITarget, name string, e *AutoscalingGroup) error {
	glog.V(2).Infof("Creating AutoscalingLaunchConfiguration with Name:%q", name)

	if e.ImageID == nil {
		return fi.RequiredField("ImageID")
	}
	image, err := t.Cloud.ResolveImage(*e.ImageID)
	if err != nil {
		return err
	}

	request := &autoscaling.CreateLaunchConfigurationInput{}
	request.LaunchConfigurationName = &name
	request.ImageId = image.ImageId
	request.InstanceType = e.InstanceType
	if e.SSHKey != nil {
		request.KeyName = e.SSHKey.Name
	}
	securityGroupIDs := []*string{}
	for _, sg := range e.SecurityGroups {
		securityGroupIDs = append(securityGroupIDs, sg.ID)
	}
	request.SecurityGroups = securityGroupIDs
	request.AssociatePublicIpAddress = e.AssociatePublicIP
	if e.BlockDeviceMappings != nil {
		request.BlockDeviceMappings = []*autoscaling.BlockDeviceMapping{}
		for _, b := range e.BlockDeviceMappings {
			request.BlockDeviceMappings = append(request.BlockDeviceMappings, b.ToAutoscaling())
		}
	}

	if e.UserData != nil {
		d, err := e.UserData.AsBytes()
		if err != nil {
			return fmt.Errorf("error rendering AutoScalingLaunchConfiguration UserData: %v", err)
		}
		request.UserData = aws.String(base64.StdEncoding.EncodeToString(d))
	}
	if e.IAMInstanceProfile != nil {
		request.IamInstanceProfile = e.IAMInstanceProfile.Name
	}

	_, err = t.Cloud.Autoscaling.CreateLaunchConfiguration(request)
	if err != nil {
		return fmt.Errorf("error creating AutoscalingLaunchConfiguration: %v", err)
	}

	return nil //return output.AddAWSTags(cloud.Tags(), v, "vpc")
}




