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

package core

// PreemptionResult describes the lower-priority bindings selected for eviction
// so that a higher-priority binding can be scheduled later.
type PreemptionResult struct {
	// Cluster is the target cluster where preemption should happen.
	Cluster string

	// Victims contains lower-priority bindings to evict from Cluster.
	Victims []VictimBinding
}

// VictimBinding identifies a lower-priority binding selected for preemption.
type VictimBinding struct {
	Namespace string
	Name      string
	Replicas  int32
	Priority  int32
}
