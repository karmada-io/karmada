package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/options"
)

type pullImageEvent struct {
	Status         string `json:"status"`
	Error          string `json:"error"`
	Progress       string `json:"progress"`
	ProgressDetail struct {
		Current int `json:"current"`
		Total   int `json:"total"`
	} `json:"progressDetail"`
}

func pullImage(ctx context.Context, cli *client.Client, imageName string) error {
	reader, err := cli.ImagePull(ctx, imageName, types.ImagePullOptions{})
	if err != nil {
		panic(err)
	}
	decoder := json.NewDecoder(reader)

	var event *pullImageEvent
	for {
		if err := decoder.Decode(&event); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		fmt.Printf("\rPulling: %+v", event.Progress)
	}
	fmt.Printf("\n%s\n", event.Status)
	return nil
}

// startPullImage start pull all image
func (d *CommandInitDockerOption) startPullImage() error {
	images := []string{
		options.EtcdImage,
		options.KarmadaAPIServerImage,
		options.KarmadaSchedulerImage,
		options.KubeControllerManagerImage,
		options.KarmadaControllerManagerImage,
		options.KarmadaWebhookImage,
		options.KarmadaAggregatedAPIServerImage,
	}

	for _, v := range images {
		klog.Infoln("Download image: ", v)
		if err := pullImage(d.ctx, d.cli, v); err != nil {
			return err
		}
	}
	return nil
}
