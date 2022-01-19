package docker

import (
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
)

func (d *CommandInitDockerOption) networkCreate() error {
	allNetworks, err := d.cli.NetworkList(d.ctx, types.NetworkListOptions{})
	if err != nil {
		return err
	}

	for _, v := range allNetworks {
		if v.Name == d.DockerNetwork {
			return fmt.Errorf("network %q already exists. run 'docker network rm %s' or specify a new name via --docker-network", d.DockerNetwork, d.DockerNetwork)
		}

	}

	_, err = d.cli.NetworkCreate(d.ctx, d.DockerNetwork, types.NetworkCreate{
		IPAM: &network.IPAM{
			Config: []network.IPAMConfig{
				{
					Subnet:  d.DockerNetworkSubnet,
					Gateway: d.DockerNetworkGateway,
				},
			},
		},
	})
	if err != nil {
		return err
	}

	return nil
}

// assignContainerIP Get the IPs of aggregated-apiserver and webhook containers
func (d *CommandInitDockerOption) assignContainerIP() error {
	net, err := d.cli.NetworkInspect(d.ctx, d.DockerNetwork, types.NetworkInspectOptions{})
	if err != nil {
		return err
	}

	ipRangeList := strings.Split(net.IPAM.Config[0].Subnet, "/")
	d.webhookContainerIP = strings.Replace(ipRangeList[0], "0.0", "0.10", 1)
	d.aggregatedAPIServerContainerIP = strings.Replace(ipRangeList[0], "0.0", "0.11", 1)
	if d.webhookContainerIP == "" && d.aggregatedAPIServerContainerIP == "" {
		return fmt.Errorf("failed to assign container ip")
	}
	return nil
}
