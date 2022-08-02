package init

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/utils/strings/slices"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/utils"
)

// CommandAddonsDisableOption options for addons list.
type CommandAddonsDisableOption struct {
	GlobalCommandOptions

	KarmadaKubeClientSet *kubernetes.Clientset

	WaitPodReadyTimeout int
}

// Complete the conditions required to be able to run disable.
func (o *CommandAddonsDisableOption) Complete() error {
	err := o.GlobalCommandOptions.Complete()
	if err != nil {
		return err
	}

	o.KarmadaKubeClientSet, err = utils.NewClientSet(o.KarmadaRestConfig)
	if err != nil {
		return err
	}

	return nil
}

// Validate Check that there are enough conditions to run the disable.
func (o *CommandAddonsDisableOption) Validate(args []string) error {
	err := validAddonNames(args)
	if err != nil {
		return err
	}

	if slices.Contains(args, EstimatorResourceName) && o.Cluster == "" {
		return fmt.Errorf("member cluster and config is needed when disable karmada-scheduler-estimator,use `--cluster=member --member-kubeconfig /root/.kube/config --member-context member1` to disable karmada-scheduler-estimator")
	}
	return nil
}

// Run start disable Karmada addons
func (o *CommandAddonsDisableOption) Run(args []string) error {
	var disableAddons = map[string]*Addon{}

	// collect disabled addons
	for _, item := range args {
		if item == "all" {
			disableAddons = Addons
			break
		}
		if addon := Addons[item]; addon != nil {
			disableAddons[item] = addon
		}
	}

	// disable addons
	for name, addon := range disableAddons {
		klog.Infof("Start to disable addon %s", name)
		if err := addon.Disable(o); err != nil {
			klog.Errorf("Disable addon %s failed", name)
			return err
		}
		klog.Infof("Successfully disable addon %s", name)
	}

	return nil
}
