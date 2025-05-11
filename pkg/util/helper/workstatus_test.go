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

package helper

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/indexregistry"
)

func TestAggregateResourceBindingWorkStatus(t *testing.T) {
	scheme := setupScheme()

	// Helper functions to create binding
	createBinding := func(name, bindingID string, clusters []string) *workv1alpha2.ResourceBinding {
		return &workv1alpha2.ResourceBinding{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      name,
				Labels: map[string]string{
					workv1alpha2.ResourceBindingPermanentIDLabel: bindingID,
				},
			},
			Spec: workv1alpha2.ResourceBindingSpec{
				Resource: workv1alpha2.ObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "test-deployment",
					Namespace:  "default",
				},
				Clusters: func() []workv1alpha2.TargetCluster {
					var targetClusters []workv1alpha2.TargetCluster
					for _, cluster := range clusters {
						targetClusters = append(targetClusters, workv1alpha2.TargetCluster{Name: cluster})
					}
					return targetClusters
				}(),
			},
		}
	}

	// Helper functions to create work
	createWork := func(name, namespace, bindingID string, applied bool, withStatus bool) *workv1alpha1.Work {
		work := &workv1alpha1.Work{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
				Labels: map[string]string{
					workv1alpha2.ResourceBindingPermanentIDLabel: bindingID,
				},
			},
			Spec: workv1alpha1.WorkSpec{
				Workload: workv1alpha1.WorkloadTemplate{
					Manifests: []workv1alpha1.Manifest{
						{
							RawExtension: runtime.RawExtension{
								Raw: []byte(`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"test-deployment","namespace":"default"}}`),
							},
						},
					},
				},
			},
		}

		if withStatus {
			status := metav1.ConditionTrue
			if !applied {
				status = metav1.ConditionFalse
			}
			work.Status = workv1alpha1.WorkStatus{
				Conditions: []metav1.Condition{
					{
						Type:    workv1alpha1.WorkApplied,
						Status:  status,
						Message: "test message",
					},
				},
				ManifestStatuses: []workv1alpha1.ManifestStatus{
					{
						Identifier: workv1alpha1.ResourceIdentifier{
							Group:     "apps",
							Version:   "v1",
							Kind:      "Deployment",
							Name:      "test-deployment",
							Namespace: "default",
						},
						Status: &runtime.RawExtension{Raw: []byte(`{"replicas": 3}`)},
						Health: "Healthy",
					},
				},
			}
		}
		return work
	}

	tests := []struct {
		name            string
		bindingID       string
		binding         *workv1alpha2.ResourceBinding
		works           []*workv1alpha1.Work
		expectedError   bool
		expectedStatus  metav1.ConditionStatus
		expectedApplied bool
		expectedEvent   string
	}{
		{
			name:      "successful single work aggregation",
			bindingID: "test-id-1",
			binding:   createBinding("binding-1", "test-id-1", []string{"member1"}),
			works: []*workv1alpha1.Work{
				createWork("work-1", "karmada-es-member1", "test-id-1", true, true),
			},
			expectedError:   false,
			expectedStatus:  metav1.ConditionTrue,
			expectedApplied: true,
			expectedEvent:   "Update ResourceBinding(default/binding-1) with AggregatedStatus successfully",
		},
		{
			name:            "work not found",
			bindingID:       "test-id-2",
			binding:         createBinding("binding-2", "test-id-2", []string{"member1"}),
			works:           []*workv1alpha1.Work{},
			expectedError:   false,
			expectedStatus:  metav1.ConditionFalse,
			expectedApplied: false,
		},
		{
			name:      "work not applied",
			bindingID: "test-id-3",
			binding:   createBinding("binding-3", "test-id-3", []string{"member1"}),
			works: []*workv1alpha1.Work{
				createWork("work-3", "karmada-es-member1", "test-id-3", false, true),
			},
			expectedError:   false,
			expectedStatus:  metav1.ConditionFalse,
			expectedApplied: false,
		},
		{
			name:      "multiple works for different clusters",
			bindingID: "test-id-4",
			binding:   createBinding("binding-4", "test-id-4", []string{"member1", "member2"}),
			works: []*workv1alpha1.Work{
				createWork("work-4-1", "karmada-es-member1", "test-id-4", true, true),
				createWork("work-4-2", "karmada-es-member2", "test-id-4", true, true),
			},
			expectedError:   false,
			expectedStatus:  metav1.ConditionTrue,
			expectedApplied: true,
		},
		{
			name:      "work without status",
			bindingID: "test-id-5",
			binding:   createBinding("binding-5", "test-id-5", []string{"member1"}),
			works: []*workv1alpha1.Work{
				createWork("work-5", "karmada-es-member1", "test-id-5", true, false),
			},
			expectedError:   false,
			expectedStatus:  metav1.ConditionFalse,
			expectedApplied: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := []client.Object{tt.binding}
			for _, work := range tt.works {
				objects = append(objects, work)
			}

			c := fake.NewClientBuilder().
				WithScheme(scheme).
				WithIndex(
					&workv1alpha1.Work{},
					indexregistry.WorkIndexByLabelResourceBindingID,
					indexregistry.GenLabelIndexerFunc(workv1alpha2.ResourceBindingPermanentIDLabel),
				).
				WithObjects(objects...).
				WithStatusSubresource(tt.binding).
				Build()

			recorder := record.NewFakeRecorder(10)

			err := AggregateResourceBindingWorkStatus(context.TODO(), c, tt.binding, recorder)
			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			// Verify updated binding
			updatedBinding := &workv1alpha2.ResourceBinding{}
			err = c.Get(context.TODO(), client.ObjectKey{Namespace: tt.binding.Namespace, Name: tt.binding.Name}, updatedBinding)
			assert.NoError(t, err)

			// Verify conditions
			fullyAppliedCond := meta.FindStatusCondition(updatedBinding.Status.Conditions, workv1alpha2.FullyApplied)
			if assert.NotNil(t, fullyAppliedCond) {
				assert.Equal(t, tt.expectedStatus, fullyAppliedCond.Status)
			}

			// Verify aggregated status
			if len(tt.works) > 0 {
				assert.Len(t, updatedBinding.Status.AggregatedStatus, len(tt.works))
				for _, status := range updatedBinding.Status.AggregatedStatus {
					assert.Equal(t, tt.expectedApplied, status.Applied)
				}
			}

			// Verify events if expected
			if tt.expectedEvent != "" {
				select {
				case event := <-recorder.Events:
					assert.Contains(t, event, tt.expectedEvent)
				default:
					t.Error("Expected event not received")
				}
			}
		})
	}
}

func TestAggregateClusterResourceBindingWorkStatus(t *testing.T) {
	scheme := setupScheme()

	// Helper function to create cluster binding
	createClusterBinding := func(name, bindingID string, clusters []string) *workv1alpha2.ClusterResourceBinding {
		return &workv1alpha2.ClusterResourceBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					workv1alpha2.ClusterResourceBindingPermanentIDLabel: bindingID,
				},
			},
			Spec: workv1alpha2.ResourceBindingSpec{
				Resource: workv1alpha2.ObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "test-deployment",
				},
				Clusters: func() []workv1alpha2.TargetCluster {
					var targetClusters []workv1alpha2.TargetCluster
					for _, cluster := range clusters {
						targetClusters = append(targetClusters, workv1alpha2.TargetCluster{Name: cluster})
					}
					return targetClusters
				}(),
			},
		}
	}

	// Helper function to create work
	createWork := func(name, namespace, bindingID string, applied bool, withStatus bool) *workv1alpha1.Work {
		workObj := &workv1alpha1.Work{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
				Labels: map[string]string{
					workv1alpha2.ClusterResourceBindingPermanentIDLabel: bindingID,
				},
			},
			Spec: workv1alpha1.WorkSpec{
				Workload: workv1alpha1.WorkloadTemplate{
					Manifests: []workv1alpha1.Manifest{
						{
							RawExtension: runtime.RawExtension{
								Raw: []byte(`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"test-deployment"}}`),
							},
						},
					},
				},
			},
		}

		if withStatus {
			status := metav1.ConditionTrue
			if !applied {
				status = metav1.ConditionFalse
			}
			workObj.Status = workv1alpha1.WorkStatus{
				Conditions: []metav1.Condition{
					{
						Type:    workv1alpha1.WorkApplied,
						Status:  status,
						Message: "test message",
					},
				},
				ManifestStatuses: []workv1alpha1.ManifestStatus{
					{
						Identifier: workv1alpha1.ResourceIdentifier{
							Group:   "apps",
							Version: "v1",
							Kind:    "Deployment",
							Name:    "test-deployment",
						},
						Status: &runtime.RawExtension{Raw: []byte(`{"replicas": 3}`)},
						Health: "Healthy",
					},
				},
			}
		}
		return workObj
	}

	tests := []struct {
		name            string
		bindingID       string
		binding         *workv1alpha2.ClusterResourceBinding
		works           []*workv1alpha1.Work
		expectedError   bool
		expectedStatus  metav1.ConditionStatus
		expectedApplied bool
		expectedEvent   string
	}{
		{
			name:      "successful single work aggregation",
			bindingID: "test-id-1",
			binding:   createClusterBinding("binding-1", "test-id-1", []string{"member1"}),
			works: []*workv1alpha1.Work{
				createWork("work-1", "karmada-es-member1", "test-id-1", true, true),
			},
			expectedError:   false,
			expectedStatus:  metav1.ConditionTrue,
			expectedApplied: true,
			expectedEvent:   "Update ClusterResourceBinding(binding-1) with AggregatedStatus successfully",
		},
		{
			name:            "no works found",
			bindingID:       "test-id-2",
			binding:         createClusterBinding("binding-2", "test-id-2", []string{"member1"}),
			works:           []*workv1alpha1.Work{},
			expectedError:   false,
			expectedStatus:  metav1.ConditionFalse,
			expectedApplied: false,
		},
		{
			name:      "work not applied",
			bindingID: "test-id-3",
			binding:   createClusterBinding("binding-3", "test-id-3", []string{"member1"}),
			works: []*workv1alpha1.Work{
				createWork("work-3", "karmada-es-member1", "test-id-3", false, true),
			},
			expectedError:   false,
			expectedStatus:  metav1.ConditionFalse,
			expectedApplied: false,
		},
		{
			name:      "multiple clusters with mixed status",
			bindingID: "test-id-4",
			binding:   createClusterBinding("binding-4", "test-id-4", []string{"member1", "member2"}),
			works: []*workv1alpha1.Work{
				createWork("work-4-1", "karmada-es-member1", "test-id-4", true, true),
				createWork("work-4-2", "karmada-es-member2", "test-id-4", false, true),
			},
			expectedError:   false,
			expectedStatus:  metav1.ConditionFalse,
			expectedApplied: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := []client.Object{tt.binding}
			for _, work := range tt.works {
				objects = append(objects, work)
			}

			c := fake.NewClientBuilder().
				WithScheme(scheme).
				WithIndex(
					&workv1alpha1.Work{},
					indexregistry.WorkIndexByLabelClusterResourceBindingID,
					indexregistry.GenLabelIndexerFunc(workv1alpha2.ClusterResourceBindingPermanentIDLabel),
				).
				WithObjects(objects...).
				WithStatusSubresource(tt.binding).
				Build()

			recorder := record.NewFakeRecorder(10)

			err := AggregateClusterResourceBindingWorkStatus(context.TODO(), c, tt.binding, recorder)
			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			// Verify updated binding
			updatedBinding := &workv1alpha2.ClusterResourceBinding{}
			err = c.Get(context.TODO(), client.ObjectKey{Name: tt.binding.Name}, updatedBinding)
			assert.NoError(t, err)

			// Verify conditions
			fullyAppliedCond := meta.FindStatusCondition(updatedBinding.Status.Conditions, workv1alpha2.FullyApplied)
			if assert.NotNil(t, fullyAppliedCond) {
				assert.Equal(t, tt.expectedStatus, fullyAppliedCond.Status)
			}

			// Verify aggregated status
			if len(tt.works) > 0 {
				assert.Len(t, updatedBinding.Status.AggregatedStatus, len(tt.works))
				// For multiple clusters case, verify specific cluster status
				for _, status := range updatedBinding.Status.AggregatedStatus {
					if strings.Contains(status.ClusterName, "member2") {
						assert.Equal(t, tt.expectedApplied, status.Applied)
					}
				}
			}

			// Verify events
			if tt.expectedEvent != "" {
				select {
				case event := <-recorder.Events:
					assert.Contains(t, event, tt.expectedEvent)
				default:
					t.Error("Expected event not received")
				}
			}
		})
	}
}

func TestBuildStatusRawExtension(t *testing.T) {
	// Use a fixed time for deterministic tests
	fixedTime := metav1.NewTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))

	tests := []struct {
		name        string
		status      interface{}
		wantErr     bool
		expectedRaw string
	}{
		{
			name: "simple pod status",
			status: corev1.PodStatus{
				Phase: corev1.PodRunning,
			},
			wantErr:     false,
			expectedRaw: `{"phase":"Running"}`,
		},
		{
			name: "complex pod status",
			status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{
					{
						Type:               corev1.PodReady,
						Status:             corev1.ConditionTrue,
						LastProbeTime:      fixedTime,
						LastTransitionTime: fixedTime,
					},
				},
			},
			wantErr:     false,
			expectedRaw: `{"phase":"Running","conditions":[{"type":"Ready","status":"True","lastProbeTime":"2024-01-01T00:00:00Z","lastTransitionTime":"2024-01-01T00:00:00Z"}]}`,
		},
		{
			name:        "nil status",
			status:      nil,
			wantErr:     false,
			expectedRaw: `null`,
		},
		{
			name:        "unmarshallable status",
			status:      make(chan int),
			wantErr:     true,
			expectedRaw: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildStatusRawExtension(tt.status)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, got)
			assert.JSONEq(t, tt.expectedRaw, string(got.Raw))
		})
	}
}

func TestAssembleWorkStatus(t *testing.T) {
	// Common test data
	statusRaw := []byte(`{"replicas": 3}`)
	baseManifest := workv1alpha1.Manifest{
		RawExtension: runtime.RawExtension{
			Raw: []byte(`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"test-deployment","namespace":"test-ns"}}`),
		},
	}

	baseObjRef := workv1alpha2.ObjectReference{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Namespace:  "test-ns",
		Name:       "test-deployment",
	}

	// Helper function to create a basic work
	createWork := func(name string, manifest workv1alpha1.Manifest) workv1alpha1.Work {
		return workv1alpha1.Work{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "karmada-es-member1",
			},
			Spec: workv1alpha1.WorkSpec{
				Workload: workv1alpha1.WorkloadTemplate{
					Manifests: []workv1alpha1.Manifest{manifest},
				},
			},
		}
	}

	tests := []struct {
		name          string
		works         []workv1alpha1.Work
		objRef        workv1alpha2.ObjectReference
		expectedItems []workv1alpha2.AggregatedStatusItem
		wantErr       bool
		errorMessage  string
	}{
		{
			name:          "empty work list",
			works:         []workv1alpha1.Work{},
			objRef:        baseObjRef,
			expectedItems: []workv1alpha2.AggregatedStatusItem{},
			wantErr:       false,
		},
		{
			name: "work with invalid manifest",
			works: []workv1alpha1.Work{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-work",
						Namespace: "karmada-es-member1",
					},
					Spec: workv1alpha1.WorkSpec{
						Workload: workv1alpha1.WorkloadTemplate{
							Manifests: []workv1alpha1.Manifest{
								{
									RawExtension: runtime.RawExtension{Raw: []byte(`invalid json`)},
								},
							},
						},
					},
				},
			},
			objRef:  baseObjRef,
			wantErr: true,
		},
		{
			name: "work being deleted",
			works: []workv1alpha1.Work{
				func() workv1alpha1.Work {
					w := createWork("test-work", baseManifest)
					now := metav1.NewTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
					w.DeletionTimestamp = &now
					return w
				}(),
			},
			objRef:        baseObjRef,
			expectedItems: []workv1alpha2.AggregatedStatusItem{},
			wantErr:       false,
		},
		{
			name: "work applied successfully with health status",
			works: []workv1alpha1.Work{
				func() workv1alpha1.Work {
					w := createWork("test-work", baseManifest)
					w.Status = workv1alpha1.WorkStatus{
						Conditions: []metav1.Condition{
							{
								Type:   workv1alpha1.WorkApplied,
								Status: metav1.ConditionTrue,
							},
						},
						ManifestStatuses: []workv1alpha1.ManifestStatus{
							{
								Identifier: workv1alpha1.ResourceIdentifier{
									Ordinal:   0,
									Group:     "apps",
									Version:   "v1",
									Kind:      "Deployment",
									Namespace: "test-ns",
									Name:      "test-deployment",
								},
								Status: &runtime.RawExtension{Raw: statusRaw},
								Health: "Healthy",
							},
						},
					}
					return w
				}(),
			},
			objRef: baseObjRef,
			expectedItems: []workv1alpha2.AggregatedStatusItem{
				{
					ClusterName: "member1",
					Status:      &runtime.RawExtension{Raw: statusRaw},
					Applied:     true,
					Health:      workv1alpha2.ResourceHealthy,
				},
			},
			wantErr: false,
		},
		{
			name: "work not applied with error message",
			works: []workv1alpha1.Work{
				func() workv1alpha1.Work {
					w := createWork("test-work", baseManifest)
					w.Status = workv1alpha1.WorkStatus{
						Conditions: []metav1.Condition{
							{
								Type:    workv1alpha1.WorkApplied,
								Status:  metav1.ConditionFalse,
								Message: "Failed to apply",
							},
						},
					}
					return w
				}(),
			},
			objRef: baseObjRef,
			expectedItems: []workv1alpha2.AggregatedStatusItem{
				{
					ClusterName:    "member1",
					Applied:        false,
					AppliedMessage: "Failed to apply",
					Health:         workv1alpha2.ResourceUnknown,
				},
			},
			wantErr: false,
		},
		{
			name: "work with unknown condition status",
			works: []workv1alpha1.Work{
				func() workv1alpha1.Work {
					w := createWork("test-work", baseManifest)
					w.Status = workv1alpha1.WorkStatus{
						Conditions: []metav1.Condition{
							{
								Type:    workv1alpha1.WorkApplied,
								Status:  metav1.ConditionUnknown,
								Message: "Status unknown",
							},
						},
					}
					return w
				}(),
			},
			objRef: baseObjRef,
			expectedItems: []workv1alpha2.AggregatedStatusItem{
				{
					ClusterName:    "member1",
					Applied:        false,
					AppliedMessage: "Status unknown",
					Health:         workv1alpha2.ResourceUnknown,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := assembleWorkStatus(tt.works, tt.objRef)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorMessage != "" {
					assert.Contains(t, err.Error(), tt.errorMessage)
				}
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedItems, got)
		})
	}
}

func TestGenerateFullyAppliedCondition(t *testing.T) {
	spec := workv1alpha2.ResourceBindingSpec{
		Clusters: []workv1alpha2.TargetCluster{
			{Name: "cluster1"},
			{Name: "cluster2"},
		},
	}
	statuses := []workv1alpha2.AggregatedStatusItem{
		{ClusterName: "cluster1", Applied: true, Health: workv1alpha2.ResourceHealthy},
		{ClusterName: "cluster2", Applied: true, Health: workv1alpha2.ResourceUnknown},
	}

	expectedTrue := metav1.ConditionTrue
	expectedFalse := metav1.ConditionFalse

	resultTrue := generateFullyAppliedCondition(spec, statuses)
	assert.Equal(t, expectedTrue, resultTrue.Status, "generateFullyAppliedCondition with fully applied statuses")

	resultFalse := generateFullyAppliedCondition(spec, statuses[:1])
	assert.Equal(t, expectedFalse, resultFalse.Status, "generateFullyAppliedCondition with partially applied statuses")
}

func TestWorksFullyApplied(t *testing.T) {
	type args struct {
		aggregatedStatuses []workv1alpha2.AggregatedStatusItem
		targetClusters     sets.Set[string]
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "no cluster",
			args: args{
				aggregatedStatuses: []workv1alpha2.AggregatedStatusItem{
					{
						ClusterName: "member1",
						Applied:     true,
					},
				},
				targetClusters: nil,
			},
			want: false,
		},
		{
			name: "no aggregatedStatuses",
			args: args{
				aggregatedStatuses: nil,
				targetClusters:     sets.New("member1"),
			},
			want: false,
		},
		{
			name: "cluster size is not equal to aggregatedStatuses",
			args: args{
				aggregatedStatuses: []workv1alpha2.AggregatedStatusItem{
					{
						ClusterName: "member1",
						Applied:     true,
					},
				},
				targetClusters: sets.New("member1", "member2"),
			},
			want: false,
		},
		{
			name: "aggregatedStatuses is equal to clusterNames and all applied",
			args: args{
				aggregatedStatuses: []workv1alpha2.AggregatedStatusItem{
					{
						ClusterName: "member1",
						Applied:     true,
					},
					{
						ClusterName: "member2",
						Applied:     true,
					},
				},
				targetClusters: sets.New("member1", "member2"),
			},
			want: true,
		},
		{
			name: "aggregatedStatuses is equal to clusterNames but not all applied",
			args: args{
				aggregatedStatuses: []workv1alpha2.AggregatedStatusItem{
					{
						ClusterName: "member1",
						Applied:     true,
					},
					{
						ClusterName: "member2",
						Applied:     false,
					},
				},
				targetClusters: sets.New("member1", "member2"),
			},
			want: false,
		},
		{
			name: "target clusters not match expected status",
			args: args{
				aggregatedStatuses: []workv1alpha2.AggregatedStatusItem{
					{
						ClusterName: "member1",
						Applied:     true,
					},
				},
				targetClusters: sets.New("member2"),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := worksFullyApplied(tt.args.aggregatedStatuses, tt.args.targetClusters)
			assert.Equal(t, tt.want, got, "worksFullyApplied() result")
		})
	}
}

func TestGetManifestIndex(t *testing.T) {
	manifest1 := workv1alpha1.Manifest{
		RawExtension: runtime.RawExtension{
			Raw: []byte(`{"apiVersion":"v1","kind":"Service","metadata":{"name":"test-service","namespace":"test-namespace"}}`),
		},
	}
	manifest2 := workv1alpha1.Manifest{
		RawExtension: runtime.RawExtension{
			Raw: []byte(`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"test-deployment","namespace":"test-namespace"}}`),
		},
	}
	manifests := []workv1alpha1.Manifest{manifest1, manifest2}

	service := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"name":      "test-service",
				"namespace": "test-namespace",
			},
		},
	}

	deployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test-deployment",
				"namespace": "test-namespace",
			},
		},
	}

	t.Run("Service", func(t *testing.T) {
		manifestRef := ManifestReference{APIVersion: service.GetAPIVersion(), Kind: service.GetKind(),
			Namespace: service.GetNamespace(), Name: service.GetName()}
		index, err := GetManifestIndex(manifests, &manifestRef)
		assert.NoError(t, err)
		assert.Equal(t, 0, index, "Service manifest index")
	})

	t.Run("Deployment", func(t *testing.T) {
		manifestRef := ManifestReference{APIVersion: deployment.GetAPIVersion(), Kind: deployment.GetKind(),
			Namespace: deployment.GetNamespace(), Name: deployment.GetName()}
		index, err := GetManifestIndex(manifests, &manifestRef)
		assert.NoError(t, err)
		assert.Equal(t, 1, index, "Deployment manifest index")
	})

	t.Run("No match", func(t *testing.T) {
		_, err := GetManifestIndex(manifests, &ManifestReference{})
		assert.Error(t, err, "Expected error for no match")
	})
}

func TestEqualIdentifier(t *testing.T) {
	testCases := []struct {
		name           string
		target         *workv1alpha1.ResourceIdentifier
		ordinal        int
		workload       *ManifestReference
		expectedOutput bool
	}{
		{
			name: "identifiers are equal",
			target: &workv1alpha1.ResourceIdentifier{
				Ordinal:   0,
				Group:     "apps",
				Version:   "v1",
				Kind:      "Deployment",
				Namespace: "default",
				Name:      "test-deployment",
			},
			ordinal: 0,
			workload: &ManifestReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Namespace:  "default",
				Name:       "test-deployment",
			},
			expectedOutput: true,
		},
		{
			name: "identifiers are not equal",
			target: &workv1alpha1.ResourceIdentifier{
				Ordinal:   1,
				Group:     "apps",
				Version:   "v1",
				Kind:      "Deployment",
				Namespace: "default",
				Name:      "test-deployment",
			},
			ordinal: 0,
			workload: &ManifestReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Namespace:  "default",
				Name:       "test-deployment",
			},
			expectedOutput: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			equal, err := equalIdentifier(tc.target, tc.ordinal, tc.workload)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedOutput, equal, "equalIdentifier() result")
		})
	}
}

func TestIsResourceApplied(t *testing.T) {
	// Create a WorkStatus struct with a WorkApplied condition set to True
	workStatus := &workv1alpha1.WorkStatus{
		Conditions: []metav1.Condition{
			{
				Type:   workv1alpha1.WorkApplied,
				Status: metav1.ConditionTrue,
			},
		},
	}

	// Call IsResourceApplied and assert that it returns true
	assert.True(t, IsResourceApplied(workStatus))
}

// Helper Functions

// setupScheme initializes a new scheme
func setupScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = workv1alpha1.Install(scheme)
	_ = workv1alpha2.Install(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	return scheme
}
