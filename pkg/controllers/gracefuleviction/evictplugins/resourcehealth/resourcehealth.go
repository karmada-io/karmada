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

package resourcehealth

import (
	"context"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/controllers/gracefuleviction/evictplugins"
)

const (
	// PluginName is the name of this plugin.
	PluginName = "ResourceHealth"
)

func init() {
	// Register this plugin to the global registry.
	plugins.RegisterPlugin(PluginName, New)
}

// ResourceHealth is a plugin that checks if the resource is healthy on all target clusters.
type ResourceHealth struct{}

// New is a factory that creates the ResourceHealth plugin.
func New() (plugins.EvictorPlugin, error) {
	return &ResourceHealth{}, nil
}

// Name returns the plugin's name.
func (p *ResourceHealth) Name() string {
	return PluginName
}

// CanEvictRB implements the check for ResourceBinding.
func (p *ResourceHealth) CanEvictRB(_ context.Context, task *workv1alpha2.GracefulEvictionTask, binding *workv1alpha2.ResourceBinding) bool {
	return isHealthy(task.FromCluster, binding.Spec.Clusters, binding.Status.AggregatedStatus)
}

// CanEvictCRB implements the check for ClusterResourceBinding.
func (p *ResourceHealth) CanEvictCRB(_ context.Context, task *workv1alpha2.GracefulEvictionTask, binding *workv1alpha2.ClusterResourceBinding) bool {
	return isHealthy(task.FromCluster, binding.Spec.Clusters, binding.Status.AggregatedStatus)
}

// isHealthy is a private helper that contains the actual health checking logic.
// It checks if the resource is healthy on all target clusters except for the one being evicted.
func isHealthy(evictingCluster string, scheduleResult []workv1alpha2.TargetCluster, observedStatus []workv1alpha2.AggregatedStatusItem) bool {
	for _, targetCluster := range scheduleResult {
		// Skip the health check for the cluster that is being evicted.
		if targetCluster.Name == evictingCluster {
			continue
		}
		var statusItem *workv1alpha2.AggregatedStatusItem
		// Find the observed status of the targetCluster.
		for i, aggregatedStatus := range observedStatus {
			if aggregatedStatus.ClusterName == targetCluster.Name {
				statusItem = &observedStatus[i]
				break
			}
		}
		// If no observed status is found, it might mean the resource hasn't been applied yet.
		// In that case, we consider it not fully healthy.
		if statusItem == nil {
			return false
		}
		// If the resource is not in a healthy state, we can't finalize the eviction yet.
		if statusItem.Health != workv1alpha2.ResourceHealthy {
			return false
		}
	}
	// If all other target clusters are healthy, return true.
	return true
}
