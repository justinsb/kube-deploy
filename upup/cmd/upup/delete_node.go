package main

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"k8s.io/kube-deploy/upup/pkg/kutil"
	kubectl_util "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/client/unversioned"
)

type DeleteNodeCmd struct {
	cobraCommand *cobra.Command
}

var deleteNode DeleteNodeCmd

func init() {
	deleteNode.cobraCommand = &cobra.Command{
		Use:   "node",
		Short: "Delete node",
		Long:  `Shutdown a node in a k8s cluster.`,
		Run: func(cmd *cobra.Command, args []string) {
			err := deleteNode.Run(args)
			if err != nil {
				glog.Exitf("%v", err)
			}
		},
	}

	deleteCmd.AddCommand(deleteNode.cobraCommand)
}

func (c *DeleteNodeCmd) Run(nodes []string) error {
	if len(nodes) == 0 {
		return fmt.Errorf("Must specify node name(s) to delete")
	}

	client, err := c.buildKubeClient()
	if err != nil {
		return err
	}

	d := &kutil.DeleteNode{
		Client: client,
	}

	for _, node := range nodes {
		err := d.DeleteNode(node)
		if err != nil {
			return fmt.Errorf("error deleting node %q: %v", node, err)
		}
	}

	return nil
}

func (c *DeleteNodeCmd) buildKubeClient() (*unversioned.Client, error) {
	clientConfig := kubectl_util.DefaultClientConfig(c.cobraCommand.Flags())

	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("error building kubernetes client config: %v", err)
	}
	kubeClient, err := unversioned.New(config)
	if err != nil {
		return nil, fmt.Errorf("error building kubernetes client: %v", err)
	}
	return kubeClient, nil
}