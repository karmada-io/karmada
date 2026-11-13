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

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

func fullV1Alpha1Spec() ResourceBindingSpec {
	return ResourceBindingSpec{
		Resource: ObjectReference{
			APIVersion:      "apps/v1",
			Kind:            "Deployment",
			Namespace:       "default",
			Name:            "nginx",
			ResourceVersion: "1000",
			ReplicaResourceRequirements: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("100m"),
			},
			Replicas: 3,
		},
		Clusters: []TargetCluster{
			{Name: "member1", Replicas: 2},
			{Name: "member2", Replicas: 1},
		},
	}
}

func fullV1Alpha1Status() ResourceBindingStatus {
	return ResourceBindingStatus{
		Conditions: []metav1.Condition{
			{
				Type:               "FullyApplied",
				Status:             metav1.ConditionTrue,
				Reason:             "Applied",
				LastTransitionTime: metav1.Now(),
			},
		},
		AggregatedStatus: []AggregatedStatusItem{
			{
				ClusterName:    "member1",
				Status:         &runtime.RawExtension{Raw: []byte(`{"ready":2}`)},
				Applied:        true,
				AppliedMessage: "applied",
			},
		},
	}
}

func TestConvertBindingSpecToHub(t *testing.T) {
	tests := []struct {
		name string
		src  ResourceBindingSpec
		want workv1alpha2.ResourceBindingSpec
	}{
		{
			name: "full spec",
			src:  fullV1Alpha1Spec(),
			want: workv1alpha2.ResourceBindingSpec{
				Resource: workv1alpha2.ObjectReference{
					APIVersion:      "apps/v1",
					Kind:            "Deployment",
					Namespace:       "default",
					Name:            "nginx",
					ResourceVersion: "1000",
				},
				ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
					ResourceRequest: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("100m"),
					},
				},
				Replicas: 3,
				Clusters: []workv1alpha2.TargetCluster{
					{Name: "member1", Replicas: 2},
					{Name: "member2", Replicas: 1},
				},
			},
		},
		{
			name: "spec without replica requirements",
			src: ResourceBindingSpec{
				Resource: ObjectReference{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "cm"},
			},
			want: workv1alpha2.ResourceBindingSpec{
				Resource: workv1alpha2.ObjectReference{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "cm"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := &workv1alpha2.ResourceBindingSpec{}
			ConvertBindingSpecToHub(&tt.src, dst)
			if !equality.Semantic.DeepEqual(tt.want, *dst) {
				t.Errorf("ConvertBindingSpecToHub() got %+v, want %+v", *dst, tt.want)
			}
		})
	}
}

func TestConvertBindingSpecFromHubDropsHubOnlyFields(t *testing.T) {
	src := workv1alpha2.ResourceBindingSpec{
		Resource: workv1alpha2.ObjectReference{
			APIVersion:      "apps/v1",
			Kind:            "Deployment",
			Namespace:       "default",
			Name:            "nginx",
			UID:             "257a80b3-888a-4c46-8f94-b45f5ae1b5f8",
			ResourceVersion: "1000",
		},
		PropagateDeps: true,
		ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
			ResourceRequest: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
			NodeClaim: &workv1alpha2.NodeClaim{
				NodeSelector: map[string]string{"disktype": "ssd"},
			},
			PriorityClassName: "high-priority",
		},
		Replicas: 2,
		Components: []workv1alpha2.Component{
			{Name: "worker", Replicas: 2},
		},
		Clusters: []workv1alpha2.TargetCluster{
			{Name: "member1", Replicas: 2, Components: []workv1alpha2.TargetComponent{{Name: "worker", Replicas: 2}}},
		},
		SchedulerName: "karmada-scheduler",
	}
	want := ResourceBindingSpec{
		Resource: ObjectReference{
			APIVersion:      "apps/v1",
			Kind:            "Deployment",
			Namespace:       "default",
			Name:            "nginx",
			ResourceVersion: "1000",
			ReplicaResourceRequirements: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("100m"),
			},
			Replicas: 2,
		},
		Clusters: []TargetCluster{{Name: "member1", Replicas: 2}},
	}
	dst := &ResourceBindingSpec{}
	ConvertBindingSpecFromHub(&src, dst)
	if !equality.Semantic.DeepEqual(want, *dst) {
		t.Errorf("ConvertBindingSpecFromHub() got %+v, want %+v", *dst, want)
	}
}

func TestConvertBindingStatusToHub(t *testing.T) {
	src := fullV1Alpha1Status()
	dst := &workv1alpha2.ResourceBindingStatus{}
	ConvertBindingStatusToHub(&src, dst)
	want := workv1alpha2.ResourceBindingStatus{
		Conditions: src.Conditions,
		AggregatedStatus: []workv1alpha2.AggregatedStatusItem{
			{
				ClusterName:    "member1",
				Status:         src.AggregatedStatus[0].Status,
				Applied:        true,
				AppliedMessage: "applied",
			},
		},
	}
	if !equality.Semantic.DeepEqual(want, *dst) {
		t.Errorf("ConvertBindingStatusToHub() got %+v, want %+v", *dst, want)
	}
}

func TestConvertBindingStatusFromHubDropsHealth(t *testing.T) {
	src := workv1alpha2.ResourceBindingStatus{
		AggregatedStatus: []workv1alpha2.AggregatedStatusItem{
			{ClusterName: "member1", Health: workv1alpha2.ResourceHealthy},
		},
	}
	dst := &ResourceBindingStatus{}
	ConvertBindingStatusFromHub(&src, dst)
	want := ResourceBindingStatus{
		AggregatedStatus: []AggregatedStatusItem{{ClusterName: "member1"}},
	}
	if !equality.Semantic.DeepEqual(want, *dst) {
		t.Errorf("ConvertBindingStatusFromHub() got %+v, want %+v", *dst, want)
	}
}

func TestResourceBindingRoundTrip(t *testing.T) {
	original := &ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx-binding",
			Namespace: "default",
			Labels:    map[string]string{"foo": "bar"},
		},
		Spec:   fullV1Alpha1Spec(),
		Status: fullV1Alpha1Status(),
	}
	hub := &workv1alpha2.ResourceBinding{}
	if err := original.ConvertTo(hub); err != nil {
		t.Fatalf("ConvertTo() error: %v", err)
	}
	restored := &ResourceBinding{}
	if err := restored.ConvertFrom(hub); err != nil {
		t.Fatalf("ConvertFrom() error: %v", err)
	}
	if !equality.Semantic.DeepEqual(original, restored) {
		t.Errorf("round trip mismatch: got %+v, want %+v", restored, original)
	}
}

func TestClusterResourceBindingRoundTrip(t *testing.T) {
	original := &ClusterResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "cluster-binding",
			Labels: map[string]string{"foo": "bar"},
		},
		Spec:   fullV1Alpha1Spec(),
		Status: fullV1Alpha1Status(),
	}
	hub := &workv1alpha2.ClusterResourceBinding{}
	if err := original.ConvertTo(hub); err != nil {
		t.Fatalf("ConvertTo() error: %v", err)
	}
	restored := &ClusterResourceBinding{}
	if err := restored.ConvertFrom(hub); err != nil {
		t.Fatalf("ConvertFrom() error: %v", err)
	}
	if !equality.Semantic.DeepEqual(original, restored) {
		t.Errorf("round trip mismatch: got %+v, want %+v", restored, original)
	}
}
