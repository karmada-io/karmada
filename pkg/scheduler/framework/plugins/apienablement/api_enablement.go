/*
Copyright 2021 The Karmada Authors.

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

package apienablement

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

const (
	// Name is the name of the plugin used in the plugin registry and configurations.
	Name = "APIEnablement"
)

// APIEnablement is a plugin that checks if the API(CRD) of the resource is installed in the target cluster.
type APIEnablement struct{}

var _ framework.FilterPlugin = &APIEnablement{}

// New instantiates the APIEnablement plugin.
func New() (framework.Plugin, error) {
	return &APIEnablement{}, nil
}

// Name returns the plugin name.
func (p *APIEnablement) Name() string {
	return Name
}

// Filter checks if the API(CRD) of the resource is enabled or installed in the target cluster.
func (p *APIEnablement) Filter(
	_ context.Context,
	bindingSpec *workv1alpha2.ResourceBindingSpec,
	_ *workv1alpha2.ResourceBindingStatus,
	cluster *clusterv1alpha1.Cluster,
) *framework.Result {
	if helper.IsAPIEnabled(cluster.Status.APIEnablements, bindingSpec.Resource.APIVersion, bindingSpec.Resource.Kind) {
		return framework.NewResult(framework.Success)
	}

	// Let the cluster pass if it is already on the list of schedule result and the cluster's
	// API enablements is incomplete, to avoid the issue that cluster be accidentally removed
	// due to untrusted API enablements.
	if bindingSpec.TargetContains(cluster.Name) &&
		!meta.IsStatusConditionTrue(cluster.Status.Conditions, clusterv1alpha1.ClusterConditionCompleteAPIEnablements) {
		return framework.NewResult(framework.Success)
	}

	klog.V(2).Infof("Cluster(%s) not fit as missing API(%s, kind=%s)", cluster.Name, bindingSpec.Resource.APIVersion, bindingSpec.Resource.Kind)

	return framework.NewResult(framework.Unschedulable, "cluster(s) did not have the API resource")
}
