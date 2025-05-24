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

package v1alpha1

const (
	// TaintClusterUnscheduler will be added when cluster becomes unschedulable
	// and removed when cluster becomes schedulable.
	TaintClusterUnscheduler = "cluster.karmada.io/unschedulable"
	// TaintClusterNotReady will be added when cluster is not ready
	// and removed when cluster becomes ready.
	TaintClusterNotReady = "cluster.karmada.io/not-ready"
	// TaintClusterUnreachable will be added when cluster becomes unreachable
	// (corresponding to ClusterConditionReady status ConditionUnknown)
	// and removed when cluster becomes reachable (ClusterConditionReady status ConditionTrue).
	TaintClusterUnreachable = "cluster.karmada.io/unreachable"

	// CacheSourceAnnotationKey is the annotation that added to a resource to
	// represent which cluster it cached from.
	CacheSourceAnnotationKey = "resource.karmada.io/cached-from-cluster"
)
