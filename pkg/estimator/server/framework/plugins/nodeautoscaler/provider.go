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

package nodeautoscaler

import (
	"context"

	"github.com/karmada-io/karmada/pkg/estimator/pb"
)

// CapacityProvider calculates potential replicas from a node autoscaler (e.g. Karpenter).
type CapacityProvider interface {
	// IsAvailable returns true if this provider can operate in the current cluster.
	IsAvailable(ctx context.Context) bool
	// GetPotentialReplicas returns the number of additional replicas that could be
	// scheduled if the autoscaler provisions new nodes.
	// hasPendingPods indicates whether there are Pending pods in this cluster,
	// used for failure detection to avoid false positives on idle clusters.
	GetPotentialReplicas(ctx context.Context, req *pb.ReplicaRequirements, hasPendingPods bool) (int32, error)
}
