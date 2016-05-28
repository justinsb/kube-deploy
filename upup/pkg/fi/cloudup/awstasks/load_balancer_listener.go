package awstasks

//import (
//	"fmt"
//
//	"github.com/aws/aws-sdk-go/aws"
//	"github.com/aws/aws-sdk-go/service/elb"
//	"github.com/golang/glog"
//	"k8s.io/kube-deploy/upup/pkg/fi"
//	"k8s.io/kube-deploy/upup/pkg/fi/cloudup/awsup"
//)

//type LoadBalancerListener struct {
//	LoadBalancer     *LoadBalancer
//	LoadBalancerPort int64
//}
//
//func (e *LoadBalancerListener) String() string {
//	return fi.TaskAsString(e)
//}

//func (e *LoadBalancerListener) Find(c *fi.Context) (*LoadBalancerListener, error) {
//	cloud := c.Cloud.(*awsup.AWSCloud)
//
//	lb, err := findELB(cloud, e.LoadBalancer.Name)
//	if err != nil {
//		return nil, err
//	}
//	if lb == nil {
//		return nil, nil
//	}
//
//	var listener *elb.Listener
//	for _, ld := range lb.ListenerDescriptions {
//		l := ld.Listener
//		if aws.Int64Value(l.LoadBalancerPort) == e.LoadBalancerPort {
//			if listener != nil {
//				return nil, fmt.Errorf("found multiple listeners with port %d", e.LoadBalancerPort)
//			}
//			listener = l
//		}
//	}
//
//	if listener == nil {
//		return nil, nil
//	}
//
//	actual := &LoadBalancerListener{}
//	actual.LoadBalancer = e.LoadBalancer
//	actual.LoadBalancerPort = aws.Int64Value(listener.LoadBalancerPort)
//
//	return actual, nil
//}

//func (e *LoadBalancerListener) Run(c *fi.Context) error {
//	return fi.DefaultDeltaRunMethod(e, c)
//}
//
//func (s *LoadBalancerListener) CheckChanges(a, e, changes *LoadBalancerListener) error {
//	if a == nil {
//		if e.LoadBalancer == nil {
//			return fi.RequiredField("LoadBalancer")
//		}
//		if e.LoadBalancerPort == 0 {
//			return fi.RequiredField("LoadBalancerPort")
//		}
//	}
//	return nil
//}

//func (_ *LoadBalancerListener) RenderAWS(t *awsup.AWSAPITarget, a, e, changes *LoadBalancerListener) error {
//	if a == nil {
//		listener := &elb.Listener{
//			LoadBalancerPort: aws.Int64(e.LoadBalancerPort),
//
//			Protocol: aws.String("TCP"),
//
//			InstanceProtocol: aws.String("TCP"),
//			InstancePort:     aws.Int64(e.LoadBalancerPort),
//		}
//
//		request := &elb.CreateLoadBalancerListenersInput{}
//		request.LoadBalancerName = &e.LoadBalancer.Name
//		request.Listeners = []*elb.Listener{listener}
//
//		glog.V(2).Infof("Creating LoadBalancer listener on %d", e.LoadBalancerPort)
//
//		_, err := t.Cloud.ELB.CreateLoadBalancerListeners(request)
//		if err != nil {
//			return fmt.Errorf("error creating LoadBalancerListeners: %v", err)
//		}
//	} else {
//		// TODO: Apply changes?
//	}
//
//	return nil
//}
