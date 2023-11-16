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

package clusterlocality

import (
	"context"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
)

const (
	// Name is the name of the plugin used in the plugin registry and configurations.
	Name = "ClusterLocality"
)

// ClusterLocality is a score plugin that favors cluster that already have the resource.
type ClusterLocality struct{}

var _ framework.ScorePlugin = &ClusterLocality{}

// New instantiates the ClusterLocality plugin.
func New() (framework.Plugin, error) {
	return &ClusterLocality{}, nil
}

// Name returns the plugin name.
func (p *ClusterLocality) Name() string {
	return Name
}

// Score calculates the score on the candidate cluster.
// If the cluster already have the resource(exists in .spec.Clusters of ResourceBinding or ClusterResourceBinding),
// then score is 100, otherwise 0.
func (p *ClusterLocality) Score(_ context.Context,
	spec *workv1alpha2.ResourceBindingSpec, cluster *clusterv1alpha1.Cluster) (int64, *framework.Result) {
	if len(spec.Clusters) == 0 {
		return framework.MinClusterScore, framework.NewResult(framework.Success)
	}

	if spec.TargetContains(cluster.Name) {
		return framework.MaxClusterScore, framework.NewResult(framework.Success)
	}

	return framework.MinClusterScore, framework.NewResult(framework.Success)
}

// ScoreExtensions of the Score plugin.
func (p *ClusterLocality) ScoreExtensions() framework.ScoreExtensions {
	return nil
}
