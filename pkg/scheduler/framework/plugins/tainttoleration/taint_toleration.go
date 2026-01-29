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

package tainttoleration

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	v1helper "k8s.io/component-helpers/scheduling/corev1"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
)

const (
	// Name is the name of the plugin used in the plugin registry and configurations.
	Name = "TaintToleration"
)

// TaintToleration is a plugin that checks if a propagation policy tolerates a cluster's taints.
type TaintToleration struct{}

var _ framework.FilterPlugin = &TaintToleration{}

// New instantiates the TaintToleration plugin.
func New() (framework.Plugin, error) {
	return &TaintToleration{}, nil
}

// Name returns the plugin name.
func (p *TaintToleration) Name() string {
	return Name
}

// Filter checks if the given tolerations in placement tolerate cluster's taints.
func (p *TaintToleration) Filter(
	_ context.Context,
	bindingSpec *workv1alpha2.ResourceBindingSpec,
	_ *workv1alpha2.ResourceBindingStatus,
	cluster *clusterv1alpha1.Cluster,
) *framework.Result {
	// skip the filter if the cluster is already in the list of scheduling results,
	// if the workload referencing by the binding can't tolerate the taint,
	// the taint-manager will evict it after a graceful period.
	if bindingSpec.TargetContains(cluster.Name) {
		return framework.NewResult(framework.Success)
	}

	filterPredicate := func(t *corev1.Taint) bool {
		return t.Effect == corev1.TaintEffectNoSchedule || t.Effect == corev1.TaintEffectNoExecute
	}

	// Note: Kubernetes v1.35 extended the toleration operators by introducing Lt and Gt,
	// and extended the ToleratesTaint method to include logger and enableComparisonOperators parameters.
	// PR: https://github.com/kubernetes/kubernetes/pull/134665
	//
	// TODO(@RainbowMango): Karmada requires more detailed design to support these new operators.
	// For now, we disable the comparison operators (enableComparisonOperators=false) to maintain
	// backward-compatible behavior.
	// With this flag set to false, the logger parameter is not actually used.
	taint, isUntolerated := v1helper.FindMatchingUntoleratedTaint(klog.Background(), cluster.Spec.Taints, bindingSpec.Placement.ClusterTolerations, filterPredicate, false)
	if !isUntolerated {
		return framework.NewResult(framework.Success)
	}

	return framework.NewResult(framework.Unschedulable, fmt.Sprintf("cluster(s) had untolerated taint {%s}", taint.ToString()))
}
