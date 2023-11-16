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

package spreadconstraint

import (
	"context"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
)

const (
	// Name is the name of the plugin used in the plugin registry and configurations.
	Name = "SpreadConstraint"
)

// SpreadConstraint is a plugin that checks if spread property in the Cluster.Spec.
type SpreadConstraint struct{}

var _ framework.FilterPlugin = &SpreadConstraint{}

// New instantiates the spreadconstraint plugin.
func New() (framework.Plugin, error) {
	return &SpreadConstraint{}, nil
}

// Name returns the plugin name.
func (p *SpreadConstraint) Name() string {
	return Name
}

// Filter checks if the cluster Provider/Zone/Region spread is null.
func (p *SpreadConstraint) Filter(
	_ context.Context,
	bindingSpec *workv1alpha2.ResourceBindingSpec,
	_ *workv1alpha2.ResourceBindingStatus,
	cluster *clusterv1alpha1.Cluster,
) *framework.Result {
	for _, spreadConstraint := range bindingSpec.Placement.SpreadConstraints {
		if spreadConstraint.SpreadByField == policyv1alpha1.SpreadByFieldProvider && cluster.Spec.Provider == "" {
			return framework.NewResult(framework.Unschedulable, "cluster(s) did not have provider property")
		} else if spreadConstraint.SpreadByField == policyv1alpha1.SpreadByFieldRegion && cluster.Spec.Region == "" {
			return framework.NewResult(framework.Unschedulable, "cluster(s) did not have region property")
		} else if spreadConstraint.SpreadByField == policyv1alpha1.SpreadByFieldZone && len(cluster.Spec.Zones) == 0 {
			return framework.NewResult(framework.Unschedulable, "cluster(s) did not have zones property")
		}
	}

	return framework.NewResult(framework.Success)
}
