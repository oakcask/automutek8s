package tools

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/api/compute/v1"
)

// Config is schema of config.yaml
type Config struct {
	Region  string        `json:"region"`
	Zone    string        `json:"zone"`
	Cluster ClusterConfig `json:"cluster"`
	Network NetworkConfig `json:"network"`
	Gate    GateConfig    `json:"gate"`
}

func (Config) ProjectID() (string, error) {
	return GetProjectID(context.Background())
}

func (config Config) RegionID() (string, error) {
	region := config.Region
	if strings.HasPrefix(region, "asia-northeast") {
		return "an", nil
	}

	return "", fmt.Errorf("unknown region for GAE Region ID: %s", region)
}

func (config Config) IngressIP() (string, error) {
	projectID, e := config.ProjectID()
	if e != nil {
		return "", e
	}

	service, e := compute.NewService(context.Background())
	if e != nil {
		return "", e
	}

	addr, e := service.Addresses.Get(projectID, config.Region, config.Network.IngressIPResourceID).Do()
	if e != nil {
		return "", e
	}

	return addr.Address, nil
}

// ClusterConfig is schema of cluster block in config.yaml
type ClusterConfig struct {
	Name             string `json:"name"`
	Location         string `json:"location"`
	VPCConnectorName string `json:"vpc_connector_name"`
}

// NetworkConfig is schema of network block in config.yaml
type NetworkConfig struct {
	PrimaryCidrRange      string `json:"primary_cidr_range"`
	PodCidrRange          string `json:"pod_cidr_range"`
	ServiceCidrRange      string `json:"service_cidr_range"`
	MasterCidrRange       string `json:"master_cidr_range"`
	VPCConnectorCidrRange string `json:"vpc_connector_cidr_range"`
	IngressIPResourceID   string `json:"ingress_ip_resource_id"`
}

// GateConfig is schema of gate block in config.yaml
type GateConfig struct {
	ServiceID string `json:"service_id"`
}
