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

package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	clienttesting "k8s.io/client-go/testing"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/component-base/featuregate"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/features"
	karmadafake "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/fake"
	clusterlister "github.com/karmada-io/karmada/pkg/generated/listers/cluster/v1alpha1"
	workv1alpha2lister "github.com/karmada-io/karmada/pkg/generated/listers/work/v1alpha2"
	schedulercache "github.com/karmada-io/karmada/pkg/scheduler/cache"
	"github.com/karmada-io/karmada/pkg/scheduler/core"
	"github.com/karmada-io/karmada/pkg/scheduler/framework"
	internalqueue "github.com/karmada-io/karmada/pkg/scheduler/internal/queue"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/grpcconnection"
)

func setFeatureGateDuringTest(tb testing.TB, gate featuregate.FeatureGate, f featuregate.Feature, value bool) func() {
	originalValue := gate.Enabled(f)
	if err := gate.(featuregate.MutableFeatureGate).Set(fmt.Sprintf("%s=%v", f, value)); err != nil {
		tb.Errorf("error setting %s=%v: %v", f, value, err)
	}
	return func() {
		if err := gate.(featuregate.MutableFeatureGate).Set(fmt.Sprintf("%s=%v", f, originalValue)); err != nil {
			tb.Errorf("error restoring %s=%v: %v", f, originalValue, err)
		}
	}
}

func componentSchedulingAnnotations(t *testing.T, placement string, components []workv1alpha2.Component) map[string]string {
	t.Helper()
	hash, err := util.GenerateComponentRequirementsHash(components)
	assert.NoError(t, err)
	return map[string]string{
		util.PolicyPlacementAnnotation:                   placement,
		util.AcceptedComponentRequirementsHashAnnotation: hash,
	}
}

func TestDoSchedule(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		binding     any
		expectError bool
	}{
		{
			name:        "invalid key format",
			key:         "invalid/key/format",
			binding:     nil,
			expectError: true,
		},
		{
			name: "ResourceBinding scheduling",
			key:  "default/test-binding",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Placement: &policyv1alpha1.Placement{
						ClusterAffinity: &policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"cluster1"},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "ClusterResourceBinding scheduling",
			key:  "test-cluster-binding",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-binding",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Placement: &policyv1alpha1.Placement{
						ClusterAffinity: &policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"cluster1"},
						},
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := karmadafake.NewClientset()
			fakeRecorder := record.NewFakeRecorder(10)

			var bindingLister *fakeBindingLister
			var clusterBindingLister *fakeClusterBindingLister

			if rb, ok := tt.binding.(*workv1alpha2.ResourceBinding); ok {
				bindingLister = &fakeBindingLister{binding: rb}
				_, err := fakeClient.WorkV1alpha2().ResourceBindings(rb.Namespace).Create(context.TODO(), rb, metav1.CreateOptions{})
				assert.NoError(t, err)
			}
			if crb, ok := tt.binding.(*workv1alpha2.ClusterResourceBinding); ok {
				clusterBindingLister = &fakeClusterBindingLister{binding: crb}
				_, err := fakeClient.WorkV1alpha2().ClusterResourceBindings().Create(context.TODO(), crb, metav1.CreateOptions{})
				assert.NoError(t, err)
			}

			mockAlgo := &mockAlgorithm{
				scheduleFunc: func(_ context.Context, _ *workv1alpha2.ResourceBindingSpec, _ *workv1alpha2.ResourceBindingStatus, _ *core.ScheduleAlgorithmOption) (core.ScheduleResult, error) {
					return core.ScheduleResult{
						SuggestedClusters: []workv1alpha2.TargetCluster{
							{Name: "cluster1", Replicas: 1},
						},
					}, nil
				},
			}

			s := &Scheduler{
				KarmadaClient:        fakeClient,
				eventRecorder:        fakeRecorder,
				bindingLister:        bindingLister,
				clusterBindingLister: clusterBindingLister,
				Algorithm:            mockAlgo,
			}

			err := s.doSchedule(tt.key)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if !tt.expectError {
				if rb, ok := tt.binding.(*workv1alpha2.ResourceBinding); ok {
					updated, err := fakeClient.WorkV1alpha2().ResourceBindings(rb.Namespace).Get(context.TODO(), rb.Name, metav1.GetOptions{})
					assert.NoError(t, err)
					assert.NotNil(t, updated.Spec.Clusters)
					assert.Len(t, updated.Spec.Clusters, 1)
					assert.Equal(t, "cluster1", updated.Spec.Clusters[0].Name)
				}
				if crb, ok := tt.binding.(*workv1alpha2.ClusterResourceBinding); ok {
					updated, err := fakeClient.WorkV1alpha2().ClusterResourceBindings().Get(context.TODO(), crb.Name, metav1.GetOptions{})
					assert.NoError(t, err)
					assert.NotNil(t, updated.Spec.Clusters)
					assert.Len(t, updated.Spec.Clusters, 1)
					assert.Equal(t, "cluster1", updated.Spec.Clusters[0].Name)
				}
			}
		})
	}
}

func TestDoScheduleSkipsStaleQueuedBindingsOutsideSchedulerOwnership(t *testing.T) {
	suspended := true
	tests := []struct {
		name           string
		resourceScoped bool
		mutate         func(*workv1alpha2.ResourceBindingSpec)
	}{
		{name: "ResourceBinding custom scheduler", resourceScoped: true, mutate: func(spec *workv1alpha2.ResourceBindingSpec) { spec.SchedulerName = "custom-scheduler" }},
		{name: "ResourceBinding scheduling suspended", resourceScoped: true, mutate: func(spec *workv1alpha2.ResourceBindingSpec) {
			spec.Suspension = &workv1alpha2.Suspension{Scheduling: &suspended}
		}},
		{name: "ClusterResourceBinding custom scheduler", mutate: func(spec *workv1alpha2.ResourceBindingSpec) { spec.SchedulerName = "custom-scheduler" }},
		{name: "ClusterResourceBinding scheduling suspended", mutate: func(spec *workv1alpha2.ResourceBindingSpec) {
			spec.Suspension = &workv1alpha2.Suspension{Scheduling: &suspended}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := workv1alpha2.ResourceBindingSpec{Placement: &policyv1alpha1.Placement{}}
			tt.mutate(&spec)
			algorithmCalls := 0
			algorithm := &mockAlgorithm{scheduleFunc: func(_ context.Context, _ *workv1alpha2.ResourceBindingSpec, _ *workv1alpha2.ResourceBindingStatus, _ *core.ScheduleAlgorithmOption) (core.ScheduleResult, error) {
				algorithmCalls++
				return core.ScheduleResult{}, nil
			}}

			if tt.resourceScoped {
				binding := &workv1alpha2.ResourceBinding{ObjectMeta: metav1.ObjectMeta{Name: "rb", Namespace: "default"}, Spec: spec}
				client := karmadafake.NewClientset(binding)
				s := &Scheduler{KarmadaClient: client, bindingLister: &fakeBindingLister{binding: binding}, Algorithm: algorithm}
				assert.NoError(t, s.doScheduleBinding(binding.Namespace, binding.Name))
				assert.Empty(t, client.Actions())
			} else {
				binding := &workv1alpha2.ClusterResourceBinding{ObjectMeta: metav1.ObjectMeta{Name: "crb"}, Spec: spec}
				client := karmadafake.NewClientset(binding)
				s := &Scheduler{KarmadaClient: client, clusterBindingLister: &fakeClusterBindingLister{binding: binding}, Algorithm: algorithm}
				assert.NoError(t, s.doScheduleClusterBinding(binding.Name))
				assert.Empty(t, client.Actions())
			}
			assert.Zero(t, algorithmCalls)
		})
	}
}

func TestDoScheduleBinding(t *testing.T) {
	tests := []struct {
		name             string
		binding          *workv1alpha2.ResourceBinding
		expectSchedule   bool
		expectError      bool
		expectedClusters []workv1alpha2.TargetCluster
		expectedEvent    string
	}{
		{
			name: "binding with changed placement",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding-1",
					Namespace: "default",
					Annotations: map[string]string{
						util.PolicyPlacementAnnotation: `{"clusterAffinity":{"clusterNames":["cluster1"]}}`,
					},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Placement: &policyv1alpha1.Placement{
						ClusterAffinity: &policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"cluster1", "cluster2"},
						},
					},
				},
			},
			expectSchedule: true,
			expectError:    false,
			expectedClusters: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 1},
				{Name: "cluster2", Replicas: 1},
			},
			expectedEvent: "Normal ScheduleBindingSucceed",
		},
		{
			name: "binding with replicas changed",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding-2",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Replicas: 2,
					Placement: &policyv1alpha1.Placement{
						ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
							ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDivided,
						},
					},
				},
				Status: workv1alpha2.ResourceBindingStatus{
					SchedulerObservedGeneration: 1,
				},
			},
			expectSchedule: true,
			expectError:    false,
			expectedClusters: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 1},
				{Name: "cluster2", Replicas: 1},
			},
			expectedEvent: "Normal ScheduleBindingSucceed",
		},
		{
			name: "binding with reschedule triggered",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding-3",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					RescheduleTriggeredAt: &metav1.Time{Time: time.Now()},
					Placement:             &policyv1alpha1.Placement{},
				},
			},
			expectSchedule: true,
			expectError:    false,
			expectedClusters: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 1},
			},
			expectedEvent: "Normal ScheduleBindingSucceed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := karmadafake.NewClientset(tt.binding)
			fakeRecorder := record.NewFakeRecorder(10)
			mockAlgorithm := &mockAlgorithm{
				scheduleFunc: func(context.Context, *workv1alpha2.ResourceBindingSpec, *workv1alpha2.ResourceBindingStatus, *core.ScheduleAlgorithmOption) (core.ScheduleResult, error) {
					return core.ScheduleResult{SuggestedClusters: tt.expectedClusters}, nil
				},
			}

			s := &Scheduler{
				KarmadaClient: fakeClient,
				bindingLister: &fakeBindingLister{binding: tt.binding},
				eventRecorder: fakeRecorder,
				Algorithm:     mockAlgorithm,
			}

			err := s.doScheduleBinding(tt.binding.Namespace, tt.binding.Name)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			updatedBinding, err := fakeClient.WorkV1alpha2().ResourceBindings(tt.binding.Namespace).Get(context.TODO(), tt.binding.Name, metav1.GetOptions{})
			assert.NoError(t, err)

			if tt.expectSchedule {
				assert.Equal(t, tt.expectedClusters, updatedBinding.Spec.Clusters)
				assert.NotEqual(t, tt.binding.Spec.Clusters, updatedBinding.Spec.Clusters)
			} else {
				assert.Equal(t, tt.binding.Spec.Clusters, updatedBinding.Spec.Clusters)
			}

			// Check for expected events
			select {
			case event := <-fakeRecorder.Events:
				assert.Contains(t, event, tt.expectedEvent)
			default:
				t.Errorf("Expected an event to be recorded")
			}
		})
	}
}

func TestDoScheduleClusterBinding(t *testing.T) {
	tests := []struct {
		name             string
		binding          *workv1alpha2.ClusterResourceBinding
		expectSchedule   bool
		expectError      bool
		expectedClusters []workv1alpha2.TargetCluster
		expectedEvent    string
	}{
		{
			name: "cluster binding with changed placement",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-binding-1",
					Annotations: map[string]string{
						util.PolicyPlacementAnnotation: `{"clusterAffinity":{"clusterNames":["cluster1"]}}`,
					},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Placement: &policyv1alpha1.Placement{
						ClusterAffinity: &policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"cluster1", "cluster2"},
						},
					},
				},
			},
			expectSchedule: true,
			expectError:    false,
			expectedClusters: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 1},
				{Name: "cluster2", Replicas: 1},
			},
			expectedEvent: "Normal ScheduleBindingSucceed",
		},
		{
			name: "cluster binding with replicas changed",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-binding-2",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Replicas: 2,
					Placement: &policyv1alpha1.Placement{
						ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{
							ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDivided,
						},
					},
				},
				Status: workv1alpha2.ResourceBindingStatus{
					SchedulerObservedGeneration: 1,
				},
			},
			expectSchedule: true,
			expectError:    false,
			expectedClusters: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 1},
				{Name: "cluster2", Replicas: 1},
			},
			expectedEvent: "Normal ScheduleBindingSucceed",
		},
		{
			name: "cluster binding with reschedule triggered",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-binding-3",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					RescheduleTriggeredAt: &metav1.Time{Time: time.Now()},
					Placement:             &policyv1alpha1.Placement{},
				},
			},
			expectSchedule: true,
			expectError:    false,
			expectedClusters: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 1},
			},
			expectedEvent: "Normal ScheduleBindingSucceed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := karmadafake.NewClientset(tt.binding)
			fakeRecorder := record.NewFakeRecorder(10)
			mockAlgorithm := &mockAlgorithm{
				scheduleFunc: func(context.Context, *workv1alpha2.ResourceBindingSpec, *workv1alpha2.ResourceBindingStatus, *core.ScheduleAlgorithmOption) (core.ScheduleResult, error) {
					return core.ScheduleResult{SuggestedClusters: tt.expectedClusters}, nil
				},
			}

			s := &Scheduler{
				KarmadaClient:        fakeClient,
				clusterBindingLister: &fakeClusterBindingLister{binding: tt.binding},
				eventRecorder:        fakeRecorder,
				Algorithm:            mockAlgorithm,
			}

			err := s.doScheduleClusterBinding(tt.binding.Name)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			updatedBinding, err := fakeClient.WorkV1alpha2().ClusterResourceBindings().Get(context.TODO(), tt.binding.Name, metav1.GetOptions{})
			assert.NoError(t, err)

			if tt.expectSchedule {
				assert.Equal(t, tt.expectedClusters, updatedBinding.Spec.Clusters)
				assert.NotEqual(t, tt.binding.Spec.Clusters, updatedBinding.Spec.Clusters)
			} else {
				assert.Equal(t, tt.binding.Spec.Clusters, updatedBinding.Spec.Clusters)
			}

			// Check for expected events
			select {
			case event := <-fakeRecorder.Events:
				assert.Contains(t, event, tt.expectedEvent)
			default:
				t.Errorf("Expected an event to be recorded")
			}
		})
	}
}

func TestScheduleResourceBindingWithClusterAffinity(t *testing.T) {
	tests := []struct {
		name           string
		binding        *workv1alpha2.ResourceBinding
		scheduleResult core.ScheduleResult
		scheduleError  error
		expectError    bool
		expectedPatch  string
		expectedEvent  string
	}{
		{
			name: "successful scheduling",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Placement: &policyv1alpha1.Placement{
						ClusterAffinity: &policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"cluster1"},
						},
					},
				},
			},
			scheduleResult: core.ScheduleResult{
				SuggestedClusters: []workv1alpha2.TargetCluster{
					{Name: "cluster1", Replicas: 1},
				},
			},
			expectError:   false,
			expectedPatch: `{"metadata":{"annotations":{"policy.karmada.io/applied-placement":"{\"clusterAffinity\":{\"clusterNames\":[\"cluster1\"]}}"}},"spec":{"clusters":[{"name":"cluster1","replicas":1}]}}`,
			expectedEvent: "Normal ScheduleBindingSucceed Binding has been scheduled successfully.",
		},
		{
			name: "scheduling error",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding-error",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Placement: &policyv1alpha1.Placement{
						ClusterAffinity: &policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"cluster1"},
						},
					},
				},
			},
			scheduleResult: core.ScheduleResult{},
			scheduleError:  errors.New("scheduling error"),
			expectError:    true,
			expectedEvent:  "Warning ScheduleBindingFailed scheduling error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := karmadafake.NewClientset(tt.binding)
			fakeRecorder := record.NewFakeRecorder(10)
			mockAlgorithm := &mockAlgorithm{
				scheduleFunc: func(context.Context, *workv1alpha2.ResourceBindingSpec, *workv1alpha2.ResourceBindingStatus, *core.ScheduleAlgorithmOption) (core.ScheduleResult, error) {
					return tt.scheduleResult, tt.scheduleError
				},
			}
			s := &Scheduler{
				KarmadaClient: fakeClient,
				eventRecorder: fakeRecorder,
				Algorithm:     mockAlgorithm,
			}

			err := s.scheduleResourceBindingWithClusterAffinity(tt.binding)

			if (err != nil) != tt.expectError {
				t.Errorf("scheduleResourceBindingWithClusterAffinity() error = %v, expectError %v", err, tt.expectError)
			}

			actions := fakeClient.Actions()
			patchActions := filterPatchActions(actions)

			if tt.expectError {
				assert.Empty(t, patchActions, "Expected no patch actions for error case")
			} else {
				assert.Len(t, patchActions, 1, "Expected one patch action")
				if len(patchActions) > 0 {
					actualPatch := string(patchActions[0].GetPatch())
					assert.JSONEq(t, tt.expectedPatch, actualPatch, "Patch does not match expected")
				}
			}

			// Check if an event was recorded
			select {
			case event := <-fakeRecorder.Events:
				assert.Contains(t, event, tt.expectedEvent, "Event does not match expected")
			default:
				t.Errorf("Expected an event to be recorded")
			}
		})
	}
}

func TestScheduleResourceBindingWithClusterAffinities(t *testing.T) {
	tests := []struct {
		name                  string
		binding               *workv1alpha2.ResourceBinding
		scheduleResults       []core.ScheduleResult
		scheduleErrors        []error
		expectError           bool
		expectedPatches       []string
		expectedEvent         string
		expectedAffinityCalls []string
	}{
		{
			name: "successful scheduling with first affinity",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Placement: &policyv1alpha1.Placement{
						ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
							{
								AffinityName: "affinity1",
								ClusterAffinity: policyv1alpha1.ClusterAffinity{
									ClusterNames: []string{"cluster1"},
								},
							},
							{
								AffinityName: "affinity2",
								ClusterAffinity: policyv1alpha1.ClusterAffinity{
									ClusterNames: []string{"cluster2"},
								},
							},
						},
					},
				},
			},
			scheduleResults: []core.ScheduleResult{
				{
					SuggestedClusters: []workv1alpha2.TargetCluster{
						{Name: "cluster1", Replicas: 1},
					},
				},
			},
			scheduleErrors: []error{nil},
			expectError:    false,
			expectedPatches: []string{
				`{"metadata":{"annotations":{"policy.karmada.io/applied-placement":"{\"clusterAffinities\":[{\"affinityName\":\"affinity1\",\"clusterNames\":[\"cluster1\"]},{\"affinityName\":\"affinity2\",\"clusterNames\":[\"cluster2\"]}]}"}},"spec":{"clusters":[{"name":"cluster1","replicas":1}]}}`,
				`{"status":{"schedulerObservingAffinityName":"affinity1"}}`,
			},
			expectedEvent: fmt.Sprintf("Normal ScheduleBindingSucceed %s Result: {cluster1:1}", successfulSchedulingMessage),
		},
		{
			name: "explicit rescheduling restarts from first affinity",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding-reschedule",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					RescheduleTriggeredAt: &metav1.Time{Time: time.Unix(2, 0)},
					Placement: &policyv1alpha1.Placement{
						ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
							{
								AffinityName: "affinity1",
								ClusterAffinity: policyv1alpha1.ClusterAffinity{
									ClusterNames: []string{"cluster1"},
								},
							},
							{
								AffinityName: "affinity2",
								ClusterAffinity: policyv1alpha1.ClusterAffinity{
									ClusterNames: []string{"cluster2"},
								},
							},
						},
					},
				},
				Status: workv1alpha2.ResourceBindingStatus{
					LastScheduledTime:             &metav1.Time{Time: time.Unix(1, 0)},
					SchedulerObservedAffinityName: "affinity2",
				},
			},
			scheduleResults: []core.ScheduleResult{
				{
					SuggestedClusters: []workv1alpha2.TargetCluster{
						{Name: "cluster1", Replicas: 1},
					},
				},
			},
			scheduleErrors: []error{nil},
			expectError:    false,
			expectedPatches: []string{
				`{"metadata":{"annotations":{"policy.karmada.io/applied-placement":"{\"clusterAffinities\":[{\"affinityName\":\"affinity1\",\"clusterNames\":[\"cluster1\"]},{\"affinityName\":\"affinity2\",\"clusterNames\":[\"cluster2\"]}]}"}},"spec":{"clusters":[{"name":"cluster1","replicas":1}]}}`,
				`{"status":{"schedulerObservingAffinityName":"affinity1"}}`,
			},
			expectedEvent:         fmt.Sprintf("Normal ScheduleBindingSucceed %s Result: {cluster1:1}", successfulSchedulingMessage),
			expectedAffinityCalls: []string{"affinity1"},
		},
		{
			name: "without explicit rescheduling resumes from observed affinity",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding-resume",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Placement: &policyv1alpha1.Placement{
						ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
							{
								AffinityName: "affinity1",
								ClusterAffinity: policyv1alpha1.ClusterAffinity{
									ClusterNames: []string{"cluster1"},
								},
							},
							{
								AffinityName: "affinity2",
								ClusterAffinity: policyv1alpha1.ClusterAffinity{
									ClusterNames: []string{"cluster2"},
								},
							},
						},
					},
				},
				Status: workv1alpha2.ResourceBindingStatus{
					SchedulerObservedAffinityName: "affinity2",
				},
			},
			scheduleResults: []core.ScheduleResult{
				{},
				{
					SuggestedClusters: []workv1alpha2.TargetCluster{
						{Name: "cluster2", Replicas: 1},
					},
				},
			},
			scheduleErrors: []error{nil, nil},
			expectError:    false,
			expectedPatches: []string{
				`{"metadata":{"annotations":{"policy.karmada.io/applied-placement":"{\"clusterAffinities\":[{\"affinityName\":\"affinity1\",\"clusterNames\":[\"cluster1\"]},{\"affinityName\":\"affinity2\",\"clusterNames\":[\"cluster2\"]}]}"}},"spec":{"clusters":[{"name":"cluster2","replicas":1}]}}`,
			},
			expectedEvent:         fmt.Sprintf("Normal ScheduleBindingSucceed %s Result: {cluster2:1}", successfulSchedulingMessage),
			expectedAffinityCalls: []string{"affinity2"},
		},
		{
			name: "successful scheduling with second affinity",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding-2",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Placement: &policyv1alpha1.Placement{
						ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
							{
								AffinityName: "affinity1",
								ClusterAffinity: policyv1alpha1.ClusterAffinity{
									ClusterNames: []string{"cluster1"},
								},
							},
							{
								AffinityName: "affinity2",
								ClusterAffinity: policyv1alpha1.ClusterAffinity{
									ClusterNames: []string{"cluster2"},
								},
							},
						},
					},
				},
			},
			scheduleResults: []core.ScheduleResult{
				{},
				{
					SuggestedClusters: []workv1alpha2.TargetCluster{
						{Name: "cluster2", Replicas: 1},
					},
				},
			},
			scheduleErrors: []error{errors.New("first affinity failed"), nil},
			expectError:    false,
			expectedPatches: []string{
				`{"metadata":{"annotations":{"policy.karmada.io/applied-placement":"{\"clusterAffinities\":[{\"affinityName\":\"affinity1\",\"clusterNames\":[\"cluster1\"]},{\"affinityName\":\"affinity2\",\"clusterNames\":[\"cluster2\"]}]}"}},"spec":{"clusters":[{"name":"cluster2","replicas":1}]}}`,
				`{"status":{"schedulerObservingAffinityName":"affinity2"}}`,
			},
			expectedEvent: "Warning ScheduleBindingFailed failed to schedule ResourceBinding(default/test-binding-2) with clusterAffiliates index(0): first affinity failed",
		},
		{
			name: "all affinities fail",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding-fail",
					Namespace: "default",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Placement: &policyv1alpha1.Placement{
						ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
							{
								AffinityName: "affinity1",
								ClusterAffinity: policyv1alpha1.ClusterAffinity{
									ClusterNames: []string{"cluster1"},
								},
							},
							{
								AffinityName: "affinity2",
								ClusterAffinity: policyv1alpha1.ClusterAffinity{
									ClusterNames: []string{"cluster2"},
								},
							},
						},
					},
				},
			},
			scheduleResults: []core.ScheduleResult{{}, {}},
			scheduleErrors:  []error{errors.New("first affinity failed"), errors.New("second affinity failed")},
			expectError:     true,
			expectedPatches: []string{},
			expectedEvent:   "Warning ScheduleBindingFailed failed to schedule ResourceBinding",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := karmadafake.NewClientset(tt.binding)
			fakeRecorder := record.NewFakeRecorder(10)
			var affinityCalls []string
			mockAlgorithm := &mockAlgorithm{
				scheduleFunc: func(_ context.Context, spec *workv1alpha2.ResourceBindingSpec, status *workv1alpha2.ResourceBindingStatus, _ *core.ScheduleAlgorithmOption) (core.ScheduleResult, error) {
					affinityCalls = append(affinityCalls, status.SchedulerObservedAffinityName)
					index := getAffinityIndex(spec.Placement.ClusterAffinities, status.SchedulerObservedAffinityName)
					if index < len(tt.scheduleResults) {
						return tt.scheduleResults[index], tt.scheduleErrors[index]
					}
					return core.ScheduleResult{}, errors.New("unexpected call to Schedule")
				},
			}
			s := &Scheduler{
				KarmadaClient: fakeClient,
				eventRecorder: fakeRecorder,
				Algorithm:     mockAlgorithm,
			}

			err := s.scheduleResourceBindingWithClusterAffinities(tt.binding)

			if (err != nil) != tt.expectError {
				t.Errorf("scheduleResourceBindingWithClusterAffinities() error = %v, expectError %v", err, tt.expectError)
			}
			if tt.expectedAffinityCalls != nil {
				assert.Equal(t, tt.expectedAffinityCalls, affinityCalls, "Schedule affinity calls do not match expected")
			}

			actions := fakeClient.Actions()
			patchActions := filterPatchActions(actions)

			if tt.expectError {
				assert.Empty(t, patchActions, "Expected no patch actions for error case")
			} else {
				assert.Len(t, patchActions, len(tt.expectedPatches), "Expected %d patch actions", len(tt.expectedPatches))
				for i, expectedPatch := range tt.expectedPatches {
					actualPatch := string(patchActions[i].GetPatch())
					assert.JSONEq(t, expectedPatch, actualPatch, "Patch %d does not match expected", i+1)
				}
			}

			// Check if an event was recorded
			select {
			case event := <-fakeRecorder.Events:
				assert.Contains(t, event, tt.expectedEvent, "Event does not match expected")
			default:
				t.Errorf("Expected an event to be recorded")
			}
		})
	}
}

func TestPatchScheduleResultForResourceBinding(t *testing.T) {
	tests := []struct {
		name            string
		oldBinding      *workv1alpha2.ResourceBinding
		placement       string
		scheduleResult  []workv1alpha2.TargetCluster
		expectError     bool
		expectedBinding *workv1alpha2.ResourceBinding
	}{
		{
			name: "successful patch",
			oldBinding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "default",
				},
			},
			placement: "test-placement",
			scheduleResult: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 1},
			},
			expectError: false,
			expectedBinding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "default",
					Annotations: map[string]string{
						util.PolicyPlacementAnnotation: "test-placement",
					},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "cluster1", Replicas: 1},
					},
				},
			},
		},
		{
			name: "no changes",
			oldBinding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "default",
					Annotations: map[string]string{
						util.PolicyPlacementAnnotation: "test-placement",
					},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "cluster1", Replicas: 1},
					},
				},
			},
			placement: "test-placement",
			scheduleResult: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 1},
			},
			expectError: false,
			expectedBinding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "default",
					Annotations: map[string]string{
						util.PolicyPlacementAnnotation: "test-placement",
					},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "cluster1", Replicas: 1},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := schedulercache.NewCache(nil, nil, 0)
			s := &Scheduler{
				KarmadaClient:  karmadafake.NewClientset(tt.oldBinding),
				schedulerCache: cache,
			}

			err := s.patchScheduleResultForResourceBinding(tt.oldBinding, tt.placement, tt.scheduleResult)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				updatedBinding, err := s.KarmadaClient.WorkV1alpha2().ResourceBindings(tt.oldBinding.Namespace).Get(context.TODO(), tt.oldBinding.Name, metav1.GetOptions{})
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedBinding.Annotations, updatedBinding.Annotations)
				assert.Equal(t, tt.expectedBinding.Spec.Clusters, updatedBinding.Spec.Clusters)
			}
		})
	}
}

func TestUpdatesAssumptions(t *testing.T) {
	defer setFeatureGateDuringTest(t, features.FeatureGate, features.SchedulingOvercommitProtection, true)()
	multiTemplateComponents := []workv1alpha2.Component{
		{Name: "jobmanager", Replicas: 1},
		{Name: "taskmanager", Replicas: 2},
	}

	t.Run("component requirements baseline does not reserve an already running workload", func(t *testing.T) {
		defer setFeatureGateDuringTest(t, features.FeatureGate, features.MultiplePodTemplatesScheduling, true)()
		placement := &policyv1alpha1.Placement{SpreadConstraints: []policyv1alpha1.SpreadConstraint{{
			SpreadByField: policyv1alpha1.SpreadByFieldCluster, MinGroups: 1, MaxGroups: 1,
		}}}
		placementJSON, err := json.Marshal(placement)
		assert.NoError(t, err)
		clusters := []workv1alpha2.TargetCluster{{Name: "cluster1", Components: []workv1alpha2.TargetComponent{
			{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 2},
		}}}
		binding := &workv1alpha2.ResourceBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "test-binding", Namespace: "default", ResourceVersion: "7", Generation: 2, Annotations: map[string]string{util.PolicyPlacementAnnotation: string(placementJSON)}},
			Spec: workv1alpha2.ResourceBindingSpec{
				Resource: workv1alpha2.ObjectReference{Namespace: "work-ns"}, Placement: placement,
				Components: multiTemplateComponents, Clusters: clusters,
			},
			Status: workv1alpha2.ResourceBindingStatus{
				SchedulerObservedGeneration: 2,
				Conditions:                  []metav1.Condition{util.NewCondition(workv1alpha2.Scheduled, workv1alpha2.BindingReasonSuccess, successfulSchedulingMessage, metav1.ConditionTrue)},
			},
		}
		cache := schedulercache.NewCache(nil, nil, 0)
		client := karmadafake.NewClientset(binding)
		algorithm := &mockAlgorithm{scheduleFunc: func(_ context.Context, _ *workv1alpha2.ResourceBindingSpec, _ *workv1alpha2.ResourceBindingStatus, _ *core.ScheduleAlgorithmOption) (core.ScheduleResult, error) {
			t.Fatal("baseline backfill must not invoke the scheduling algorithm")
			return core.ScheduleResult{}, nil
		}}
		s := &Scheduler{
			KarmadaClient: client, schedulerCache: cache, bindingLister: &fakeBindingLister{binding: binding},
			clusterLister: testClusterLister(t, "cluster1"), Algorithm: algorithm,
		}

		assert.NoError(t, s.doScheduleBinding(binding.Namespace, binding.Name))
		assert.Empty(t, cache.AssigningResourceBindings().GetAssumedWorkloads("cluster1"))
		assertScalePatches(t, client.Actions(), true, "7")
	})

	t.Run("multi-template: writes full components on first scheduling", func(t *testing.T) {
		oldBinding := &workv1alpha2.ResourceBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "test-binding", Namespace: "default"},
			Spec: workv1alpha2.ResourceBindingSpec{
				Resource:   workv1alpha2.ObjectReference{Namespace: "work-ns"},
				Components: multiTemplateComponents,
				Clusters:   nil,
			},
		}

		cache := schedulercache.NewCache(nil, nil, 0)
		s := &Scheduler{
			KarmadaClient:  karmadafake.NewClientset(oldBinding),
			schedulerCache: cache,
		}

		err := s.patchScheduleResultForResourceBinding(oldBinding, "test-placement", []workv1alpha2.TargetCluster{
			{Name: "cluster1"},
		})
		assert.NoError(t, err)

		assumed := cache.AssigningResourceBindings().GetAssumedWorkloads("cluster1")
		assert.Len(t, assumed, 1)
		assert.Equal(t, "work-ns", assumed[0].Namespace)
		assert.Len(t, assumed[0].Components, 2)
		assert.Equal(t, int32(1), assumed[0].Components[0].Replicas)
		assert.Equal(t, int32(2), assumed[0].Components[1].Replicas)
	})

	t.Run("multi-template: always writes full components on reschedule", func(t *testing.T) {
		// Components scaled up; Option B always writes the full current components.
		scaledComponents := []workv1alpha2.Component{
			{Name: "jobmanager", Replicas: 1},
			{Name: "taskmanager", Replicas: 4},
		}
		oldBinding := &workv1alpha2.ResourceBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "test-binding", Namespace: "default"},
			Spec: workv1alpha2.ResourceBindingSpec{
				Resource:   workv1alpha2.ObjectReference{Namespace: "work-ns"},
				Components: scaledComponents,
				Clusters:   []workv1alpha2.TargetCluster{{Name: "cluster1"}},
			},
		}

		cache := schedulercache.NewCache(nil, nil, 0)
		s := &Scheduler{
			KarmadaClient:  karmadafake.NewClientset(oldBinding),
			schedulerCache: cache,
		}

		err := s.patchScheduleResultForResourceBinding(oldBinding, "test-placement", []workv1alpha2.TargetCluster{
			{Name: "cluster1"},
		})
		assert.NoError(t, err)

		assumed := cache.AssigningResourceBindings().GetAssumedWorkloads("cluster1")
		assert.Len(t, assumed, 1)
		// Full components are written, not just the delta.
		assert.Len(t, assumed[0].Components, 2)
		assert.Equal(t, int32(1), assumed[0].Components[0].Replicas)
		assert.Equal(t, int32(4), assumed[0].Components[1].Replicas)
	})

	t.Run("multi-template: removed cluster releases assumption", func(t *testing.T) {
		oldBinding := &workv1alpha2.ResourceBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "test-binding", Namespace: "default"},
			Spec: workv1alpha2.ResourceBindingSpec{
				Resource:   workv1alpha2.ObjectReference{Namespace: "work-ns"},
				Components: multiTemplateComponents,
				Clusters: []workv1alpha2.TargetCluster{
					{Name: "cluster1"},
					{Name: "cluster2"},
				},
			},
		}

		cache := schedulercache.NewCache(nil, nil, 0)
		bindingKey := "default/test-binding"
		cache.AssigningResourceBindings().Assume(bindingKey, "cluster2", schedulercache.AssumedWorkload{
			Namespace:  "work-ns",
			Components: multiTemplateComponents,
		})

		s := &Scheduler{
			KarmadaClient:  karmadafake.NewClientset(oldBinding),
			schedulerCache: cache,
		}

		err := s.patchScheduleResultForResourceBinding(oldBinding, "test-placement", []workv1alpha2.TargetCluster{
			{Name: "cluster1"}, // cluster2 removed
		})
		assert.NoError(t, err)
		assert.Empty(t, cache.AssigningResourceBindings().GetAssumedWorkloads("cluster2"))
	})

	t.Run("single-template: wraps per-cluster replicas as component", func(t *testing.T) {
		oldBinding := &workv1alpha2.ResourceBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "deploy-binding", Namespace: "default"},
			Spec: workv1alpha2.ResourceBindingSpec{
				Resource: workv1alpha2.ObjectReference{Name: "my-deploy", Namespace: "work-ns"},
				ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
					ResourceRequest: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
				},
				Clusters: []workv1alpha2.TargetCluster{{Name: "cluster1", Replicas: 2}},
			},
		}

		cache := schedulercache.NewCache(nil, nil, 0)
		s := &Scheduler{
			KarmadaClient:  karmadafake.NewClientset(oldBinding),
			schedulerCache: cache,
		}

		err := s.patchScheduleResultForResourceBinding(oldBinding, "test-placement", []workv1alpha2.TargetCluster{
			{Name: "cluster1", Replicas: 5},
		})
		assert.NoError(t, err)

		assumed := cache.AssigningResourceBindings().GetAssumedWorkloads("cluster1")
		assert.Len(t, assumed, 1)
		assert.Equal(t, "work-ns", assumed[0].Namespace)
		assert.Len(t, assumed[0].Components, 1)
		assert.Equal(t, "my-deploy", assumed[0].Components[0].Name)
		// replicas come from the new scheduleResult, not from oldBinding.Spec.Clusters
		assert.Equal(t, int32(5), assumed[0].Components[0].Replicas)
		assert.NotNil(t, assumed[0].Components[0].ReplicaRequirements)
		assert.Equal(t, resource.MustParse("500m"), assumed[0].Components[0].ReplicaRequirements.ResourceRequest[corev1.ResourceCPU])
	})

	t.Run("single-template: per-cluster replica counts are independent", func(t *testing.T) {
		oldBinding := &workv1alpha2.ResourceBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "deploy-binding", Namespace: "default"},
			Spec: workv1alpha2.ResourceBindingSpec{
				Resource: workv1alpha2.ObjectReference{Name: "my-deploy", Namespace: "work-ns"},
				ReplicaRequirements: &workv1alpha2.ReplicaRequirements{
					ResourceRequest: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("1"),
					},
				},
				Clusters: []workv1alpha2.TargetCluster{
					{Name: "cluster1", Replicas: 3},
					{Name: "cluster2", Replicas: 3},
				},
			},
		}

		cache := schedulercache.NewCache(nil, nil, 0)
		s := &Scheduler{
			KarmadaClient:  karmadafake.NewClientset(oldBinding),
			schedulerCache: cache,
		}

		err := s.patchScheduleResultForResourceBinding(oldBinding, "test-placement", []workv1alpha2.TargetCluster{
			{Name: "cluster1", Replicas: 4},
			{Name: "cluster2", Replicas: 2},
		})
		assert.NoError(t, err)

		assumed1 := cache.AssigningResourceBindings().GetAssumedWorkloads("cluster1")
		assert.Len(t, assumed1, 1)
		assert.Equal(t, int32(4), assumed1[0].Components[0].Replicas)

		assumed2 := cache.AssigningResourceBindings().GetAssumedWorkloads("cluster2")
		assert.Len(t, assumed2, 1)
		assert.Equal(t, int32(2), assumed2[0].Components[0].Replicas)
	})

	t.Run("no-components: no assumption written", func(t *testing.T) {
		oldBinding := &workv1alpha2.ResourceBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "test-binding", Namespace: "default"},
			Spec: workv1alpha2.ResourceBindingSpec{
				Resource:   workv1alpha2.ObjectReference{Namespace: "work-ns"},
				Components: nil,
				Clusters:   []workv1alpha2.TargetCluster{{Name: "cluster1"}},
			},
		}

		cache := schedulercache.NewCache(nil, nil, 0)
		s := &Scheduler{
			KarmadaClient:  karmadafake.NewClientset(oldBinding),
			schedulerCache: cache,
		}

		err := s.patchScheduleResultForResourceBinding(oldBinding, "test-placement", []workv1alpha2.TargetCluster{
			{Name: "cluster1"},
		})
		assert.NoError(t, err)
		assert.Empty(t, cache.AssigningResourceBindings().GetAssumedWorkloads("cluster1"))
	})
}

func TestScheduleClusterResourceBindingWithClusterAffinity(t *testing.T) {
	tests := []struct {
		name           string
		binding        *workv1alpha2.ClusterResourceBinding
		scheduleResult core.ScheduleResult
		scheduleError  error
		expectError    bool
	}{
		{
			name: "successful scheduling",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-binding",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Placement: &policyv1alpha1.Placement{
						ClusterAffinity: &policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"cluster1", "cluster2"},
						},
					},
				},
			},
			scheduleResult: core.ScheduleResult{
				SuggestedClusters: []workv1alpha2.TargetCluster{
					{Name: "cluster1", Replicas: 1},
					{Name: "cluster2", Replicas: 1},
				},
			},
			scheduleError: nil,
			expectError:   false,
		},
		{
			name: "scheduling error",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-binding-error",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Placement: &policyv1alpha1.Placement{
						ClusterAffinity: &policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"cluster1"},
						},
					},
				},
			},
			scheduleResult: core.ScheduleResult{},
			scheduleError:  errors.New("scheduling error"),
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := karmadafake.NewClientset(tt.binding)
			fakeRecorder := record.NewFakeRecorder(10)
			mockAlgorithm := &mockAlgorithm{
				scheduleFunc: func(_ context.Context, _ *workv1alpha2.ResourceBindingSpec, _ *workv1alpha2.ResourceBindingStatus, _ *core.ScheduleAlgorithmOption) (core.ScheduleResult, error) {
					return tt.scheduleResult, tt.scheduleError
				},
			}
			s := &Scheduler{
				KarmadaClient: fakeClient,
				eventRecorder: fakeRecorder,
				Algorithm:     mockAlgorithm,
			}

			err := s.scheduleClusterResourceBindingWithClusterAffinity(tt.binding)

			if (err != nil) != tt.expectError {
				t.Errorf("scheduleClusterResourceBindingWithClusterAffinity() error = %v, expectError %v", err, tt.expectError)
			}

			// Check if a patch was applied
			actions := fakeClient.Actions()
			patchActions := filterPatchActions(actions)
			if tt.expectError {
				assert.Empty(t, patchActions, "Expected no patch actions for error case")
			} else {
				assert.NotEmpty(t, patchActions, "Expected patch actions for success case")
			}

			// Check if an event was recorded
			select {
			case event := <-fakeRecorder.Events:
				if tt.expectError {
					assert.Contains(t, event, "ScheduleBindingFailed", "Expected ScheduleBindingFailed event")
				} else {
					assert.Contains(t, event, "ScheduleBindingSucceed", "Expected ScheduleBindingSucceed event")
				}
			default:
				t.Errorf("Expected an event to be recorded")
			}
		})
	}
}

func TestScheduleClusterResourceBindingWithClusterAffinities(t *testing.T) {
	tests := []struct {
		name                  string
		binding               *workv1alpha2.ClusterResourceBinding
		scheduleResults       []core.ScheduleResult
		scheduleErrors        []error
		expectError           bool
		expectedEvent         string
		expectedAffinityCalls []string
	}{
		{
			name: "successful scheduling with first affinity",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-binding",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Placement: &policyv1alpha1.Placement{
						ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
							{
								AffinityName: "affinity1",
								ClusterAffinity: policyv1alpha1.ClusterAffinity{
									ClusterNames: []string{"cluster1"},
								},
							},
							{
								AffinityName: "affinity2",
								ClusterAffinity: policyv1alpha1.ClusterAffinity{
									ClusterNames: []string{"cluster2"},
								},
							},
						},
					},
				},
			},
			scheduleResults: []core.ScheduleResult{
				{
					SuggestedClusters: []workv1alpha2.TargetCluster{
						{Name: "cluster1", Replicas: 1},
					},
				},
			},
			scheduleErrors: []error{nil},
			expectError:    false,
			expectedEvent:  fmt.Sprintf("Normal ScheduleBindingSucceed %s Result {cluster1:1}", successfulSchedulingMessage),
		},
		{
			name: "explicit rescheduling restarts from first affinity",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-binding-reschedule",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					RescheduleTriggeredAt: &metav1.Time{Time: time.Unix(2, 0)},
					Placement: &policyv1alpha1.Placement{
						ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
							{
								AffinityName: "affinity1",
								ClusterAffinity: policyv1alpha1.ClusterAffinity{
									ClusterNames: []string{"cluster1"},
								},
							},
							{
								AffinityName: "affinity2",
								ClusterAffinity: policyv1alpha1.ClusterAffinity{
									ClusterNames: []string{"cluster2"},
								},
							},
						},
					},
				},
				Status: workv1alpha2.ResourceBindingStatus{
					LastScheduledTime:             &metav1.Time{Time: time.Unix(1, 0)},
					SchedulerObservedAffinityName: "affinity2",
				},
			},
			scheduleResults: []core.ScheduleResult{
				{
					SuggestedClusters: []workv1alpha2.TargetCluster{
						{Name: "cluster1", Replicas: 1},
					},
				},
			},
			scheduleErrors:        []error{nil},
			expectError:           false,
			expectedEvent:         fmt.Sprintf("Normal ScheduleBindingSucceed %s Result {cluster1:1}", successfulSchedulingMessage),
			expectedAffinityCalls: []string{"affinity1"},
		},
		{
			name: "without explicit rescheduling resumes from observed affinity",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-binding-resume",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Placement: &policyv1alpha1.Placement{
						ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
							{
								AffinityName: "affinity1",
								ClusterAffinity: policyv1alpha1.ClusterAffinity{
									ClusterNames: []string{"cluster1"},
								},
							},
							{
								AffinityName: "affinity2",
								ClusterAffinity: policyv1alpha1.ClusterAffinity{
									ClusterNames: []string{"cluster2"},
								},
							},
						},
					},
				},
				Status: workv1alpha2.ResourceBindingStatus{
					SchedulerObservedAffinityName: "affinity2",
				},
			},
			scheduleResults: []core.ScheduleResult{
				{},
				{
					SuggestedClusters: []workv1alpha2.TargetCluster{
						{Name: "cluster2", Replicas: 1},
					},
				},
			},
			scheduleErrors:        []error{nil, nil},
			expectError:           false,
			expectedEvent:         fmt.Sprintf("Normal ScheduleBindingSucceed %s Result {cluster2:1}", successfulSchedulingMessage),
			expectedAffinityCalls: []string{"affinity2"},
		},
		{
			name: "successful scheduling with second affinity",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-binding-2",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Placement: &policyv1alpha1.Placement{
						ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
							{
								AffinityName: "affinity1",
								ClusterAffinity: policyv1alpha1.ClusterAffinity{
									ClusterNames: []string{"cluster1"},
								},
							},
							{
								AffinityName: "affinity2",
								ClusterAffinity: policyv1alpha1.ClusterAffinity{
									ClusterNames: []string{"cluster2"},
								},
							},
						},
					},
				},
			},
			scheduleResults: []core.ScheduleResult{
				{},
				{
					SuggestedClusters: []workv1alpha2.TargetCluster{
						{Name: "cluster2", Replicas: 1},
					},
				},
			},
			scheduleErrors: []error{errors.New("first affinity failed"), nil},
			expectError:    false,
			expectedEvent:  "Warning ScheduleBindingFailed failed to schedule ClusterResourceBinding(test-cluster-binding-2) with clusterAffiliates index(0): first affinity failed",
		},
		{
			name: "all affinities fail",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster-binding-fail",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Placement: &policyv1alpha1.Placement{
						ClusterAffinities: []policyv1alpha1.ClusterAffinityTerm{
							{
								AffinityName: "affinity1",
								ClusterAffinity: policyv1alpha1.ClusterAffinity{
									ClusterNames: []string{"cluster1"},
								},
							},
							{
								AffinityName: "affinity2",
								ClusterAffinity: policyv1alpha1.ClusterAffinity{
									ClusterNames: []string{"cluster2"},
								},
							},
						},
					},
				},
			},
			scheduleResults: []core.ScheduleResult{{}, {}},
			scheduleErrors:  []error{errors.New("first affinity failed"), errors.New("second affinity failed")},
			expectError:     true,
			expectedEvent:   "Warning ScheduleBindingFailed failed to schedule ClusterResourceBinding(test-cluster-binding-fail) with clusterAffiliates index(0): first affinity failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := karmadafake.NewClientset(tt.binding)
			fakeRecorder := record.NewFakeRecorder(10)
			var affinityCalls []string
			mockAlgorithm := &mockAlgorithm{
				scheduleFunc: func(_ context.Context, spec *workv1alpha2.ResourceBindingSpec, status *workv1alpha2.ResourceBindingStatus, _ *core.ScheduleAlgorithmOption) (core.ScheduleResult, error) {
					affinityCalls = append(affinityCalls, status.SchedulerObservedAffinityName)
					index := getAffinityIndex(spec.Placement.ClusterAffinities, status.SchedulerObservedAffinityName)
					if index < len(tt.scheduleResults) {
						return tt.scheduleResults[index], tt.scheduleErrors[index]
					}
					return core.ScheduleResult{}, errors.New("unexpected call to Schedule")
				},
			}
			s := &Scheduler{
				KarmadaClient: fakeClient,
				eventRecorder: fakeRecorder,
				Algorithm:     mockAlgorithm,
			}

			err := s.scheduleClusterResourceBindingWithClusterAffinities(tt.binding)

			if (err != nil) != tt.expectError {
				t.Errorf("scheduleClusterResourceBindingWithClusterAffinities() error = %v, expectError %v", err, tt.expectError)
			}
			if tt.expectedAffinityCalls != nil {
				assert.Equal(t, tt.expectedAffinityCalls, affinityCalls, "Schedule affinity calls do not match expected")
			}

			// Check if an event was recorded
			select {
			case event := <-fakeRecorder.Events:
				assert.Contains(t, event, tt.expectedEvent, "Event does not match expected")
			default:
				t.Errorf("Expected an event to be recorded")
			}
		})
	}
}

func TestComponentScaleSchedulingPreservesAcceptedResult(t *testing.T) {
	defer setFeatureGateDuringTest(t, features.FeatureGate, features.MultiplePodTemplatesScheduling, true)()

	placement := &policyv1alpha1.Placement{SpreadConstraints: []policyv1alpha1.SpreadConstraint{{
		SpreadByField: policyv1alpha1.SpreadByFieldCluster, MinGroups: 1, MaxGroups: 1,
	}}}
	placementJSON, err := json.Marshal(placement)
	assert.NoError(t, err)
	desired := []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 6}}
	accepted := []workv1alpha2.TargetComponent{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}}
	validResult := core.ScheduleResult{SuggestedClusters: []workv1alpha2.TargetCluster{{
		Name: "cluster1", Components: []workv1alpha2.TargetComponent{{Name: "taskmanager", Replicas: 6}, {Name: "jobmanager", Replicas: 1}},
	}}}

	tests := []struct {
		name        string
		accepted    []workv1alpha2.TargetComponent
		result      core.ScheduleResult
		err         error
		wantError   bool
		wantPatched bool
	}{
		{name: "valid result", accepted: accepted, result: validResult, wantPatched: true},
		{name: "fit error", accepted: accepted, err: &framework.FitError{}, wantError: true},
		{name: "empty result", accepted: accepted, wantError: true},
		{name: "different target", accepted: accepted, result: core.ScheduleResult{SuggestedClusters: []workv1alpha2.TargetCluster{{Name: "cluster2", Components: validResult.SuggestedClusters[0].Components}}}, wantError: true},
		{name: "partial snapshot", accepted: accepted, result: core.ScheduleResult{SuggestedClusters: []workv1alpha2.TargetCluster{{Name: "cluster1", Components: []workv1alpha2.TargetComponent{{Name: "taskmanager", Replicas: 6}}}}}, wantError: true},
	}

	for _, resourceScoped := range []bool{true, false} {
		for _, tt := range tests {
			name := fmt.Sprintf("resourceScoped=%t/%s", resourceScoped, tt.name)
			t.Run(name, func(t *testing.T) {
				oldClusters := []workv1alpha2.TargetCluster{{Name: "cluster1", Components: tt.accepted}}
				var option *core.ScheduleAlgorithmOption
				algorithm := &mockAlgorithm{scheduleFunc: func(_ context.Context, _ *workv1alpha2.ResourceBindingSpec, _ *workv1alpha2.ResourceBindingStatus, got *core.ScheduleAlgorithmOption) (core.ScheduleResult, error) {
					option = got
					return tt.result, tt.err
				}}
				annotations := componentSchedulingAnnotations(t, string(placementJSON), desired)
				if resourceScoped {
					binding := &workv1alpha2.ResourceBinding{
						ObjectMeta: metav1.ObjectMeta{Name: "rb", Namespace: "default", ResourceVersion: "7", Annotations: annotations},
						Spec:       workv1alpha2.ResourceBindingSpec{Placement: placement, Components: desired, Clusters: oldClusters},
					}
					client := karmadafake.NewClientset(binding)
					s := &Scheduler{KarmadaClient: client, Algorithm: algorithm, eventRecorder: record.NewFakeRecorder(10)}
					err := s.scheduleResourceBindingWithOptions(binding, true)
					assert.Equal(t, tt.wantError, err != nil)
					assertScalePatches(t, client.Actions(), tt.wantPatched, "7")
					updated, getErr := client.WorkV1alpha2().ResourceBindings(binding.Namespace).Get(context.Background(), binding.Name, metav1.GetOptions{})
					assert.NoError(t, getErr)
					if tt.wantPatched {
						assert.Equal(t, validResult.SuggestedClusters, updated.Spec.Clusters)
					} else {
						assert.Equal(t, oldClusters, updated.Spec.Clusters)
					}
				} else {
					binding := &workv1alpha2.ClusterResourceBinding{
						ObjectMeta: metav1.ObjectMeta{Name: "crb", ResourceVersion: "7", Annotations: annotations},
						Spec:       workv1alpha2.ResourceBindingSpec{Placement: placement, Components: desired, Clusters: oldClusters},
					}
					client := karmadafake.NewClientset(binding)
					s := &Scheduler{KarmadaClient: client, Algorithm: algorithm, eventRecorder: record.NewFakeRecorder(10)}
					err := s.scheduleClusterResourceBindingWithOptions(binding, true)
					assert.Equal(t, tt.wantError, err != nil)
					assertScalePatches(t, client.Actions(), tt.wantPatched, "7")
					updated, getErr := client.WorkV1alpha2().ClusterResourceBindings().Get(context.Background(), binding.Name, metav1.GetOptions{})
					assert.NoError(t, getErr)
					if tt.wantPatched {
						assert.Equal(t, validResult.SuggestedClusters, updated.Spec.Clusters)
					} else {
						assert.Equal(t, oldClusters, updated.Spec.Clusters)
					}
				}
				assert.NotNil(t, option)
				assert.True(t, option.IsMultiComponentScale)
			})
		}
	}
}

func TestComponentRequirementsTransitionRouting(t *testing.T) {
	defer setFeatureGateDuringTest(t, features.FeatureGate, features.MultiplePodTemplatesScheduling, true)()

	placement := &policyv1alpha1.Placement{SpreadConstraints: []policyv1alpha1.SpreadConstraint{{
		SpreadByField: policyv1alpha1.SpreadByFieldCluster, MinGroups: 1, MaxGroups: 1,
	}}}
	placementJSON, err := json.Marshal(placement)
	assert.NoError(t, err)
	acceptedRequirements := []workv1alpha2.Component{
		{Name: "jobmanager", Replicas: 1},
		{Name: "taskmanager", Replicas: 4, ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
			ResourceRequest: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
		}},
	}
	acceptedResult := []workv1alpha2.TargetCluster{{Name: "cluster1", Components: []workv1alpha2.TargetComponent{
		{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4},
	}}}
	successCondition := util.NewCondition(workv1alpha2.Scheduled, workv1alpha2.BindingReasonSuccess, successfulSchedulingMessage, metav1.ConditionTrue)

	tests := []struct {
		name            string
		desiredReplicas int32
		desiredCPU      string
		includeHash     bool
		observed        int64
		statusSuccess   bool
		divided         bool
		wantMainPatch   bool
		wantReason      string
		wantEvents      int
	}{
		{name: "requirements-only change is rejected", desiredReplicas: 4, desiredCPU: "500m", includeHash: true, observed: 1, statusSuccess: true, wantReason: workv1alpha2.BindingReasonUnschedulable, wantEvents: 2},
		{name: "replicas and requirements change is rejected", desiredReplicas: 6, desiredCPU: "500m", includeHash: true, observed: 1, statusSuccess: true, wantReason: workv1alpha2.BindingReasonUnschedulable, wantEvents: 2},
		{name: "current successful complete result establishes missing hash", desiredReplicas: 4, desiredCPU: "100m", observed: 2, statusSuccess: true, wantMainPatch: true, wantReason: workv1alpha2.BindingReasonSuccess},
		{name: "unobserved result cannot establish missing hash", desiredReplicas: 4, desiredCPU: "100m", observed: 1, statusSuccess: true, wantReason: workv1alpha2.BindingReasonUnschedulable, wantEvents: 2},
		{name: "failed result cannot establish missing hash", desiredReplicas: 4, desiredCPU: "100m", observed: 2, wantReason: workv1alpha2.BindingReasonUnschedulable, wantEvents: 2},
		{name: "divided result cannot establish missing hash", desiredReplicas: 4, desiredCPU: "100m", observed: 2, statusSuccess: true, divided: true, wantReason: workv1alpha2.BindingReasonUnschedulable, wantEvents: 2},
		{name: "replica change with a missing hash is rejected", desiredReplicas: 6, desiredCPU: "100m", observed: 2, statusSuccess: true, wantReason: workv1alpha2.BindingReasonUnschedulable, wantEvents: 2},
	}

	for _, resourceScoped := range []bool{true, false} {
		for _, tt := range tests {
			t.Run(fmt.Sprintf("resourceScoped=%t/%s", resourceScoped, tt.name), func(t *testing.T) {
				desired := []workv1alpha2.Component{
					{Name: "jobmanager", Replicas: 1},
					{Name: "taskmanager", Replicas: tt.desiredReplicas, ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
						ResourceRequest: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(tt.desiredCPU)},
					}},
				}
				annotations := map[string]string{util.PolicyPlacementAnnotation: string(placementJSON)}
				acceptedHash, hashErr := util.GenerateComponentRequirementsHash(acceptedRequirements)
				assert.NoError(t, hashErr)
				if tt.includeHash {
					annotations[util.AcceptedComponentRequirementsHashAnnotation] = acceptedHash
				}

				casePlacement := placement.DeepCopy()
				if tt.divided {
					casePlacement.ReplicaScheduling = &policyv1alpha1.ReplicaSchedulingStrategy{ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDivided}
				}
				algorithmCalls := 0
				algorithm := &mockAlgorithm{scheduleFunc: func(_ context.Context, _ *workv1alpha2.ResourceBindingSpec, _ *workv1alpha2.ResourceBindingStatus, option *core.ScheduleAlgorithmOption) (core.ScheduleResult, error) {
					algorithmCalls++
					assert.True(t, option.IsMultiComponentScale)
					return core.ScheduleResult{SuggestedClusters: acceptedResult}, nil
				}}
				recorder := record.NewFakeRecorder(10)
				condition := metav1.Condition{Type: workv1alpha2.Scheduled, Status: metav1.ConditionFalse, Reason: workv1alpha2.BindingReasonNoClusterFit}
				if tt.statusSuccess {
					condition = successCondition
				}
				status := workv1alpha2.ResourceBindingStatus{SchedulerObservedGeneration: tt.observed, Conditions: []metav1.Condition{condition}}

				var updatedAnnotations map[string]string
				var updatedConditions []metav1.Condition
				var actions []clienttesting.Action
				if resourceScoped {
					binding := &workv1alpha2.ResourceBinding{
						ObjectMeta: metav1.ObjectMeta{Name: "rb", Namespace: "default", ResourceVersion: "7", Generation: 2, Annotations: annotations},
						Spec:       workv1alpha2.ResourceBindingSpec{Placement: casePlacement, Components: desired, Clusters: acceptedResult},
						Status:     status,
					}
					client := karmadafake.NewClientset(binding)
					s := &Scheduler{KarmadaClient: client, bindingLister: &fakeBindingLister{binding: binding}, clusterLister: testClusterLister(t, "cluster1"), Algorithm: algorithm, eventRecorder: recorder}
					scheduleErr := s.doScheduleBinding(binding.Namespace, binding.Name)
					assert.NoError(t, scheduleErr)
					updated, getErr := client.WorkV1alpha2().ResourceBindings(binding.Namespace).Get(context.Background(), binding.Name, metav1.GetOptions{})
					assert.NoError(t, getErr)
					assert.Equal(t, acceptedResult, updated.Spec.Clusters)
					updatedAnnotations, updatedConditions, actions = updated.Annotations, updated.Status.Conditions, client.Actions()
				} else {
					binding := &workv1alpha2.ClusterResourceBinding{
						ObjectMeta: metav1.ObjectMeta{Name: "crb", ResourceVersion: "7", Generation: 2, Annotations: annotations},
						Spec:       workv1alpha2.ResourceBindingSpec{Placement: casePlacement, Components: desired, Clusters: acceptedResult},
						Status:     status,
					}
					client := karmadafake.NewClientset(binding)
					s := &Scheduler{KarmadaClient: client, clusterBindingLister: &fakeClusterBindingLister{binding: binding}, clusterLister: testClusterLister(t, "cluster1"), Algorithm: algorithm, eventRecorder: recorder}
					scheduleErr := s.doScheduleClusterBinding(binding.Name)
					assert.NoError(t, scheduleErr)
					updated, getErr := client.WorkV1alpha2().ClusterResourceBindings().Get(context.Background(), binding.Name, metav1.GetOptions{})
					assert.NoError(t, getErr)
					assert.Equal(t, acceptedResult, updated.Spec.Clusters)
					updatedAnnotations, updatedConditions, actions = updated.Annotations, updated.Status.Conditions, client.Actions()
				}

				assert.Zero(t, algorithmCalls)
				assert.Equal(t, tt.wantMainPatch, len(filterMainResourcePatches(actions)) == 1)
				if tt.wantMainPatch {
					desiredHash, desiredHashErr := util.GenerateComponentRequirementsHash(desired)
					assert.NoError(t, desiredHashErr)
					assert.Equal(t, desiredHash, updatedAnnotations[util.AcceptedComponentRequirementsHashAnnotation])
				} else {
					assert.Equal(t, annotations[util.AcceptedComponentRequirementsHashAnnotation], updatedAnnotations[util.AcceptedComponentRequirementsHashAnnotation])
				}
				assert.NotEmpty(t, updatedConditions)
				assert.Equal(t, tt.wantReason, updatedConditions[0].Reason)
				assert.Equal(t, tt.wantReason == workv1alpha2.BindingReasonSuccess, updatedConditions[0].Status == metav1.ConditionTrue)
				assert.Len(t, recorder.Events, tt.wantEvents)
			})
		}
	}
}

func TestUnavailableComponentTargetPreservesResultUntilFailoverSucceeds(t *testing.T) {
	defer setFeatureGateDuringTest(t, features.FeatureGate, features.MultiplePodTemplatesScheduling, true)()

	placement := &policyv1alpha1.Placement{SpreadConstraints: []policyv1alpha1.SpreadConstraint{{
		SpreadByField: policyv1alpha1.SpreadByFieldCluster, MinGroups: 1, MaxGroups: 1,
	}}}
	placementJSON, err := json.Marshal(placement)
	assert.NoError(t, err)
	components := []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}}
	oldClusters := []workv1alpha2.TargetCluster{{Name: "cluster1", Components: []workv1alpha2.TargetComponent{
		{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4},
	}}}
	newClusters := []workv1alpha2.TargetCluster{{Name: "cluster2", Components: []workv1alpha2.TargetComponent{
		{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4},
	}}}
	successCondition := util.NewCondition(workv1alpha2.Scheduled, workv1alpha2.BindingReasonSuccess, successfulSchedulingMessage, metav1.ConditionTrue)

	tests := []struct {
		name         string
		includeHash  bool
		hashMismatch bool
		fitError     bool
	}{
		{name: "accepted result no fit", includeHash: true, fitError: true},
		{name: "missing hash no fit", fitError: true},
		{name: "missing hash feasible alternative"},
		{name: "changed requirements feasible alternative", includeHash: true, hashMismatch: true},
	}
	for _, resourceScoped := range []bool{true, false} {
		for _, tt := range tests {
			t.Run(fmt.Sprintf("resourceScoped=%t/%s", resourceScoped, tt.name), func(t *testing.T) {
				annotations := map[string]string{util.PolicyPlacementAnnotation: string(placementJSON)}
				if tt.includeHash {
					hashComponents := components
					if tt.hashMismatch {
						hashComponents = append([]workv1alpha2.Component(nil), components...)
						hashComponents[1] = *components[1].DeepCopy()
						hashComponents[1].ReplicaRequirements = &workv1alpha2.ComponentReplicaRequirements{PriorityClassName: "previous"}
					}
					hash, hashErr := util.GenerateComponentRequirementsHash(hashComponents)
					assert.NoError(t, hashErr)
					annotations[util.AcceptedComponentRequirementsHashAnnotation] = hash
				}
				algorithmCalls := 0
				algorithm := &mockAlgorithm{scheduleFunc: func(_ context.Context, _ *workv1alpha2.ResourceBindingSpec, _ *workv1alpha2.ResourceBindingStatus, option *core.ScheduleAlgorithmOption) (core.ScheduleResult, error) {
					algorithmCalls++
					assert.False(t, option.IsMultiComponentScale)
					if tt.fitError {
						return core.ScheduleResult{}, &framework.FitError{}
					}
					return core.ScheduleResult{SuggestedClusters: newClusters}, nil
				}}
				recorder := record.NewFakeRecorder(10)
				status := workv1alpha2.ResourceBindingStatus{SchedulerObservedGeneration: 2, Conditions: []metav1.Condition{successCondition}}

				var gotClusters []workv1alpha2.TargetCluster
				var gotAnnotations map[string]string
				var gotConditions []metav1.Condition
				var actions []clienttesting.Action
				if resourceScoped {
					binding := &workv1alpha2.ResourceBinding{ObjectMeta: metav1.ObjectMeta{Name: "rb", Namespace: "default", ResourceVersion: "7", Generation: 2, Annotations: annotations}, Spec: workv1alpha2.ResourceBindingSpec{Placement: placement, Components: components, Clusters: oldClusters}, Status: status}
					client := karmadafake.NewClientset(binding)
					s := &Scheduler{KarmadaClient: client, bindingLister: &fakeBindingLister{binding: binding}, clusterLister: testClusterLister(t), Algorithm: algorithm, eventRecorder: recorder}
					scheduleErr := s.doScheduleBinding(binding.Namespace, binding.Name)
					assert.Equal(t, tt.fitError, scheduleErr != nil)
					updated, getErr := client.WorkV1alpha2().ResourceBindings(binding.Namespace).Get(context.Background(), binding.Name, metav1.GetOptions{})
					assert.NoError(t, getErr)
					gotClusters, gotAnnotations, gotConditions, actions = updated.Spec.Clusters, updated.Annotations, updated.Status.Conditions, client.Actions()
				} else {
					binding := &workv1alpha2.ClusterResourceBinding{ObjectMeta: metav1.ObjectMeta{Name: "crb", ResourceVersion: "7", Generation: 2, Annotations: annotations}, Spec: workv1alpha2.ResourceBindingSpec{Placement: placement, Components: components, Clusters: oldClusters}, Status: status}
					client := karmadafake.NewClientset(binding)
					s := &Scheduler{KarmadaClient: client, clusterBindingLister: &fakeClusterBindingLister{binding: binding}, clusterLister: testClusterLister(t), Algorithm: algorithm, eventRecorder: recorder}
					scheduleErr := s.doScheduleClusterBinding(binding.Name)
					assert.Equal(t, tt.fitError, scheduleErr != nil)
					updated, getErr := client.WorkV1alpha2().ClusterResourceBindings().Get(context.Background(), binding.Name, metav1.GetOptions{})
					assert.NoError(t, getErr)
					gotClusters, gotAnnotations, gotConditions, actions = updated.Spec.Clusters, updated.Annotations, updated.Status.Conditions, client.Actions()
				}

				assert.Equal(t, 1, algorithmCalls)
				if tt.fitError {
					assert.Equal(t, oldClusters, gotClusters)
					assert.Equal(t, annotations[util.AcceptedComponentRequirementsHashAnnotation], gotAnnotations[util.AcceptedComponentRequirementsHashAnnotation])
					assert.Empty(t, filterMainResourcePatches(actions))
					assert.Equal(t, workv1alpha2.BindingReasonNoClusterFit, gotConditions[0].Reason)
					assert.Equal(t, metav1.ConditionFalse, gotConditions[0].Status)
				} else {
					assert.Equal(t, newClusters, gotClusters)
					assertAcceptedComponentRequirementsHash(t, &workv1alpha2.ResourceBindingSpec{Placement: placement, Components: components, Clusters: gotClusters}, gotAnnotations, false)
					assertScalePatches(t, actions, true, "7")
					assert.Equal(t, metav1.ConditionTrue, gotConditions[0].Status)
				}
				assert.Len(t, recorder.Events, 2)
			})
		}
	}
}

func TestUnsupportedComponentScaleErrorReturnedToRoutingCaller(t *testing.T) {
	defer setFeatureGateDuringTest(t, features.FeatureGate, features.MultiplePodTemplatesScheduling, true)()

	placement := &policyv1alpha1.Placement{SpreadConstraints: []policyv1alpha1.SpreadConstraint{{
		SpreadByField: policyv1alpha1.SpreadByFieldCluster, MinGroups: 1, MaxGroups: 1,
	}}}
	desired := []workv1alpha2.Component{{Name: "jobmanager", Replicas: 2}, {Name: "taskmanager", Replicas: 3}}
	accepted := []workv1alpha2.TargetCluster{{Name: "cluster1", Components: []workv1alpha2.TargetComponent{
		{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4},
	}}}
	annotations := componentSchedulingAnnotations(t, "{}", desired)

	for _, resourceScoped := range []bool{true, false} {
		t.Run(fmt.Sprintf("resourceScoped=%t", resourceScoped), func(t *testing.T) {
			recorder := record.NewFakeRecorder(10)
			if resourceScoped {
				binding := &workv1alpha2.ResourceBinding{ObjectMeta: metav1.ObjectMeta{Name: "rb", Namespace: "default", Annotations: annotations}, Spec: workv1alpha2.ResourceBindingSpec{Placement: placement, Components: desired, Clusters: accepted}}
				client := karmadafake.NewClientset(binding)
				s := &Scheduler{KarmadaClient: client, eventRecorder: recorder}
				err := s.scheduleResourceBindingWithOptions(binding, true)
				var unsupportedErr *unsupportedComponentScaleError
				assert.ErrorAs(t, err, &unsupportedErr)
			} else {
				binding := &workv1alpha2.ClusterResourceBinding{ObjectMeta: metav1.ObjectMeta{Name: "crb", Annotations: annotations}, Spec: workv1alpha2.ResourceBindingSpec{Placement: placement, Components: desired, Clusters: accepted}}
				client := karmadafake.NewClientset(binding)
				s := &Scheduler{KarmadaClient: client, eventRecorder: recorder}
				err := s.scheduleClusterResourceBindingWithOptions(binding, true)
				var unsupportedErr *unsupportedComponentScaleError
				assert.ErrorAs(t, err, &unsupportedErr)
			}
			assert.Len(t, recorder.Events, 2)
		})
	}
}

type componentScaleRoutingCase struct {
	name               string
	configure          func(*policyv1alpha1.Placement, *[]workv1alpha2.Component, *[]workv1alpha2.TargetComponent, *workv1alpha2.ResourceBindingStatus)
	missingTarget      bool
	placementMismatch  bool
	legacyPlacement    bool
	explicitReschedule bool
	wantCalls          int
	wantScale          bool
	wantReuseAccepted  bool
	wantSnapshot       bool
	wantMainPatch      bool
	wantMetadata       bool
	wantFailed         bool
	wantReason         string
	fitError           bool
	initial            bool
}

type componentScaleRoutingObservation struct {
	algorithmCalls      int
	scaleOption         bool
	reuseAcceptedOption bool
}

func TestComponentScaleRouting(t *testing.T) {
	defer setFeatureGateDuringTest(t, features.FeatureGate, features.MultiplePodTemplatesScheduling, true)()

	basePlacement := &policyv1alpha1.Placement{SpreadConstraints: []policyv1alpha1.SpreadConstraint{{
		SpreadByField: policyv1alpha1.SpreadByFieldCluster, MinGroups: 1, MaxGroups: 1,
	}}}
	baseDesired := []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 6}}
	baseAccepted := []workv1alpha2.TargetComponent{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}}
	tests := []componentScaleRoutingCase{
		{name: "supported scale pins current target", wantCalls: 1, wantScale: true, wantSnapshot: true, wantMainPatch: true},
		{name: "current successful duplicated legacy result is backfilled", configure: func(_ *policyv1alpha1.Placement, _ *[]workv1alpha2.Component, accepted *[]workv1alpha2.TargetComponent, status *workv1alpha2.ResourceBindingStatus) {
			*accepted = nil
			status.SchedulerObservedGeneration = 2
			status.Conditions = []metav1.Condition{util.NewCondition(workv1alpha2.Scheduled, workv1alpha2.BindingReasonSuccess, successfulSchedulingMessage, metav1.ConditionTrue)}
		}, wantSnapshot: true, wantMainPatch: true, wantMetadata: true},
		{name: "unobserved legacy result is rejected", configure: func(_ *policyv1alpha1.Placement, _ *[]workv1alpha2.Component, accepted *[]workv1alpha2.TargetComponent, _ *workv1alpha2.ResourceBindingStatus) {
			*accepted = nil
		}, wantFailed: true},
		{name: "failed legacy result is rejected", configure: func(_ *policyv1alpha1.Placement, _ *[]workv1alpha2.Component, accepted *[]workv1alpha2.TargetComponent, status *workv1alpha2.ResourceBindingStatus) {
			*accepted = nil
			status.SchedulerObservedGeneration = 2
			status.Conditions = []metav1.Condition{util.NewCondition(workv1alpha2.Scheduled, workv1alpha2.BindingReasonNoClusterFit, "no cluster fit", metav1.ConditionFalse)}
		}, wantFailed: true},
		{name: "legacy result with changed placement is rejected", configure: func(_ *policyv1alpha1.Placement, _ *[]workv1alpha2.Component, accepted *[]workv1alpha2.TargetComponent, status *workv1alpha2.ResourceBindingStatus) {
			*accepted = nil
			status.SchedulerObservedGeneration = 2
			status.Conditions = []metav1.Condition{util.NewCondition(workv1alpha2.Scheduled, workv1alpha2.BindingReasonSuccess, successfulSchedulingMessage, metav1.ConditionTrue)}
		}, placementMismatch: true, wantFailed: true},
		{name: "current divided legacy result is rejected", configure: func(p *policyv1alpha1.Placement, _ *[]workv1alpha2.Component, accepted *[]workv1alpha2.TargetComponent, status *workv1alpha2.ResourceBindingStatus) {
			*accepted = nil
			p.ReplicaScheduling = &policyv1alpha1.ReplicaSchedulingStrategy{ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDivided}
			status.SchedulerObservedGeneration = 2
			status.Conditions = []metav1.Condition{util.NewCondition(workv1alpha2.Scheduled, workv1alpha2.BindingReasonSuccess, successfulSchedulingMessage, metav1.ConditionTrue)}
		}, wantFailed: true},
		{name: "placement and components changed", placementMismatch: true, wantFailed: true},
		{name: "placement-only change with accepted components uses ordinary scheduling", configure: func(_ *policyv1alpha1.Placement, desired *[]workv1alpha2.Component, accepted *[]workv1alpha2.TargetComponent, _ *workv1alpha2.ResourceBindingStatus) {
			*desired = []workv1alpha2.Component{{Name: "jobmanager", Replicas: (*accepted)[0].Replicas}, {Name: "taskmanager", Replicas: (*accepted)[1].Replicas}}
		}, placementMismatch: true, wantCalls: 1, wantSnapshot: true, wantMainPatch: true},
		{name: "placement-only no-fit preserves accepted result", configure: func(_ *policyv1alpha1.Placement, desired *[]workv1alpha2.Component, accepted *[]workv1alpha2.TargetComponent, _ *workv1alpha2.ResourceBindingStatus) {
			*desired = []workv1alpha2.Component{{Name: "jobmanager", Replicas: (*accepted)[0].Replicas}, {Name: "taskmanager", Replicas: (*accepted)[1].Replicas}}
		}, placementMismatch: true, fitError: true, wantCalls: 1, wantFailed: true, wantReason: workv1alpha2.BindingReasonNoClusterFit},
		{name: "new placement is no longer single-cluster applicable", configure: func(p *policyv1alpha1.Placement, _ *[]workv1alpha2.Component, _ *[]workv1alpha2.TargetComponent, _ *workv1alpha2.ResourceBindingStatus) {
			p.SpreadConstraints[0].MaxGroups = 2
		}, legacyPlacement: true, wantFailed: true},
		{name: "explicit transition out of single-cluster applicability uses full recovery", configure: func(p *policyv1alpha1.Placement, _ *[]workv1alpha2.Component, _ *[]workv1alpha2.TargetComponent, status *workv1alpha2.ResourceBindingStatus) {
			p.SpreadConstraints[0].MaxGroups = 2
			lastScheduled := metav1.NewTime(time.Unix(1, 0))
			status.LastScheduledTime = &lastScheduled
		}, legacyPlacement: true, explicitReschedule: true, wantCalls: 1, wantMainPatch: true},
		{name: "placement-only change cannot erase accepted result", configure: func(p *policyv1alpha1.Placement, desired *[]workv1alpha2.Component, accepted *[]workv1alpha2.TargetComponent, _ *workv1alpha2.ResourceBindingStatus) {
			p.SpreadConstraints[0].MaxGroups = 2
			*desired = []workv1alpha2.Component{{Name: "jobmanager", Replicas: (*accepted)[0].Replicas}, {Name: "taskmanager", Replicas: (*accepted)[1].Replicas}}
		}, legacyPlacement: true, wantFailed: true},
		{name: "ordered cluster affinities", configure: func(p *policyv1alpha1.Placement, _ *[]workv1alpha2.Component, _ *[]workv1alpha2.TargetComponent, status *workv1alpha2.ResourceBindingStatus) {
			p.ClusterAffinities = []policyv1alpha1.ClusterAffinityTerm{{AffinityName: "primary"}}
			status.SchedulerObservedAffinityName = "primary"
		}, wantFailed: true},
		{name: "mixed directions", configure: func(_ *policyv1alpha1.Placement, desired *[]workv1alpha2.Component, _ *[]workv1alpha2.TargetComponent, _ *workv1alpha2.ResourceBindingStatus) {
			*desired = []workv1alpha2.Component{{Name: "jobmanager", Replicas: 2}, {Name: "taskmanager", Replicas: 3}}
		}, wantFailed: true},
		{name: "explicit reschedule and components changed uses full recovery", configure: func(_ *policyv1alpha1.Placement, _ *[]workv1alpha2.Component, _ *[]workv1alpha2.TargetComponent, status *workv1alpha2.ResourceBindingStatus) {
			lastScheduled := metav1.NewTime(time.Unix(1, 0))
			status.LastScheduledTime = &lastScheduled
		}, explicitReschedule: true, wantCalls: 1, wantSnapshot: true, wantMainPatch: true},
		{name: "explicit reschedule with mixed directions uses full recovery", configure: func(_ *policyv1alpha1.Placement, desired *[]workv1alpha2.Component, _ *[]workv1alpha2.TargetComponent, status *workv1alpha2.ResourceBindingStatus) {
			*desired = []workv1alpha2.Component{{Name: "jobmanager", Replicas: 2}, {Name: "taskmanager", Replicas: 3}}
			lastScheduled := metav1.NewTime(time.Unix(1, 0))
			status.LastScheduledTime = &lastScheduled
		}, explicitReschedule: true, wantCalls: 1, wantSnapshot: true, wantMainPatch: true},
		{name: "explicit reschedule with component name change uses full recovery", configure: func(_ *policyv1alpha1.Placement, desired *[]workv1alpha2.Component, _ *[]workv1alpha2.TargetComponent, status *workv1alpha2.ResourceBindingStatus) {
			*desired = []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "worker", Replicas: 4}}
			lastScheduled := metav1.NewTime(time.Unix(1, 0))
			status.LastScheduledTime = &lastScheduled
		}, explicitReschedule: true, wantCalls: 1, wantSnapshot: true, wantMainPatch: true},
		{name: "explicit reschedule with requirements change uses full recovery", configure: func(_ *policyv1alpha1.Placement, desired *[]workv1alpha2.Component, accepted *[]workv1alpha2.TargetComponent, status *workv1alpha2.ResourceBindingStatus) {
			*desired = []workv1alpha2.Component{
				{Name: "jobmanager", Replicas: (*accepted)[0].Replicas},
				{Name: "taskmanager", Replicas: (*accepted)[1].Replicas, ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{PriorityClassName: "high-priority"}},
			}
			lastScheduled := metav1.NewTime(time.Unix(1, 0))
			status.LastScheduledTime = &lastScheduled
		}, explicitReschedule: true, wantCalls: 1, wantSnapshot: true, wantMainPatch: true},
		{name: "explicit reschedule with accepted components uses ordinary scheduling", configure: func(_ *policyv1alpha1.Placement, desired *[]workv1alpha2.Component, accepted *[]workv1alpha2.TargetComponent, status *workv1alpha2.ResourceBindingStatus) {
			*desired = []workv1alpha2.Component{{Name: "jobmanager", Replicas: (*accepted)[0].Replicas}, {Name: "taskmanager", Replicas: (*accepted)[1].Replicas}}
			lastScheduled := metav1.NewTime(time.Unix(1, 0))
			status.LastScheduledTime = &lastScheduled
		}, explicitReschedule: true, wantCalls: 1, wantSnapshot: true, wantMainPatch: true},
		{name: "explicit reschedule no-fit preserves accepted result", configure: func(_ *policyv1alpha1.Placement, desired *[]workv1alpha2.Component, accepted *[]workv1alpha2.TargetComponent, status *workv1alpha2.ResourceBindingStatus) {
			*desired = []workv1alpha2.Component{{Name: "jobmanager", Replicas: (*accepted)[0].Replicas}, {Name: "taskmanager", Replicas: (*accepted)[1].Replicas}}
			lastScheduled := metav1.NewTime(time.Unix(1, 0))
			status.LastScheduledTime = &lastScheduled
		}, explicitReschedule: true, fitError: true, wantCalls: 1, wantFailed: true, wantReason: workv1alpha2.BindingReasonNoClusterFit},
		{name: "steady duplicated no-fit preserves accepted result", configure: func(_ *policyv1alpha1.Placement, desired *[]workv1alpha2.Component, accepted *[]workv1alpha2.TargetComponent, _ *workv1alpha2.ResourceBindingStatus) {
			*desired = []workv1alpha2.Component{{Name: "jobmanager", Replicas: (*accepted)[0].Replicas}, {Name: "taskmanager", Replicas: (*accepted)[1].Replicas}}
		}, fitError: true, wantCalls: 1, wantReuseAccepted: true, wantFailed: true, wantReason: workv1alpha2.BindingReasonNoClusterFit},
		{name: "missing target uses ordinary failover", missingTarget: true, wantCalls: 1, wantMainPatch: true},
		{name: "missing target takes precedence over compatible placement change", missingTarget: true, placementMismatch: true, wantCalls: 1, wantMainPatch: true},
		{name: "initial scheduling remains ordinary", initial: true, wantCalls: 1, wantSnapshot: true, wantMainPatch: true},
		{name: "accepted multi-component result to one component requires explicit recovery", configure: func(_ *policyv1alpha1.Placement, desired *[]workv1alpha2.Component, _ *[]workv1alpha2.TargetComponent, _ *workv1alpha2.ResourceBindingStatus) {
			*desired = []workv1alpha2.Component{{Name: "worker", Replicas: 6}}
		}, wantFailed: true},
		{name: "accepted multi-component result to no components requires explicit recovery", configure: func(_ *policyv1alpha1.Placement, desired *[]workv1alpha2.Component, _ *[]workv1alpha2.TargetComponent, _ *workv1alpha2.ResourceBindingStatus) {
			*desired = nil
		}, wantFailed: true},
	}

	for _, resourceScoped := range []bool{true, false} {
		for _, tt := range tests {
			t.Run(fmt.Sprintf("resourceScoped=%t/%s", resourceScoped, tt.name), func(t *testing.T) {
				runComponentScaleRoutingCase(t, tt, resourceScoped, basePlacement, baseDesired, baseAccepted)
			})
		}
	}
}

type explicitPendingComponentResultRecoveryCase struct {
	name           string
	triggerUnix    int64
	ordered        bool
	divided        bool
	completeResult bool
	outcome        string
	wantCalls      int
	wantError      bool
	wantMainPatch  bool
	wantReason     string
}

type explicitPendingComponentResultRecoveryFixture struct {
	basePlacement     *policyv1alpha1.Placement
	desired           []workv1alpha2.Component
	legacyClusters    []workv1alpha2.TargetCluster
	recoveredClusters []workv1alpha2.TargetCluster
}

type explicitPendingComponentResultRecoveryResult struct {
	spec        workv1alpha2.ResourceBindingSpec
	annotations map[string]string
	status      workv1alpha2.ResourceBindingStatus
	generation  int64
	actions     []clienttesting.Action
	scheduleErr error
}

func TestExplicitPendingComponentResultRecovery(t *testing.T) {
	defer setFeatureGateDuringTest(t, features.FeatureGate, features.MultiplePodTemplatesScheduling, true)()

	fixture := explicitPendingComponentResultRecoveryFixture{
		basePlacement: &policyv1alpha1.Placement{SpreadConstraints: []policyv1alpha1.SpreadConstraint{{
			SpreadByField: policyv1alpha1.SpreadByFieldCluster, MinGroups: 1, MaxGroups: 1,
		}}},
		desired:        []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}},
		legacyClusters: []workv1alpha2.TargetCluster{{Name: "cluster1"}},
		recoveredClusters: []workv1alpha2.TargetCluster{{Name: "cluster2", Components: []workv1alpha2.TargetComponent{
			{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4},
		}}},
	}

	tests := []explicitPendingComponentResultRecoveryCase{
		{name: "valid trigger recovers with full scheduling", triggerUnix: 2, outcome: "success", wantCalls: 1, wantMainPatch: true, wantReason: workv1alpha2.BindingReasonSuccess},
		{name: "fit error preserves legacy result", triggerUnix: 2, outcome: "fit-error", wantCalls: 1, wantError: true, wantReason: workv1alpha2.BindingReasonNoClusterFit},
		{name: "invalid result preserves legacy result", triggerUnix: 2, outcome: "invalid-result", wantCalls: 1, wantError: true, wantReason: workv1alpha2.BindingReasonSchedulerError},
		{name: "divided legacy missing trigger remains fail closed", divided: true, wantReason: workv1alpha2.BindingReasonUnschedulable},
		{name: "divided legacy stale trigger remains fail closed", triggerUnix: 1, divided: true, wantReason: workv1alpha2.BindingReasonUnschedulable},
		{name: "ordered affinities with a persisted result remain fail closed", triggerUnix: 2, ordered: true, completeResult: true, wantReason: workv1alpha2.BindingReasonUnschedulable},
		{name: "divided complete result with missing hash recovers", triggerUnix: 2, divided: true, completeResult: true, outcome: "success", wantCalls: 1, wantMainPatch: true, wantReason: workv1alpha2.BindingReasonSuccess},
		{name: "divided complete result fit error is preserved", triggerUnix: 2, divided: true, completeResult: true, outcome: "fit-error", wantCalls: 1, wantError: true, wantReason: workv1alpha2.BindingReasonNoClusterFit},
		{name: "divided complete result without trigger remains fail closed", divided: true, completeResult: true, wantReason: workv1alpha2.BindingReasonUnschedulable},
	}

	for _, resourceScoped := range []bool{true, false} {
		for _, tt := range tests {
			t.Run(fmt.Sprintf("resourceScoped=%t/%s", resourceScoped, tt.name), func(t *testing.T) {
				runExplicitPendingComponentResultRecoveryCase(t, tt, resourceScoped, fixture)
			})
		}
	}
}

func TestExplicitRecoveryLeavesMultiComponentResultShape(t *testing.T) {
	defer setFeatureGateDuringTest(t, features.FeatureGate, features.MultiplePodTemplatesScheduling, true)()
	placement := &policyv1alpha1.Placement{SpreadConstraints: []policyv1alpha1.SpreadConstraint{{
		SpreadByField: policyv1alpha1.SpreadByFieldCluster,
		MinGroups:     1,
		MaxGroups:     1,
	}}}
	placementJSON, err := json.Marshal(placement)
	assert.NoError(t, err)
	acceptedDesired := []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}}
	acceptedClusters := []workv1alpha2.TargetCluster{{Name: "cluster1", Components: []workv1alpha2.TargetComponent{
		{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4},
	}}}
	lastScheduled := metav1.NewTime(time.Unix(1, 0))
	triggeredAt := metav1.NewTime(time.Unix(2, 0))

	for _, desired := range [][]workv1alpha2.Component{
		{{Name: "worker", Replicas: 2}},
		nil,
	} {
		for _, resourceScoped := range []bool{true, false} {
			name := fmt.Sprintf("resourceScoped=%t/desiredComponents=%d", resourceScoped, len(desired))
			t.Run(name, func(t *testing.T) {
				annotations := componentSchedulingAnnotations(t, string(placementJSON), acceptedDesired)
				annotations[acceptedComponentResultGenerationAnnotation] = "2"
				annotations[acceptedComponentSchedulingSpecHashAnnotation] = "v1:sha256:accepted"
				status := workv1alpha2.ResourceBindingStatus{
					SchedulerObservedGeneration: 2,
					LastScheduledTime:           &lastScheduled,
					Conditions: []metav1.Condition{
						util.NewCondition(workv1alpha2.Scheduled, workv1alpha2.BindingReasonSuccess, successfulSchedulingMessage, metav1.ConditionTrue),
					},
				}
				algorithmCalls := 0
				algorithm := &mockAlgorithm{scheduleFunc: func(_ context.Context, _ *workv1alpha2.ResourceBindingSpec, _ *workv1alpha2.ResourceBindingStatus, option *core.ScheduleAlgorithmOption) (core.ScheduleResult, error) {
					algorithmCalls++
					assert.False(t, option.IsMultiComponentScale)
					assert.False(t, option.ReuseAcceptedComponentTarget)
					return core.ScheduleResult{SuggestedClusters: []workv1alpha2.TargetCluster{{Name: "cluster2"}}}, nil
				}}
				spec := workv1alpha2.ResourceBindingSpec{
					Placement: placement, Components: desired, Clusters: acceptedClusters, RescheduleTriggeredAt: &triggeredAt,
				}

				var updatedSpec workv1alpha2.ResourceBindingSpec
				var updatedAnnotations map[string]string
				var actions []clienttesting.Action
				if resourceScoped {
					binding := &workv1alpha2.ResourceBinding{
						ObjectMeta: metav1.ObjectMeta{Name: "rb", Namespace: "default", ResourceVersion: "7", Generation: 2, Annotations: annotations},
						Spec:       spec,
						Status:     status,
					}
					client := karmadafake.NewClientset(binding)
					s := &Scheduler{KarmadaClient: client, bindingLister: &fakeBindingLister{binding: binding}, clusterLister: testClusterLister(t, "cluster1", "cluster2"), Algorithm: algorithm, eventRecorder: record.NewFakeRecorder(10)}
					assert.NoError(t, s.doScheduleBinding(binding.Namespace, binding.Name))
					updated, getErr := client.WorkV1alpha2().ResourceBindings(binding.Namespace).Get(context.Background(), binding.Name, metav1.GetOptions{})
					assert.NoError(t, getErr)
					updatedSpec, updatedAnnotations, actions = updated.Spec, updated.Annotations, client.Actions()
				} else {
					binding := &workv1alpha2.ClusterResourceBinding{
						ObjectMeta: metav1.ObjectMeta{Name: "crb", ResourceVersion: "7", Generation: 2, Annotations: annotations},
						Spec:       spec,
						Status:     status,
					}
					client := karmadafake.NewClientset(binding)
					s := &Scheduler{KarmadaClient: client, clusterBindingLister: &fakeClusterBindingLister{binding: binding}, clusterLister: testClusterLister(t, "cluster1", "cluster2"), Algorithm: algorithm, eventRecorder: record.NewFakeRecorder(10)}
					assert.NoError(t, s.doScheduleClusterBinding(binding.Name))
					updated, getErr := client.WorkV1alpha2().ClusterResourceBindings().Get(context.Background(), binding.Name, metav1.GetOptions{})
					assert.NoError(t, getErr)
					updatedSpec, updatedAnnotations, actions = updated.Spec, updated.Annotations, client.Actions()
				}

				assert.Equal(t, 1, algorithmCalls)
				assert.Equal(t, []workv1alpha2.TargetCluster{{Name: "cluster2"}}, updatedSpec.Clusters)
				assert.NotContains(t, updatedAnnotations, util.AcceptedComponentRequirementsHashAnnotation)
				assert.NotContains(t, updatedAnnotations, acceptedComponentResultGenerationAnnotation)
				assert.NotContains(t, updatedAnnotations, acceptedComponentSchedulingSpecHashAnnotation)
				assert.Len(t, filterMainResourcePatches(actions), 1)
			})
		}
	}
}

func runExplicitPendingComponentResultRecoveryCase(t *testing.T, tt explicitPendingComponentResultRecoveryCase, resourceScoped bool, fixture explicitPendingComponentResultRecoveryFixture) {
	t.Helper()
	placement, initialClusters, status, annotations, rescheduleTriggeredAt := prepareExplicitPendingComponentResultRecoveryCase(t, tt, fixture)
	algorithmCalls := 0
	algorithm := explicitPendingComponentResultRecoveryAlgorithm(t, tt, fixture.recoveredClusters, &algorithmCalls)
	recorder := record.NewFakeRecorder(10)
	result := runExplicitPendingComponentResultRecoveryScheduling(t, tt, resourceScoped, fixture.desired, placement, initialClusters, status, annotations, rescheduleTriggeredAt, algorithm, recorder)
	assertExplicitPendingComponentResultRecovery(t, tt, fixture.recoveredClusters, initialClusters, algorithmCalls, recorder, result)
}

func prepareExplicitPendingComponentResultRecoveryCase(t *testing.T, tt explicitPendingComponentResultRecoveryCase, fixture explicitPendingComponentResultRecoveryFixture) (*policyv1alpha1.Placement, []workv1alpha2.TargetCluster, workv1alpha2.ResourceBindingStatus, map[string]string, *metav1.Time) {
	t.Helper()
	placement := fixture.basePlacement.DeepCopy()
	initialClusters := fixture.legacyClusters
	if tt.divided {
		placement.ReplicaScheduling = &policyv1alpha1.ReplicaSchedulingStrategy{ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDivided}
	}
	if tt.completeResult {
		initialClusters = []workv1alpha2.TargetCluster{{Name: "cluster1", Components: fixture.recoveredClusters[0].Components}}
	}
	lastScheduled := metav1.NewTime(time.Unix(1, 0))
	status := workv1alpha2.ResourceBindingStatus{
		SchedulerObservedGeneration: 2,
		LastScheduledTime:           &lastScheduled,
		Conditions: []metav1.Condition{
			util.NewCondition(workv1alpha2.Scheduled, workv1alpha2.BindingReasonSuccess, successfulSchedulingMessage, metav1.ConditionTrue),
		},
	}
	if tt.ordered {
		placement.ClusterAffinities = []policyv1alpha1.ClusterAffinityTerm{{AffinityName: "primary"}}
		status.SchedulerObservedAffinityName = "primary"
	}
	placementJSON, err := json.Marshal(placement)
	assert.NoError(t, err)
	annotations := map[string]string{util.PolicyPlacementAnnotation: string(placementJSON)}
	var rescheduleTriggeredAt *metav1.Time
	if tt.triggerUnix != 0 {
		triggeredAt := metav1.NewTime(time.Unix(tt.triggerUnix, 0))
		rescheduleTriggeredAt = &triggeredAt
	}
	return placement, initialClusters, status, annotations, rescheduleTriggeredAt
}

func explicitPendingComponentResultRecoveryAlgorithm(t *testing.T, tt explicitPendingComponentResultRecoveryCase, recoveredClusters []workv1alpha2.TargetCluster, algorithmCalls *int) *mockAlgorithm {
	t.Helper()
	return &mockAlgorithm{scheduleFunc: func(_ context.Context, _ *workv1alpha2.ResourceBindingSpec, _ *workv1alpha2.ResourceBindingStatus, option *core.ScheduleAlgorithmOption) (core.ScheduleResult, error) {
		(*algorithmCalls)++
		if assert.NotNil(t, option) {
			assert.False(t, option.IsMultiComponentScale)
		}
		switch tt.outcome {
		case "success":
			return core.ScheduleResult{SuggestedClusters: recoveredClusters}, nil
		case "fit-error":
			return core.ScheduleResult{}, &framework.FitError{}
		case "invalid-result":
			return core.ScheduleResult{SuggestedClusters: []workv1alpha2.TargetCluster{{
				Name:       "cluster2",
				Components: recoveredClusters[0].Components[:1],
			}}}, nil
		default:
			t.Errorf("algorithm called for fail-closed case %q", tt.name)
			return core.ScheduleResult{}, errors.New("unexpected algorithm call")
		}
	}}
}

func runExplicitPendingComponentResultRecoveryScheduling(t *testing.T, tt explicitPendingComponentResultRecoveryCase, resourceScoped bool, desired []workv1alpha2.Component, placement *policyv1alpha1.Placement, initialClusters []workv1alpha2.TargetCluster, status workv1alpha2.ResourceBindingStatus, annotations map[string]string, rescheduleTriggeredAt *metav1.Time, algorithm *mockAlgorithm, recorder *record.FakeRecorder) explicitPendingComponentResultRecoveryResult {
	t.Helper()
	if resourceScoped {
		binding := &workv1alpha2.ResourceBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "rb", Namespace: "default", ResourceVersion: "7", Generation: 2, Annotations: annotations},
			Spec: workv1alpha2.ResourceBindingSpec{
				Placement: placement, Components: desired, Clusters: initialClusters, RescheduleTriggeredAt: rescheduleTriggeredAt,
			},
			Status: status,
		}
		client := karmadafake.NewClientset(binding)
		if tt.wantMainPatch {
			simulateGenerationIncrementOnMainPatch(client, "resourcebindings", 3, "8")
		}
		s := &Scheduler{KarmadaClient: client, bindingLister: &fakeBindingLister{binding: binding}, clusterLister: testClusterLister(t, "cluster1", "cluster2"), Algorithm: algorithm, eventRecorder: recorder}
		scheduleErr := s.doScheduleBinding(binding.Namespace, binding.Name)
		updated, getErr := client.WorkV1alpha2().ResourceBindings(binding.Namespace).Get(context.Background(), binding.Name, metav1.GetOptions{})
		assert.NoError(t, getErr)
		return explicitPendingComponentResultRecoveryResult{updated.Spec, updated.Annotations, updated.Status, updated.Generation, client.Actions(), scheduleErr}
	}

	binding := &workv1alpha2.ClusterResourceBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "crb", ResourceVersion: "7", Generation: 2, Annotations: annotations},
		Spec: workv1alpha2.ResourceBindingSpec{
			Placement: placement, Components: desired, Clusters: initialClusters, RescheduleTriggeredAt: rescheduleTriggeredAt,
		},
		Status: status,
	}
	client := karmadafake.NewClientset(binding)
	if tt.wantMainPatch {
		simulateGenerationIncrementOnMainPatch(client, "clusterresourcebindings", 3, "8")
	}
	s := &Scheduler{KarmadaClient: client, clusterBindingLister: &fakeClusterBindingLister{binding: binding}, clusterLister: testClusterLister(t, "cluster1", "cluster2"), Algorithm: algorithm, eventRecorder: recorder}
	scheduleErr := s.doScheduleClusterBinding(binding.Name)
	updated, getErr := client.WorkV1alpha2().ClusterResourceBindings().Get(context.Background(), binding.Name, metav1.GetOptions{})
	assert.NoError(t, getErr)
	return explicitPendingComponentResultRecoveryResult{updated.Spec, updated.Annotations, updated.Status, updated.Generation, client.Actions(), scheduleErr}
}

func assertExplicitPendingComponentResultRecovery(t *testing.T, tt explicitPendingComponentResultRecoveryCase, recoveredClusters, initialClusters []workv1alpha2.TargetCluster, algorithmCalls int, recorder *record.FakeRecorder, result explicitPendingComponentResultRecoveryResult) {
	t.Helper()
	assert.Equal(t, tt.wantCalls, algorithmCalls)
	assert.Equal(t, tt.wantError, result.scheduleErr != nil)
	patches := filterMainResourcePatches(result.actions)
	assert.Equal(t, tt.wantMainPatch, len(patches) == 1)
	if tt.wantMainPatch {
		assert.Equal(t, recoveredClusters, result.spec.Clusters)
		assert.Equal(t, int64(3), result.generation)
		assert.Equal(t, int64(3), result.status.SchedulerObservedGeneration)
		assertAcceptedComponentRequirementsHash(t, &result.spec, result.annotations, false)
		assert.Equal(t, "3", result.annotations[acceptedComponentResultGenerationAnnotation])
		assert.Regexp(t, `^v1:sha256:[0-9a-f]{64}$`, result.annotations[acceptedComponentSchedulingSpecHashAnnotation])
		assert.True(t, isAcceptedComponentSchedulingSpecHashMatched(&result.spec, result.annotations))
		assert.False(t, util.RescheduleRequired(result.spec.RescheduleTriggeredAt, result.status.LastScheduledTime))
		assertPatchResourceVersion(t, patches[0], "7")
	} else {
		assert.Equal(t, initialClusters, result.spec.Clusters)
		assert.Equal(t, int64(2), result.generation)
		assert.Empty(t, result.annotations[util.AcceptedComponentRequirementsHashAnnotation])
		assert.Empty(t, result.annotations[acceptedComponentResultGenerationAnnotation])
		assert.Empty(t, result.annotations[acceptedComponentSchedulingSpecHashAnnotation])
	}
	if assert.NotEmpty(t, result.status.Conditions) {
		assert.Equal(t, tt.wantReason, result.status.Conditions[0].Reason)
		assert.Equal(t, tt.wantMainPatch, result.status.Conditions[0].Status == metav1.ConditionTrue)
	}
	assert.Len(t, recorder.Events, 2)
}

func runComponentScaleRoutingCase(t *testing.T, tt componentScaleRoutingCase, resourceScoped bool, basePlacement *policyv1alpha1.Placement, baseDesired []workv1alpha2.Component, baseAccepted []workv1alpha2.TargetComponent) {
	t.Helper()
	placement := basePlacement.DeepCopy()
	desired := append([]workv1alpha2.Component(nil), baseDesired...)
	accepted := append([]workv1alpha2.TargetComponent(nil), baseAccepted...)
	status := workv1alpha2.ResourceBindingStatus{}
	if tt.configure != nil {
		tt.configure(placement, &desired, &accepted, &status)
	}
	rescheduleTriggeredAt, annotations, oldClusters := prepareComponentRoutingState(t, tt, placement, basePlacement, baseDesired, accepted)

	observation := &componentScaleRoutingObservation{}
	algorithm := newComponentScaleRoutingAlgorithm(tt, observation)
	clusterLister := testClusterLister(t, "cluster1")
	if tt.missingTarget {
		clusterLister = testClusterLister(t)
	}
	recorder := record.NewFakeRecorder(10)
	if resourceScoped {
		binding := &workv1alpha2.ResourceBinding{ObjectMeta: metav1.ObjectMeta{Name: "rb", Namespace: "default", ResourceVersion: "7", Generation: 2, Annotations: annotations}, Spec: workv1alpha2.ResourceBindingSpec{Placement: placement, Components: desired, Clusters: oldClusters, RescheduleTriggeredAt: rescheduleTriggeredAt}, Status: status}
		client := karmadafake.NewClientset(binding)
		s := &Scheduler{KarmadaClient: client, bindingLister: &fakeBindingLister{binding: binding}, clusterLister: clusterLister, Algorithm: algorithm, eventRecorder: recorder}
		scheduleErr := s.doScheduleBinding(binding.Namespace, binding.Name)
		assert.Equal(t, tt.fitError, scheduleErr != nil)
		updated, getErr := client.WorkV1alpha2().ResourceBindings(binding.Namespace).Get(context.Background(), binding.Name, metav1.GetOptions{})
		assert.NoError(t, getErr)
		assertComponentScaleRoutingResult(t, updated.Spec.Clusters, updated.Status.Conditions, oldClusters, desired, tt.wantFailed, tt.wantReason, tt.wantSnapshot, tt.missingTarget)
		assertAcceptedComponentRequirementsHash(t, &updated.Spec, updated.Annotations, tt.wantFailed)
		if tt.wantMetadata {
			assert.NotEmpty(t, updated.Annotations[acceptedComponentResultGenerationAnnotation])
			assert.True(t, isAcceptedComponentSchedulingSpecHashMatched(&updated.Spec, updated.Annotations))
		}
		assertComponentRoutingPatches(t, client.Actions(), tt.wantMainPatch)
	} else {
		binding := &workv1alpha2.ClusterResourceBinding{ObjectMeta: metav1.ObjectMeta{Name: "crb", ResourceVersion: "7", Generation: 2, Annotations: annotations}, Spec: workv1alpha2.ResourceBindingSpec{Placement: placement, Components: desired, Clusters: oldClusters, RescheduleTriggeredAt: rescheduleTriggeredAt}, Status: status}
		client := karmadafake.NewClientset(binding)
		s := &Scheduler{KarmadaClient: client, clusterBindingLister: &fakeClusterBindingLister{binding: binding}, clusterLister: clusterLister, Algorithm: algorithm, eventRecorder: recorder}
		scheduleErr := s.doScheduleClusterBinding(binding.Name)
		assert.Equal(t, tt.fitError, scheduleErr != nil)
		updated, getErr := client.WorkV1alpha2().ClusterResourceBindings().Get(context.Background(), binding.Name, metav1.GetOptions{})
		assert.NoError(t, getErr)
		assertComponentScaleRoutingResult(t, updated.Spec.Clusters, updated.Status.Conditions, oldClusters, desired, tt.wantFailed, tt.wantReason, tt.wantSnapshot, tt.missingTarget)
		assertAcceptedComponentRequirementsHash(t, &updated.Spec, updated.Annotations, tt.wantFailed)
		if tt.wantMetadata {
			assert.NotEmpty(t, updated.Annotations[acceptedComponentResultGenerationAnnotation])
			assert.True(t, isAcceptedComponentSchedulingSpecHashMatched(&updated.Spec, updated.Annotations))
		}
		assertComponentRoutingPatches(t, client.Actions(), tt.wantMainPatch)
	}
	assert.Equal(t, tt.wantCalls, observation.algorithmCalls)
	assert.Equal(t, tt.wantScale, observation.scaleOption)
	assert.Equal(t, tt.wantReuseAccepted, observation.reuseAcceptedOption)
	if tt.wantFailed {
		assert.Len(t, recorder.Events, 2)
		for range 2 {
			assert.Contains(t, <-recorder.Events, "Warning ScheduleBindingFailed")
		}
	}
}

func newComponentScaleRoutingAlgorithm(tt componentScaleRoutingCase, observation *componentScaleRoutingObservation) *mockAlgorithm {
	return &mockAlgorithm{scheduleFunc: func(_ context.Context, spec *workv1alpha2.ResourceBindingSpec, _ *workv1alpha2.ResourceBindingStatus, option *core.ScheduleAlgorithmOption) (core.ScheduleResult, error) {
		observation.algorithmCalls++
		observation.scaleOption = option.IsMultiComponentScale
		observation.reuseAcceptedOption = option.ReuseAcceptedComponentTarget
		if tt.fitError {
			return core.ScheduleResult{}, &framework.FitError{}
		}
		components := make([]workv1alpha2.TargetComponent, len(spec.Components))
		for i := range spec.Components {
			components[i] = workv1alpha2.TargetComponent{Name: spec.Components[i].Name, Replicas: spec.Components[i].Replicas}
		}
		switch {
		case tt.missingTarget:
			return core.ScheduleResult{SuggestedClusters: []workv1alpha2.TargetCluster{{Name: "cluster2", Components: components}}}, nil
		case tt.initial || option.IsMultiComponentScale || tt.explicitReschedule && util.IsMultiTemplateSchedulingApplicable(spec):
			return core.ScheduleResult{SuggestedClusters: []workv1alpha2.TargetCluster{{Name: "cluster1", Components: components}}}, nil
		case tt.explicitReschedule:
			return core.ScheduleResult{SuggestedClusters: []workv1alpha2.TargetCluster{{Name: "cluster1"}}}, nil
		default:
			return core.ScheduleResult{SuggestedClusters: spec.Clusters}, nil
		}
	}}
}

func prepareComponentRoutingState(t *testing.T, tt componentScaleRoutingCase, placement, basePlacement *policyv1alpha1.Placement, baseDesired []workv1alpha2.Component, accepted []workv1alpha2.TargetComponent) (*metav1.Time, map[string]string, []workv1alpha2.TargetCluster) {
	t.Helper()
	var rescheduleTriggeredAt *metav1.Time
	if tt.explicitReschedule {
		triggeredAt := metav1.NewTime(time.Unix(2, 0))
		rescheduleTriggeredAt = &triggeredAt
	}
	appliedPlacement := placement
	if tt.placementMismatch {
		appliedPlacement = &policyv1alpha1.Placement{}
	} else if tt.legacyPlacement {
		appliedPlacement = basePlacement.DeepCopy()
	}
	placementJSON, err := json.Marshal(appliedPlacement)
	assert.NoError(t, err)
	annotations := componentSchedulingAnnotations(t, string(placementJSON), baseDesired)
	oldClusters := []workv1alpha2.TargetCluster{{Name: "cluster1", Components: accepted}}
	if len(accepted) == 0 {
		delete(annotations, util.AcceptedComponentRequirementsHashAnnotation)
	}
	if tt.initial {
		oldClusters = nil
		delete(annotations, util.AcceptedComponentRequirementsHashAnnotation)
	}
	return rescheduleTriggeredAt, annotations, oldClusters
}

func assertAcceptedComponentRequirementsHash(t *testing.T, spec *workv1alpha2.ResourceBindingSpec, annotations map[string]string, rejected bool) {
	t.Helper()
	if rejected || !util.IsBindingComponentsAccepted(spec) {
		return
	}
	want, err := util.GenerateComponentRequirementsHash(spec.Components)
	assert.NoError(t, err)
	assert.Equal(t, want, annotations[util.AcceptedComponentRequirementsHashAnnotation])
}

func TestSetAcceptedComponentMetadataRemovesStaleValues(t *testing.T) {
	defer setFeatureGateDuringTest(t, features.FeatureGate, features.MultiplePodTemplatesScheduling, true)()
	spec := &workv1alpha2.ResourceBindingSpec{
		Placement: &policyv1alpha1.Placement{SpreadConstraints: []policyv1alpha1.SpreadConstraint{{
			SpreadByField: policyv1alpha1.SpreadByFieldCluster, MinGroups: 1, MaxGroups: 2,
		}}},
		Components: []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}},
		Clusters:   []workv1alpha2.TargetCluster{{Name: "cluster1"}},
	}
	annotations := map[string]string{
		util.AcceptedComponentRequirementsHashAnnotation: "v1:sha256:old-requirements",
		acceptedComponentResultGenerationAnnotation:      "7",
		acceptedComponentSchedulingSpecHashAnnotation:    "v1:sha256:old-spec",
		util.ResourceTemplateSpecificationHashAnnotation: "v1:sha256:source",
	}

	assert.NoError(t, setAcceptedComponentRequirementsHash(spec, annotations))
	assert.NoError(t, setAcceptedComponentResultMetadata(spec, annotations, 7, nil))
	assert.NotContains(t, annotations, util.AcceptedComponentRequirementsHashAnnotation)
	assert.NotContains(t, annotations, acceptedComponentResultGenerationAnnotation)
	assert.NotContains(t, annotations, acceptedComponentSchedulingSpecHashAnnotation)
	assert.Equal(t, "v1:sha256:source", annotations[util.ResourceTemplateSpecificationHashAnnotation])
}

func assertComponentRoutingPatches(t *testing.T, actions []clienttesting.Action, wantMainPatch bool) {
	t.Helper()
	patches := filterMainResourcePatches(actions)
	if !assert.Equal(t, wantMainPatch, len(patches) == 1) {
		return
	}
	if wantMainPatch && assert.NotEmpty(t, patches) {
		assertPatchResourceVersion(t, patches[0], "7")
	}
}

func assertComponentScaleRoutingResult(t *testing.T, gotClusters []workv1alpha2.TargetCluster, conditions []metav1.Condition, oldClusters []workv1alpha2.TargetCluster, desired []workv1alpha2.Component, wantFailed bool, wantReason string, wantSnapshot, missingTarget bool) {
	t.Helper()
	if wantFailed {
		assert.Equal(t, oldClusters, gotClusters)
		if assert.NotEmpty(t, conditions) {
			assert.Equal(t, metav1.ConditionFalse, conditions[0].Status)
			if wantReason == "" {
				wantReason = workv1alpha2.BindingReasonUnschedulable
			}
			assert.Equal(t, wantReason, conditions[0].Reason)
		}
	}
	if wantSnapshot {
		if assert.Len(t, gotClusters, 1) {
			assert.Equal(t, "cluster1", gotClusters[0].Name)
			wantComponents := make([]workv1alpha2.TargetComponent, len(desired))
			for i := range desired {
				wantComponents[i] = workv1alpha2.TargetComponent{Name: desired[i].Name, Replicas: desired[i].Replicas}
			}
			assert.Equal(t, wantComponents, gotClusters[0].Components)
		}
	}
	if missingTarget && assert.NotEmpty(t, gotClusters) {
		assert.Equal(t, "cluster2", gotClusters[0].Name)
	}
}

func TestStaleAcceptedComponentResultGenerationUsesNormalScheduling(t *testing.T) {
	defer setFeatureGateDuringTest(t, features.FeatureGate, features.MultiplePodTemplatesScheduling, true)()
	placement := &policyv1alpha1.Placement{SpreadConstraints: []policyv1alpha1.SpreadConstraint{{
		SpreadByField: policyv1alpha1.SpreadByFieldCluster, MinGroups: 1, MaxGroups: 1,
	}}}
	placementJSON, err := json.Marshal(placement)
	assert.NoError(t, err)
	components := []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}}
	clusters := []workv1alpha2.TargetCluster{{Name: "cluster1", Components: []workv1alpha2.TargetComponent{
		{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4},
	}}}

	for _, resourceScoped := range []bool{true, false} {
		t.Run(fmt.Sprintf("resourceScoped=%t", resourceScoped), func(t *testing.T) {
			algorithmCalls := 0
			algorithm := &mockAlgorithm{scheduleFunc: func(_ context.Context, spec *workv1alpha2.ResourceBindingSpec, _ *workv1alpha2.ResourceBindingStatus, _ *core.ScheduleAlgorithmOption) (core.ScheduleResult, error) {
				algorithmCalls++
				assert.Equal(t, "app=frontend", spec.WorkloadAffinityGroups.AffinityGroup)
				return core.ScheduleResult{SuggestedClusters: spec.Clusters}, nil
			}}
			annotations := componentSchedulingAnnotations(t, string(placementJSON), components)
			annotations[acceptedComponentResultGenerationAnnotation] = "1"
			status := workv1alpha2.ResourceBindingStatus{
				SchedulerObservedGeneration: 1,
				Conditions: []metav1.Condition{
					util.NewCondition(workv1alpha2.Scheduled, workv1alpha2.BindingReasonSuccess, successfulSchedulingMessage, metav1.ConditionTrue),
				},
			}
			spec := workv1alpha2.ResourceBindingSpec{
				Placement:              placement,
				Components:             components,
				Clusters:               clusters,
				WorkloadAffinityGroups: &workv1alpha2.WorkloadAffinityGroups{AffinityGroup: "app=frontend"},
			}
			acceptedSpec := spec.DeepCopy()
			acceptedSpec.WorkloadAffinityGroups = nil
			acceptedSpecHash, hashErr := generateAcceptedComponentSchedulingSpecHash(acceptedSpec)
			assert.NoError(t, hashErr)
			annotations[acceptedComponentSchedulingSpecHashAnnotation] = acceptedSpecHash
			if resourceScoped {
				binding := &workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "rb", Namespace: "default", Generation: 2, Annotations: annotations},
					Spec:       spec,
					Status:     status,
				}
				client := karmadafake.NewClientset(binding)
				s := &Scheduler{KarmadaClient: client, bindingLister: &fakeBindingLister{binding: binding}, clusterLister: testClusterLister(t, "cluster1"), Algorithm: algorithm, eventRecorder: record.NewFakeRecorder(10)}
				assert.NoError(t, s.doScheduleBinding(binding.Namespace, binding.Name))
				updated, getErr := client.WorkV1alpha2().ResourceBindings(binding.Namespace).Get(context.Background(), binding.Name, metav1.GetOptions{})
				assert.NoError(t, getErr)
				assert.Equal(t, int64(2), updated.Status.SchedulerObservedGeneration)
				assert.Equal(t, metav1.ConditionTrue, updated.Status.Conditions[0].Status)
				assert.Equal(t, clusters, updated.Spec.Clusters)
				assert.Equal(t, "2", updated.Annotations[acceptedComponentResultGenerationAnnotation])
			} else {
				binding := &workv1alpha2.ClusterResourceBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "crb", Generation: 2, Annotations: annotations},
					Spec:       spec,
					Status:     status,
				}
				client := karmadafake.NewClientset(binding)
				s := &Scheduler{KarmadaClient: client, clusterBindingLister: &fakeClusterBindingLister{binding: binding}, clusterLister: testClusterLister(t, "cluster1"), Algorithm: algorithm, eventRecorder: record.NewFakeRecorder(10)}
				assert.NoError(t, s.doScheduleClusterBinding(binding.Name))
				updated, getErr := client.WorkV1alpha2().ClusterResourceBindings().Get(context.Background(), binding.Name, metav1.GetOptions{})
				assert.NoError(t, getErr)
				assert.Equal(t, int64(2), updated.Status.SchedulerObservedGeneration)
				assert.Equal(t, metav1.ConditionTrue, updated.Status.Conditions[0].Status)
				assert.Equal(t, clusters, updated.Spec.Clusters)
				assert.Equal(t, "2", updated.Annotations[acceptedComponentResultGenerationAnnotation])
			}
			assert.Equal(t, 1, algorithmCalls)
		})
	}
}

func TestAcceptedComponentStatusRetryUsesPersistedResultGeneration(t *testing.T) {
	defer setFeatureGateDuringTest(t, features.FeatureGate, features.MultiplePodTemplatesScheduling, true)()
	const (
		initialGeneration = int64(2)
		patchedGeneration = int64(3)
	)
	placement := &policyv1alpha1.Placement{SpreadConstraints: []policyv1alpha1.SpreadConstraint{{
		SpreadByField: policyv1alpha1.SpreadByFieldCluster, MinGroups: 1, MaxGroups: 1,
	}}}
	placementJSON, err := json.Marshal(placement)
	assert.NoError(t, err)
	desired := []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 6}}
	accepted := []workv1alpha2.TargetCluster{{Name: "cluster1", Components: []workv1alpha2.TargetComponent{
		{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4},
	}}}
	scheduled := []workv1alpha2.TargetCluster{{Name: "cluster1", Components: []workv1alpha2.TargetComponent{
		{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 6},
	}}}

	for _, resourceScoped := range []bool{true, false} {
		t.Run(fmt.Sprintf("resourceScoped=%t", resourceScoped), func(t *testing.T) {
			algorithmCalls := 0
			algorithm := &mockAlgorithm{scheduleFunc: func(_ context.Context, _ *workv1alpha2.ResourceBindingSpec, _ *workv1alpha2.ResourceBindingStatus, _ *core.ScheduleAlgorithmOption) (core.ScheduleResult, error) {
				algorithmCalls++
				return core.ScheduleResult{SuggestedClusters: scheduled}, nil
			}}
			annotations := componentSchedulingAnnotations(t, string(placementJSON), desired)
			annotations[acceptedComponentResultGenerationAnnotation] = "1"
			status := workv1alpha2.ResourceBindingStatus{
				SchedulerObservedGeneration: 1,
				Conditions: []metav1.Condition{
					util.NewCondition(workv1alpha2.Scheduled, workv1alpha2.BindingReasonSuccess, successfulSchedulingMessage, metav1.ConditionTrue),
				},
			}
			if resourceScoped {
				binding := &workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "rb", Namespace: "default", ResourceVersion: "7", Generation: initialGeneration, Annotations: annotations},
					Spec:       workv1alpha2.ResourceBindingSpec{Placement: placement, Components: desired, Clusters: accepted},
					Status:     status,
				}
				client := karmadafake.NewClientset(binding)
				simulateGenerationIncrementOnMainPatch(client, "resourcebindings", patchedGeneration, "8")
				failNextStatusPatch(t, client, "resourcebindings", "8")
				s := &Scheduler{KarmadaClient: client, bindingLister: &fakeBindingLister{binding: binding}, clusterLister: testClusterLister(t, "cluster1"), Algorithm: algorithm, eventRecorder: record.NewFakeRecorder(10)}
				assert.EqualError(t, s.doScheduleBinding(binding.Namespace, binding.Name), "injected status patch failure")
				persisted, getErr := client.WorkV1alpha2().ResourceBindings(binding.Namespace).Get(context.Background(), binding.Name, metav1.GetOptions{})
				assert.NoError(t, getErr)
				assert.Equal(t, patchedGeneration, persisted.Generation)
				assert.Equal(t, "8", persisted.ResourceVersion)
				assert.Equal(t, int64(1), persisted.Status.SchedulerObservedGeneration)
				assert.Equal(t, "3", persisted.Annotations[acceptedComponentResultGenerationAnnotation])
				assert.Equal(t, scheduled, persisted.Spec.Clusters)

				s.bindingLister = &fakeBindingLister{binding: persisted}
				assert.NoError(t, s.doScheduleBinding(binding.Namespace, binding.Name))
				updated, getErr := client.WorkV1alpha2().ResourceBindings(binding.Namespace).Get(context.Background(), binding.Name, metav1.GetOptions{})
				assert.NoError(t, getErr)
				assert.Equal(t, patchedGeneration, updated.Status.SchedulerObservedGeneration)
				assert.Equal(t, metav1.ConditionTrue, updated.Status.Conditions[0].Status)
				assertScalePatches(t, client.Actions(), true, "7")
			} else {
				binding := &workv1alpha2.ClusterResourceBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "crb", ResourceVersion: "7", Generation: initialGeneration, Annotations: annotations},
					Spec:       workv1alpha2.ResourceBindingSpec{Placement: placement, Components: desired, Clusters: accepted},
					Status:     status,
				}
				client := karmadafake.NewClientset(binding)
				simulateGenerationIncrementOnMainPatch(client, "clusterresourcebindings", patchedGeneration, "8")
				failNextStatusPatch(t, client, "clusterresourcebindings", "8")
				s := &Scheduler{KarmadaClient: client, clusterBindingLister: &fakeClusterBindingLister{binding: binding}, clusterLister: testClusterLister(t, "cluster1"), Algorithm: algorithm, eventRecorder: record.NewFakeRecorder(10)}
				assert.EqualError(t, s.doScheduleClusterBinding(binding.Name), "injected status patch failure")
				persisted, getErr := client.WorkV1alpha2().ClusterResourceBindings().Get(context.Background(), binding.Name, metav1.GetOptions{})
				assert.NoError(t, getErr)
				assert.Equal(t, patchedGeneration, persisted.Generation)
				assert.Equal(t, "8", persisted.ResourceVersion)
				assert.Equal(t, int64(1), persisted.Status.SchedulerObservedGeneration)
				assert.Equal(t, "3", persisted.Annotations[acceptedComponentResultGenerationAnnotation])
				assert.Equal(t, scheduled, persisted.Spec.Clusters)

				s.clusterBindingLister = &fakeClusterBindingLister{binding: persisted}
				assert.NoError(t, s.doScheduleClusterBinding(binding.Name))
				updated, getErr := client.WorkV1alpha2().ClusterResourceBindings().Get(context.Background(), binding.Name, metav1.GetOptions{})
				assert.NoError(t, getErr)
				assert.Equal(t, patchedGeneration, updated.Status.SchedulerObservedGeneration)
				assert.Equal(t, metav1.ConditionTrue, updated.Status.Conditions[0].Status)
				assertScalePatches(t, client.Actions(), true, "7")
			}
			assert.Equal(t, 1, algorithmCalls)
		})
	}
}

func TestAcceptedComponentStatusRepairAfterConfigOnlyGenerationChange(t *testing.T) {
	defer setFeatureGateDuringTest(t, features.FeatureGate, features.MultiplePodTemplatesScheduling, true)()
	placement := &policyv1alpha1.Placement{
		ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDivided},
		SpreadConstraints: []policyv1alpha1.SpreadConstraint{{
			SpreadByField: policyv1alpha1.SpreadByFieldCluster, MinGroups: 1, MaxGroups: 1,
		}},
	}
	placementJSON, err := json.Marshal(placement)
	assert.NoError(t, err)
	components := []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}}
	scheduled := []workv1alpha2.TargetCluster{{Name: "cluster1", Components: []workv1alpha2.TargetComponent{
		{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4},
	}}}
	failureCondition := util.NewCondition(workv1alpha2.Scheduled, workv1alpha2.BindingReasonNoClusterFit, "failure from an earlier schedule", metav1.ConditionFalse)
	tests := []struct {
		name       string
		conditions []metav1.Condition
	}{
		{name: "missing condition"},
		{name: "unrelated old failure", conditions: []metav1.Condition{failureCondition}},
	}

	for _, resourceScoped := range []bool{true, false} {
		for _, tt := range tests {
			t.Run(fmt.Sprintf("resourceScoped=%t/%s", resourceScoped, tt.name), func(t *testing.T) {
				algorithmCalls := 0
				algorithm := &mockAlgorithm{scheduleFunc: func(_ context.Context, _ *workv1alpha2.ResourceBindingSpec, _ *workv1alpha2.ResourceBindingStatus, option *core.ScheduleAlgorithmOption) (core.ScheduleResult, error) {
					algorithmCalls++
					assert.False(t, option.IsMultiComponentScale)
					return core.ScheduleResult{SuggestedClusters: scheduled}, nil
				}}
				annotations := map[string]string{util.PolicyPlacementAnnotation: string(placementJSON)}
				status := workv1alpha2.ResourceBindingStatus{Conditions: append([]metav1.Condition(nil), tt.conditions...)}
				spec := workv1alpha2.ResourceBindingSpec{
					Resource:   workv1alpha2.ObjectReference{APIVersion: "flink.apache.org/v1beta1", Kind: "FlinkDeployment", Name: "flink", ResourceVersion: "workload-rv-1"},
					Placement:  placement,
					Components: components,
				}

				var (
					actions []clienttesting.Action
					updated workv1alpha2.ResourceBindingStatus
				)
				if resourceScoped {
					binding := &workv1alpha2.ResourceBinding{
						ObjectMeta: metav1.ObjectMeta{Name: "rb", Namespace: "default", ResourceVersion: "7", Generation: 2, Annotations: annotations},
						Spec:       spec,
						Status:     status,
					}
					client := karmadafake.NewClientset(binding)
					simulateGenerationIncrementOnMainPatch(client, "resourcebindings", 3, "8")
					simulateConcurrentConfigOnlyUpdateOnNextStatusPatch(t, client, "resourcebindings", "8", "9", 4)
					s := &Scheduler{KarmadaClient: client, bindingLister: &fakeBindingLister{binding: binding}, clusterLister: testClusterLister(t, "cluster1"), Algorithm: algorithm, eventRecorder: record.NewFakeRecorder(10)}
					firstErr := s.doScheduleBinding(binding.Namespace, binding.Name)
					assert.True(t, apierrors.IsConflict(firstErr), "expected resourceVersion conflict, got %v", firstErr)
					persisted, getErr := client.WorkV1alpha2().ResourceBindings(binding.Namespace).Get(context.Background(), binding.Name, metav1.GetOptions{})
					assert.NoError(t, getErr)
					assert.Equal(t, int64(4), persisted.Generation)
					assert.Equal(t, "workload-rv-2", persisted.Spec.Resource.ResourceVersion)
					assert.Equal(t, int64(0), persisted.Status.SchedulerObservedGeneration)
					assert.Equal(t, tt.conditions, persisted.Status.Conditions)
					assert.Equal(t, "3", persisted.Annotations[acceptedComponentResultGenerationAnnotation])
					assert.True(t, isAcceptedComponentSchedulingSpecHashMatched(&persisted.Spec, persisted.Annotations))

					s.bindingLister = &fakeBindingLister{binding: persisted}
					assert.NoError(t, s.doScheduleBinding(binding.Namespace, binding.Name))
					result, getErr := client.WorkV1alpha2().ResourceBindings(binding.Namespace).Get(context.Background(), binding.Name, metav1.GetOptions{})
					assert.NoError(t, getErr)
					updated, actions = result.Status, client.Actions()
				} else {
					binding := &workv1alpha2.ClusterResourceBinding{
						ObjectMeta: metav1.ObjectMeta{Name: "crb", ResourceVersion: "7", Generation: 2, Annotations: annotations},
						Spec:       spec,
						Status:     status,
					}
					client := karmadafake.NewClientset(binding)
					simulateGenerationIncrementOnMainPatch(client, "clusterresourcebindings", 3, "8")
					simulateConcurrentConfigOnlyUpdateOnNextStatusPatch(t, client, "clusterresourcebindings", "8", "9", 4)
					s := &Scheduler{KarmadaClient: client, clusterBindingLister: &fakeClusterBindingLister{binding: binding}, clusterLister: testClusterLister(t, "cluster1"), Algorithm: algorithm, eventRecorder: record.NewFakeRecorder(10)}
					firstErr := s.doScheduleClusterBinding(binding.Name)
					assert.True(t, apierrors.IsConflict(firstErr), "expected resourceVersion conflict, got %v", firstErr)
					persisted, getErr := client.WorkV1alpha2().ClusterResourceBindings().Get(context.Background(), binding.Name, metav1.GetOptions{})
					assert.NoError(t, getErr)
					assert.Equal(t, int64(4), persisted.Generation)
					assert.Equal(t, "workload-rv-2", persisted.Spec.Resource.ResourceVersion)
					assert.Equal(t, int64(0), persisted.Status.SchedulerObservedGeneration)
					assert.Equal(t, tt.conditions, persisted.Status.Conditions)
					assert.Equal(t, "3", persisted.Annotations[acceptedComponentResultGenerationAnnotation])
					assert.True(t, isAcceptedComponentSchedulingSpecHashMatched(&persisted.Spec, persisted.Annotations))

					s.clusterBindingLister = &fakeClusterBindingLister{binding: persisted}
					assert.NoError(t, s.doScheduleClusterBinding(binding.Name))
					result, getErr := client.WorkV1alpha2().ClusterResourceBindings().Get(context.Background(), binding.Name, metav1.GetOptions{})
					assert.NoError(t, getErr)
					updated, actions = result.Status, client.Actions()
				}

				assert.Equal(t, 1, algorithmCalls)
				assert.Equal(t, int64(4), updated.SchedulerObservedGeneration)
				if assert.Len(t, updated.Conditions, 1) {
					assert.Equal(t, metav1.ConditionTrue, updated.Conditions[0].Status)
					assert.Equal(t, workv1alpha2.BindingReasonSuccess, updated.Conditions[0].Reason)
				}
				assertScalePatches(t, actions, true, "7")
				statusPatches := filterStatusPatches(actions)
				if assert.Len(t, statusPatches, 2) {
					assertPatchResourceVersion(t, statusPatches[0], "8")
					assertPatchResourceVersion(t, statusPatches[1], "9")
				}
			})
		}
	}
}

func TestAcceptedComponentStatusRepairConflictsWithConcurrentDetectorUpdate(t *testing.T) {
	defer setFeatureGateDuringTest(t, features.FeatureGate, features.MultiplePodTemplatesScheduling, true)()
	placement := &policyv1alpha1.Placement{SpreadConstraints: []policyv1alpha1.SpreadConstraint{{
		SpreadByField: policyv1alpha1.SpreadByFieldCluster, MinGroups: 1, MaxGroups: 1,
	}}}
	placementJSON, err := json.Marshal(placement)
	assert.NoError(t, err)
	components := []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}}
	clusters := []workv1alpha2.TargetCluster{{Name: "cluster1", Components: []workv1alpha2.TargetComponent{
		{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4},
	}}}
	lastScheduled := metav1.NewTime(time.Unix(1, 0))
	status := workv1alpha2.ResourceBindingStatus{
		SchedulerObservedGeneration: 2,
		LastScheduledTime:           &lastScheduled,
		Conditions: []metav1.Condition{
			util.NewCondition(workv1alpha2.Scheduled, workv1alpha2.BindingReasonSuccess, successfulSchedulingMessage, metav1.ConditionTrue),
		},
	}

	for _, resourceScoped := range []bool{true, false} {
		t.Run(fmt.Sprintf("resourceScoped=%t", resourceScoped), func(t *testing.T) {
			algorithmCalls := 0
			algorithm := &mockAlgorithm{scheduleFunc: func(_ context.Context, spec *workv1alpha2.ResourceBindingSpec, _ *workv1alpha2.ResourceBindingStatus, _ *core.ScheduleAlgorithmOption) (core.ScheduleResult, error) {
				algorithmCalls++
				return core.ScheduleResult{SuggestedClusters: spec.Clusters}, nil
			}}
			annotations := componentSchedulingAnnotations(t, string(placementJSON), components)
			annotations[acceptedComponentResultGenerationAnnotation] = "3"
			spec := workv1alpha2.ResourceBindingSpec{Placement: placement, Components: components, Clusters: clusters}
			acceptedSpecHash, hashErr := generateAcceptedComponentSchedulingSpecHash(&spec)
			assert.NoError(t, hashErr)
			annotations[acceptedComponentSchedulingSpecHashAnnotation] = acceptedSpecHash
			if resourceScoped {
				binding := &workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "rb", Namespace: "default", ResourceVersion: "8", Generation: 3, Annotations: annotations},
					Spec:       spec,
					Status:     status,
				}
				client := karmadafake.NewClientset(binding)
				simulateConcurrentDetectorUpdateOnNextStatusPatch(t, client, "resourcebindings", "8", "9", 4)
				s := &Scheduler{KarmadaClient: client, bindingLister: &fakeBindingLister{binding: binding}, clusterLister: testClusterLister(t, "cluster1"), Algorithm: algorithm, eventRecorder: record.NewFakeRecorder(10)}
				firstErr := s.doScheduleBinding(binding.Namespace, binding.Name)
				assert.True(t, apierrors.IsConflict(firstErr), "expected resourceVersion conflict, got %v", firstErr)
				persisted, getErr := client.WorkV1alpha2().ResourceBindings(binding.Namespace).Get(context.Background(), binding.Name, metav1.GetOptions{})
				assert.NoError(t, getErr)
				assert.Equal(t, int64(4), persisted.Generation)
				assert.Equal(t, int64(2), persisted.Status.SchedulerObservedGeneration)
				assert.Equal(t, lastScheduled, *persisted.Status.LastScheduledTime)
				assert.NotNil(t, persisted.Spec.RescheduleTriggeredAt)

				s.bindingLister = &fakeBindingLister{binding: persisted}
				assert.NoError(t, s.doScheduleBinding(binding.Namespace, binding.Name))
				updated, getErr := client.WorkV1alpha2().ResourceBindings(binding.Namespace).Get(context.Background(), binding.Name, metav1.GetOptions{})
				assert.NoError(t, getErr)
				assert.Equal(t, int64(4), updated.Status.SchedulerObservedGeneration)
				assert.Equal(t, "4", updated.Annotations[acceptedComponentResultGenerationAnnotation])
				assertScalePatches(t, client.Actions(), true, "9")
			} else {
				binding := &workv1alpha2.ClusterResourceBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "crb", ResourceVersion: "8", Generation: 3, Annotations: annotations},
					Spec:       spec,
					Status:     status,
				}
				client := karmadafake.NewClientset(binding)
				simulateConcurrentDetectorUpdateOnNextStatusPatch(t, client, "clusterresourcebindings", "8", "9", 4)
				s := &Scheduler{KarmadaClient: client, clusterBindingLister: &fakeClusterBindingLister{binding: binding}, clusterLister: testClusterLister(t, "cluster1"), Algorithm: algorithm, eventRecorder: record.NewFakeRecorder(10)}
				firstErr := s.doScheduleClusterBinding(binding.Name)
				assert.True(t, apierrors.IsConflict(firstErr), "expected resourceVersion conflict, got %v", firstErr)
				persisted, getErr := client.WorkV1alpha2().ClusterResourceBindings().Get(context.Background(), binding.Name, metav1.GetOptions{})
				assert.NoError(t, getErr)
				assert.Equal(t, int64(4), persisted.Generation)
				assert.Equal(t, int64(2), persisted.Status.SchedulerObservedGeneration)
				assert.Equal(t, lastScheduled, *persisted.Status.LastScheduledTime)
				assert.NotNil(t, persisted.Spec.RescheduleTriggeredAt)

				s.clusterBindingLister = &fakeClusterBindingLister{binding: persisted}
				assert.NoError(t, s.doScheduleClusterBinding(binding.Name))
				updated, getErr := client.WorkV1alpha2().ClusterResourceBindings().Get(context.Background(), binding.Name, metav1.GetOptions{})
				assert.NoError(t, getErr)
				assert.Equal(t, int64(4), updated.Status.SchedulerObservedGeneration)
				assert.Equal(t, "4", updated.Annotations[acceptedComponentResultGenerationAnnotation])
				assertScalePatches(t, client.Actions(), true, "9")
			}
			assert.Equal(t, 1, algorithmCalls)
		})
	}
}

func TestAcceptedComponentRollbackRepairsFailedTransition(t *testing.T) {
	defer setFeatureGateDuringTest(t, features.FeatureGate, features.MultiplePodTemplatesScheduling, true)()
	placement := &policyv1alpha1.Placement{
		ReplicaScheduling: &policyv1alpha1.ReplicaSchedulingStrategy{ReplicaSchedulingType: policyv1alpha1.ReplicaSchedulingTypeDivided},
		SpreadConstraints: []policyv1alpha1.SpreadConstraint{{
			SpreadByField: policyv1alpha1.SpreadByFieldCluster, MinGroups: 1, MaxGroups: 1,
		}},
	}
	placementJSON, err := json.Marshal(placement)
	assert.NoError(t, err)
	components := []workv1alpha2.Component{{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4}}
	clusters := []workv1alpha2.TargetCluster{{Name: "cluster1", Components: []workv1alpha2.TargetComponent{
		{Name: "jobmanager", Replicas: 1}, {Name: "taskmanager", Replicas: 4},
	}}}
	acceptedSpec := workv1alpha2.ResourceBindingSpec{
		Resource:   workv1alpha2.ObjectReference{APIVersion: "flink.apache.org/v1beta1", Kind: "FlinkDeployment", Name: "flink", ResourceVersion: "accepted-rv"},
		Placement:  placement,
		Components: components,
		Clusters:   clusters,
	}
	acceptedSpecHash, err := generateAcceptedComponentSchedulingSpecHash(&acceptedSpec)
	assert.NoError(t, err)
	rolledBackSpec := *acceptedSpec.DeepCopy()
	rolledBackSpec.Resource.ResourceVersion = "rollback-rv"
	rolledBackSpec.Components[0], rolledBackSpec.Components[1] = rolledBackSpec.Components[1], rolledBackSpec.Components[0]
	rolledBackSpec.Clusters[0].Components[0], rolledBackSpec.Clusters[0].Components[1] = rolledBackSpec.Clusters[0].Components[1], rolledBackSpec.Clusters[0].Components[0]
	failedCondition := util.NewCondition(
		workv1alpha2.Scheduled,
		workv1alpha2.BindingReasonNoClusterFit,
		componentTransitionFailureMessagePrefix+"insufficient resources",
		metav1.ConditionFalse,
	)

	for _, resourceScoped := range []bool{true, false} {
		t.Run(fmt.Sprintf("resourceScoped=%t", resourceScoped), func(t *testing.T) {
			algorithmCalls := 0
			algorithm := &mockAlgorithm{scheduleFunc: func(_ context.Context, _ *workv1alpha2.ResourceBindingSpec, _ *workv1alpha2.ResourceBindingStatus, _ *core.ScheduleAlgorithmOption) (core.ScheduleResult, error) {
				algorithmCalls++
				return core.ScheduleResult{}, nil
			}}
			annotations := componentSchedulingAnnotations(t, string(placementJSON), components)
			annotations[acceptedComponentResultGenerationAnnotation] = "1"
			annotations[acceptedComponentSchedulingSpecHashAnnotation] = acceptedSpecHash
			status := workv1alpha2.ResourceBindingStatus{SchedulerObservedGeneration: 1, Conditions: []metav1.Condition{failedCondition}}
			if resourceScoped {
				binding := &workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "rb", Namespace: "default", ResourceVersion: "7", Generation: 3, Annotations: annotations},
					Spec:       rolledBackSpec,
					Status:     status,
				}
				client := karmadafake.NewClientset(binding)
				s := &Scheduler{KarmadaClient: client, bindingLister: &fakeBindingLister{binding: binding}, clusterLister: testClusterLister(t, "cluster1"), Algorithm: algorithm, eventRecorder: record.NewFakeRecorder(10)}
				assert.NoError(t, s.doScheduleBinding(binding.Namespace, binding.Name))
				updated, getErr := client.WorkV1alpha2().ResourceBindings(binding.Namespace).Get(context.Background(), binding.Name, metav1.GetOptions{})
				assert.NoError(t, getErr)
				assert.Equal(t, int64(3), updated.Status.SchedulerObservedGeneration)
				assert.Equal(t, metav1.ConditionTrue, updated.Status.Conditions[0].Status)
				assert.Empty(t, filterMainResourcePatches(client.Actions()))
			} else {
				binding := &workv1alpha2.ClusterResourceBinding{
					ObjectMeta: metav1.ObjectMeta{Name: "crb", ResourceVersion: "7", Generation: 3, Annotations: annotations},
					Spec:       rolledBackSpec,
					Status:     status,
				}
				client := karmadafake.NewClientset(binding)
				s := &Scheduler{KarmadaClient: client, clusterBindingLister: &fakeClusterBindingLister{binding: binding}, clusterLister: testClusterLister(t, "cluster1"), Algorithm: algorithm, eventRecorder: record.NewFakeRecorder(10)}
				assert.NoError(t, s.doScheduleClusterBinding(binding.Name))
				updated, getErr := client.WorkV1alpha2().ClusterResourceBindings().Get(context.Background(), binding.Name, metav1.GetOptions{})
				assert.NoError(t, getErr)
				assert.Equal(t, int64(3), updated.Status.SchedulerObservedGeneration)
				assert.Equal(t, metav1.ConditionTrue, updated.Status.Conditions[0].Status)
				assert.Empty(t, filterMainResourcePatches(client.Actions()))
			}
			assert.Zero(t, algorithmCalls)
		})
	}
}

func TestWorkerAndScheduleNext(t *testing.T) {
	testScheme := setupScheme()

	resourceBinding := &workv1alpha2.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-binding",
			Namespace: "default",
		},
		Spec: workv1alpha2.ResourceBindingSpec{
			Placement: &policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{"cluster1"},
				},
			},
		},
	}

	clusterResourceBinding := &workv1alpha2.ClusterResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cluster-binding",
		},
		Spec: workv1alpha2.ResourceBindingSpec{
			Placement: &policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{"cluster1"},
				},
			},
		},
	}

	fakeClient := karmadafake.NewClientset(resourceBinding, clusterResourceBinding)

	testCases := []struct {
		name         string
		key          string
		priority     int32
		shutdown     bool
		expectResult bool
	}{
		{
			name:         "Schedule ResourceBinding",
			key:          "default/test-binding",
			priority:     10,
			shutdown:     false,
			expectResult: true,
		},
		{
			name:         "Schedule ClusterResourceBinding",
			key:          "test-cluster-binding",
			priority:     5,
			shutdown:     false,
			expectResult: true,
		},
	}

	// enable "PriorityBasedScheduling" feature gate.
	_ = features.FeatureGate.Set("PriorityBasedScheduling=true")
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			queue := internalqueue.NewSchedulingQueue()
			bindingLister := &fakeBindingLister{binding: resourceBinding}
			clusterBindingLister := &fakeClusterBindingLister{binding: clusterResourceBinding}

			mockAlgo := &mockAlgorithm{
				scheduleFunc: func(_ context.Context, _ *workv1alpha2.ResourceBindingSpec, _ *workv1alpha2.ResourceBindingStatus, _ *core.ScheduleAlgorithmOption) (core.ScheduleResult, error) {
					return core.ScheduleResult{
						SuggestedClusters: []workv1alpha2.TargetCluster{
							{Name: "cluster1", Replicas: 1},
						},
					}, nil
				},
			}

			eventBroadcaster := record.NewBroadcaster()
			eventRecorder := eventBroadcaster.NewRecorder(testScheme, corev1.EventSource{Component: "test-scheduler"})

			s := &Scheduler{
				KarmadaClient:        fakeClient,
				priorityQueue:        queue,
				bindingLister:        bindingLister,
				clusterBindingLister: clusterBindingLister,
				Algorithm:            mockAlgo,
				eventRecorder:        eventRecorder,
			}

			s.priorityQueue.Push(&internalqueue.QueuedBindingInfo{
				NamespacedKey: tc.key,
				Priority:      tc.priority,
			})

			if tc.shutdown {
				s.priorityQueue.Close()
			}

			result := s.scheduleNext()

			assert.Equal(t, tc.expectResult, result, "scheduleNext return value mismatch")

			if !tc.shutdown {
				assert.Equal(t, 0, s.priorityQueue.Len(), "Queue should be empty after processing")
			}
		})
	}
}

func TestPlacementChanged(t *testing.T) {
	tests := []struct {
		name                 string
		placement            *policyv1alpha1.Placement
		appliedPlacementStr  string
		observedAffinityName string
		want                 bool
	}{
		{
			name: "placement changed",
			placement: &policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{"cluster1", "cluster2"},
				},
			},
			appliedPlacementStr:  `{"clusterAffinity":{"clusterNames":["cluster1"]}}`,
			observedAffinityName: "",
			want:                 true,
		},
		{
			name: "placement not changed",
			placement: &policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{"cluster1", "cluster2"},
				},
			},
			appliedPlacementStr:  `{"clusterAffinity":{"clusterNames":["cluster1","cluster2"]}}`,
			observedAffinityName: "",
			want:                 false,
		},
		{
			name: "invalid applied placement string",
			placement: &policyv1alpha1.Placement{
				ClusterAffinity: &policyv1alpha1.ClusterAffinity{
					ClusterNames: []string{"cluster1", "cluster2"},
				},
			},
			appliedPlacementStr:  `invalid json`,
			observedAffinityName: "",
			want:                 true,
		},
		{
			name:                 "empty placement",
			placement:            &policyv1alpha1.Placement{},
			appliedPlacementStr:  `{}`,
			observedAffinityName: "",
			want:                 false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rb := &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-name",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Placement: tt.placement,
				},
				Status: workv1alpha2.ResourceBindingStatus{
					SchedulerObservedAffinityName: tt.observedAffinityName,
				},
			}
			got := placementChanged(*rb.Spec.Placement, tt.appliedPlacementStr, rb.Status.SchedulerObservedAffinityName)
			assert.Equal(t, tt.want, got, "placementChanged() result mismatch")
		})
	}
}

func TestCreateScheduler(t *testing.T) {
	dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	karmadaClient := karmadafake.NewClientset()
	kubeClient := fake.NewClientset()
	port := 10025
	serviceNamespace := "tenant1"
	servicePrefix := "test-service-prefix"
	schedulerName := "test-scheduler"
	timeout := metav1.Duration{Duration: 5 * time.Second}

	mockPlugins := []string{"plugin1", "plugin2"}
	mockRateLimiterOptions := ratelimiterflag.Options{}

	testcases := []struct {
		name                                string
		opts                                []Option
		enableSchedulerEstimator            bool
		schedulerEstimatorPort              int
		disableSchedulerEstimatorInPullMode bool
		schedulerEstimatorTimeout           metav1.Duration
		schedulerEstimatorServiceNamespace  string
		schedulerEstimatorServicePrefix     string
		schedulerName                       string
		schedulerEstimatorClientConfig      *grpcconnection.ClientConfig
		enableEmptyWorkloadPropagation      bool
		plugins                             []string
		rateLimiterOptions                  ratelimiterflag.Options
		enablePriorityBasedScheduling       bool
	}{
		{
			name:                     "scheduler with default configuration",
			opts:                     nil,
			enableSchedulerEstimator: false,
		},
		{
			name: "scheduler with enableSchedulerEstimator enabled",
			opts: []Option{
				WithEnableSchedulerEstimator(true),
				WithSchedulerEstimatorConnection(port, "", "", "", false),
			},
			enableSchedulerEstimator: true,
			schedulerEstimatorPort:   port,
		},
		{
			name: "scheduler with enableSchedulerEstimator disabled, WithSchedulerEstimatorConnection enabled",
			opts: []Option{
				WithEnableSchedulerEstimator(false),
				WithSchedulerEstimatorConnection(port, "", "", "", false),
			},
			enableSchedulerEstimator: false,
		},
		{
			name: "scheduler with disableSchedulerEstimatorInPullMode enabled",
			opts: []Option{
				WithEnableSchedulerEstimator(true),
				WithSchedulerEstimatorConnection(port, "", "", "", false),
				WithDisableSchedulerEstimatorInPullMode(true),
			},
			enableSchedulerEstimator:            true,
			schedulerEstimatorPort:              port,
			disableSchedulerEstimatorInPullMode: true,
		},
		{
			name: "scheduler with SchedulerEstimatorServicePrefix enabled",
			opts: []Option{
				WithEnableSchedulerEstimator(true),
				WithSchedulerEstimatorConnection(port, "", "", "", false),
				WithSchedulerEstimatorServicePrefix(servicePrefix),
			},
			enableSchedulerEstimator:        true,
			schedulerEstimatorPort:          port,
			schedulerEstimatorServicePrefix: servicePrefix,
		},
		{
			name: "scheduler with custom SchedulerEstimatorServiceNamespace set",
			opts: []Option{
				WithEnableSchedulerEstimator(true),
				WithSchedulerEstimatorConnection(port, "", "", "", false),
				WithSchedulerEstimatorServiceNamespace(serviceNamespace),
			},
			enableSchedulerEstimator:           true,
			schedulerEstimatorPort:             port,
			schedulerEstimatorServiceNamespace: serviceNamespace,
		},
		{
			name: "scheduler with SchedulerName enabled",
			opts: []Option{
				WithSchedulerName(schedulerName),
			},
			schedulerName: schedulerName,
		},
		{
			name: "scheduler with EnableEmptyWorkloadPropagation enabled",
			opts: []Option{
				WithEnableEmptyWorkloadPropagation(true),
			},
			enableEmptyWorkloadPropagation: true,
		},
		{
			name: "scheduler with SchedulerEstimatorTimeout enabled",
			opts: []Option{
				WithEnableSchedulerEstimator(true),
				WithSchedulerEstimatorConnection(port, "", "", "", false),
				WithSchedulerEstimatorTimeout(timeout),
			},
			enableSchedulerEstimator:  true,
			schedulerEstimatorPort:    port,
			schedulerEstimatorTimeout: timeout,
		},
		{
			name: "scheduler with EnableSchedulerPlugin",
			opts: []Option{
				WithEnableSchedulerPlugin(mockPlugins),
			},
			plugins: mockPlugins,
		},
		{
			name: "scheduler with PriorityBasedScheduling enabled",
			opts: []Option{
				WithRateLimiterOptions(mockRateLimiterOptions),
			},
			enablePriorityBasedScheduling: true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			sche, err := NewScheduler(dynamicClient, karmadaClient, kubeClient, tc.opts...)
			if err != nil {
				t.Errorf("create scheduler error: %s", err)
			}

			if tc.enableSchedulerEstimator != sche.enableSchedulerEstimator {
				t.Errorf("unexpected enableSchedulerEstimator want %v, got %v", tc.enableSchedulerEstimator, sche.enableSchedulerEstimator)
			}

			if tc.enableSchedulerEstimator && tc.schedulerEstimatorPort != sche.schedulerEstimatorClientConfig.TargetPort {
				t.Errorf("unexpected schedulerEstimatorPort want %v, got %v", tc.schedulerEstimatorPort, sche.schedulerEstimatorClientConfig.TargetPort)
			}

			if tc.disableSchedulerEstimatorInPullMode != sche.disableSchedulerEstimatorInPullMode {
				t.Errorf("unexpected disableSchedulerEstimatorInPullMode want %v, got %v", tc.disableSchedulerEstimatorInPullMode, sche.disableSchedulerEstimatorInPullMode)
			}

			if tc.schedulerEstimatorServiceNamespace != sche.schedulerEstimatorServiceNamespace {
				t.Errorf("unexpected schedulerEstimatorServiceNamespace want %v, got %v", tc.schedulerEstimatorServiceNamespace, sche.schedulerEstimatorServiceNamespace)
			}

			if tc.schedulerEstimatorServicePrefix != sche.schedulerEstimatorServicePrefix {
				t.Errorf("unexpected schedulerEstimatorServicePrefix want %v, got %v", tc.schedulerEstimatorServicePrefix, sche.schedulerEstimatorServicePrefix)
			}

			if tc.schedulerName != sche.schedulerName {
				t.Errorf("unexpected schedulerName want %v, got %v", tc.schedulerName, sche.schedulerName)
			}

			if tc.enableEmptyWorkloadPropagation != sche.enableEmptyWorkloadPropagation {
				t.Errorf("unexpected enableEmptyWorkloadPropagation want %v, got %v", tc.enableEmptyWorkloadPropagation, sche.enableEmptyWorkloadPropagation)
			}
			if len(tc.plugins) > 0 && sche.Algorithm == nil {
				t.Errorf("expected Algorithm to be set when plugins are provided")
			}
			if tc.enablePriorityBasedScheduling && sche.priorityQueue == nil {
				t.Errorf("expected priorityQueue to be set when feature gate %q is enabled", features.PriorityBasedScheduling)
			}
		})
	}
}

func TestPatchBindingStatusCondition(t *testing.T) {
	oneHourBefore := time.Now().Add(-1 * time.Hour).Round(time.Second)
	oneHourAfter := time.Now().Add(1 * time.Hour).Round(time.Second)

	successCondition := util.NewCondition(workv1alpha2.Scheduled, workv1alpha2.BindingReasonSuccess, successfulSchedulingMessage, metav1.ConditionTrue)
	failureCondition := util.NewCondition(workv1alpha2.Scheduled, workv1alpha2.BindingReasonSchedulerError, "schedule error", metav1.ConditionFalse)
	noClusterFitCondition := util.NewCondition(workv1alpha2.Scheduled, workv1alpha2.BindingReasonNoClusterFit, "0/0 clusters are available", metav1.ConditionFalse)
	unschedulableCondition := util.NewCondition(workv1alpha2.Scheduled, workv1alpha2.BindingReasonUnschedulable, "insufficient resources in the clusters", metav1.ConditionFalse)

	successCondition.LastTransitionTime = metav1.Time{Time: oneHourBefore}
	failureCondition.LastTransitionTime = metav1.Time{Time: oneHourAfter}
	noClusterFitCondition.LastTransitionTime = metav1.Time{Time: oneHourAfter}
	unschedulableCondition.LastTransitionTime = metav1.Time{Time: oneHourAfter}

	karmadaClient := karmadafake.NewClientset()

	tests := []struct {
		name                  string
		binding               *workv1alpha2.ResourceBinding
		newScheduledCondition metav1.Condition
		expected              *workv1alpha2.ResourceBinding
	}{
		{
			name: "add success condition",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-1", Namespace: "default", Generation: 1},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{},
			},
			newScheduledCondition: successCondition,
			expected: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-1", Namespace: "default", Generation: 1},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{successCondition}, SchedulerObservedGeneration: 1},
			},
		},
		{
			name: "add failure condition",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-2", Namespace: "default"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{},
			},
			newScheduledCondition: failureCondition,
			expected: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-2", Namespace: "default"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{failureCondition}},
			},
		},
		{
			name: "add no cluster available condition",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-3", Namespace: "default"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{},
			},
			newScheduledCondition: noClusterFitCondition,
			expected: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-3", Namespace: "default"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{noClusterFitCondition}},
			},
		},
		{
			name: "add unschedulable condition",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-4", Namespace: "default"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{},
			},
			newScheduledCondition: unschedulableCondition,
			expected: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-4", Namespace: "default"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{unschedulableCondition}},
			},
		},
		{
			name: "replace to success condition",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-5", Namespace: "default", Generation: 1},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{failureCondition}, SchedulerObservedGeneration: 2},
			},
			newScheduledCondition: successCondition,
			expected: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-5", Namespace: "default"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{successCondition}, SchedulerObservedGeneration: 1},
			},
		},
		{
			name: "replace failure condition",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-6", Namespace: "default"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{successCondition}},
			},
			newScheduledCondition: failureCondition,
			expected: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-6", Namespace: "default"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{failureCondition}},
			},
		},
		{
			name: "replace to unschedulable condition",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-7", Namespace: "default"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{failureCondition}},
			},
			newScheduledCondition: unschedulableCondition,
			expected: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-7", Namespace: "default"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{unschedulableCondition}},
			},
		},
		{
			name: "replace to no cluster fit condition",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-8", Namespace: "default"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{failureCondition}},
			},
			newScheduledCondition: noClusterFitCondition,
			expected: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-8", Namespace: "default"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{noClusterFitCondition}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := karmadaClient.WorkV1alpha2().ResourceBindings(test.binding.Namespace).Create(context.TODO(), test.binding, metav1.CreateOptions{})
			if err != nil {
				t.Fatal(err)
			}
			err = patchBindingStatusCondition(karmadaClient, test.binding, test.newScheduledCondition)
			if err != nil {
				t.Error(err)
			}
			res, err := karmadaClient.WorkV1alpha2().ResourceBindings(test.binding.Namespace).Get(context.TODO(), test.binding.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			res.Status.LastScheduledTime = nil
			if !reflect.DeepEqual(res.Status, test.expected.Status) {
				t.Errorf("expected status: %v, but got: %v", test.expected.Status, res.Status)
			}
		})
	}
}

func TestPatchBindingStatusWithAffinityName(t *testing.T) {
	karmadaClient := karmadafake.NewClientset()

	tests := []struct {
		name         string
		binding      *workv1alpha2.ResourceBinding
		affinityName string
		expected     *workv1alpha2.ResourceBinding
	}{
		{
			name: "add affinityName in status",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-1", Namespace: "default", Generation: 1},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{},
			},
			affinityName: "group1",
			expected: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-1", Namespace: "default", Generation: 1},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{SchedulerObservedAffinityName: "group1"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := karmadaClient.WorkV1alpha2().ResourceBindings(test.binding.Namespace).Create(context.TODO(), test.binding, metav1.CreateOptions{})
			if err != nil {
				t.Fatal(err)
			}
			err = patchBindingStatusWithAffinityName(karmadaClient, test.binding, test.affinityName)
			if err != nil {
				t.Error(err)
			}
			res, err := karmadaClient.WorkV1alpha2().ResourceBindings(test.binding.Namespace).Get(context.TODO(), test.binding.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(res.Status, test.expected.Status) {
				t.Errorf("expected status: %v, but got: %v", test.expected.Status, res.Status)
			}
		})
	}
}

func TestPatchClusterBindingStatusCondition(t *testing.T) {
	oneHourBefore := time.Now().Add(-1 * time.Hour).Round(time.Second)
	oneHourAfter := time.Now().Add(1 * time.Hour).Round(time.Second)

	successCondition := util.NewCondition(workv1alpha2.Scheduled, workv1alpha2.BindingReasonSuccess, successfulSchedulingMessage, metav1.ConditionTrue)
	failureCondition := util.NewCondition(workv1alpha2.Scheduled, workv1alpha2.BindingReasonSchedulerError, "schedule error", metav1.ConditionFalse)
	noClusterFitCondition := util.NewCondition(workv1alpha2.Scheduled, workv1alpha2.BindingReasonNoClusterFit, "0/0 clusters are available", metav1.ConditionFalse)
	unschedulableCondition := util.NewCondition(workv1alpha2.Scheduled, workv1alpha2.BindingReasonUnschedulable, "insufficient resources in the clusters", metav1.ConditionFalse)

	successCondition.LastTransitionTime = metav1.Time{Time: oneHourBefore}
	failureCondition.LastTransitionTime = metav1.Time{Time: oneHourAfter}
	noClusterFitCondition.LastTransitionTime = metav1.Time{Time: oneHourAfter}
	unschedulableCondition.LastTransitionTime = metav1.Time{Time: oneHourAfter}

	karmadaClient := karmadafake.NewClientset()

	tests := []struct {
		name                  string
		binding               *workv1alpha2.ClusterResourceBinding
		newScheduledCondition metav1.Condition
		expected              *workv1alpha2.ClusterResourceBinding
	}{
		{
			name: "add success condition",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-1", Generation: 1},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{},
			},
			newScheduledCondition: successCondition,
			expected: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-1"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{successCondition}, SchedulerObservedGeneration: 1},
			},
		},
		{
			name: "add failure condition",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-2"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{},
			},
			newScheduledCondition: failureCondition,
			expected: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-2"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{failureCondition}},
			},
		},
		{
			name: "add unschedulable condition",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-3"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{},
			},
			newScheduledCondition: unschedulableCondition,
			expected: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-3"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{unschedulableCondition}},
			},
		},
		{
			name: "add no cluster fit condition",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-4"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{},
			},
			newScheduledCondition: noClusterFitCondition,
			expected: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-4"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{noClusterFitCondition}},
			},
		},
		{
			name: "replace to success condition",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-5", Generation: 1},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{failureCondition}, SchedulerObservedGeneration: 2},
			},
			newScheduledCondition: successCondition,
			expected: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-5"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{successCondition}, SchedulerObservedGeneration: 1},
			},
		},
		{
			name: "replace failure condition",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-6"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{successCondition}},
			},
			newScheduledCondition: failureCondition,
			expected: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-6"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{failureCondition}},
			},
		},
		{
			name: "replace to unschedulable condition",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-7"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{failureCondition}},
			},
			newScheduledCondition: unschedulableCondition,
			expected: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-7"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{unschedulableCondition}},
			},
		},
		{
			name: "replace to no cluster fit condition",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-8"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{failureCondition}},
			},
			newScheduledCondition: noClusterFitCondition,
			expected: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-8"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{noClusterFitCondition}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := karmadaClient.WorkV1alpha2().ClusterResourceBindings().Create(context.TODO(), test.binding, metav1.CreateOptions{})
			if err != nil {
				t.Fatal(err)
			}
			err = patchClusterBindingStatusCondition(karmadaClient, test.binding, test.newScheduledCondition)
			if err != nil {
				t.Error(err)
			}
			res, err := karmadaClient.WorkV1alpha2().ClusterResourceBindings().Get(context.TODO(), test.binding.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			res.Status.LastScheduledTime = nil
			if !reflect.DeepEqual(res.Status, test.expected.Status) {
				t.Errorf("expected status: %v, but got: %v", test.expected.Status, res.Status)
			}
		})
	}
}

func TestPatchClusterBindingStatusWithAffinityName(t *testing.T) {
	karmadaClient := karmadafake.NewClientset()

	tests := []struct {
		name         string
		binding      *workv1alpha2.ClusterResourceBinding
		affinityName string
		expected     *workv1alpha2.ClusterResourceBinding
	}{
		{
			name: "add affinityName in status",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "crb-1", Generation: 1},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status: workv1alpha2.ResourceBindingStatus{
					Conditions:                  []metav1.Condition{util.NewCondition(workv1alpha2.Scheduled, workv1alpha2.BindingReasonSuccess, successfulSchedulingMessage, metav1.ConditionTrue)},
					SchedulerObservedGeneration: 1,
				},
			},
			affinityName: "group1",
			expected: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "crb-1"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status: workv1alpha2.ResourceBindingStatus{
					SchedulerObservedAffinityName: "group1",
					Conditions:                    []metav1.Condition{util.NewCondition(workv1alpha2.Scheduled, workv1alpha2.BindingReasonSuccess, successfulSchedulingMessage, metav1.ConditionTrue)},
					SchedulerObservedGeneration:   1,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := karmadaClient.WorkV1alpha2().ClusterResourceBindings().Create(context.TODO(), test.binding, metav1.CreateOptions{})
			if err != nil {
				t.Fatal(err)
			}
			err = patchClusterBindingStatusWithAffinityName(karmadaClient, test.binding, test.affinityName)
			if err != nil {
				t.Error(err)
			}
			res, err := karmadaClient.WorkV1alpha2().ClusterResourceBindings().Get(context.TODO(), test.binding.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(res.Status, test.expected.Status) {
				t.Errorf("expected status: %v, but got: %v", test.expected.Status, res.Status)
			}
		})
	}
}

func TestRecordScheduleResultEventForResourceBinding(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(10)
	scheduler := &Scheduler{eventRecorder: fakeRecorder}

	tests := []struct {
		name           string
		rb             *workv1alpha2.ResourceBinding
		scheduleResult []workv1alpha2.TargetCluster
		schedulerErr   error
		expectedEvents int
		expectedMsg    string
	}{
		{
			name:           "nil ResourceBinding",
			rb:             nil,
			scheduleResult: nil,
			schedulerErr:   nil,
			expectedEvents: 0,
			expectedMsg:    "",
		},
		{
			name: "successful scheduling",
			rb: &workv1alpha2.ResourceBinding{
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						Kind:       "Deployment",
						APIVersion: "apps/v1",
						Namespace:  "default",
						Name:       "test-deployment",
						UID:        "12345",
					},
				},
			},
			scheduleResult: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 1},
				{Name: "cluster2", Replicas: 2},
			},
			schedulerErr:   nil,
			expectedEvents: 2,
			expectedMsg: fmt.Sprintf("%s Result: {%s}", successfulSchedulingMessage, targetClustersToString([]workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 1},
				{Name: "cluster2", Replicas: 2},
			}))},
		{
			name: "scheduling error",
			rb: &workv1alpha2.ResourceBinding{
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						Kind:       "Deployment",
						APIVersion: "apps/v1",
						Namespace:  "default",
						Name:       "test-deployment",
						UID:        "12345",
					},
				},
			},
			scheduleResult: nil,
			schedulerErr:   fmt.Errorf("scheduling error"),
			expectedEvents: 2,
			expectedMsg:    "scheduling error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeRecorder.Events = make(chan string, 10)

			scheduler.recordScheduleResultEventForResourceBinding(test.rb, test.scheduleResult, test.schedulerErr)

			if len(fakeRecorder.Events) != test.expectedEvents {
				t.Errorf("expected %d events, got %d", test.expectedEvents, len(fakeRecorder.Events))
			}

			for i := 0; i < test.expectedEvents; i++ {
				select {
				case event := <-fakeRecorder.Events:
					if !contains(event, test.expectedMsg) {
						t.Errorf("expected event message to contain %q, got %q", test.expectedMsg, event)
					}
				default:
					t.Error("expected event not found")
				}
			}
		})
	}
}

func contains(event, msg string) bool {
	return len(event) >= len(msg) && event[len(event)-len(msg):] == msg
}

func TestRecordScheduleResultEventForClusterResourceBinding(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(10)
	scheduler := &Scheduler{eventRecorder: fakeRecorder}

	tests := []struct {
		name           string
		crb            *workv1alpha2.ClusterResourceBinding
		scheduleResult []workv1alpha2.TargetCluster
		schedulerErr   error
		expectedEvents int
		expectedMsg    string
	}{
		{
			name:           "nil ClusterResourceBinding",
			crb:            nil,
			scheduleResult: nil,
			schedulerErr:   nil,
			expectedEvents: 0,
			expectedMsg:    "",
		},
		{
			name: "successful scheduling",
			crb: &workv1alpha2.ClusterResourceBinding{
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						Kind:       "Deployment",
						APIVersion: "apps/v1",
						Namespace:  "default",
						Name:       "test-deployment",
						UID:        "12345",
					},
				},
			},
			scheduleResult: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 1},
				{Name: "cluster2", Replicas: 2},
			},
			schedulerErr:   nil,
			expectedEvents: 2,
			expectedMsg: fmt.Sprintf("%s Result {%s}", successfulSchedulingMessage, targetClustersToString([]workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 1},
				{Name: "cluster2", Replicas: 2},
			})),
		},
		{
			name: "scheduling error",
			crb: &workv1alpha2.ClusterResourceBinding{
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						Kind:       "Deployment",
						APIVersion: "apps/v1",
						Namespace:  "default",
						Name:       "test-deployment",
						UID:        "12345",
					},
				},
			},
			scheduleResult: nil,
			schedulerErr:   fmt.Errorf("scheduling error"),
			expectedEvents: 2,
			expectedMsg:    "scheduling error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeRecorder.Events = make(chan string, 10)

			scheduler.recordScheduleResultEventForClusterResourceBinding(test.crb, test.scheduleResult, test.schedulerErr)

			if len(fakeRecorder.Events) != test.expectedEvents {
				t.Errorf("expected %d events, got %d", test.expectedEvents, len(fakeRecorder.Events))
			}

			for i := 0; i < test.expectedEvents; i++ {
				select {
				case event := <-fakeRecorder.Events:
					if !contains(event, test.expectedMsg) {
						t.Errorf("expected event message to contain %q, got %q", test.expectedMsg, event)
					}
				default:
					t.Error("expected event not found")
				}
			}
		})
	}
}

func TestTargetClustersToString(t *testing.T) {
	tests := []struct {
		name           string
		tcs            []workv1alpha2.TargetCluster
		expectedOutput string
	}{
		{
			name:           "empty slice",
			tcs:            []workv1alpha2.TargetCluster{},
			expectedOutput: "",
		},
		{
			name: "single cluster",
			tcs: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 1},
			},
			expectedOutput: "cluster1:1",
		},
		{
			name: "multiple clusters",
			tcs: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 1},
				{Name: "cluster2", Replicas: 2},
			},
			expectedOutput: "cluster1:1, cluster2:2",
		},
		{
			name: "clusters with zero replicas",
			tcs: []workv1alpha2.TargetCluster{
				{Name: "cluster1", Replicas: 0},
				{Name: "cluster2", Replicas: 2},
			},
			expectedOutput: "cluster1:0, cluster2:2",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := targetClustersToString(test.tcs)
			if result != test.expectedOutput {
				t.Errorf("expected %q, got %q", test.expectedOutput, result)
			}
		})
	}
}

// Helper Functions

// Helper function to setup scheme for testing
func setupScheme() *runtime.Scheme {
	s := runtime.NewScheme()

	_ = scheme.AddToScheme(s)
	_ = workv1alpha2.Install(s)
	_ = policyv1alpha1.Install(s)

	return s
}

// Helper function to filter patch actions
func filterPatchActions(actions []clienttesting.Action) []clienttesting.PatchAction {
	var patchActions []clienttesting.PatchAction
	for _, action := range actions {
		if patch, ok := action.(clienttesting.PatchAction); ok {
			patchActions = append(patchActions, patch)
		}
	}
	return patchActions
}

func filterMainResourcePatches(actions []clienttesting.Action) []clienttesting.PatchAction {
	var patchActions []clienttesting.PatchAction
	for _, action := range actions {
		patch, ok := action.(clienttesting.PatchAction)
		if ok && action.GetSubresource() == "" {
			patchActions = append(patchActions, patch)
		}
	}
	return patchActions
}

func filterStatusPatches(actions []clienttesting.Action) []clienttesting.PatchAction {
	var patchActions []clienttesting.PatchAction
	for _, action := range actions {
		patch, ok := action.(clienttesting.PatchAction)
		if ok && action.GetSubresource() == "status" {
			patchActions = append(patchActions, patch)
		}
	}
	return patchActions
}

func simulateGenerationIncrementOnMainPatch(client *karmadafake.Clientset, resource string, generation int64, resourceVersion string) {
	client.PrependReactor("patch", resource, func(action clienttesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "" {
			return false, nil, nil
		}
		patchAction, ok := action.(clienttesting.PatchAction)
		if !ok {
			return true, nil, fmt.Errorf("expected patch action, got %T", action)
		}
		object, err := client.Tracker().Get(action.GetResource(), action.GetNamespace(), patchAction.GetName())
		if err != nil {
			return true, nil, err
		}
		original, err := json.Marshal(object)
		if err != nil {
			return true, nil, err
		}
		patched, err := jsonpatch.MergePatch(original, patchAction.GetPatch())
		if err != nil {
			return true, nil, err
		}
		switch binding := object.(type) {
		case *workv1alpha2.ResourceBinding:
			updated := binding.DeepCopy()
			if err := json.Unmarshal(patched, updated); err != nil {
				return true, nil, err
			}
			updated.Generation = generation
			updated.ResourceVersion = resourceVersion
			object = updated
		case *workv1alpha2.ClusterResourceBinding:
			updated := binding.DeepCopy()
			if err := json.Unmarshal(patched, updated); err != nil {
				return true, nil, err
			}
			updated.Generation = generation
			updated.ResourceVersion = resourceVersion
			object = updated
		default:
			return true, nil, fmt.Errorf("unexpected object type %T", object)
		}
		if err := client.Tracker().Update(action.GetResource(), object, action.GetNamespace()); err != nil {
			return true, nil, err
		}
		return true, object, nil
	})
}

func failNextStatusPatch(t *testing.T, client *karmadafake.Clientset, resource, expectedResourceVersion string) {
	t.Helper()
	failed := false
	client.PrependReactor("patch", resource, func(action clienttesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "status" || failed {
			return false, nil, nil
		}
		failed = true
		patchAction, ok := action.(clienttesting.PatchAction)
		if !ok {
			return true, nil, fmt.Errorf("expected patch action, got %T", action)
		}
		assertPatchResourceVersion(t, patchAction, expectedResourceVersion)
		return true, nil, errors.New("injected status patch failure")
	})
}

func simulateConcurrentDetectorUpdateOnNextStatusPatch(t *testing.T, client *karmadafake.Clientset, resource, expectedResourceVersion, updatedResourceVersion string, updatedGeneration int64) {
	triggeredAt := metav1.NewTime(time.Unix(2, 0))
	simulateConcurrentSpecUpdateOnNextStatusPatch(t, client, resource, expectedResourceVersion, updatedResourceVersion, updatedGeneration, func(spec *workv1alpha2.ResourceBindingSpec) {
		spec.RescheduleTriggeredAt = &triggeredAt
	})
}

func simulateConcurrentConfigOnlyUpdateOnNextStatusPatch(t *testing.T, client *karmadafake.Clientset, resource, expectedResourceVersion, updatedResourceVersion string, updatedGeneration int64) {
	simulateConcurrentSpecUpdateOnNextStatusPatch(t, client, resource, expectedResourceVersion, updatedResourceVersion, updatedGeneration, func(spec *workv1alpha2.ResourceBindingSpec) {
		spec.Resource.ResourceVersion = "workload-rv-2"
	})
}

func simulateConcurrentSpecUpdateOnNextStatusPatch(t *testing.T, client *karmadafake.Clientset, resource, expectedResourceVersion, updatedResourceVersion string, updatedGeneration int64, updateSpec func(*workv1alpha2.ResourceBindingSpec)) {
	t.Helper()
	updatedOnce := false
	client.PrependReactor("patch", resource, func(action clienttesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "status" || updatedOnce {
			return false, nil, nil
		}
		updatedOnce = true
		patchAction, ok := action.(clienttesting.PatchAction)
		if !ok {
			return true, nil, fmt.Errorf("expected patch action, got %T", action)
		}
		assertPatchResourceVersion(t, patchAction, expectedResourceVersion)
		object, err := client.Tracker().Get(action.GetResource(), action.GetNamespace(), patchAction.GetName())
		if err != nil {
			return true, nil, err
		}
		switch binding := object.(type) {
		case *workv1alpha2.ResourceBinding:
			updated := binding.DeepCopy()
			updated.Generation = updatedGeneration
			updated.ResourceVersion = updatedResourceVersion
			updateSpec(&updated.Spec)
			object = updated
		case *workv1alpha2.ClusterResourceBinding:
			updated := binding.DeepCopy()
			updated.Generation = updatedGeneration
			updated.ResourceVersion = updatedResourceVersion
			updateSpec(&updated.Spec)
			object = updated
		default:
			return true, nil, fmt.Errorf("unexpected object type %T", object)
		}
		if err := client.Tracker().Update(action.GetResource(), object, action.GetNamespace()); err != nil {
			return true, nil, err
		}
		return true, nil, apierrors.NewConflict(action.GetResource().GroupResource(), patchAction.GetName(), errors.New("concurrent detector update"))
	})
}

func assertScalePatches(t *testing.T, actions []clienttesting.Action, wantPatched bool, resourceVersion string) {
	t.Helper()
	patches := filterMainResourcePatches(actions)
	if !wantPatched {
		assert.Empty(t, patches)
		return
	}
	if assert.Len(t, patches, 1) {
		assertPatchResourceVersion(t, patches[0], resourceVersion)
	}
}

func assertPatchResourceVersion(t *testing.T, action clienttesting.PatchAction, want string) {
	t.Helper()
	var patch struct {
		Metadata struct {
			ResourceVersion string `json:"resourceVersion"`
		} `json:"metadata"`
	}
	assert.NoError(t, json.Unmarshal(action.GetPatch(), &patch))
	assert.Equal(t, want, patch.Metadata.ResourceVersion)
}

func testClusterLister(t *testing.T, names ...string) clusterlister.ClusterLister {
	t.Helper()
	indexer := toolscache.NewIndexer(toolscache.MetaNamespaceKeyFunc, toolscache.Indexers{})
	for _, name := range names {
		assert.NoError(t, indexer.Add(&clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: name}}))
	}
	return clusterlister.NewClusterLister(indexer)
}

// Mock Implementations

type mockAlgorithm struct {
	scheduleFunc func(context.Context, *workv1alpha2.ResourceBindingSpec, *workv1alpha2.ResourceBindingStatus, *core.ScheduleAlgorithmOption) (core.ScheduleResult, error)
}

func (m *mockAlgorithm) Schedule(ctx context.Context, spec *workv1alpha2.ResourceBindingSpec, status *workv1alpha2.ResourceBindingStatus, option *core.ScheduleAlgorithmOption) (core.ScheduleResult, error) {
	return m.scheduleFunc(ctx, spec, status, option)
}

type fakeBindingLister struct {
	binding *workv1alpha2.ResourceBinding
}

func (f *fakeBindingLister) List(_ labels.Selector) (ret []*workv1alpha2.ResourceBinding, err error) {
	return []*workv1alpha2.ResourceBinding{f.binding}, nil
}

func (f *fakeBindingLister) ResourceBindings(_ string) workv1alpha2lister.ResourceBindingNamespaceLister {
	return &fakeBindingNamespaceLister{binding: f.binding}
}

type fakeBindingNamespaceLister struct {
	binding *workv1alpha2.ResourceBinding
}

func (f *fakeBindingNamespaceLister) List(_ labels.Selector) (ret []*workv1alpha2.ResourceBinding, err error) {
	return []*workv1alpha2.ResourceBinding{f.binding}, nil
}

func (f *fakeBindingNamespaceLister) Get(_ string) (*workv1alpha2.ResourceBinding, error) {
	return f.binding, nil
}

type fakeClusterBindingLister struct {
	binding *workv1alpha2.ClusterResourceBinding
}

func (f *fakeClusterBindingLister) List(_ labels.Selector) (ret []*workv1alpha2.ClusterResourceBinding, err error) {
	return []*workv1alpha2.ClusterResourceBinding{f.binding}, nil
}

func (f *fakeClusterBindingLister) Get(_ string) (*workv1alpha2.ClusterResourceBinding, error) {
	return f.binding, nil
}

// mockSchedulingQueue records which method handleErr calls.
type mockSchedulingQueue struct {
	pushUnschedulableCalled bool
	pushBackoffCalled       bool
	forgetCalled            bool
}

func (m *mockSchedulingQueue) Push(_ *internalqueue.QueuedBindingInfo)       {}
func (m *mockSchedulingQueue) Pop() (*internalqueue.QueuedBindingInfo, bool) { return nil, false }
func (m *mockSchedulingQueue) Done(_ *internalqueue.QueuedBindingInfo)       {}
func (m *mockSchedulingQueue) Len() int                                      { return 0 }
func (m *mockSchedulingQueue) Run()                                          {}
func (m *mockSchedulingQueue) Close()                                        {}

func (m *mockSchedulingQueue) PushUnschedulableIfNotPresent(_ *internalqueue.QueuedBindingInfo) {
	m.pushUnschedulableCalled = true
}

func (m *mockSchedulingQueue) PushBackoffIfNotPresent(_ *internalqueue.QueuedBindingInfo) {
	m.pushBackoffCalled = true
}

func (m *mockSchedulingQueue) Forget(_ *internalqueue.QueuedBindingInfo) {
	m.forgetCalled = true
}

func TestHandleErr(t *testing.T) {
	tests := []struct {
		name                    string
		err                     error
		expectPushUnschedulable bool
		expectPushBackoff       bool
		expectForget            bool
	}{
		{
			name:         "nil error calls Forget",
			err:          nil,
			expectForget: true,
		},
		{
			name:              "generic error calls PushBackoffIfNotPresent",
			err:               fmt.Errorf("some transient error"),
			expectPushBackoff: true,
		},
		{
			name:                    "bare UnschedulableError calls PushUnschedulableIfNotPresent",
			err:                     &framework.UnschedulableError{Message: "insufficient replicas"},
			expectPushUnschedulable: true,
		},
		{
			name: "wrapped UnschedulableError calls PushUnschedulableIfNotPresent",
			err: fmt.Errorf("failed to assign replicas: %w",
				fmt.Errorf("failed to scale up: %w",
					&framework.UnschedulableError{Message: "insufficient replicas"})),
			expectPushUnschedulable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockSchedulingQueue{}
			s := &Scheduler{priorityQueue: mock}
			bindingInfo := &internalqueue.QueuedBindingInfo{NamespacedKey: "default/test"}

			s.handleErr(tt.err, bindingInfo)

			assert.Equal(t, tt.expectPushUnschedulable, mock.pushUnschedulableCalled, "PushUnschedulableIfNotPresent")
			assert.Equal(t, tt.expectPushBackoff, mock.pushBackoffCalled, "PushBackoffIfNotPresent")
			assert.Equal(t, tt.expectForget, mock.forgetCalled, "Forget")
		})
	}
}
