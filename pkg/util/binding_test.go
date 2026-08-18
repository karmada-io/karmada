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

package util

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/features"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

const (
	ClusterMember1 = "member1"
	ClusterMember2 = "member2"
)

func TestGetBindingClusterNames(t *testing.T) {
	tests := []struct {
		name     string
		binding  *workv1alpha2.ResourceBinding
		expected []string
	}{
		{
			name: "nil",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "demo-name",
					Namespace: "demo-ns",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{},
				},
				Status: workv1alpha2.ResourceBindingStatus{},
			},
			expected: nil,
		},
		{
			name: "not nil",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "demo-name",
					Namespace: "demo-ns",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name: ClusterMember1,
						},
						{
							Name: ClusterMember2,
						},
					},
				},
				Status: workv1alpha2.ResourceBindingStatus{},
			},
			expected: []string{ClusterMember1, ClusterMember2},
		},
	}

	for i := range tests {
		t.Run(tests[i].name, func(t *testing.T) {
			got := GetBindingClusterNames(&tests[i].binding.Spec)
			if !reflect.DeepEqual(got, tests[i].expected) {
				t.Errorf("GetBindingClusterNames() = %v, want %v", got, tests[i].expected)
			}
		})
	}
}

func TestIsBindingReplicasChanged(t *testing.T) {
	tests := []struct {
		name        string
		bindingSpec *workv1alpha2.ResourceBindingSpec
		strategy    *policyv1alpha1.ReplicaSchedulingStrategy
		expected    bool
	}{
		{
			name:        "nil strategy",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{},
			strategy:    nil,
			expected:    false,
		},
		{
			name:        "empty strategy",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{},
			strategy:    &policyv1alpha1.ReplicaSchedulingStrategy{},
			expected:    false,
		},
		{
			name: "Duplicated strategy and replicas not changed",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Replicas: 5,
				Clusters: []workv1alpha2.TargetCluster{
					{
						Name:     ClusterMember1,
						Replicas: 5,
					},
					{
						Name:     ClusterMember2,
						Replicas: 5,
					},
				}},
			strategy: &policyv1alpha1.ReplicaSchedulingStrategy{ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated},
			expected: false,
		},
		{
			name: "Duplicated strategy and replicas changed",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Replicas: 5,
				Clusters: []workv1alpha2.TargetCluster{
					{
						Name:     ClusterMember1,
						Replicas: 3,
					},
					{
						Name:     ClusterMember2,
						Replicas: 5,
					},
				}},
			strategy: &policyv1alpha1.ReplicaSchedulingStrategy{ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated},
			expected: true,
		},
		{
			name: "Divided strategy and replicas not changed",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Replicas: 5,
				Clusters: []workv1alpha2.TargetCluster{
					{
						Name:     ClusterMember1,
						Replicas: 2,
					},
					{
						Name:     ClusterMember2,
						Replicas: 3,
					},
				},
			},
			strategy: &policyv1alpha1.ReplicaSchedulingStrategy{ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDivided},
			expected: false,
		},
		{
			name: "Divided strategy and replicas changed",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Replicas: 5,
				Clusters: []workv1alpha2.TargetCluster{
					{
						Name:     ClusterMember1,
						Replicas: 3,
					},
					{
						Name:     ClusterMember2,
						Replicas: 3,
					},
				},
			},
			strategy: &policyv1alpha1.ReplicaSchedulingStrategy{ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDivided},
			expected: true,
		},
	}

	// Component-based workload failover tests require the MultiplePodTemplatesScheduling feature gate.
	componentTests := []struct {
		name        string
		bindingSpec *workv1alpha2.ResourceBindingSpec
		strategy    *policyv1alpha1.ReplicaSchedulingStrategy
		expected    bool
	}{
		{
			name: "single component with empty clusters should trigger rescheduling",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Components: []workv1alpha2.Component{
					{Name: "component1", Replicas: 3},
				},
				Clusters: []workv1alpha2.TargetCluster{},
			},
			strategy: &policyv1alpha1.ReplicaSchedulingStrategy{ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated},
			expected: true,
		},
		{
			name: "multiple components with empty clusters should trigger rescheduling",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Components: []workv1alpha2.Component{
					{Name: "component1", Replicas: 3},
					{Name: "component2", Replicas: 2},
				},
				Clusters: []workv1alpha2.TargetCluster{},
			},
			strategy: &policyv1alpha1.ReplicaSchedulingStrategy{ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated},
			expected: true,
		},
		{
			name: "single component with non-empty clusters should not trigger rescheduling",
			bindingSpec: &workv1alpha2.ResourceBindingSpec{
				Components: []workv1alpha2.Component{
					{Name: "component1", Replicas: 3},
				},
				Clusters: []workv1alpha2.TargetCluster{
					{Name: ClusterMember1},
				},
			},
			strategy: &policyv1alpha1.ReplicaSchedulingStrategy{ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDuplicated},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.bindingSpec == nil {
				t.FailNow()
			}
			got := IsBindingReplicasChanged(tt.bindingSpec, tt.strategy)
			if got != tt.expected {
				t.Errorf("IsBindingReplicasChanged() = %v, want %v", got, tt.expected)
			}
		})
	}

	// Run component-based tests with the feature gate enabled.
	originalFeatureGates := features.FeatureGate.DeepCopy()
	if err := features.FeatureGate.Set(fmt.Sprintf("%s=true", features.MultiplePodTemplatesScheduling)); err != nil {
		t.Fatalf("Failed to enable feature gate %s: %v", features.MultiplePodTemplatesScheduling, err)
	}
	t.Cleanup(func() {
		features.FeatureGate = originalFeatureGates
	})
	for _, tt := range componentTests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBindingReplicasChanged(tt.bindingSpec, tt.strategy)
			if got != tt.expected {
				t.Errorf("IsBindingReplicasChanged() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestClassifyComponentReplicaTransition(t *testing.T) {
	tests := []struct {
		name     string
		desired  []workv1alpha2.Component
		accepted []workv1alpha2.TargetComponent
		want     ComponentScaleDirection
	}{
		{
			name:     "equal snapshots ignore order",
			desired:  []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}},
			accepted: []workv1alpha2.TargetComponent{{Name: "taskmanager", Replicas: 4}, {Name: "jobmanager", Replicas: 1}},
			want:     ComponentScaleEqual,
		},
		{
			name:     "pure scale up",
			desired:  []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 6}},
			accepted: []workv1alpha2.TargetComponent{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}},
			want:     ComponentScaleUp,
		},
		{
			name:     "pure scale down",
			desired:  []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 2}},
			accepted: []workv1alpha2.TargetComponent{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}},
			want:     ComponentScaleDown,
		},
		{
			name:     "mixed directions",
			desired:  []workv1alpha2.Component{{Name: "jobmanager", Replicas: 2}, {Name: "taskmanager", Replicas: 3}},
			accepted: []workv1alpha2.TargetComponent{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}},
			want:     ComponentScaleMixed,
		},
		{
			name:    "missing accepted snapshot",
			desired: []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}},
			want:    ComponentScaleUnknown,
		},
		{
			name:     "duplicate desired name",
			desired:  []workv1alpha2.Component{{Name: "worker", Replicas: 2}, {Name: "worker", Replicas: 3}},
			accepted: []workv1alpha2.TargetComponent{{Name: "worker", Replicas: 2}, {Name: "server", Replicas: 1}},
			want:     ComponentScaleUnknown,
		},
		{
			name:     "duplicate accepted name",
			desired:  []workv1alpha2.Component{{Name: "worker", Replicas: 2}, {Name: "server", Replicas: 1}},
			accepted: []workv1alpha2.TargetComponent{{Name: "worker", Replicas: 2}, {Name: "worker", Replicas: 3}},
			want:     ComponentScaleUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyComponentReplicaTransition(tt.desired, tt.accepted); got != tt.want {
				t.Fatalf("ClassifyComponentReplicaTransition() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsMultiTemplateSchedulingApplicable(t *testing.T) {
	placement := &policyv1alpha1.Placement{SpreadConstraints: []policyv1alpha1.SpreadConstraint{{
		SpreadByField: policyv1alpha1.SpreadByFieldCluster,
		MinGroups:     1,
		MaxGroups:     1,
	}}}
	spec := &workv1alpha2.ResourceBindingSpec{
		Components: []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}},
		Placement:  placement,
	}
	if !IsMultiTemplateSchedulingApplicable(spec) {
		t.Fatal("single-cluster component placement should be applicable")
	}

	spec.Placement = placement.DeepCopy()
	spec.Placement.ClusterAffinities = []policyv1alpha1.ClusterAffinityTerm{{AffinityName: "primary"}}
	if IsMultiTemplateSchedulingApplicable(spec) {
		t.Fatal("ordered cluster affinities must remain outside the component-result protocol")
	}
}

func TestGenerateComponentRequirementsHash(t *testing.T) {
	requirements := &workv1alpha2.ComponentReplicaRequirements{
		NodeClaim: &workv1alpha2.NodeClaim{
			NodeSelector: map[string]string{"zone": "zone-a", "disk": "ssd"},
			Tolerations:  []corev1.Toleration{{Key: "dedicated", Operator: corev1.TolerationOpEqual, Value: "batch"}},
		},
		ResourceRequest: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("128Mi"),
			corev1.ResourceCPU:    resource.MustParse("500m"),
		},
		PriorityClassName: "high-priority",
	}
	components := []workv1alpha2.Component{
		{Name: "taskmanager", Replicas: 4, ReplicaRequirements: requirements},
		{Name: "jobmanager", Replicas: 1},
	}

	hash, err := GenerateComponentRequirementsHash(components)
	if err != nil {
		t.Fatal(err)
	}
	const goldenHash = "v1:sha256:e2b8de82572efc96f29ac7a4dcc9f65b426df2c85d9374eb288bdb861444b40d"
	if hash != goldenHash {
		t.Fatalf("GenerateComponentRequirementsHash() = %q, want %q", hash, goldenHash)
	}
	if !strings.HasPrefix(hash, "v1:sha256:") {
		t.Fatalf("GenerateComponentRequirementsHash() = %q, want versioned SHA-256", hash)
	}

	reordered := []workv1alpha2.Component{
		{Name: "jobmanager", Replicas: 10},
		{Name: "taskmanager", Replicas: 8, ReplicaRequirements: requirements.DeepCopy()},
	}
	reorderedHash, err := GenerateComponentRequirementsHash(reordered)
	if err != nil {
		t.Fatal(err)
	}
	if hash != reorderedHash {
		t.Fatal("requirements hash must ignore component order and replica counts")
	}

	changed := []workv1alpha2.Component{*components[0].DeepCopy(), components[1]}
	changed[0].ReplicaRequirements.ResourceRequest[corev1.ResourceCPU] = resource.MustParse("1")
	changedHash, err := GenerateComponentRequirementsHash(changed)
	if err != nil {
		t.Fatal(err)
	}
	if hash == changedHash {
		t.Fatal("requirements hash must change with scheduling requirements")
	}
}

func TestIsBindingComponentResultPending(t *testing.T) {
	originalFeatureGates := features.FeatureGate.DeepCopy()
	if err := features.FeatureGate.Set(fmt.Sprintf("%s=true", features.MultiplePodTemplatesScheduling)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { features.FeatureGate = originalFeatureGates })

	components := []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}}
	hash, err := GenerateComponentRequirementsHash(components)
	if err != nil {
		t.Fatal(err)
	}
	acceptedComponents := []workv1alpha2.TargetComponent{{Name: "taskmanager", Replicas: 4}, {Name: "jobmanager", Replicas: 1}}
	accepted := []workv1alpha2.TargetCluster{{Name: ClusterMember1, Components: acceptedComponents}}
	matchingAnnotations := map[string]string{AcceptedComponentRequirementsHashAnnotation: hash}
	placement := componentSchedulingPlacement()

	tests := []struct {
		name        string
		placement   *policyv1alpha1.Placement
		components  []workv1alpha2.Component
		clusters    []workv1alpha2.TargetCluster
		annotations map[string]string
		want        bool
	}{
		{name: "no scheduling result", placement: placement, components: components, want: true},
		{name: "legacy result", placement: placement, components: components, clusters: []workv1alpha2.TargetCluster{{Name: ClusterMember1}}, want: true},
		{name: "replica mismatch", placement: placement, components: []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 6}}, clusters: accepted, annotations: matchingAnnotations, want: true},
		{name: "accepted result missing hash", placement: placement, components: components, clusters: accepted, want: true},
		{name: "accepted result and requirements", placement: placement, components: components, clusters: accepted, annotations: matchingAnnotations},
		{name: "accepted result transitioning to one component", placement: placement, components: components[:1], clusters: accepted, annotations: matchingAnnotations, want: true},
		{name: "accepted result transitioning to zero components", placement: placement, clusters: accepted, annotations: matchingAnnotations, want: true},
		{name: "accepted result leaving supported placement", placement: &policyv1alpha1.Placement{}, components: components, clusters: accepted, annotations: matchingAnnotations, want: true},
		{name: "multiple component-bearing targets", placement: placement, components: components, clusters: []workv1alpha2.TargetCluster{{Name: ClusterMember1, Components: acceptedComponents}, {Name: ClusterMember2, Components: acceptedComponents}}, annotations: matchingAnnotations, want: true},
		{name: "ordinary placement without component result", placement: &policyv1alpha1.Placement{}, components: components, clusters: []workv1alpha2.TargetCluster{{Name: ClusterMember1}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &workv1alpha2.ResourceBindingSpec{Placement: tt.placement, Components: tt.components, Clusters: tt.clusters}
			if got := IsBindingComponentResultPending(spec, tt.annotations); got != tt.want {
				t.Fatalf("IsBindingComponentResultPending() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsBindingComponentScaleSupported(t *testing.T) {
	originalFeatureGates := features.FeatureGate.DeepCopy()
	if err := features.FeatureGate.Set(fmt.Sprintf("%s=true", features.MultiplePodTemplatesScheduling)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { features.FeatureGate = originalFeatureGates })

	newSpec := func(desired []workv1alpha2.Component, accepted []workv1alpha2.TargetComponent) *workv1alpha2.ResourceBindingSpec {
		return &workv1alpha2.ResourceBindingSpec{
			Placement:  componentSchedulingPlacement(),
			Components: desired,
			Clusters:   []workv1alpha2.TargetCluster{{Name: ClusterMember1, Components: accepted}},
		}
	}
	tests := []struct {
		name string
		spec *workv1alpha2.ResourceBindingSpec
		want bool
	}{
		{
			name: "pure scale up",
			spec: newSpec(
				[]workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 6}},
				[]workv1alpha2.TargetComponent{{Name: "taskmanager", Replicas: 4}, {Name: "jobmanager", Replicas: 1}},
			),
			want: true,
		},
		{
			name: "pure scale down",
			spec: newSpec(
				[]workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 2}},
				[]workv1alpha2.TargetComponent{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}},
			),
			want: true,
		},
		{
			name: "mixed scale is unsupported",
			spec: newSpec(
				[]workv1alpha2.Component{{Name: "jobmanager", Replicas: 2}, {Name: "taskmanager", Replicas: 3}},
				[]workv1alpha2.TargetComponent{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}},
			),
		},
		{
			name: "legacy result is unknown",
			spec: newSpec(
				[]workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}}, nil,
			),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsBindingComponentScaleSupported(tt.spec); got != tt.want {
				t.Fatalf("IsBindingComponentScaleSupported() = %v, want %v", got, tt.want)
			}
		})
	}
}

func componentSchedulingPlacement() *policyv1alpha1.Placement {
	return &policyv1alpha1.Placement{SpreadConstraints: []policyv1alpha1.SpreadConstraint{{
		SpreadByField: policyv1alpha1.SpreadByFieldCluster,
		MinGroups:     1,
		MaxGroups:     1,
	}}}
}

func TestGetSumOfReplicas(t *testing.T) {
	tests := []struct {
		name     string
		clusters []workv1alpha2.TargetCluster
		expected int32
	}{
		{
			name:     "empty",
			clusters: []workv1alpha2.TargetCluster{},
			expected: 0,
		},
		{
			name: "not empty",
			clusters: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 2,
				},
				{
					Name:     ClusterMember2,
					Replicas: 3,
				},
			},
			expected: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetSumOfReplicas(tt.clusters)
			if got != tt.expected {
				t.Errorf("GetSumOfReplicas() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConvertToClusterNames(t *testing.T) {
	tests := []struct {
		name     string
		clusters []workv1alpha2.TargetCluster
		expected sets.Set[string]
	}{
		{
			name:     "empty",
			clusters: []workv1alpha2.TargetCluster{},
			expected: sets.New[string](),
		},
		{
			name: "not empty",
			clusters: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 2,
				},
				{
					Name:     ClusterMember2,
					Replicas: 3,
				},
			},
			expected: sets.New(ClusterMember1, ClusterMember2),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertToClusterNames(tt.clusters)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ConvertToClusterNames() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMergeTargetClusters(t *testing.T) {
	tests := []struct {
		name     string
		old      []workv1alpha2.TargetCluster
		new      []workv1alpha2.TargetCluster
		expected []workv1alpha2.TargetCluster
	}{
		{
			name:     "empty",
			old:      []workv1alpha2.TargetCluster{},
			new:      []workv1alpha2.TargetCluster{},
			expected: []workv1alpha2.TargetCluster{},
		},
		{
			name: "old clusters are empty",
			old:  []workv1alpha2.TargetCluster{},
			new: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember2,
					Replicas: 3,
				},
			},
			expected: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember2,
					Replicas: 3,
				},
			},
		},
		{
			name: "new clusters are empty",
			old: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 3,
				},
			},
			new: []workv1alpha2.TargetCluster{},
			expected: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 3,
				},
			},
		},
		{
			name: "no cluster with the same name",
			old: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 2,
				},
			},
			new: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember2,
					Replicas: 3,
				},
			},
			expected: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 2,
				},
				{
					Name:     ClusterMember2,
					Replicas: 3,
				},
			},
		},
		{
			name: "some clusters have the same name in the old and new clusters",
			old: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 2,
				},
			},
			new: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 4,
				},
				{
					Name:     ClusterMember2,
					Replicas: 3,
				},
			},
			expected: []workv1alpha2.TargetCluster{
				{
					Name:     ClusterMember1,
					Replicas: 6,
				},
				{
					Name:     ClusterMember2,
					Replicas: 3,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergeTargetClusters(tt.old, tt.new)
			if !testhelper.IsScheduleResultEqual(got, tt.expected) {
				t.Errorf("MergeTargetClusters() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRescheduleRequired(t *testing.T) {
	currentTime := metav1.Now()
	previousTime := metav1.Time{Time: time.Now().Add(-1 * time.Minute)}

	tests := []struct {
		name                  string
		rescheduleTriggeredAt *metav1.Time
		lastScheduledTime     *metav1.Time
		want                  bool
	}{
		{
			name:                  "rescheduleTriggeredAt is nil",
			rescheduleTriggeredAt: nil,
			lastScheduledTime:     &currentTime,
			want:                  false,
		},
		{
			name:                  "lastScheduledTime is nil",
			rescheduleTriggeredAt: &currentTime,
			lastScheduledTime:     nil,
			want:                  false,
		},
		{
			name:                  "rescheduleTriggeredAt is before than lastScheduledTime",
			rescheduleTriggeredAt: &previousTime,
			lastScheduledTime:     &currentTime,
			want:                  false,
		},
		{
			name:                  "rescheduleTriggeredAt is later than lastScheduledTime",
			rescheduleTriggeredAt: &currentTime,
			lastScheduledTime:     &previousTime,
			want:                  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RescheduleRequired(tt.rescheduleTriggeredAt, tt.lastScheduledTime); got != tt.want {
				t.Errorf("RescheduleRequired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergePolicySuspension(t *testing.T) {
	tests := []struct {
		name              string
		bindingSuspension *workv1alpha2.Suspension
		policySuspension  *policyv1alpha1.Suspension
		want              *workv1alpha2.Suspension
	}{
		{
			name:              "both nil returns nil",
			bindingSuspension: nil,
			policySuspension:  nil,
			want:              nil,
		},
		{
			name: "binding suspension only preserves scheduling when policy suspension nil",
			bindingSuspension: &workv1alpha2.Suspension{
				Scheduling: new(true),
			},
			policySuspension: nil,
			want: &workv1alpha2.Suspension{
				Scheduling: new(true),
			},
		},
		{
			name: "cleanup of binding suspension preserves scheduling field",
			bindingSuspension: &workv1alpha2.Suspension{
				Suspension: policyv1alpha1.Suspension{
					Dispatching: new(true),
				},
				Scheduling: new(true),
			},
			policySuspension: nil,
			want: &workv1alpha2.Suspension{
				Scheduling: new(true),
			},
		},
		{
			name: "if the scheduling not set and policy suspension nil, will return nil",
			bindingSuspension: &workv1alpha2.Suspension{
				Suspension: policyv1alpha1.Suspension{
					Dispatching: new(true),
				},
			},
			policySuspension: nil,
			want:             nil,
		},
		{
			name:              "policy suspension set and no existing binding creates new suspension from policy",
			bindingSuspension: nil,
			policySuspension: &policyv1alpha1.Suspension{
				Dispatching: new(true),
			},
			want: &workv1alpha2.Suspension{
				Suspension: policyv1alpha1.Suspension{
					Dispatching: new(true),
				},
			},
		},
		{
			name: "should merge policy suspension and binding suspension",
			bindingSuspension: &workv1alpha2.Suspension{
				Scheduling: new(true),
			},
			policySuspension: &policyv1alpha1.Suspension{
				Dispatching: new(true),
			},
			want: &workv1alpha2.Suspension{
				Suspension: policyv1alpha1.Suspension{
					Dispatching: new(true),
				},
				Scheduling: new(true),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergePolicySuspension(tt.bindingSuspension, tt.policySuspension)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MergePolicySuspension() got = %v, want %v", got, tt.want)
			}
		})
	}
}
