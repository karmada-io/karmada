/*
Copyright 2022 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package init

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/utils/strings/slices"

	cmdutil "github.com/karmada-io/karmada/pkg/karmadactl/util"
	"github.com/karmada-io/karmada/pkg/karmadactl/util/apiclient"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// CommandAddonsDisableOption options for addons list.
type CommandAddonsDisableOption struct {
	GlobalCommandOptions

	KarmadaKubeClientSet *kubernetes.Clientset

	Force bool
}

// Complete the conditions required to be able to run disable.
func (o *CommandAddonsDisableOption) Complete() error {
	err := o.GlobalCommandOptions.Complete()
	if err != nil {
		return err
	}

	o.KarmadaKubeClientSet, err = apiclient.NewClientSet(o.KarmadaRestConfig)
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

	if slices.Contains(args, names.KarmadaSchedulerEstimatorComponentName) && o.Cluster == "" {
		return fmt.Errorf("member cluster and config is needed when disable karmada-scheduler-estimator,use `--cluster=member --member-kubeconfig /root/.kube/config --member-context member1` to disable karmada-scheduler-estimator")
	}
	return nil
}

// Run start disable Karmada addons
func (o *CommandAddonsDisableOption) Run(args []string) error {
	fmt.Printf("Disable Karmada addon %s\n", args)
	if !o.Force && !cmdutil.DeleteConfirmation() {
		return nil
	}

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
