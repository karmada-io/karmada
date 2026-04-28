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

package v1alpha1

// ExplicitPriority returns the explicit priority declared
// by '.spec.Priority'.
func (p *PropagationSpec) ExplicitPriority() int32 {
	if p.Priority == nil {
		return 0
	}
	return *p.Priority
}

// ExplicitPriority returns the explicit priority of a PropagationPolicy.
func (p *PropagationPolicy) ExplicitPriority() int32 {
	return p.Spec.ExplicitPriority()
}

// ExplicitPriority returns the explicit priority of a ClusterPropagationPolicy.
func (p *ClusterPropagationPolicy) ExplicitPriority() int32 {
	return p.Spec.ExplicitPriority()
}

// ReplicaSchedulingType returns the replica assignment strategy which is
// "Duplicated" or "Divided". Returns "Duplicated" if the replica strategy is nil.
func (p *Placement) ReplicaSchedulingType() ReplicaSchedulingType {
	if p.ReplicaScheduling == nil {
		return ReplicaSchedulingTypeDuplicated
	}

	return p.ReplicaScheduling.ReplicaSchedulingType
}
