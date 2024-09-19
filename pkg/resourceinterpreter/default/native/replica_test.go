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

package native

import (
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

func Test_getAllDefaultReplicaInterpreter(t *testing.T) {
	expectedKinds := []schema.GroupVersionKind{
		{Group: "apps", Version: "v1", Kind: "Deployment"},
		{Group: "apps", Version: "v1", Kind: "StatefulSet"},
		{Group: "batch", Version: "v1", Kind: "Job"},
		{Group: "", Version: "v1", Kind: "Pod"},
	}

	got := getAllDefaultReplicaInterpreter()

	if len(got) != len(expectedKinds) {
		t.Errorf("getAllDefaultReplicaInterpreter() length = %d, want %d", len(got), len(expectedKinds))
	}

	for _, key := range expectedKinds {
		_, exists := got[key]
		if !exists {
			t.Errorf("getAllDefaultReplicaInterpreter() missing key %v", key)
		}
	}
}

func Test_deployReplica(t *testing.T) {
	object := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To[int32](2),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					NodeSelector: map[string]string{"foo": "foo1"},
					Tolerations:  []corev1.Toleration{{Key: "foo", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule}},
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{{
									MatchFields: []corev1.NodeSelectorRequirement{{Key: "foo", Operator: corev1.NodeSelectorOpExists}},
								}},
							},
						},
					},
				},
			},
		},
	}

	unstructuredObject, err := helper.ToUnstructured(object)
	if err != nil {
		klog.Errorf("Failed to transform object, error: %v", err)
		return
	}
	gotReplica, gotRequirement, err := deployReplica(unstructuredObject)

	wantReplica := int32(2)
	wantRequirement := &workv1alpha2.ReplicaRequirements{
		NodeClaim: &workv1alpha2.NodeClaim{
			NodeSelector: map[string]string{"foo": "foo1"},
			Tolerations:  []corev1.Toleration{{Key: "foo", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule}},
			HardNodeAffinity: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{{
					MatchFields: []corev1.NodeSelectorRequirement{{Key: "foo", Operator: corev1.NodeSelectorOpExists}},
				}},
			},
		},
	}

	if err != nil {
		t.Errorf("deployReplica() error = %v, want nil", err)
	}

	assert.Equalf(t, wantReplica, gotReplica, "deployReplica(%v)", unstructuredObject)
	assert.Equalf(t, wantRequirement, gotRequirement, "deployReplica(%v)", unstructuredObject)
}

func Test_statefulSetReplica(t *testing.T) {
	object := &appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Replicas: ptr.To[int32](2),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					NodeSelector: map[string]string{"foo": "foo1"},
					Tolerations:  []corev1.Toleration{{Key: "foo", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule}},
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{{
									MatchFields: []corev1.NodeSelectorRequirement{{Key: "foo", Operator: corev1.NodeSelectorOpExists}},
								}},
							},
						},
					},
				},
			},
		},
	}
	unstructuredObject, err := helper.ToUnstructured(object)
	if err != nil {
		klog.Errorf("Failed to transform object, error: %v", err)
		return
	}
	gotReplica, gotRequirement, err := statefulSetReplica(unstructuredObject)

	wantReplica := int32(2)
	wantRequirement := &workv1alpha2.ReplicaRequirements{
		NodeClaim: &workv1alpha2.NodeClaim{
			NodeSelector: map[string]string{"foo": "foo1"},
			Tolerations:  []corev1.Toleration{{Key: "foo", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule}},
			HardNodeAffinity: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{{
					MatchFields: []corev1.NodeSelectorRequirement{{Key: "foo", Operator: corev1.NodeSelectorOpExists}},
				}},
			},
		},
	}

	if err != nil {
		t.Errorf("statefulSetReplica() error = %v, want nil", err)
	}

	assert.Equalf(t, wantReplica, gotReplica, "statefulSetReplica(%v)", unstructuredObject)
	assert.Equalf(t, wantRequirement, gotRequirement, "statefulSetReplica(%v)", unstructuredObject)
}

func Test_jobReplica(t *testing.T) {
	object := &batchv1.Job{
		Spec: batchv1.JobSpec{
			Parallelism: ptr.To[int32](3),
			Completions: ptr.To[int32](2),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					NodeSelector: map[string]string{"foo": "foo1"},
					Tolerations:  []corev1.Toleration{{Key: "foo", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule}},
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{{
									MatchFields: []corev1.NodeSelectorRequirement{{Key: "foo", Operator: corev1.NodeSelectorOpExists}},
								}},
							},
						},
					},
				},
			},
		},
	}
	unstructuredObject, err := helper.ToUnstructured(object)
	if err != nil {
		klog.Errorf("Failed to transform object, error: %v", err)
		return
	}
	gotReplica, gotRequirement, err := jobReplica(unstructuredObject)

	wantReplica := int32(2)
	wantRequirement := &workv1alpha2.ReplicaRequirements{
		NodeClaim: &workv1alpha2.NodeClaim{
			NodeSelector: map[string]string{"foo": "foo1"},
			Tolerations:  []corev1.Toleration{{Key: "foo", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule}},
			HardNodeAffinity: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{{
					MatchFields: []corev1.NodeSelectorRequirement{{Key: "foo", Operator: corev1.NodeSelectorOpExists}},
				}},
			},
		},
	}

	if err != nil {
		t.Errorf("jobReplica() error = %v, want nil", err)
	}

	assert.Equalf(t, wantReplica, gotReplica, "jobReplica(%v)", unstructuredObject)
	assert.Equalf(t, wantRequirement, gotRequirement, "jobReplica(%v)", unstructuredObject)
}

func Test_podReplica(t *testing.T) {
	object := &corev1.Pod{
		Spec: corev1.PodSpec{
			NodeSelector: map[string]string{"foo": "foo1"},
			Tolerations:  []corev1.Toleration{{Key: "foo", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule}},
			Affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{{
							MatchFields: []corev1.NodeSelectorRequirement{{Key: "foo", Operator: corev1.NodeSelectorOpExists}},
						}},
					},
				},
			},
		},
	}
	unstructuredObject, err := helper.ToUnstructured(object)
	if err != nil {
		klog.Errorf("Failed to transform object, error: %v", err)
		return
	}
	gotReplica, gotRequirement, err := podReplica(unstructuredObject)

	wantReplica := int32(1)
	wantRequirement := &workv1alpha2.ReplicaRequirements{
		NodeClaim: &workv1alpha2.NodeClaim{
			NodeSelector: map[string]string{"foo": "foo1"},
			Tolerations:  []corev1.Toleration{{Key: "foo", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule}},
			HardNodeAffinity: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{{
					MatchFields: []corev1.NodeSelectorRequirement{{Key: "foo", Operator: corev1.NodeSelectorOpExists}},
				}},
			},
		},
	}

	if err != nil {
		t.Errorf("podReplica() error = %v, want nil", err)
	}

	assert.Equalf(t, wantReplica, gotReplica, "podReplica(%v)", unstructuredObject)
	assert.Equalf(t, wantRequirement, gotRequirement, "podReplica(%v)", unstructuredObject)
}
