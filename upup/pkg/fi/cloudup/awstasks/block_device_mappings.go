package awstasks

import (
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"k8s.io/kube-deploy/upup/pkg/fi"
)

type BlockDeviceMapping struct {
	DeviceName  *string
	VirtualName *string
}

func BlockDeviceMappingFromEC2(i *ec2.BlockDeviceMapping) *BlockDeviceMapping {
	o := &BlockDeviceMapping{}
	o.DeviceName = i.DeviceName
	o.VirtualName = i.VirtualName
	return o
}

func (i *BlockDeviceMapping) ToEC2() *ec2.BlockDeviceMapping {
	o := &ec2.BlockDeviceMapping{}
	o.DeviceName = i.DeviceName
	o.VirtualName = i.VirtualName
	return o
}

func BlockDeviceMappingFromAutoscaling(i *autoscaling.BlockDeviceMapping) *BlockDeviceMapping {
	o := &BlockDeviceMapping{}
	o.DeviceName = i.DeviceName
	o.VirtualName = i.VirtualName
	return o
}

func (i *BlockDeviceMapping) ToAutoscaling() *autoscaling.BlockDeviceMapping {
	o := &autoscaling.BlockDeviceMapping{}
	o.DeviceName = i.DeviceName
	o.VirtualName = i.VirtualName
	return o
}

var _ fi.HasDependencies = &BlockDeviceMapping{}

func (f *BlockDeviceMapping) GetDependencies(tasks map[string]fi.Task) []fi.Task {
	return nil
}
