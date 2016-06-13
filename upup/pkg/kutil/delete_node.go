package kutil

import (
	"fmt"
	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"strings"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
)

type DeleteNode struct {
	Client *unversioned.Client
}

func (d*DeleteNode) DeleteNode(nodeName string) error {
	node, err := d.Client.Nodes().Get(nodeName)
	if err != nil {
		return fmt.Errorf("unable to find node %q: %v", nodeName, err)
	}

	providerID := node.Spec.ProviderID
	if providerID == "" {
		return fmt.Errorf("ProviderID not set on node: %q", nodeName)
	}

	if strings.HasPrefix(providerID, "aws://") {
		tokens := strings.Split(strings.TrimPrefix(providerID, "aws://"), "/")
		region := ""
		awsID := ""
		if len(tokens) == 2 {
			zone := tokens[0]
			awsID = tokens[1]

			if len(zone) > 1 {
				region = zone[:len(zone) - 1]
			}
		}

		if region == "" || !strings.HasPrefix(awsID, "i-") {
			return fmt.Errorf("ProviderID does not appear to be a valid AWS provider id: %q", providerID)
		}

		// TODO: Prelaunch replacement?

		// TODO: Mark unschedulable / evict pods?
		// kubectl patch nodes $NODENAME -p '{"spec": {"unschedulable": true}}'

		glog.Infof("Shutting down AWS instance %q", awsID)



		// TODO: Create helper function or abstraction (do we want to use fi.Cloud?)

		config := aws.NewConfig().WithRegion(region)
		ec2Client := ec2.New(session.New(), config)

		request := &ec2.TerminateInstancesInput{
			InstanceIds: []*string{&awsID},
		}
		_, err := ec2Client.TerminateInstances(request)
		if err != nil {
			return fmt.Errorf("error deleting instance %q: %v", awsID, err)
		}
	} else {
		return fmt.Errorf("Unknown ProviderID: %q", providerID)
	}

	return nil
}