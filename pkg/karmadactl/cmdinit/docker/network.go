package docker

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	"k8s.io/klog/v2"
)

func (d *CommandInitDockerOption) networkCreate() error {
	allNetworks, err := d.cli.NetworkList(d.ctx, types.NetworkListOptions{})
	if err != nil {
		return err
	}

	for _, v := range allNetworks {
		if v.Name == d.DockerNetwork {
			klog.Warningf("network %q already exists. Subnet %q .", d.DockerNetwork, v.IPAM.Config[0].Subnet)
			return nil
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

	if !ping(d.aggregatedAPIServerContainerIP) && !ping(d.webhookContainerIP) {
		return fmt.Errorf("ip %q and %q seem to be used. You can specify a new docker network, see help command --docker-network", d.aggregatedAPIServerContainerIP, d.webhookContainerIP)
	}
	return nil
}

// ping Check if the assigned container IP is in use.
func ping(ip string) bool {
	var available = "false"
	command := fmt.Sprintf("ping -c 1 %s  > /dev/null && echo true || echo false", ip)
	out, err := exec.Command("/bin/sh", "-c", command).Output()
	if err != nil {
		klog.Exit(err)
	}
	klog.Infof("Is IP %q already in use : %s ", ip, strings.TrimSpace(string(out)))

	return strings.TrimSpace(string(out)) == available
}
