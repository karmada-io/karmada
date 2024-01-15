/*
Copyright 2024 The Karmada Authors.

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

package framework

import (
	"context"

	"k8s.io/client-go/informers"

	"github.com/karmada-io/karmada/pkg/estimator/pb"
)

// Framework manages the set of plugins in use by the estimator.
type Framework interface {
	Handle
	// RunEstimateReplicasPlugins runs the set of configured EstimateReplicasPlugins
	// for estimating replicas based on the given replicaRequirements.
	// It returns an integer and an error.
	// The integer represents the minimum calculated value of estimated replicas from each EstimateReplicasPlugin.
	RunEstimateReplicasPlugins(ctx context.Context, replicaRequirements *pb.ReplicaRequirements) (int32, error)
	// TODO(wengyao04): we can add filter and score plugin extension points if needed in the future
}

// Plugin is the parent type for all the scheduling framework plugins.
type Plugin interface {
	Name() string
}

// EstimateReplicasPlugin is an interface for replica estimation plugins.
// These estimators are used to estimate the replicas for a given pb.ReplicaRequirements
type EstimateReplicasPlugin interface {
	Plugin
	// Estimate is called for each MaxAvailableReplicas request.
	// It returns an integer and an error
	// The integer representing the number of calculated replica for the given replicaRequirements
	Estimate(ctx context.Context, replicaRequirements *pb.ReplicaRequirements) (int32, error)
}

// Handle provides data and some tools that plugins can use. It is
// passed to the plugin factories at the time of plugin initialization. Plugins
// must store and use this handle to call framework functions.
// We follow the design pattern as kubernetes scheduler framework
type Handle interface {
	SharedInformerFactory() informers.SharedInformerFactory
}
