package awstasks

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
	"k8s.io/kube-deploy/upup/pkg/fi"
	"k8s.io/kube-deploy/upup/pkg/fi/cloudup/awsup"
)

//go:generate fitask -type=ElasticIP
type ElasticIP struct {
	Name *string

	ID       *string
	PublicIP *string

	// Because ElasticIPs don't supporting tagging (sadly), we instead tag on
	// a different resource
	TagUsingKey   *string
	TagOnResource fi.Task
}

var _ fi.HasAddress = &ElasticIP{}

func (e *ElasticIP) FindAddress(context *fi.Context) (*string, error) {
	actual, err := e.find(context.Cloud.(*awsup.AWSCloud))
	if err != nil {
		return nil, fmt.Errorf("error querying for ElasticIP: %v", err)
	}
	if actual == nil {
		return nil, nil
	}
	return actual.PublicIP, nil
}

func (e *ElasticIP) Find(context *fi.Context) (*ElasticIP, error) {
	return e.find(context.Cloud.(*awsup.AWSCloud))
}

func (e *ElasticIP) findTagOnResourceID(cloud *awsup.AWSCloud) (*string, error) {
	if e.TagOnResource == nil {
		return nil, nil
	}

	var tagOnResource TaggableResource
	var ok bool
	if tagOnResource, ok = e.TagOnResource.(TaggableResource); !ok {
		return nil, fmt.Errorf("TagOnResource must implement TaggableResource (type is %T)", e.TagOnResource)
	}

	id, err := tagOnResource.FindResourceID(cloud)
	if err != nil {
		return nil, fmt.Errorf("error trying to find id of TagOnResource: %v", err)
	}
	return id, err
}

func (e *ElasticIP) find(cloud *awsup.AWSCloud) (*ElasticIP, error) {
	publicIP := e.PublicIP
	allocationID := e.ID

	tagOnResourceID, err := e.findTagOnResourceID(cloud)
	if err != nil {
		return nil, err
	}
	// Find via tag on foreign resource
	if allocationID == nil && publicIP == nil && e.TagUsingKey != nil && tagOnResourceID != nil {
		var filters []*ec2.Filter
		filters = append(filters, awsup.NewEC2Filter("key", *e.TagUsingKey))
		filters = append(filters, awsup.NewEC2Filter("resource-id", *tagOnResourceID))

		request := &ec2.DescribeTagsInput{
			Filters: filters,
		}

		response, err := cloud.EC2.DescribeTags(request)
		if err != nil {
			return nil, fmt.Errorf("error listing tags: %v", err)
		}

		if response == nil || len(response.Tags) == 0 {
			return nil, nil
		}

		if len(response.Tags) != 1 {
			return nil, fmt.Errorf("found multiple tags for: %v", e)
		}
		t := response.Tags[0]
		publicIP = t.Value
		glog.V(2).Infof("Found public IP via tag: %v", *publicIP)
	}

	if publicIP != nil || allocationID != nil {
		request := &ec2.DescribeAddressesInput{}
		if allocationID != nil {
			request.AllocationIds = []*string{allocationID}
		} else if publicIP != nil {
			request.Filters = []*ec2.Filter{awsup.NewEC2Filter("public-ip", *publicIP)}
		}

		response, err := cloud.EC2.DescribeAddresses(request)
		if err != nil {
			return nil, fmt.Errorf("error listing ElasticIPs: %v", err)
		}

		if response == nil || len(response.Addresses) == 0 {
			return nil, nil
		}

		if len(response.Addresses) != 1 {
			return nil, fmt.Errorf("found multiple ElasticIPs for: %v", e)
		}
		a := response.Addresses[0]
		actual := &ElasticIP{
			ID:       a.AllocationId,
			PublicIP: a.PublicIp,
		}

		// These two are weird properties; we copy them so they don't come up as changes
		actual.TagUsingKey = e.TagUsingKey
		actual.TagOnResource = e.TagOnResource

		e.ID = actual.ID

		return actual, nil
	}

	return nil, nil
}

func (e *ElasticIP) Run(c *fi.Context) error {
	return fi.DefaultDeltaRunMethod(e, c)
}

func (s *ElasticIP) CheckChanges(a, e, changes *ElasticIP) error {
	return nil
}

func (_ *ElasticIP) RenderAWS(t *awsup.AWSAPITarget, a, e, changes *ElasticIP) error {
	var publicIP *string

	tagOnResourceID, err := e.findTagOnResourceID(t.Cloud)
	if err != nil {
		return err
	}

	if a == nil {
		if tagOnResourceID == nil || e.TagUsingKey == nil {
			return fmt.Errorf("cannot create ElasticIP without TagOnResource being set (would leak)")
		}
		glog.V(2).Infof("Creating ElasticIP for VPC")

		request := &ec2.AllocateAddressInput{}
		request.Domain = aws.String(ec2.DomainTypeVpc)

		response, err := t.Cloud.EC2.AllocateAddress(request)
		if err != nil {
			return fmt.Errorf("error creating ElasticIP: %v", err)
		}

		e.ID = response.AllocationId
		e.PublicIP = response.PublicIp
		publicIP = response.PublicIp
	} else {
		publicIP = a.PublicIP
	}

	if publicIP != nil && e.TagUsingKey != nil && tagOnResourceID != nil {
		tags := map[string]string{
			*e.TagUsingKey: *publicIP,
		}
		err := t.AddAWSTags(*tagOnResourceID, tags)
		if err != nil {
			return fmt.Errorf("error adding tags to resource for ElasticIP: %v", err)
		}
	}
	return nil
}
