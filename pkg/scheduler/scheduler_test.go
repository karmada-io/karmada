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
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/features"
	karmadafake "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/fake"
	workv1alpha2lister "github.com/karmada-io/karmada/pkg/generated/listers/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/scheduler/core"
	internalqueue "github.com/karmada-io/karmada/pkg/scheduler/internal/queue"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/grpcconnection"
)

func TestDoSchedule(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		binding     interface{}
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
			fakeClient := karmadafake.NewSimpleClientset()
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
			fakeClient := karmadafake.NewSimpleClientset(tt.binding)
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
			fakeClient := karmadafake.NewSimpleClientset(tt.binding)
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
			fakeClient := karmadafake.NewSimpleClientset(tt.binding)
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
		name            string
		binding         *workv1alpha2.ResourceBinding
		scheduleResults []core.ScheduleResult
		scheduleErrors  []error
		expectError     bool
		expectedPatches []string
		expectedEvent   string
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
			expectedEvent: "Normal ScheduleBindingSucceed Binding has been scheduled successfully.",
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
			fakeClient := karmadafake.NewSimpleClientset(tt.binding)
			fakeRecorder := record.NewFakeRecorder(10)
			mockAlgorithm := &mockAlgorithm{
				scheduleFunc: func(_ context.Context, spec *workv1alpha2.ResourceBindingSpec, status *workv1alpha2.ResourceBindingStatus, _ *core.ScheduleAlgorithmOption) (core.ScheduleResult, error) {
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
				if strings.Contains(event, "ScheduleBindingFailed") {
					assert.Contains(t, event, tt.expectedEvent, "Event does not match expected")
				} else {
					assert.Contains(t, event, "ScheduleBindingSucceed", "Expected ScheduleBindingSucceed event")
				}
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
			s := &Scheduler{
				KarmadaClient: karmadafake.NewSimpleClientset(tt.oldBinding),
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
			fakeClient := karmadafake.NewSimpleClientset(tt.binding)
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
		name            string
		binding         *workv1alpha2.ClusterResourceBinding
		scheduleResults []core.ScheduleResult
		scheduleErrors  []error
		expectError     bool
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := karmadafake.NewSimpleClientset(tt.binding)
			fakeRecorder := record.NewFakeRecorder(10)
			mockAlgorithm := &mockAlgorithm{
				scheduleFunc: func(_ context.Context, spec *workv1alpha2.ResourceBindingSpec, status *workv1alpha2.ResourceBindingStatus, _ *core.ScheduleAlgorithmOption) (core.ScheduleResult, error) {
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

	fakeClient := karmadafake.NewSimpleClientset(resourceBinding, clusterResourceBinding)

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
			want:                 false,
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
	karmadaClient := karmadafake.NewSimpleClientset()
	kubeClient := fake.NewSimpleClientset()
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

	karmadaClient := karmadafake.NewSimpleClientset()

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
	karmadaClient := karmadafake.NewSimpleClientset()

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

	karmadaClient := karmadafake.NewSimpleClientset()

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
	karmadaClient := karmadafake.NewSimpleClientset()

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
