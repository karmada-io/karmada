/*
Copyright 2025 The Karmada Authors.

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

// Package options provides the flags for the karmada-controller-manager.
package options

import (
	"flag"

	"github.com/karmada-io/karmada/pkg/sharedcli/klogflag"
	"k8s.io/apimachinery/pkg/util/validation/field"
	cliflag "k8s.io/component-base/cli/flag"
)

// KarmadaControllerManagerOptions is the main context object for the karmada-controller-manager.
type KarmadaControllerManagerOptions struct {
	Generic *GenericOptions

	Failover *FailoverOptions
}

// NewKarmadaControllerManagerOptions creates a new KarmadaControllerManagerOptions object with default parameters.
func NewKarmadaControllerManagerOptions() *KarmadaControllerManagerOptions {
	return &KarmadaControllerManagerOptions{
		Generic:  NewGenericOptions(),
		Failover: NewFailoverOptions(),
	}
}

// Flags returns flags for a specific KarmadaControllerManager by section name.
func (s *KarmadaControllerManagerOptions) Flags(allControllers, disabledByDefaultControllers []string) cliflag.NamedFlagSets {
	fss := cliflag.NamedFlagSets{}

	// Set generic flags
	genericFlagSet := fss.FlagSet("generic")
	// Add the flag(--kubeconfig) that is added by controller-runtime
	// (https://github.com/kubernetes-sigs/controller-runtime/blob/v0.11.1/pkg/client/config/config.go#L39),
	// and update the flag usage.
	genericFlagSet.AddGoFlagSet(flag.CommandLine)
	genericFlagSet.Lookup("kubeconfig").Usage = "Path to karmada control plane kubeconfig file."
	s.Generic.AddFlags(genericFlagSet, allControllers, disabledByDefaultControllers)

	s.Failover.AddFlags(fss.FlagSet("failover"))

	// Set klog flags
	klogflag.Add(fss.FlagSet("logs"))

	return fss
}

// Validate checks Options and return a slice of found errs.
func (o *KarmadaControllerManagerOptions) Validate() field.ErrorList {
	errs := field.ErrorList{}
	errs = append(errs, o.Generic.Validate()...)
	errs = append(errs, o.Failover.Validate()...)
	return errs
}
