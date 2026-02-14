/*
Copyright 2026 The Karmada Authors.

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

package workloadaffinity

import (
	"context"

	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
	"github.com/karmada-io/karmada/pkg/util/indexregistry"
	"github.com/karmada-io/karmada/pkg/util/names"
)

const (
	// Name is the name of the plugin used in the plugin registry and configurations.
	Name = "WorkloadAffinity"
)

// WorkloadAffinity is a plugin that checks if a cluster matches the workload affinity group constraint.
// It ensures workloads in the same affinity group are scheduled to the same cluster.
type WorkloadAffinity struct{}

var (
	_ framework.FilterPlugin            = &WorkloadAffinity{}
	_ framework.FilterPluginWithContext = &WorkloadAffinity{}
)

// New instantiates the workload affinity plugin.
func New() (framework.Plugin, error) {
	return &WorkloadAffinity{}, nil
}

// Name returns the plugin name.
func (p *WorkloadAffinity) Name() string {
	return Name
}

// Filter implements FilterPlugin interface for backward compatibility.
// TODO(@RainbowMango): Reconsider the plugin architecture to avoid requiring new plugins to implement this interface.
// This method should never be called as the framework will use FilterWithContext when available.
func (p *WorkloadAffinity) Filter(context.Context, *workv1alpha2.ResourceBindingSpec, *workv1alpha2.ResourceBindingStatus, *clusterv1alpha1.Cluster) *framework.Result {
	// This implementation should never be reached because the framework detects FilterPluginWithContext
	// and calls FilterWithContext instead. Return Unschedulable as a safeguard.
	klog.Warningf("Filter() was called unexpectedly for plugin %s, this should not happen", Name)
	return framework.NewResult(framework.Unschedulable, "plugin should use FilterWithContext method")
}

// FilterWithContext checks if the cluster matches the workload affinity group constraint.
// It only implements the new FilterWithContext interface and does not implement the old Filter interface.
// If the binding has a workload affinity group specified, it ensures the cluster
// is selected only if it matches the affinity group criteria (same cluster as other workloads in the group).
func (p *WorkloadAffinity) FilterWithContext(filterCtx *framework.FilterContext) *framework.Result {
	bindingSpec := filterCtx.BindingSpec
	cluster := filterCtx.Cluster

	// If the cluster is already in the target list, always allow it to pass
	// This ensures the scheduler never deletes scheduled resources by this plugin.
	if bindingSpec.TargetContains(cluster.Name) {
		return framework.NewResult(framework.Success)
	}

	// If no workload affinity groups spec, skip this filter
	if bindingSpec.WorkloadAffinityGroups == nil || bindingSpec.WorkloadAffinityGroups.AffinityGroup == "" {
		return framework.NewResult(framework.Success)
	}

	affinityGroup := bindingSpec.WorkloadAffinityGroups.AffinityGroup

	if filterCtx.ResourceBindingIndexer == nil {
		return framework.NewResult(framework.Unschedulable, "ResourceBindingIndexer is nil")
	}

	// TODO(@RainbowMango): Check assigning binding list to avoid cache delay
	objs, err := filterCtx.ResourceBindingIndexer.ByIndex(indexregistry.ResourceBindingIndexByAffinityGroup, affinityGroup)
	if err != nil {
		klog.Errorf("Failed to list ResourceBinding, err: %v", err)
		return framework.NewResult(framework.Unschedulable, "failed to list ResourceBindings")
	}

	bindingName := names.GenerateBindingName(bindingSpec.Resource.Kind, bindingSpec.Resource.Name)
	for _, obj := range objs {
		rb, ok := obj.(*workv1alpha2.ResourceBinding)
		if !ok {
			continue
		}
		if rb == nil || rb.Spec.WorkloadAffinityGroups == nil || rb.GetNamespace() != bindingSpec.Resource.Namespace {
			continue
		}

		if rb.Spec.Clusters == nil {
			continue
		}

		if rb.GetName() == bindingName {
			continue
		}

		if !rb.Spec.TargetContains(cluster.Name) {
			klog.Infof("cluster %s does not match the affinity group %s because ResourceBinding %s does not target this cluster, deny scheduling", cluster.GetName(), affinityGroup, rb.GetName())
			return framework.NewResult(framework.Unschedulable, "ResourceBinding with the same affinity group does not target this cluster")
		}
	}

	klog.Infof("checking cluster: %s, with affinity group: %s", cluster.GetName(), affinityGroup)
	return framework.NewResult(framework.Success)
}
