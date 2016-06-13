package awstasks

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
	"k8s.io/kube-deploy/upup/pkg/fi"
	"k8s.io/kube-deploy/upup/pkg/fi/cloudup/awsup"
	"k8s.io/kube-deploy/upup/pkg/fi/cloudup/terraform"
)

//go:generate fitask -type=VPC
type VPC struct {
	Name               *string
	ID                 *string
	CIDR               *string
	EnableDNSHostnames *bool
	EnableDNSSupport   *bool

	// Shared is set if this is a shared VPC
	Shared *bool
}

var _ fi.CompareWithID = &VPC{}

func (e *VPC) CompareWithID() *string {
	return e.ID
}

func (e *VPC) Find(c *fi.Context) (*VPC, error) {
	cloud := c.Cloud.(*awsup.AWSCloud)

	request := &ec2.DescribeVpcsInput{}

	if fi.StringValue(e.ID) != "" {
		request.VpcIds = []*string{e.ID}
	} else {
		request.Filters = cloud.BuildFilters(e.Name)
	}

	response, err := cloud.EC2.DescribeVpcs(request)
	if err != nil {
		return nil, fmt.Errorf("error listing VPCs: %v", err)
	}
	if response == nil || len(response.Vpcs) == 0 {
		return nil, nil
	}

	if len(response.Vpcs) != 1 {
		return nil, fmt.Errorf("found multiple VPCs matching tags")
	}
	vpc := response.Vpcs[0]
	actual := &VPC{
		ID:   vpc.VpcId,
		CIDR: vpc.CidrBlock,
		Name: findNameTag(vpc.Tags),
	}

	glog.V(4).Infof("found matching VPC %v", actual)
	e.ID = actual.ID

	if actual.ID != nil {
		request := &ec2.DescribeVpcAttributeInput{VpcId: actual.ID, Attribute: aws.String(ec2.VpcAttributeNameEnableDnsSupport)}
		response, err := cloud.EC2.DescribeVpcAttribute(request)
		if err != nil {
			return nil, fmt.Errorf("error querying for dns support: %v", err)
		}
		actual.EnableDNSSupport = response.EnableDnsSupport.Value
	}

	if actual.ID != nil {
		request := &ec2.DescribeVpcAttributeInput{VpcId: actual.ID, Attribute: aws.String(ec2.VpcAttributeNameEnableDnsHostnames)}
		response, err := cloud.EC2.DescribeVpcAttribute(request)
		if err != nil {
			return nil, fmt.Errorf("error querying for dns support: %v", err)
		}
		actual.EnableDNSHostnames = response.EnableDnsHostnames.Value
	}

	return actual, nil
}

func (s *VPC) CheckChanges(a, e, changes *VPC) error {
	if a == nil {
		if e.CIDR == nil {
			// TODO: Auto-assign CIDR?
			return fi.RequiredField("CIDR")
		}
	}
	if a != nil {
		if changes.CIDR != nil {
			// TODO: Do we want to destroy & recreate the VPC?
			return fi.CannotChangeField("CIDR")
		}
	}
	return nil
}

func (e *VPC) Run(c *fi.Context) error {
	return fi.DefaultDeltaRunMethod(e, c)
}

func (_ *VPC) RenderAWS(t *awsup.AWSAPITarget, a, e, changes *VPC) error {
	shared := fi.BoolValue(e.Shared)
	if shared {
		// Verify the VPC was found and matches our required settings
		if a == nil {
			return fmt.Errorf("VPC with id %q not found", fi.StringValue(e.ID))
		}

		if changes != nil && changes.EnableDNSSupport != nil {
			return fmt.Errorf("VPC with id %q was set to be shared, but did not have EnableDNSSupport=true", fi.StringValue(e.ID))
		}

		if changes != nil && changes.EnableDNSHostnames != nil {
			return fmt.Errorf("VPC with id %q was set to be shared, but did not have EnableDNSHostnames=true", fi.StringValue(e.ID))
		}

		return nil
	}

	if a == nil {
		glog.V(2).Infof("Creating VPC with CIDR: %q", *e.CIDR)

		request := &ec2.CreateVpcInput{
			CidrBlock: e.CIDR,
		}

		response, err := t.Cloud.EC2.CreateVpc(request)
		if err != nil {
			return fmt.Errorf("error creating VPC: %v", err)
		}

		e.ID = response.Vpc.VpcId
	}

	if changes.EnableDNSSupport != nil {
		request := &ec2.ModifyVpcAttributeInput{
			VpcId:            e.ID,
			EnableDnsSupport: &ec2.AttributeBooleanValue{Value: changes.EnableDNSSupport},
		}

		_, err := t.Cloud.EC2.ModifyVpcAttribute(request)
		if err != nil {
			return fmt.Errorf("error modifying VPC attribute: %v", err)
		}
	}

	if changes.EnableDNSHostnames != nil {
		request := &ec2.ModifyVpcAttributeInput{
			VpcId:              e.ID,
			EnableDnsHostnames: &ec2.AttributeBooleanValue{Value: changes.EnableDNSHostnames},
		}

		_, err := t.Cloud.EC2.ModifyVpcAttribute(request)
		if err != nil {
			return fmt.Errorf("error modifying VPC attribute: %v", err)
		}
	}

	return t.AddAWSTags(*e.ID, t.Cloud.BuildTags(e.Name))
}

type terraformVPC struct {
	CIDR               *string           `json:"cidr_block,omitempty"`
	EnableDNSHostnames *bool             `json:"enable_dns_hostnames,omitempty"`
	EnableDNSSupport   *bool             `json:"enable_dns_support,omitempty"`
	Tags               map[string]string `json:"tags,omitempty"`
}

func (_ *VPC) RenderTerraform(t *terraform.TerraformTarget, a, e, changes *VPC) error {
	cloud := t.Cloud.(*awsup.AWSCloud)

	shared := fi.BoolValue(e.Shared)
	if shared {
		// Not terraform owned / managed
		return nil
	}

	tf := &terraformVPC{
		CIDR:               e.CIDR,
		Tags:               cloud.BuildTags(e.Name),
		EnableDNSHostnames: e.EnableDNSHostnames,
		EnableDNSSupport:   e.EnableDNSSupport,
	}

	return t.RenderResource("aws_vpc", *e.Name, tf)
}

func (e *VPC) TerraformLink() *terraform.Literal {
	shared := fi.BoolValue(e.Shared)
	if shared {
		if e.ID == nil {
			glog.Fatalf("ID must be set, if VPC is shared: %s", e)
		}

		glog.V(4).Infof("reusing existing VPC with id %q", *e.ID)
		return terraform.LiteralFromStringValue(*e.ID)
	}

	return terraform.LiteralProperty("aws_vpc", *e.Name, "id")
}
