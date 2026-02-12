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

package workloadantiaffinity

import (
	"context"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
)

const (
	// Name is the name of the plugin used in the plugin registry and configurations.
	Name = "WorkloadAntiAffinity"
)

// WorkloadAntiAffinity is a plugin that checks if a cluster matches the workload anti-affinity group constraint.
// It separates workloads in the same anti-affinity group across different clusters.
type WorkloadAntiAffinity struct{}

var (
	_ framework.FilterPlugin            = &WorkloadAntiAffinity{}
	_ framework.FilterPluginWithContext = &WorkloadAntiAffinity{}
)

// New instantiates the workload anti-affinity plugin.
func New() (framework.Plugin, error) {
	return &WorkloadAntiAffinity{}, nil
}

// Name returns the plugin name.
func (p *WorkloadAntiAffinity) Name() string {
	return Name
}

// Filter implements FilterPlugin interface for backward compatibility.
// TODO(@RainbowMango): Reconsider the plugin architecture to avoid requiring new plugins to implement this interface.
// This method should never be called as the framework will use FilterWithContext when available.
func (p *WorkloadAntiAffinity) Filter(context.Context, *workv1alpha2.ResourceBindingSpec, *workv1alpha2.ResourceBindingStatus, *clusterv1alpha1.Cluster) *framework.Result {
	// This implementation should never be reached because the framework detects FilterPluginWithContext
	// and calls FilterWithContext instead. Return Unschedulable as a safeguard.
	klog.Warningf("Filter() was called unexpectedly for plugin %s, this should not happen", Name)
	return framework.NewResult(framework.Unschedulable, "plugin should use FilterWithContext method")
}

// FilterWithContext checks if the cluster matches the workload anti-affinity group constraint.
// It only implements the new FilterWithContext interface and does not implement the old Filter interface.
// If the binding has a workload anti-affinity group specified, it ensures the cluster
// is not selected if it matches the anti-affinity group criteria.
func (p *WorkloadAntiAffinity) FilterWithContext(filterCtx *framework.FilterContext) *framework.Result {
	bindingSpec := filterCtx.BindingSpec
	cluster := filterCtx.Cluster

	// If the cluster is already in the target list, always allow it to pass
	// This ensures the scheduler never deletes scheduled resources by this plugin.
	if bindingSpec.TargetContains(cluster.Name) {
		return framework.NewResult(framework.Success)
	}

	// If no workload anti-affinity groups spec, skip this filter
	if bindingSpec.WorkloadAffinityGroups == nil || bindingSpec.WorkloadAffinityGroups.AntiAffinityGroup == "" {
		return framework.NewResult(framework.Success)
	}

	antiAffinityGroup := bindingSpec.WorkloadAffinityGroups.AntiAffinityGroup

	if filterCtx.ResourceBindingLister == nil {
		return framework.NewResult(framework.Unschedulable, "ResourceBindingLister is nil")
	}

	// TODO(@RainbowMango): 1. Use index to accelerate lister query
	//                      2. Filter out self to avoid self-exclusion during reschedule
	//                      3. Check assigning binding list to avoid cache delay
	bindings, err := filterCtx.ResourceBindingLister.ResourceBindings(bindingSpec.Resource.Namespace).List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list ResourceBinding, err: %v", err)
		return framework.NewResult(framework.Unschedulable, "failed to list ResourceBindings")
	}

	for _, rb := range bindings {
		if rb == nil || rb.Spec.WorkloadAffinityGroups == nil {
			continue
		}

		if !rb.Spec.TargetContains(cluster.GetName()) {
			continue
		}

		if rb.Spec.WorkloadAffinityGroups.AntiAffinityGroup == antiAffinityGroup {
			klog.Infof("Cluster %s already has workload with anti-affinity group %s, deny scheduling", cluster.GetName(), antiAffinityGroup)
			return framework.NewResult(framework.Unschedulable, "cluster already has workload with the same anti-affinity group")
		}
	}

	klog.Infof("checking cluster: %s, with anti-affinity group: %s", cluster.GetName(), antiAffinityGroup)

	// If cluster does not match anti-affinity group label, allow scheduling
	return framework.NewResult(framework.Success)
}
