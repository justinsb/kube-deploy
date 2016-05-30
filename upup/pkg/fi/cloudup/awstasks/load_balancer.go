package awstasks

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/golang/glog"
	"k8s.io/kube-deploy/upup/pkg/fi"
	"k8s.io/kube-deploy/upup/pkg/fi/cloudup/awsup"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"strconv"
)

//go:generate fitask -type=LoadBalancer
type LoadBalancer struct {
	Name           *string

	ID             *string

	DNSName        *string
	HostedZoneId   *string

	Subnets        []*Subnet
	SecurityGroups []*SecurityGroup

	Listeners      map[string]*LoadBalancerListener
}

type LoadBalancerListener struct {
	InstancePort int
}

func (e *LoadBalancerListener) mapToAWS(loadBalancerPort int64) *elb.Listener {
	return &elb.Listener{
		LoadBalancerPort: aws.Int64(loadBalancerPort),

		Protocol: aws.String("TCP"),

		InstanceProtocol: aws.String("TCP"),
		InstancePort:     aws.Int64(int64(e.InstancePort)),
	}
}

var _ fi.HasDependencies = &LoadBalancerListener{}

func (e *LoadBalancerListener) GetDependencies(tasks map[string]fi.Task) []fi.Task {
	return nil
}

func findELB(cloud *awsup.AWSCloud, name string) (*elb.LoadBalancerDescription, error) {
	request := &elb.DescribeLoadBalancersInput{
		LoadBalancerNames: []*string{&name},
	}

	var found []*elb.LoadBalancerDescription
	err := cloud.ELB.DescribeLoadBalancersPages(request, func(p *elb.DescribeLoadBalancersOutput, lastPage bool) (shouldContinue bool) {
		for _, lb := range p.LoadBalancerDescriptions {
			if aws.StringValue(lb.LoadBalancerName) == name {
				found = append(found, lb)
			} else {
				glog.Warningf("Got ELB with unexpected name")
			}
		}

		return true
	})

	if err != nil {
		if awsError, ok := err.(awserr.Error); ok {
			if awsError.Code() == "LoadBalancerNotFound" {
				return nil, nil
			}
		}

		return nil, fmt.Errorf("error listing ELBs: %v", err)
	}

	if len(found) == 0 {
		return nil, nil
	}

	if len(found) != 1 {
		return nil, fmt.Errorf("Found multiple ELBs with name %q", name)
	}

	return found[0], nil
}

func (e *LoadBalancer) Find(c *fi.Context) (*LoadBalancer, error) {
	cloud := c.Cloud.(*awsup.AWSCloud)

	lb, err := findELB(cloud, fi.StringValue(e.Name))
	if err != nil {
		return nil, err
	}
	if lb == nil {
		return nil, nil
	}

	actual := &LoadBalancer{}
	actual.Name = e.Name
	actual.ID = e.DNSName
	actual.DNSName = lb.DNSName
	actual.HostedZoneId = lb.CanonicalHostedZoneNameID
	for _, subnet := range lb.Subnets {
		actual.Subnets = append(actual.Subnets, &Subnet{ID: subnet})
	}

	for _, sg := range lb.SecurityGroups {
		actual.SecurityGroups = append(actual.SecurityGroups, &SecurityGroup{ID: sg})
	}

	actual.Listeners = make(map[string]*LoadBalancerListener)

	for _, ld := range lb.ListenerDescriptions {
		l := ld.Listener
		loadBalancerPort := strconv.FormatInt(aws.Int64Value(l.LoadBalancerPort), 10)

		actualListener := &LoadBalancerListener{}
		actualListener.InstancePort = int(aws.Int64Value(l.InstancePort))
		actual.Listeners[loadBalancerPort] = actualListener
	}

	// Avoid spurious mismatches
	if subnetSlicesEqualIgnoreOrder(actual.Subnets, e.Subnets) {
		actual.Subnets = e.Subnets
	}

	if e.DNSName == nil {
		e.DNSName = actual.DNSName
	}
	if e.HostedZoneId == nil {
		e.HostedZoneId = actual.HostedZoneId
	}

	return actual, nil
}

func (e *LoadBalancer) Run(c *fi.Context) error {
	return fi.DefaultDeltaRunMethod(e, c)
}

func (s *LoadBalancer) CheckChanges(a, e, changes *LoadBalancer) error {
	if a == nil {
		if fi.StringValue(e.Name) == "" {
			return fi.RequiredField("Name")
		}
		if len(e.SecurityGroups) == 0 {
			return fi.RequiredField("SecurityGroups")
		}
		if len(e.Subnets) == 0 {
			return fi.RequiredField("Subnets")
		}
	}
	return nil
}

func (_ *LoadBalancer) RenderAWS(t *awsup.AWSAPITarget, a, e, changes *LoadBalancer) error {
	if a == nil {
		request := &elb.CreateLoadBalancerInput{}
		request.LoadBalancerName = e.Name

		for _, subnet := range e.Subnets {
			request.Subnets = append(request.Subnets, subnet.ID)
		}

		for _, sg := range e.SecurityGroups {
			request.SecurityGroups = append(request.SecurityGroups, sg.ID)
		}

		request.Listeners = []*elb.Listener{}

		for loadBalancerPort, listener := range e.Listeners {
			loadBalancerPortInt, err := strconv.ParseInt(loadBalancerPort, 10, 64)
			if err != nil {
				return fmt.Errorf("error parsing load balancer listener port: %q", loadBalancerPort)
			}
			awsListener := listener.mapToAWS(loadBalancerPortInt)
			request.Listeners = append(request.Listeners, awsListener)
		}

		glog.V(2).Infof("Creating ELB with Name:%q", *e.Name)

		response, err := t.Cloud.ELB.CreateLoadBalancer(request)
		if err != nil {
			return fmt.Errorf("error creating ELB: %v", err)
		}

		e.DNSName = response.DNSName
		e.ID = response.DNSName

		lb, err := findELB(t.Cloud, *e.Name)
		if err != nil {
			return err
		}
		if lb == nil {
			// TODO: Retry?  Is this async
			return fmt.Errorf("Unable to find newly created ELB")
		}

		e.HostedZoneId = lb.CanonicalHostedZoneNameID
	} else {
		if changes.Subnets != nil {
			return fmt.Errorf("subnet changes on LoadBalancer not yet implemented")
		}

		if changes.Listeners != nil {
			request := &elb.CreateLoadBalancerListenersInput{}
			request.LoadBalancerName = e.Name

			for loadBalancerPort, listener := range changes.Listeners {
				loadBalancerPortInt, err := strconv.ParseInt(loadBalancerPort, 10, 64)
				if err != nil {
					return fmt.Errorf("error parsing load balancer listener port: %q", loadBalancerPort)
				}
				awsListener := listener.mapToAWS(loadBalancerPortInt)
				request.Listeners = append(request.Listeners, awsListener)
			}

			glog.V(2).Infof("Creating LoadBalancer listeners")

			_, err := t.Cloud.ELB.CreateLoadBalancerListeners(request)
			if err != nil {
				return fmt.Errorf("error creating LoadBalancerListeners: %v", err)
			}
		}
	}

	return t.AddAWSTags(*e.ID, t.Cloud.BuildTags(e.Name, nil))
}



