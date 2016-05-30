package awstasks

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
	"k8s.io/kube-deploy/upup/pkg/fi"
	"k8s.io/kube-deploy/upup/pkg/fi/cloudup/awsup"
)

//go:generate fitask -type=SecurityGroup
type SecurityGroup struct {
	Name        *string

	ID          *string
	Description *string
	VPC         *VPC
}

func (e *SecurityGroup) Find(c *fi.Context) (*SecurityGroup, error) {
	cloud := c.Cloud.(*awsup.AWSCloud)

	var vpcID *string
	if e.VPC != nil {
		vpcID = e.VPC.ID
	}

	if vpcID == nil || e.Name == nil {
		return nil, nil
	}

	filters := cloud.BuildFilters(nil) // TODO: Do we need any filters here - done by group-name
	filters = append(filters, awsup.NewEC2Filter("vpc-id", *vpcID))
	filters = append(filters, awsup.NewEC2Filter("group-name", *e.Name))

	request := &ec2.DescribeSecurityGroupsInput{
		Filters: filters,
	}

	response, err := cloud.EC2.DescribeSecurityGroups(request)
	if err != nil {
		return nil, fmt.Errorf("error listing SecurityGroups: %v", err)
	}
	if response == nil || len(response.SecurityGroups) == 0 {
		return nil, nil
	}

	if len(response.SecurityGroups) != 1 {
		return nil, fmt.Errorf("found multiple SecurityGroups matching tags")
	}
	sg := response.SecurityGroups[0]
	actual := &SecurityGroup{
		ID:          sg.GroupId,
		Name:        sg.GroupName,
		Description: sg.Description,
		VPC:         &VPC{ID: sg.VpcId},
	}

	glog.V(2).Infof("found matching SecurityGroup %q", *actual.ID)
	e.ID = actual.ID

	return actual, nil
}

func (e *SecurityGroup) Run(c *fi.Context) error {
	return fi.DefaultDeltaRunMethod(e, c)
}

func (_ *SecurityGroup) CheckChanges(a, e, changes *SecurityGroup) error {
	if a != nil {
		if changes.ID != nil {
			return fi.CannotChangeField("ID")
		}
		if changes.Name != nil {
			return fi.CannotChangeField("Name")
		}
		if changes.VPC != nil {
			return fi.CannotChangeField("VPC")
		}
	}
	return nil
}

func (_ *SecurityGroup) RenderAWS(t *awsup.AWSAPITarget, a, e, changes *SecurityGroup) error {
	if a == nil {
		glog.V(2).Infof("Creating SecurityGroup with Name:%q VPC:%q", *e.Name, *e.VPC.ID)

		request := &ec2.CreateSecurityGroupInput{
			VpcId:       e.VPC.ID,
			GroupName:   e.Name,
			Description: e.Description,
		}

		response, err := t.Cloud.EC2.CreateSecurityGroup(request)
		if err != nil {
			return fmt.Errorf("error creating SecurityGroup: %v", err)
		}

		e.ID = response.GroupId
	}

	return t.AddAWSTags(*e.ID, t.Cloud.BuildTags(e.Name, nil))
}
