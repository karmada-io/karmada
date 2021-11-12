/*
Copyright The Karmada Authors.

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

package apiinstalled

import (
	"context"

	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

const (
	// Name is the name of the plugin used in the plugin registry and configurations.
	Name = "APIInstalled"
)

// APIInstalled is a plugin that checks if the API(CRD) of the resource is installed in the target cluster.
type APIInstalled struct{}

var _ framework.FilterPlugin = &APIInstalled{}
var _ framework.ScorePlugin = &APIInstalled{}

// New instantiates the APIInstalled plugin.
func New() framework.Plugin {
	return &APIInstalled{}
}

// Name returns the plugin name.
func (p *APIInstalled) Name() string {
	return Name
}

// Filter checks if the API(CRD) of the resource is installed in the target cluster.
func (p *APIInstalled) Filter(ctx context.Context, placement *policyv1alpha1.Placement, resource *workv1alpha2.ObjectReference, cluster *clusterv1alpha1.Cluster) *framework.Result {
	if !helper.IsAPIEnabled(cluster.Status.APIEnablements, resource.APIVersion, resource.Kind) {
		klog.V(2).Infof("cluster(%s) not fit as missing API(%s, kind=%s)", cluster.Name, resource.APIVersion, resource.Kind)
		return framework.NewResult(framework.Unschedulable, "no such API resource")
	}

	return framework.NewResult(framework.Success)
}

// Score calculates the score on the candidate cluster.
func (p *APIInstalled) Score(ctx context.Context, placement *policyv1alpha1.Placement, cluster *clusterv1alpha1.Cluster) (float64, *framework.Result) {
	return 0, framework.NewResult(framework.Success)
}
