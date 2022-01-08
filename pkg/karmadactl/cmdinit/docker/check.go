package docker

import (
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

// WaitContainerReady wait container ready
func (d *CommandInitDockerOption) WaitContainerReady(containerID string, sleepTime, timeout time.Duration) error {
	// wait
	klog.Warningf("wait container start...")
	time.Sleep(sleepTime)
	container, err := d.cli.ContainerInspect(d.ctx, containerID)
	if err != nil {
		return err
	}
	containerIP := container.NetworkSettings.Networks["karmada"].IPAddress
	if err := wait.Poll(time.Second, timeout, func() (bool, error) {
		if !container.State.Running {
			klog.Warningf("Container: %s not ready. Status: %s. ContainerIP: %s", strings.Trim(container.Name, "/"), container.State.Status, containerIP)
			return false, nil
		}
		return true, nil
	}); err != nil {
		return err
	}
	klog.Infof("Container: %s is ready. Status: %s. ContainerIP: %s", strings.Trim(container.Name, "/"), container.State.Status, containerIP)
	return nil
}
