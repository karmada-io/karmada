/*
Copyright 2023 The Karmada Authors.

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

package clustereviction

import (
	"context"

	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
)

const (
	// Name is the name of the plugin used in the plugin registry and configurations.
	Name = "ClusterEviction"
)

// ClusterEviction is a plugin that checks if the target cluster is in the GracefulEvictionTasks which means it is in the process of eviction.
type ClusterEviction struct{}

var _ framework.FilterPlugin = &ClusterEviction{}

// New instantiates the ClusterEviction plugin.
func New() (framework.Plugin, error) {
	return &ClusterEviction{}, nil
}

// Name returns the plugin name.
func (p *ClusterEviction) Name() string {
	return Name
}

// Filter checks if the target cluster is in the GracefulEvictionTasks which means it is in the process of eviction.
func (p *ClusterEviction) Filter(_ context.Context, bindingSpec *workv1alpha2.ResourceBindingSpec, _ *workv1alpha2.ResourceBindingStatus, cluster *clusterv1alpha1.Cluster) *framework.Result {
	if bindingSpec.ClusterInGracefulEvictionTasks(cluster.Name) {
		klog.V(2).Infof("Cluster(%s) is in the process of eviction.", cluster.Name)
		return framework.NewResult(framework.Unschedulable, "cluster(s) is in the process of eviction")
	}

	return framework.NewResult(framework.Success)
}
