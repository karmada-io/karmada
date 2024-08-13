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
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
)

const (
	// InvalidClusterID indicate a invalid cluster
	InvalidClusterID = -1
	// InvalidReplicas indicate that don't care about the available resource
	InvalidReplicas = -1
)

// AvailableReplicasFunc defines a function type that calculates the available replicas for clusters
// based on the given resource binding specification.
type AvailableReplicasFunc func(clusters []*clusterv1alpha1.Cluster, spec *workv1alpha2.ResourceBindingSpec) []workv1alpha2.TargetCluster

// SelectionCtx represents the context for selecting clusters.
type SelectionCtx struct {
	ClusterScores framework.ClusterScoreList        // List of cluster scores.
	Placement     *policyv1alpha1.Placement         // Placement policy.
	Spec          *workv1alpha2.ResourceBindingSpec // Resource binding specification.
	ReplicasFunc  AvailableReplicasFunc             // Function to calculate available replicas.
}

// Selection defines the interface for selecting clusters.
type Selection interface {
	// Elect selects clusters based on some criteria and returns the selected clusters.
	Elect() ([]*clusterv1alpha1.Cluster, error)
}

// SelectionFactory defines the interface for creating Selection instances.
type SelectionFactory interface {
	// Create initializes a Selection instance with the given context.
	Create(ctx SelectionCtx) (Selection, error)
}

// SelectionRegistry is a map that registers SelectionFactory instances by name.
var SelectionRegistry = make(map[string]SelectionFactory)

// DefaultSelectionFactoryName is the default name for the selection factory.
var DefaultSelectionFactoryName = "group"
