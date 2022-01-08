package docker

import (
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
)

func (d *CommandInitDockerOption) networkCreate() error {
	allNetworks, err := d.cli.NetworkList(d.ctx, types.NetworkListOptions{})
	if err != nil {
		return err
	}

	for _, v := range allNetworks {
		if v.Name == "karmada" {
			return fmt.Errorf("network karmada already exists. run 'docker network rm karmada'")
		}
	}

	_, err = d.cli.NetworkCreate(d.ctx, "karmada", types.NetworkCreate{
		IPAM: &network.IPAM{
			Config: []network.IPAMConfig{
				{
					Subnet:  "166.233.0.0/16",
					Gateway: "166.233.0.1",
				},
			},
		},
	})
	if err != nil {
		return err
	}

	return nil
}
