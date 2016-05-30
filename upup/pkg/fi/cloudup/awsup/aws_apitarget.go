package awsup

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/golang/glog"
	"k8s.io/kube-deploy/upup/pkg/fi"
	"time"
)

type AWSAPITarget struct {
	Cloud *AWSCloud
}

var _ fi.Target = &AWSAPITarget{}

func NewAWSAPITarget(cloud *AWSCloud) *AWSAPITarget {
	return &AWSAPITarget{
		Cloud: cloud,
	}
}

func (t *AWSAPITarget) Finish(taskMap map[string]fi.Task) error {
	return nil
}

func (t *AWSAPITarget) AddAWSTags(id string, expected map[string]string) error {
	actual, err := t.Cloud.GetTags(id)
	if err != nil {
		return fmt.Errorf("unexpected error fetching tags for resource: %v", err)
	}

	missing := map[string]string{}
	for k, v := range expected {
		actualValue, found := actual[k]
		if found && actualValue == v {
			continue
		}
		missing[k] = v
	}

	if len(missing) != 0 {
		glog.V(4).Infof("adding tags to %q: %v", id, missing)

		err := t.Cloud.CreateELBTags(id, missing)
		if err != nil {
			return fmt.Errorf("error adding tags to resource %q: %v", id, err)
		}
	}

	return nil
}

func (t *AWSAPITarget) AddELBTags(loadBalancerName string, expected map[string]string) error {
	actual, err := t.Cloud.GetELBTags(loadBalancerName)
	if err != nil {
		return fmt.Errorf("unexpected error fetching tags for resource: %v", err)
	}

	missing := map[string]string{}
	for k, v := range expected {
		actualValue, found := actual[k]
		if found && actualValue == v {
			continue
		}
		missing[k] = v
	}

	if len(missing) != 0 {
		glog.V(4).Infof("adding tags to %q: %v", loadBalancerName, missing)
		err := t.Cloud.CreateELBTags(loadBalancerName, missing)
		if err != nil {
			return fmt.Errorf("error adding tags to ELB %q: %v", loadBalancerName, err)
		}
	}

	return nil
}

func (t *AWSAPITarget) WaitForInstanceRunning(instanceID string) error {
	attempt := 0
	for {
		instance, err := t.Cloud.DescribeInstance(instanceID)
		if err != nil {
			return fmt.Errorf("error while waiting for instance to be running: %v", err)
		}

		if instance == nil {
			// TODO: Wait if we _just_ created the instance?
			return fmt.Errorf("instance not found while waiting for instance to be running")
		}

		state := "?"
		if instance.State != nil {
			state = aws.StringValue(instance.State.Name)
		}
		if state == "running" {
			return nil
		}
		glog.Infof("Waiting for instance %q to be running (current state is %q)", instanceID, state)

		time.Sleep(10 * time.Second)
		attempt++
		if attempt > 30 {
			return fmt.Errorf("timeout waiting for instance %q to be running, state was %q", instanceID, state)
		}
	}
}
