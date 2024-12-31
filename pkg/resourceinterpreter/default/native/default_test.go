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
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

var e = NewDefaultInterpreter()

func TestHookEnabled(t *testing.T) {
	tests := []struct {
		name          string
		kind          schema.GroupVersionKind
		operationType configv1alpha1.InterpreterOperation
		want          bool
	}{
		{
			name: "deployment with interpretreplica operation enabled",
			kind: schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
			operationType: configv1alpha1.InterpreterOperationInterpretReplica,
			want:          true,
		}, {
			name: "statefulset with revisereplica operation enabled",
			kind: schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "StatefulSet",
			},
			operationType: configv1alpha1.InterpreterOperationReviseReplica,
			want:          true,
		}, {
			name: "persistentvolumeclaim with retain operation enabled",
			kind: schema.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "PersistentVolumeClaim",
			},
			operationType: configv1alpha1.InterpreterOperationRetain,
			want:          true,
		}, {
			name: "ingress with aggregatestatus operation enabled",
			kind: schema.GroupVersionKind{
				Group:   "networking.k8s.io",
				Version: "v1",
				Kind:    "Ingress",
			},
			operationType: configv1alpha1.InterpreterOperationAggregateStatus,
			want:          true,
		}, {
			name: "serviceimport with interpretdependency operation enabled",
			kind: schema.GroupVersionKind{
				Group:   "multicluster.x-k8s.io",
				Version: "v1alpha1",
				Kind:    "ServiceImport",
			},
			operationType: configv1alpha1.InterpreterOperationInterpretDependency,
			want:          true,
		}, {
			name: "deployment with interpretstatus operation enabled",
			kind: schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
			operationType: configv1alpha1.InterpreterOperationInterpretStatus,
			want:          true,
		}, {
			name: "poddisruptionbudget with interprethealth operation enabled",
			kind: schema.GroupVersionKind{
				Group:   "policy",
				Version: "v1",
				Kind:    "PodDisruptionBudget",
			},
			operationType: configv1alpha1.InterpreterOperationInterpretHealth,
			want:          true,
		}, {
			name: "foot5zmh with prune operation enabled",
			kind: schema.GroupVersionKind{
				Group:   "example-stgzr.karmada.io",
				Version: "v1alpha1",
				Kind:    "Foot5zmh",
			},
			operationType: configv1alpha1.InterpreterOperationPrune,
			want:          false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := e.HookEnabled(tt.kind, tt.operationType)
			if got != tt.want {
				t.Errorf("HookEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetReplicas(t *testing.T) {
	tests := []struct {
		name            string
		object          *unstructured.Unstructured
		wantReplica     int32
		wantRequirement *workv1alpha2.ReplicaRequirements
		wantErr         bool
	}{
		{
			name: "desired replica exists",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "fake-deployment",
						"namespace": "default",
						"labels":    map[string]interface{}{"app": "my-app"},
					},
					"spec": map[string]interface{}{
						"replicas": int64(2),
						"selector": map[string]interface{}{
							"matchLabels": map[string]interface{}{"app": "my-app"},
						},
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"labels": map[string]interface{}{"app": "my-app"},
							},
							"spec": map[string]interface{}{
								"nodeSelector": map[string]interface{}{
									"foo": "foo1",
								},
								"tolerations": []interface{}{
									map[string]interface{}{
										"key":      "foo",
										"operator": "Exists",
										"effect":   "NoSchedule",
									},
								},
								"affinity": map[string]interface{}{
									"nodeAffinity": map[string]interface{}{
										"requiredDuringSchedulingIgnoredDuringExecution": map[string]interface{}{
											"nodeSelectorTerms": []interface{}{
												map[string]interface{}{
													"matchFields": []interface{}{
														map[string]interface{}{
															"key":      "foo",
															"operator": "Exists",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantReplica: 2,
			wantRequirement: &workv1alpha2.ReplicaRequirements{
				NodeClaim: &workv1alpha2.NodeClaim{
					NodeSelector: map[string]string{
						"foo": "foo1",
					},
					Tolerations: []corev1.Toleration{
						{
							Key:      "foo",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
					HardNodeAffinity: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchFields: []corev1.NodeSelectorRequirement{
									{
										Key:      "foo",
										Operator: corev1.NodeSelectorOpExists,
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		}, {
			name: "cannot get desired replica of given kind and apiversion",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "fake-pod",
						"namespace": "default",
						"labels":    map[string]interface{}{"app": "my-app"},
					},
				},
			},
			wantReplica:     0,
			wantRequirement: &workv1alpha2.ReplicaRequirements{},
			wantErr:         true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotReplica, gotRequirement, err := e.GetReplicas(tt.object)
			if (err != nil) != tt.wantErr {
				t.Errorf("e.GetReplicas() error = %v, want %v", err, tt.wantErr)
			}
			assert.Equalf(t, tt.wantReplica, gotReplica, "e.GetReplicas(%v)", tt.object)
			assert.Equalf(t, tt.wantRequirement, gotRequirement, "e.GetReplicas(%v)", tt.object)
		})
	}
}

func TestReviseReplica(t *testing.T) {
	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		replica int64
		want    *unstructured.Unstructured
		wantErr bool
	}{
		{
			name: "revise deployment replica",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "fake-deployment",
					},
					"spec": map[string]interface{}{
						"replicas": int64(1),
					},
				},
			},
			replica: 3,
			want: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "fake-deployment",
					},
					"spec": map[string]interface{}{
						"replicas": int64(3),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "cannot revise deployment replica of given kind and apiversion",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Job",
					"metadata": map[string]interface{}{
						"name": "fake-deployment",
					},
					"spec": map[string]interface{}{
						"replicas": int64(1),
					},
				},
			},
			replica: 3,
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := e.ReviseReplica(tt.object, tt.replica)
			if (err != nil) != tt.wantErr {
				t.Errorf("e.ReviseReplica() error = %v, want %v", err, tt.wantErr)
			}
			assert.Equalf(t, tt.want, got, "e.ReviseReplica(%v,%v)", tt.object, tt.replica)
		})
	}
}

func TestRetain(t *testing.T) {
	tests := []struct {
		name     string
		desired  *unstructured.Unstructured
		observed *unstructured.Unstructured
		want     *unstructured.Unstructured
		wantErr  bool
	}{
		{
			name: "retain observed replica",
			desired: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "nginx",
					},
					"spec": map[string]interface{}{
						"replicas": int32(2),
					},
				},
			},
			observed: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "nginx",
					},
					"spec": map[string]interface{}{
						"replicas": int32(4),
					},
				},
			},
			want: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "nginx",
					},
					"spec": map[string]interface{}{
						"replicas": int32(2),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "cannot retain observed replica of given kind and apiversion",
			desired: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name": "nginx",
					},
					"spec": map[string]interface{}{
						"replicas": int32(2),
					},
				},
			},
			observed: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name": "nginx",
					},
					"spec": map[string]interface{}{
						"replicas": int32(4),
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := e.Retain(tt.desired, tt.observed)
			if (err != nil) != tt.wantErr {
				t.Errorf("e.Retain() error = %v, want %v", err, tt.wantErr)
			}
			assert.Equalf(t, tt.want, got, "e.Retain(%v,%v)", tt.desired, tt.observed)
		})
	}
}

func TestAggregateStatus(t *testing.T) {
	statusMap := map[string]interface{}{
		"replicas":            1,
		"readyReplicas":       1,
		"updatedReplicas":     1,
		"availableReplicas":   1,
		"unavailableReplicas": 0,
	}
	raw, err := helper.BuildStatusRawExtension(statusMap)
	if err != nil {
		klog.Errorf("Failed to build raw, error: %v", err)
		return
	}
	tests := []struct {
		name                  string
		object                *unstructured.Unstructured
		aggregatedStatusItems []workv1alpha2.AggregatedStatusItem
		want                  *unstructured.Unstructured
		wantErr               bool
	}{
		{
			name: "update deployment status",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
				},
			},
			aggregatedStatusItems: []workv1alpha2.AggregatedStatusItem{
				{
					ClusterName: "member1",
					Status:      raw,
					Applied:     true,
				},
				{
					ClusterName: "member2",
					Status:      raw,
					Applied:     true,
				},
			},
			want: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"creationTimestamp": nil,
					},
					"spec": map[string]interface{}{
						"selector": nil,
						"strategy": map[string]interface{}{},
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"creationTimestamp": nil,
							},
							"spec": map[string]interface{}{
								"containers": nil,
							},
						},
					},
					"status": map[string]interface{}{
						"availableReplicas": int64(2),
						"readyReplicas":     int64(2),
						"replicas":          int64(2),
						"updatedReplicas":   int64(2),
					},
				},
			},
			wantErr: false,
		}, {
			name: "cannot update deployment status for given kind and apiversion",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Job",
				},
			},
			aggregatedStatusItems: []workv1alpha2.AggregatedStatusItem{
				{
					ClusterName: "member1",
					Status:      raw,
					Applied:     true,
				},
				{
					ClusterName: "member2",
					Status:      raw,
					Applied:     true,
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := e.AggregateStatus(tt.object, tt.aggregatedStatusItems)
			if (err != nil) != tt.wantErr {
				t.Errorf("e.AggregateStatus() error = %v, want %v", err, tt.wantErr)
			}
			assert.Equalf(t, tt.want, got, "e.AggregateStatus(%v,%v)", tt.object, tt.aggregatedStatusItems)
		})
	}
}

func TestGetDependencies(t *testing.T) {
	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		want    []configv1alpha1.DependentObjectReference
		wantErr bool
	}{
		{
			name: "deployment with dependencies 2",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "fake-deployment",
						"namespace": namespace,
					},
					"spec": map[string]interface{}{
						"replicas": 3,
						"template": map[string]interface{}{
							"spec": testPairs[1].podSpecsWithDependencies.Object,
						},
					},
				},
			},
			want: []configv1alpha1.DependentObjectReference{
				{
					APIVersion: "v1",
					Kind:       "ConfigMap",
					Namespace:  namespace,
					Name:       "initcontainer-envfrom-configmap",
				},
				{
					APIVersion: "v1",
					Kind:       "Secret",
					Namespace:  namespace,
					Name:       "test-secret",
				},
				{
					APIVersion: "v1",
					Kind:       "ServiceAccount",
					Namespace:  namespace,
					Name:       "test-serviceaccount",
				},
			},
			wantErr: false,
		},
		{
			name: "can not get dependencies of given kind and apiversion",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "CronJob",
					"metadata": map[string]interface{}{
						"name":      "fake-cronjob",
						"namespace": namespace,
					},
					"spec": map[string]interface{}{
						"replicas": 3,
						"template": map[string]interface{}{
							"spec": testPairs[1].podSpecsWithDependencies.Object,
						},
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := e.GetDependencies(tt.object)
			if (err != nil) != tt.wantErr {
				t.Errorf("e.GetDependencies() error = %v, want %v", err, tt.wantErr)
			}
			assert.Equalf(t, tt.want, got, "e.GetDependencies(%v)", tt.object)
		})
	}
}

func TestReflectStatus(t *testing.T) {
	currStatus := policyv1.PodDisruptionBudgetStatus{
		CurrentHealthy:     1,
		DesiredHealthy:     1,
		DisruptionsAllowed: 1,
		ExpectedPods:       1,
	}
	wantRawExtension, err := helper.BuildStatusRawExtension(&currStatus)
	if err != nil {
		klog.Errorf("Failed to build wantRawExtension, error: %v", err)
		return
	}
	testRawExtension, err := helper.BuildStatusRawExtension(map[string]interface{}{"key": "value"})
	if err != nil {
		klog.Errorf("Failed to build testRawExtension, error: %v", err)
		return
	}
	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		want    *runtime.RawExtension
		wantErr bool
	}{
		{
			name: "object have correct format status",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "policy/v1",
					"kind":       "PodDisruptionBudget",
					"status": map[string]interface{}{
						"currentHealthy":     int64(1),
						"desiredHealthy":     int64(1),
						"disruptionsAllowed": int64(1),
						"expectedPods":       int64(1),
					},
				},
			},
			want:    wantRawExtension,
			wantErr: false,
		},
		{
			name: "missing build-in handler",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{"key": "value"},
				},
			},
			want:    testRawExtension,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := e.ReflectStatus(tt.object)
			if (err != nil) != tt.wantErr {
				t.Errorf("e.ReflectStatus() error = %v, want %v", err, tt.wantErr)
			}
			assert.Equalf(t, tt.want, got, "e.ReflectStatus(%v)", tt.object)
		})
	}
}

func TestInterpretHealth(t *testing.T) {
	tests := []struct {
		name    string
		object  *unstructured.Unstructured
		want    bool
		wantErr bool
	}{
		{
			name: "deployment healthy",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":       "fake-deployment",
						"generation": 1,
					},
					"spec": map[string]interface{}{
						"replicas": 3,
					},
					"status": map[string]interface{}{
						"availableReplicas":  3,
						"updatedReplicas":    3,
						"observedGeneration": 1,
					},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "cannot get health state for given kind and apiversion",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "policy/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":       "fake-deployment",
						"generation": 1,
					},
					"spec": map[string]interface{}{
						"replicas": 3,
					},
					"status": map[string]interface{}{
						"availableReplicas":  3,
						"updatedReplicas":    3,
						"observedGeneration": 1,
					},
				},
			},
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := e.InterpretHealth(tt.object)
			if (err != nil) != tt.wantErr {
				t.Errorf("e.InterpretHealth() error = %v, want %v", err, tt.wantErr)
			}
			assert.Equalf(t, tt.want, got, "e.InterpretHealth(%v)", tt.object)
		})
	}
}
