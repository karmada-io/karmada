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

package plugins

import (
	"context"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

// EvictorPlugin is the interface that all graceful eviction plugins must implement.
// Its core responsibility is to determine if an eviction task can be safely finalized.
type EvictorPlugin interface {
	// Name returns the name of the plugin.
	Name() string

	// CanEvictRB decides whether a specific eviction task for a ResourceBinding can be finalized.
	// Returns true if the task can be cleaned up.
	CanEvictRB(ctx context.Context, task *workv1alpha2.GracefulEvictionTask, binding *workv1alpha2.ResourceBinding) bool

	// CanEvictCRB decides whether a specific eviction task for a ClusterResourceBinding can be finalized.
	// Returns true if the task can be cleaned up.
	CanEvictCRB(ctx context.Context, task *workv1alpha2.GracefulEvictionTask, binding *workv1alpha2.ClusterResourceBinding) bool
}

// PluginFactory is a function that builds an EvictorPlugin.
type PluginFactory func() (EvictorPlugin, error)
