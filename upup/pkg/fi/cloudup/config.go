package cloudup

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"github.com/golang/glog"
)

type CloudConfig struct {
	CloudProvider                 string

	// The version of kubernetes to install
	KubernetesVersion             string

	// The Node initializer technique to use: cloudinit or nodeup
	NodeInit                      string

	// The CIDR used for the AWS VPC / GCE Network, or otherwise allocated to k8s
	// This is a real CIDR, not the internal k8s overlay
	NetworkCIDR                   string

	// The DNS zone we should use when configuring DNS
	DNSZone                       string

	InstancePrefix                string
	ClusterName                   string
	AllocateNodeCIDRs             bool
	Zones                         []string
	MasterZones                   []string
	NodeZones                     []string
	Region                        string
	Project                       string

	Multizone                     bool

	ClusterIPRange                string
	ServiceClusterIPRange         string
	MasterIPRange                 string
	NonMasqueradeCidr             string

	NetworkProvider               string

	HairpinMode                   string

	OpencontrailTag               string
	OpencontrailKubernetesTag     string
	OpencontrailPublicSubnet      string

	EnableClusterMonitoring       string
	EnableL7LoadBalancing         string
	EnableClusterUI               bool

	EnableClusterDNS              bool
	DNSReplicas                   int
	DNSServerIP                   string
	DNSDomain                     string

	EnableClusterLogging          bool
	EnableNodeLogging             bool
	LoggingDestination            string
	ElasticsearchLoggingReplicas  int

	EnableClusterRegistry         bool
	ClusterRegistryDisk           string
	ClusterRegistryDiskSize       int

	EnableCustomMetrics           bool

	MasterInternalName            string

	RegisterMasterKubelet         bool
	MasterVolumeType              string
	MasterVolumeSize              int
	MasterTag                     string
	MasterMachineType             string
	MasterImage                   string
	MasterPublicIP                string
	MasterPublicName              string

	NodeImage                     string
	NodeCount                     int
	NodeInstancePrefix            string
	NodeLabels                    string
	NodeMachineType               string
	NodeTag                       string

	KubeUser                      string

	// These are moved to CAStore / SecretStore
	//KubePassword			string
	//KubeletToken                  string
	//KubeProxyToken                string
	//BearerToken                   string
	//CACert                        []byte
	//CAKey                         []byte
	//KubeletCert                   []byte
	//KubeletKey                    []byte
	//MasterCert                    []byte
	//MasterKey                     []byte
	//KubecfgCert                   []byte
	//KubecfgKey                    []byte

	AdmissionControl              string
	RuntimeConfig                 string

	KubeImageTag                  string
	KubeDockerRegistry            string
	KubeAddonRegistry             string

	KubeletPort                   int

	KubeApiserverRequestTimeout   int

	TerminatedPodGcThreshold      string

	EnableManifestURL             bool
	ManifestURL                   string
	ManifestURLHeader             string

	TestCluster                   string

	DockerOptions                 string
	DockerStorage                 string
	ExtraDockerOpts               string

	E2EStorageTestEnvironment     string
	KubeletTestArgs               string
	KubeletTestLogLevel           string
	DockerTestArgs                string
	DockerTestLogLevel            string
	ApiserverTestArgs             string
	ApiserverTestLogLevel         string
	ControllerManagerTestArgs     string
	ControllerManagerTestLogLevel string
	SchedulerTestArgs             string
	SchedulerTestLogLevel         string
	KubeProxyTestArgs             string
	KubeProxyTestLogLevel         string

	Assets                        []string

	NodeUpTags                    []string

	NodeUp                        NodeUpConfig
}

type NodeUpConfig struct {
	Location string
	Hash     string
}

func (c *CloudConfig) WellKnownServiceIP(id int) (net.IP, error) {
	_, cidr, err := net.ParseCIDR(c.ServiceClusterIPRange)
	if err != nil {
		return nil, fmt.Errorf("error parsing ServiceClusterIPRange: %v", err)
	}

	ip4 := cidr.IP.To4()
	if ip4 != nil {
		n := binary.BigEndian.Uint32(ip4)
		n += uint32(id)
		serviceIP := make(net.IP, len(ip4))
		binary.BigEndian.PutUint32(serviceIP, n)
		return serviceIP, nil
	}

	ip6 := cidr.IP.To16()
	if ip6 != nil {
		baseIPInt := big.NewInt(0)
		baseIPInt.SetBytes(ip6)
		serviceIPInt := big.NewInt(0)
		serviceIPInt.Add(big.NewInt(int64(id)), baseIPInt)
		serviceIP := make(net.IP, len(ip6))
		serviceIPBytes := serviceIPInt.Bytes()
		for i := range serviceIPBytes {
			serviceIP[len(serviceIP) - len(serviceIPBytes) + i] = serviceIPBytes[i]
		}
		return serviceIP, nil
	}

	return nil, fmt.Errorf("Unexpected IP address type for ServiceClusterIPRange: %s", c.ServiceClusterIPRange)
}

func (c *CloudConfig) SubnetCIDR(zone string) (string, error) {
	index := -1
	for i, z := range c.Zones {
		if z == zone {
			index = i
			break
		}
	}
	if index == -1 {
		return "", fmt.Errorf("zone not configured: %q", zone)
	}

	_, cidr, err := net.ParseCIDR(c.NetworkCIDR)
	if err != nil {
		return "", fmt.Errorf("Invalid NetworkCIDR: %q", c.NetworkCIDR)
	}

	networkLength, _ := cidr.Mask.Size()

	// We assume a maximum of 8 subnets per network
	// TODO: Does this make sense on GCE?
	networkLength += 3

	ip4 := cidr.IP.To4()
	if ip4 != nil {
		n := binary.BigEndian.Uint32(ip4)
		n += uint32(index) << uint(32 - networkLength)
		subnetIP := make(net.IP, len(ip4))
		binary.BigEndian.PutUint32(subnetIP, n)
		subnetCIDR := subnetIP.String() + "/" + strconv.Itoa(networkLength)
		glog.V(2).Infof("Computed CIDR for subnet in zone %q as %q", zone, subnetCIDR)
		return subnetCIDR, nil
	}

	return "", fmt.Errorf("Unexpected IP address type for NetworkCIDR: %s", c.NetworkCIDR)
}

