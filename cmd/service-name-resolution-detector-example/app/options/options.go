/*
Copyright 2024 The Karmada Authors.

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

package options

import (
	"k8s.io/apimachinery/pkg/util/validation/field"
	cliflag "k8s.io/component-base/cli/flag"

	"github.com/karmada-io/karmada/pkg/sharedcli/klogflag"
)

// Options contains everything necessary to create and run cluster-problem-detector.
type Options struct {
	Generic *GenericOptions
	Coredns *CorednsOptions
}

// NewOptions return default options.
func NewOptions() *Options {
	return &Options{
		Generic: NewGenericOptions(),
		Coredns: NewCorednsOptions(),
	}
}

// Flags return all flag sets for options.
func (o *Options) Flags(allDetectors []string) cliflag.NamedFlagSets {
	fss := cliflag.NamedFlagSets{}

	o.Generic.AddFlags(fss.FlagSet("generic"), allDetectors)
	o.Coredns.AddFlags(fss.FlagSet("coredns"))

	klogflag.Add(fss.FlagSet("logs"))

	return fss
}

// Complete fills in fields required to have valid data.
func (o *Options) Complete() error {
	if o == nil {
		return nil
	}

	if err := o.Generic.Complete(); err != nil {
		return err
	}
	if err := o.Coredns.Complete(); err != nil {
		return err
	}
	return nil
}

// Validate checks options and return a slice of found errs.
func (o *Options) Validate() field.ErrorList {
	total := field.ErrorList{}
	if errs := o.Generic.Validate(); len(errs) != 0 {
		total = append(total, errs...)
	}
	if errs := o.Coredns.Validate(); len(errs) != 0 {
		total = append(total, errs...)
	}
	return total
}
