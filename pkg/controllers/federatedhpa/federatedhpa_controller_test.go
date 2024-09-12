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

package federatedhpa

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/lifted/selectors"
)

type MockClient struct {
	mock.Mock
}

func (m *MockClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	args := m.Called(ctx, key, obj, opts)
	return args.Error(0)
}

func (m *MockClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	args := m.Called(ctx, list, opts)
	return args.Error(0)
}

func (m *MockClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	args := m.Called(ctx, obj, opts)
	return args.Error(0)
}

func (m *MockClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	args := m.Called(ctx, obj, opts)
	return args.Error(0)
}

func (m *MockClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	args := m.Called(ctx, obj, opts)
	return args.Error(0)
}

func (m *MockClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	args := m.Called(ctx, obj, patch, opts)
	return args.Error(0)
}

func (m *MockClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	args := m.Called(ctx, obj, opts)
	return args.Error(0)
}

func (m *MockClient) Status() client.StatusWriter {
	args := m.Called()
	return args.Get(0).(client.StatusWriter)
}

func (m *MockClient) Scheme() *runtime.Scheme {
	args := m.Called()
	return args.Get(0).(*runtime.Scheme)
}

func (m *MockClient) SubResource(subResource string) client.SubResourceClient {
	args := m.Called(subResource)
	return args.Get(0).(client.SubResourceClient)
}

func (m *MockClient) RESTMapper() meta.RESTMapper {
	args := m.Called()
	return args.Get(0).(meta.RESTMapper)
}

func (m *MockClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	args := m.Called(obj)
	return args.Get(0).(schema.GroupVersionKind), args.Error(1)
}

func (m *MockClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	args := m.Called(obj)
	return args.Bool(0), args.Error(1)
}

// TestGetBindingByLabel verifies the behavior of getBindingByLabel function
func TestGetBindingByLabel(t *testing.T) {
	tests := []struct {
		name          string
		resourceLabel map[string]string
		resourceRef   autoscalingv2.CrossVersionObjectReference
		bindingList   *workv1alpha2.ResourceBindingList
		expectedError string
	}{
		{
			name: "Successful retrieval",
			resourceLabel: map[string]string{
				policyv1alpha1.PropagationPolicyPermanentIDLabel: "test-policy-id",
			},
			resourceRef: autoscalingv2.CrossVersionObjectReference{
				Kind:       "Deployment",
				Name:       "test-deployment",
				APIVersion: "apps/v1",
			},
			bindingList: &workv1alpha2.ResourceBindingList{
				Items: []workv1alpha2.ResourceBinding{
					{
						Spec: workv1alpha2.ResourceBindingSpec{
							Resource: workv1alpha2.ObjectReference{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
								Name:       "test-deployment",
							},
						},
					},
				},
			},
		},
		{
			name:          "Empty resource label",
			resourceLabel: map[string]string{},
			expectedError: "target resource has no label",
		},
		{
			name: "No matching bindings",
			resourceLabel: map[string]string{
				policyv1alpha1.PropagationPolicyPermanentIDLabel: "test-policy-id",
			},
			resourceRef: autoscalingv2.CrossVersionObjectReference{
				Kind:       "Deployment",
				Name:       "non-existent-deployment",
				APIVersion: "apps/v1",
			},
			bindingList: &workv1alpha2.ResourceBindingList{
				Items: []workv1alpha2.ResourceBinding{},
			},
			expectedError: "length of binding list is zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockClient)
			controller := &FHPAController{
				Client: mockClient,
			}

			ctx := context.Background()

			if tt.bindingList != nil {
				mockClient.On("List", ctx, mock.AnythingOfType("*v1alpha2.ResourceBindingList"), mock.Anything).
					Return(nil).
					Run(func(args mock.Arguments) {
						arg := args.Get(1).(*workv1alpha2.ResourceBindingList)
						*arg = *tt.bindingList
					})
			}

			binding, err := controller.getBindingByLabel(ctx, tt.resourceLabel, tt.resourceRef)

			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, binding)
				assert.Equal(t, tt.resourceRef.Name, binding.Spec.Resource.Name)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestGetTargetCluster checks the getTargetCluster function's handling of various cluster states
func TestGetTargetCluster(t *testing.T) {
	tests := []struct {
		name             string
		binding          *workv1alpha2.ResourceBinding
		clusters         map[string]*clusterv1alpha1.Cluster
		getErrors        map[string]error
		expectedClusters []string
		expectedError    string
	}{
		{
			name: "Two clusters, one ready and one not ready",
			binding: &workv1alpha2.ResourceBinding{
				Spec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "cluster1"},
						{Name: "cluster2"},
					},
				},
			},
			clusters: map[string]*clusterv1alpha1.Cluster{
				"cluster1": {
					Status: clusterv1alpha1.ClusterStatus{
						Conditions: []metav1.Condition{
							{Type: clusterv1alpha1.ClusterConditionReady, Status: metav1.ConditionTrue},
						},
					},
				},
				"cluster2": {
					Status: clusterv1alpha1.ClusterStatus{
						Conditions: []metav1.Condition{
							{Type: clusterv1alpha1.ClusterConditionReady, Status: metav1.ConditionFalse},
						},
					},
				},
			},
			expectedClusters: []string{"cluster1"},
		},
		{
			name: "Empty binding.Spec.Clusters",
			binding: &workv1alpha2.ResourceBinding{
				Spec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{},
				},
			},
			expectedError: "binding has no schedulable clusters",
		},
		{
			name: "Client.Get returns error",
			binding: &workv1alpha2.ResourceBinding{
				Spec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "cluster1"},
					},
				},
			},
			getErrors: map[string]error{
				"cluster1": errors.New("get error"),
			},
			expectedError: "get error",
		},
		{
			name: "Multiple ready and not ready clusters",
			binding: &workv1alpha2.ResourceBinding{
				Spec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{Name: "cluster1"},
						{Name: "cluster2"},
						{Name: "cluster3"},
						{Name: "cluster4"},
						{Name: "cluster5"},
					},
				},
			},
			clusters: map[string]*clusterv1alpha1.Cluster{
				"cluster1": {
					Status: clusterv1alpha1.ClusterStatus{
						Conditions: []metav1.Condition{
							{Type: clusterv1alpha1.ClusterConditionReady, Status: metav1.ConditionTrue},
						},
					},
				},
				"cluster2": {
					Status: clusterv1alpha1.ClusterStatus{
						Conditions: []metav1.Condition{
							{Type: clusterv1alpha1.ClusterConditionReady, Status: metav1.ConditionFalse},
						},
					},
				},
				"cluster3": {
					Status: clusterv1alpha1.ClusterStatus{
						Conditions: []metav1.Condition{
							{Type: clusterv1alpha1.ClusterConditionReady, Status: metav1.ConditionTrue},
						},
					},
				},
				"cluster4": {
					Status: clusterv1alpha1.ClusterStatus{
						Conditions: []metav1.Condition{
							{Type: clusterv1alpha1.ClusterConditionReady, Status: metav1.ConditionFalse},
						},
					},
				},
				"cluster5": {
					Status: clusterv1alpha1.ClusterStatus{
						Conditions: []metav1.Condition{
							{Type: clusterv1alpha1.ClusterConditionReady, Status: metav1.ConditionTrue},
						},
					},
				},
			},
			expectedClusters: []string{"cluster1", "cluster3", "cluster5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockClient)
			controller := &FHPAController{
				Client: mockClient,
			}

			ctx := context.Background()

			for _, targetCluster := range tt.binding.Spec.Clusters {
				var err error
				if tt.getErrors != nil {
					err = tt.getErrors[targetCluster.Name]
				}

				mockClient.On("Get", ctx, types.NamespacedName{Name: targetCluster.Name}, mock.AnythingOfType("*v1alpha1.Cluster"), mock.Anything).
					Return(err).
					Run(func(args mock.Arguments) {
						if tt.clusters != nil {
							arg := args.Get(2).(*clusterv1alpha1.Cluster)
							*arg = *tt.clusters[targetCluster.Name]
						}
					})
			}

			clusters, err := controller.getTargetCluster(ctx, tt.binding)

			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedClusters, clusters)
			}

			mockClient.AssertExpectations(t)

			for _, targetCluster := range tt.binding.Spec.Clusters {
				mockClient.AssertCalled(t, "Get", ctx, types.NamespacedName{Name: targetCluster.Name}, mock.AnythingOfType("*v1alpha1.Cluster"), mock.Anything)
			}
		})
	}
}

// TestValidateAndParseSelector ensures proper parsing and validation of selectors
func TestValidateAndParseSelector(t *testing.T) {
	tests := []struct {
		name          string
		selector      string
		expectedError bool
	}{
		{
			name:          "Valid selector",
			selector:      "app=myapp",
			expectedError: false,
		},
		{
			name:          "Invalid selector",
			selector:      "invalid=selector=format",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := &FHPAController{
				hpaSelectors:    selectors.NewBiMultimap(),
				hpaSelectorsMux: sync.Mutex{},
				EventRecorder:   &record.FakeRecorder{},
			}

			hpa := &autoscalingv1alpha1.FederatedHPA{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-hpa",
					Namespace: "default",
				},
			}

			parsedSelector, err := controller.validateAndParseSelector(hpa, tt.selector, []*corev1.Pod{})

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, parsedSelector)
				assert.Contains(t, err.Error(), "couldn't convert selector into a corresponding internal selector object")
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, parsedSelector)
			}
		})
	}
}

// TestRecordInitialRecommendation verifies correct recording of initial recommendations
func TestRecordInitialRecommendation(t *testing.T) {
	tests := []struct {
		name             string
		key              string
		currentReplicas  int32
		initialRecs      []timestampedRecommendation
		expectedCount    int
		expectedReplicas int32
	}{
		{
			name:             "New recommendation",
			key:              "test-hpa-1",
			currentReplicas:  3,
			initialRecs:      nil,
			expectedCount:    1,
			expectedReplicas: 3,
		},
		{
			name:            "Existing recommendations",
			key:             "test-hpa-2",
			currentReplicas: 5,
			initialRecs: []timestampedRecommendation{
				{recommendation: 3, timestamp: time.Now().Add(-1 * time.Minute)},
			},
			expectedCount:    1,
			expectedReplicas: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := &FHPAController{
				recommendations: make(map[string][]timestampedRecommendation),
			}

			if tt.initialRecs != nil {
				controller.recommendations[tt.key] = tt.initialRecs
			}

			controller.recordInitialRecommendation(tt.currentReplicas, tt.key)

			assert.Len(t, controller.recommendations[tt.key], tt.expectedCount)
			assert.Equal(t, tt.expectedReplicas, controller.recommendations[tt.key][0].recommendation)

			if tt.initialRecs == nil {
				assert.WithinDuration(t, time.Now(), controller.recommendations[tt.key][0].timestamp, 2*time.Second)
			} else {
				assert.Equal(t, tt.initialRecs[0].timestamp, controller.recommendations[tt.key][0].timestamp)
			}
		})
	}
}

// TestStabilizeRecommendation checks the stabilization logic for recommendations
func TestStabilizeRecommendation(t *testing.T) {
	tests := []struct {
		name                   string
		key                    string
		initialRecommendations []timestampedRecommendation
		newRecommendation      int32
		expectedStabilized     int32
		expectedStoredCount    int
	}{
		{
			name:                   "No previous recommendations",
			key:                    "test-hpa-1",
			initialRecommendations: []timestampedRecommendation{},
			newRecommendation:      5,
			expectedStabilized:     5,
			expectedStoredCount:    1,
		},
		{
			name: "With previous recommendations within window",
			key:  "test-hpa-2",
			initialRecommendations: []timestampedRecommendation{
				{recommendation: 3, timestamp: time.Now().Add(-30 * time.Second)},
				{recommendation: 4, timestamp: time.Now().Add(-45 * time.Second)},
			},
			newRecommendation:   2,
			expectedStabilized:  4,
			expectedStoredCount: 3,
		},
		{
			name: "With old recommendation outside window",
			key:  "test-hpa-3",
			initialRecommendations: []timestampedRecommendation{
				{recommendation: 7, timestamp: time.Now().Add(-2 * time.Minute)},
				{recommendation: 4, timestamp: time.Now().Add(-45 * time.Second)},
			},
			newRecommendation:   5,
			expectedStabilized:  5,
			expectedStoredCount: 2,
		},
		{
			name: "All recommendations outside window",
			key:  "test-hpa-4",
			initialRecommendations: []timestampedRecommendation{
				{recommendation: 7, timestamp: time.Now().Add(-2 * time.Minute)},
				{recommendation: 8, timestamp: time.Now().Add(-3 * time.Minute)},
			},
			newRecommendation:   3,
			expectedStabilized:  3,
			expectedStoredCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := &FHPAController{
				recommendations:              make(map[string][]timestampedRecommendation),
				DownscaleStabilisationWindow: time.Minute,
			}
			controller.recommendations[tt.key] = tt.initialRecommendations

			stabilized := controller.stabilizeRecommendation(tt.key, tt.newRecommendation)

			assert.Equal(t, tt.expectedStabilized, stabilized, "Unexpected stabilized recommendation")
			assert.Len(t, controller.recommendations[tt.key], tt.expectedStoredCount, "Unexpected number of stored recommendations")
			assert.True(t, containsRecommendation(controller.recommendations[tt.key], tt.newRecommendation), "New recommendation not found in stored recommendations")

			oldCount := countOldRecommendations(controller.recommendations[tt.key], controller.DownscaleStabilisationWindow)
			assert.LessOrEqual(t, oldCount, 1, "Too many recommendations older than stabilization window")
		})
	}
}

// TestNormalizeDesiredReplicas verifies the normalization of desired replicas
func TestNormalizeDesiredReplicas(t *testing.T) {
	testCases := []struct {
		name                         string
		currentReplicas              int32
		desiredReplicas              int32
		minReplicas                  int32
		maxReplicas                  int32
		recommendations              []timestampedRecommendation
		expectedReplicas             int32
		expectedAbleToScale          autoscalingv2.HorizontalPodAutoscalerConditionType
		expectedAbleToScaleReason    string
		expectedScalingLimited       corev1.ConditionStatus
		expectedScalingLimitedReason string
	}{
		{
			name:                         "scale up within limits",
			currentReplicas:              2,
			desiredReplicas:              4,
			minReplicas:                  1,
			maxReplicas:                  10,
			recommendations:              []timestampedRecommendation{{4, time.Now()}},
			expectedReplicas:             4,
			expectedAbleToScale:          autoscalingv2.AbleToScale,
			expectedAbleToScaleReason:    "ReadyForNewScale",
			expectedScalingLimited:       corev1.ConditionFalse,
			expectedScalingLimitedReason: "DesiredWithinRange",
		},
		{
			name:                         "scale down stabilized",
			currentReplicas:              5,
			desiredReplicas:              3,
			minReplicas:                  1,
			maxReplicas:                  10,
			recommendations:              []timestampedRecommendation{{4, time.Now().Add(-1 * time.Minute)}, {3, time.Now()}},
			expectedReplicas:             4,
			expectedAbleToScale:          autoscalingv2.AbleToScale,
			expectedAbleToScaleReason:    "ScaleDownStabilized",
			expectedScalingLimited:       corev1.ConditionFalse,
			expectedScalingLimitedReason: "DesiredWithinRange",
		},
		{
			name:                         "at min replicas",
			currentReplicas:              2,
			desiredReplicas:              1,
			minReplicas:                  2,
			maxReplicas:                  10,
			recommendations:              []timestampedRecommendation{{1, time.Now()}},
			expectedReplicas:             2,
			expectedAbleToScale:          autoscalingv2.AbleToScale,
			expectedAbleToScaleReason:    "ReadyForNewScale",
			expectedScalingLimited:       corev1.ConditionTrue,
			expectedScalingLimitedReason: "TooFewReplicas",
		},
		{
			name:                         "at max replicas",
			currentReplicas:              10,
			desiredReplicas:              12,
			minReplicas:                  1,
			maxReplicas:                  10,
			recommendations:              []timestampedRecommendation{{12, time.Now()}},
			expectedReplicas:             10,
			expectedAbleToScale:          autoscalingv2.AbleToScale,
			expectedAbleToScaleReason:    "ReadyForNewScale",
			expectedScalingLimited:       corev1.ConditionTrue,
			expectedScalingLimitedReason: "TooManyReplicas",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			controller := &FHPAController{
				recommendations:              make(map[string][]timestampedRecommendation),
				DownscaleStabilisationWindow: 5 * time.Minute,
			}
			controller.recommendations["test-hpa"] = tc.recommendations

			hpa := &autoscalingv1alpha1.FederatedHPA{
				Spec: autoscalingv1alpha1.FederatedHPASpec{
					MinReplicas: &tc.minReplicas,
					MaxReplicas: tc.maxReplicas,
				},
			}

			normalized := controller.normalizeDesiredReplicas(hpa, "test-hpa", tc.currentReplicas, tc.desiredReplicas, tc.minReplicas)

			assert.Equal(t, tc.expectedReplicas, normalized, "Unexpected normalized replicas")

			ableToScaleCondition := getCondition(hpa.Status.Conditions, autoscalingv2.AbleToScale)
			assert.NotNil(t, ableToScaleCondition, "AbleToScale condition not found")
			assert.Equal(t, corev1.ConditionTrue, ableToScaleCondition.Status, "Unexpected AbleToScale condition status")
			assert.Equal(t, tc.expectedAbleToScaleReason, ableToScaleCondition.Reason, "Unexpected AbleToScale condition reason")

			scalingLimitedCondition := getCondition(hpa.Status.Conditions, autoscalingv2.ScalingLimited)
			assert.NotNil(t, scalingLimitedCondition, "ScalingLimited condition not found")
			assert.Equal(t, tc.expectedScalingLimited, scalingLimitedCondition.Status, "Unexpected ScalingLimited condition status")
			assert.Equal(t, tc.expectedScalingLimitedReason, scalingLimitedCondition.Reason, "Unexpected ScalingLimited condition reason")
		})
	}
}

// TestNormalizeDesiredReplicasWithBehaviors checks replica normalization with scaling behaviors
func TestNormalizeDesiredReplicasWithBehaviors(t *testing.T) {
	defaultStabilizationWindowSeconds := int32(300)
	defaultSelectPolicy := autoscalingv2.MaxChangePolicySelect

	tests := []struct {
		name                  string
		hpa                   *autoscalingv1alpha1.FederatedHPA
		key                   string
		currentReplicas       int32
		prenormalizedReplicas int32
		expectedReplicas      int32
	}{
		{
			name: "Scale up with behavior",
			hpa: createTestHPA(1, 10, &autoscalingv2.HorizontalPodAutoscalerBehavior{
				ScaleUp:   createTestScalingRules(&defaultStabilizationWindowSeconds, &defaultSelectPolicy, []autoscalingv2.HPAScalingPolicy{{Type: autoscalingv2.PercentScalingPolicy, Value: 200, PeriodSeconds: 60}}),
				ScaleDown: createTestScalingRules(&defaultStabilizationWindowSeconds, &defaultSelectPolicy, []autoscalingv2.HPAScalingPolicy{{Type: autoscalingv2.PercentScalingPolicy, Value: 100, PeriodSeconds: 60}}),
			}),
			key:                   "test-hpa",
			currentReplicas:       5,
			prenormalizedReplicas: 15,
			expectedReplicas:      10,
		},
		{
			name: "Scale down with behavior",
			hpa: createTestHPA(1, 10, &autoscalingv2.HorizontalPodAutoscalerBehavior{
				ScaleUp:   createTestScalingRules(&defaultStabilizationWindowSeconds, &defaultSelectPolicy, []autoscalingv2.HPAScalingPolicy{{Type: autoscalingv2.PercentScalingPolicy, Value: 200, PeriodSeconds: 60}}),
				ScaleDown: createTestScalingRules(&defaultStabilizationWindowSeconds, &defaultSelectPolicy, []autoscalingv2.HPAScalingPolicy{{Type: autoscalingv2.PercentScalingPolicy, Value: 50, PeriodSeconds: 60}}),
			}),
			key:                   "test-hpa",
			currentReplicas:       8,
			prenormalizedReplicas: 2,
			expectedReplicas:      4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := &FHPAController{
				recommendations:              make(map[string][]timestampedRecommendation),
				DownscaleStabilisationWindow: 5 * time.Minute,
			}

			normalized := controller.normalizeDesiredReplicasWithBehaviors(tt.hpa, tt.key, tt.currentReplicas, tt.prenormalizedReplicas, *tt.hpa.Spec.MinReplicas)
			assert.Equal(t, tt.expectedReplicas, normalized, "Unexpected normalized replicas")
		})
	}
}

// TestGetReplicasChangePerPeriod ensures correct calculation of replica changes over time
func TestGetReplicasChangePerPeriod(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name           string
		periodSeconds  int32
		scaleEvents    []timestampedScaleEvent
		expectedChange int32
	}{
		{
			name:           "No events",
			periodSeconds:  60,
			scaleEvents:    []timestampedScaleEvent{},
			expectedChange: 0,
		},
		{
			name:          "Single event within period",
			periodSeconds: 60,
			scaleEvents: []timestampedScaleEvent{
				{replicaChange: 3, timestamp: now.Add(-30 * time.Second)},
			},
			expectedChange: 3,
		},
		{
			name:          "Multiple events, some outside period",
			periodSeconds: 60,
			scaleEvents: []timestampedScaleEvent{
				{replicaChange: 3, timestamp: now.Add(-30 * time.Second)},
				{replicaChange: 2, timestamp: now.Add(-45 * time.Second)},
				{replicaChange: 1, timestamp: now.Add(-70 * time.Second)},
			},
			expectedChange: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			change := getReplicasChangePerPeriod(tt.periodSeconds, tt.scaleEvents)
			assert.Equal(t, tt.expectedChange, change, "Unexpected change in replicas")
		})
	}
}

// TestGetUnableComputeReplicaCountCondition verifies condition creation for compute failures
func TestGetUnableComputeReplicaCountCondition(t *testing.T) {
	tests := []struct {
		name            string
		object          runtime.Object
		reason          string
		err             error
		expectedEvent   string
		expectedMessage string
	}{
		{
			name:            "FederatedHPA with simple error",
			object:          createTestFederatedHPA("test-hpa", "default"),
			reason:          "TestReason",
			err:             fmt.Errorf("test error"),
			expectedEvent:   "Warning TestReason test error",
			expectedMessage: "the HPA was unable to compute the replica count: test error",
		},
		{
			name:            "Different object type",
			object:          &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"}},
			reason:          "PodError",
			err:             fmt.Errorf("pod error"),
			expectedEvent:   "Warning PodError pod error",
			expectedMessage: "the HPA was unable to compute the replica count: pod error",
		},
		{
			name:            "Complex error message",
			object:          createTestFederatedHPA("complex-hpa", "default"),
			reason:          "ComplexError",
			err:             fmt.Errorf("error: %v", fmt.Errorf("nested error")),
			expectedEvent:   "Warning ComplexError error: nested error",
			expectedMessage: "the HPA was unable to compute the replica count: error: nested error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeRecorder := record.NewFakeRecorder(10)
			controller := &FHPAController{
				EventRecorder: fakeRecorder,
			}

			condition := controller.getUnableComputeReplicaCountCondition(tt.object, tt.reason, tt.err)

			assert.Equal(t, autoscalingv2.ScalingActive, condition.Type, "Unexpected condition type")
			assert.Equal(t, corev1.ConditionFalse, condition.Status, "Unexpected condition status")
			assert.Equal(t, tt.reason, condition.Reason, "Unexpected condition reason")
			assert.Equal(t, tt.expectedMessage, condition.Message, "Unexpected condition message")

			select {
			case event := <-fakeRecorder.Events:
				assert.Equal(t, tt.expectedEvent, event, "Unexpected event recorded")
			case <-time.After(time.Second):
				t.Error("Expected an event to be recorded, but none was")
			}
		})
	}
}

// TestStoreScaleEvent checks proper storage of scaling events
func TestStoreScaleEvent(t *testing.T) {
	tests := []struct {
		name         string
		behavior     *autoscalingv2.HorizontalPodAutoscalerBehavior
		key          string
		prevReplicas int32
		newReplicas  int32
		expectedUp   int
		expectedDown int
	}{
		{
			name: "Scale up event",
			behavior: &autoscalingv2.HorizontalPodAutoscalerBehavior{
				ScaleUp: &autoscalingv2.HPAScalingRules{
					StabilizationWindowSeconds: ptr.To[int32](int32(60)),
				},
			},
			key:          "test-hpa",
			prevReplicas: 5,
			newReplicas:  10,
			expectedUp:   1,
			expectedDown: 0,
		},
		{
			name: "Scale down event",
			behavior: &autoscalingv2.HorizontalPodAutoscalerBehavior{
				ScaleDown: &autoscalingv2.HPAScalingRules{
					StabilizationWindowSeconds: ptr.To[int32](int32(60)),
				},
			},
			key:          "test-hpa",
			prevReplicas: 10,
			newReplicas:  5,
			expectedUp:   0,
			expectedDown: 1,
		},
		{
			name:         "Nil behavior",
			behavior:     nil,
			key:          "test-hpa",
			prevReplicas: 5,
			newReplicas:  5,
			expectedUp:   0,
			expectedDown: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := &FHPAController{
				scaleUpEvents:   make(map[string][]timestampedScaleEvent),
				scaleDownEvents: make(map[string][]timestampedScaleEvent),
			}

			controller.storeScaleEvent(tt.behavior, tt.key, tt.prevReplicas, tt.newReplicas)

			assert.Len(t, controller.scaleUpEvents[tt.key], tt.expectedUp, "Unexpected number of scale up events")
			assert.Len(t, controller.scaleDownEvents[tt.key], tt.expectedDown, "Unexpected number of scale down events")
		})
	}
}

// TestStabilizeRecommendationWithBehaviors verifies recommendation stabilization with behaviors
func TestStabilizeRecommendationWithBehaviors(t *testing.T) {
	now := time.Now()
	upWindow := int32(300)   // 5 minutes
	downWindow := int32(600) // 10 minutes

	tests := []struct {
		name                   string
		args                   NormalizationArg
		initialRecommendations []timestampedRecommendation
		expectedReplicas       int32
		expectedReason         string
		expectedMessage        string
	}{
		{
			name: "Scale up stabilized",
			args: NormalizationArg{
				Key:             "test-hpa-1",
				DesiredReplicas: 10,
				CurrentReplicas: 5,
				ScaleUpBehavior: &autoscalingv2.HPAScalingRules{
					StabilizationWindowSeconds: &upWindow,
				},
				ScaleDownBehavior: &autoscalingv2.HPAScalingRules{
					StabilizationWindowSeconds: &downWindow,
				},
			},
			initialRecommendations: []timestampedRecommendation{
				{recommendation: 8, timestamp: now.Add(-2 * time.Minute)},
				{recommendation: 7, timestamp: now.Add(-4 * time.Minute)},
			},
			expectedReplicas: 7,
			expectedReason:   "ScaleUpStabilized",
			expectedMessage:  "recent recommendations were lower than current one, applying the lowest recent recommendation",
		},
		{
			name: "Scale down stabilized",
			args: NormalizationArg{
				Key:             "test-hpa-2",
				DesiredReplicas: 3,
				CurrentReplicas: 8,
				ScaleUpBehavior: &autoscalingv2.HPAScalingRules{
					StabilizationWindowSeconds: &upWindow,
				},
				ScaleDownBehavior: &autoscalingv2.HPAScalingRules{
					StabilizationWindowSeconds: &downWindow,
				},
			},
			initialRecommendations: []timestampedRecommendation{
				{recommendation: 5, timestamp: now.Add(-5 * time.Minute)},
				{recommendation: 4, timestamp: now.Add(-8 * time.Minute)},
			},
			expectedReplicas: 5,
			expectedReason:   "ScaleDownStabilized",
			expectedMessage:  "recent recommendations were higher than current one, applying the highest recent recommendation",
		},
		{
			name: "No change needed",
			args: NormalizationArg{
				Key:             "test-hpa-3",
				DesiredReplicas: 5,
				CurrentReplicas: 5,
				ScaleUpBehavior: &autoscalingv2.HPAScalingRules{
					StabilizationWindowSeconds: &upWindow,
				},
				ScaleDownBehavior: &autoscalingv2.HPAScalingRules{
					StabilizationWindowSeconds: &downWindow,
				},
			},
			initialRecommendations: []timestampedRecommendation{
				{recommendation: 5, timestamp: now.Add(-1 * time.Minute)},
			},
			expectedReplicas: 5,
			expectedReason:   "ScaleUpStabilized",
			expectedMessage:  "recent recommendations were lower than current one, applying the lowest recent recommendation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := &FHPAController{
				recommendations: make(map[string][]timestampedRecommendation),
			}

			controller.recommendations[tt.args.Key] = tt.initialRecommendations

			gotReplicas, gotReason, gotMessage := controller.stabilizeRecommendationWithBehaviors(tt.args)

			assert.Equal(t, tt.expectedReplicas, gotReplicas, "Unexpected stabilized replicas")
			assert.Equal(t, tt.expectedReason, gotReason, "Unexpected stabilization reason")
			assert.Equal(t, tt.expectedMessage, gotMessage, "Unexpected stabilization message")

			storedRecommendations := controller.recommendations[tt.args.Key]
			assert.True(t, containsRecommendation(storedRecommendations, tt.args.DesiredReplicas), "New recommendation not found in stored recommendations")
			assert.Len(t, storedRecommendations, len(tt.initialRecommendations)+1, "Unexpected number of stored recommendations")
		})
	}
}

// TestConvertDesiredReplicasWithBehaviorRate verifies replica conversion with behavior rates
func TestConvertDesiredReplicasWithBehaviorRate(t *testing.T) {
	tests := []struct {
		name             string
		args             NormalizationArg
		scaleUpEvents    []timestampedScaleEvent
		scaleDownEvents  []timestampedScaleEvent
		expectedReplicas int32
		expectedReason   string
		expectedMessage  string
	}{
		{
			name: "Scale up within limits",
			args: NormalizationArg{
				Key: "test-hpa",
				ScaleUpBehavior: &autoscalingv2.HPAScalingRules{
					Policies: []autoscalingv2.HPAScalingPolicy{
						{Type: autoscalingv2.PercentScalingPolicy, Value: 200, PeriodSeconds: 60},
					},
				},
				MinReplicas:     1,
				MaxReplicas:     10,
				CurrentReplicas: 5,
				DesiredReplicas: 8,
			},
			expectedReplicas: 8,
			expectedReason:   "DesiredWithinRange",
			expectedMessage:  "the desired count is within the acceptable range",
		},
		{
			name: "Scale down within limits",
			args: NormalizationArg{
				Key: "test-hpa",
				ScaleDownBehavior: &autoscalingv2.HPAScalingRules{
					Policies: []autoscalingv2.HPAScalingPolicy{
						{Type: autoscalingv2.PercentScalingPolicy, Value: 100, PeriodSeconds: 60},
					},
				},
				MinReplicas:     1,
				MaxReplicas:     10,
				CurrentReplicas: 5,
				DesiredReplicas: 3,
			},
			expectedReplicas: 3,
			expectedReason:   "DesiredWithinRange",
			expectedMessage:  "the desired count is within the acceptable range",
		},
		{
			name: "Scale up beyond MaxReplicas",
			args: NormalizationArg{
				Key: "test-hpa",
				ScaleUpBehavior: &autoscalingv2.HPAScalingRules{
					Policies: []autoscalingv2.HPAScalingPolicy{
						{Type: autoscalingv2.PercentScalingPolicy, Value: 200, PeriodSeconds: 60},
					},
				},
				MinReplicas:     1,
				MaxReplicas:     10,
				CurrentReplicas: 8,
				DesiredReplicas: 12,
			},
			expectedReplicas: 10,
			expectedReason:   "TooManyReplicas",
			expectedMessage:  "the desired replica count is more than the maximum replica count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := &FHPAController{
				scaleUpEvents:   make(map[string][]timestampedScaleEvent),
				scaleDownEvents: make(map[string][]timestampedScaleEvent),
			}
			controller.scaleUpEvents[tt.args.Key] = tt.scaleUpEvents
			controller.scaleDownEvents[tt.args.Key] = tt.scaleDownEvents

			replicas, reason, message := controller.convertDesiredReplicasWithBehaviorRate(tt.args)

			assert.Equal(t, tt.expectedReplicas, replicas, "Unexpected number of replicas")
			assert.Equal(t, tt.expectedReason, reason, "Unexpected reason")
			assert.Equal(t, tt.expectedMessage, message, "Unexpected message")
		})
	}
}

// TestConvertDesiredReplicasWithRules checks replica conversion using basic rules
func TestConvertDesiredReplicasWithRules(t *testing.T) {
	tests := []struct {
		name              string
		currentReplicas   int32
		desiredReplicas   int32
		hpaMinReplicas    int32
		hpaMaxReplicas    int32
		expectedReplicas  int32
		expectedCondition string
		expectedReason    string
	}{
		{
			name:              "Desired within range",
			currentReplicas:   5,
			desiredReplicas:   7,
			hpaMinReplicas:    3,
			hpaMaxReplicas:    10,
			expectedReplicas:  7,
			expectedCondition: "DesiredWithinRange",
			expectedReason:    "the desired count is within the acceptable range",
		},
		{
			name:              "Desired below min",
			currentReplicas:   5,
			desiredReplicas:   2,
			hpaMinReplicas:    3,
			hpaMaxReplicas:    10,
			expectedReplicas:  3,
			expectedCondition: "TooFewReplicas",
			expectedReason:    "the desired replica count is less than the minimum replica count",
		},
		{
			name:              "Desired above max",
			currentReplicas:   5,
			desiredReplicas:   15,
			hpaMinReplicas:    3,
			hpaMaxReplicas:    10,
			expectedReplicas:  10,
			expectedCondition: "TooManyReplicas",
			expectedReason:    "the desired replica count is more than the maximum replica count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replicas, condition, reason := convertDesiredReplicasWithRules(tt.currentReplicas, tt.desiredReplicas, tt.hpaMinReplicas, tt.hpaMaxReplicas)
			assert.Equal(t, tt.expectedReplicas, replicas, "Unexpected number of replicas")
			assert.Equal(t, tt.expectedCondition, condition, "Unexpected condition")
			assert.Equal(t, tt.expectedReason, reason, "Unexpected reason")
		})
	}
}

// TestCalculateScaleUpLimitWithScalingRules verifies scale-up limit calculation with rules
func TestCalculateScaleUpLimit(t *testing.T) {
	tests := []struct {
		name            string
		currentReplicas int32
		expectedLimit   int32
	}{
		{
			name:            "Small scale up",
			currentReplicas: 1,
			expectedLimit:   4,
		},
		{
			name:            "Medium scale up",
			currentReplicas: 10,
			expectedLimit:   20,
		},
		{
			name:            "Large scale up",
			currentReplicas: 100,
			expectedLimit:   200,
		},
		{
			name:            "Zero replicas",
			currentReplicas: 0,
			expectedLimit:   4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit := calculateScaleUpLimit(tt.currentReplicas)
			assert.Equal(t, tt.expectedLimit, limit, "Unexpected scale up limit")
		})
	}
}

// TestMarkScaleEventsOutdated ensures proper marking of outdated scale events
func TestMarkScaleEventsOutdated(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name                string
		scaleEvents         []timestampedScaleEvent
		longestPolicyPeriod int32
		expectedOutdated    []bool
	}{
		{
			name: "All events within period",
			scaleEvents: []timestampedScaleEvent{
				{timestamp: now.Add(-30 * time.Second)},
				{timestamp: now.Add(-60 * time.Second)},
			},
			longestPolicyPeriod: 120,
			expectedOutdated:    []bool{false, false},
		},
		{
			name: "Some events outdated",
			scaleEvents: []timestampedScaleEvent{
				{timestamp: now.Add(-30 * time.Second)},
				{timestamp: now.Add(-90 * time.Second)},
				{timestamp: now.Add(-150 * time.Second)},
			},
			longestPolicyPeriod: 120,
			expectedOutdated:    []bool{false, false, true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			markScaleEventsOutdated(tt.scaleEvents, tt.longestPolicyPeriod)
			for i, event := range tt.scaleEvents {
				assert.Equal(t, tt.expectedOutdated[i], event.outdated, "Unexpected outdated status for event %d", i)
			}
		})
	}
}

// TestGetLongestPolicyPeriod checks retrieval of the longest policy period
func TestGetLongestPolicyPeriod(t *testing.T) {
	tests := []struct {
		name           string
		scalingRules   *autoscalingv2.HPAScalingRules
		expectedPeriod int32
	}{
		{
			name: "Single policy",
			scalingRules: &autoscalingv2.HPAScalingRules{
				Policies: []autoscalingv2.HPAScalingPolicy{
					{PeriodSeconds: 60},
				},
			},
			expectedPeriod: 60,
		},
		{
			name: "Multiple policies",
			scalingRules: &autoscalingv2.HPAScalingRules{
				Policies: []autoscalingv2.HPAScalingPolicy{
					{PeriodSeconds: 60},
					{PeriodSeconds: 120},
					{PeriodSeconds: 30},
				},
			},
			expectedPeriod: 120,
		},
		{
			name: "No policies",
			scalingRules: &autoscalingv2.HPAScalingRules{
				Policies: []autoscalingv2.HPAScalingPolicy{},
			},
			expectedPeriod: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			period := getLongestPolicyPeriod(tt.scalingRules)
			assert.Equal(t, tt.expectedPeriod, period, "Unexpected longest policy period")
		})
	}
}

// TestCalculateScaleUpLimitWithScalingRules verifies scale-up limit calculation with rules
func TestCalculateScaleUpLimitWithScalingRules(t *testing.T) {
	baseTime := time.Now()
	disabledPolicy := autoscalingv2.DisabledPolicySelect
	minChangePolicy := autoscalingv2.MinChangePolicySelect

	tests := []struct {
		name            string
		currentReplicas int32
		scaleUpEvents   []timestampedScaleEvent
		scaleDownEvents []timestampedScaleEvent
		scalingRules    *autoscalingv2.HPAScalingRules
		expectedLimit   int32
	}{
		{
			name:            "No previous events",
			currentReplicas: 5,
			scaleUpEvents:   []timestampedScaleEvent{},
			scaleDownEvents: []timestampedScaleEvent{},
			scalingRules: &autoscalingv2.HPAScalingRules{
				Policies: []autoscalingv2.HPAScalingPolicy{
					{Type: autoscalingv2.PodsScalingPolicy, Value: 4, PeriodSeconds: 60},
				},
			},
			expectedLimit: 9,
		},
		{
			name:            "With previous scale up event",
			currentReplicas: 5,
			scaleUpEvents: []timestampedScaleEvent{
				{replicaChange: 2, timestamp: baseTime.Add(-30 * time.Second)},
			},
			scaleDownEvents: []timestampedScaleEvent{},
			scalingRules: &autoscalingv2.HPAScalingRules{
				Policies: []autoscalingv2.HPAScalingPolicy{
					{Type: autoscalingv2.PodsScalingPolicy, Value: 4, PeriodSeconds: 60},
				},
			},
			expectedLimit: 7,
		},
		{
			name:            "Disabled policy",
			currentReplicas: 5,
			scaleUpEvents:   []timestampedScaleEvent{},
			scaleDownEvents: []timestampedScaleEvent{},
			scalingRules: &autoscalingv2.HPAScalingRules{
				SelectPolicy: &disabledPolicy,
				Policies: []autoscalingv2.HPAScalingPolicy{
					{Type: autoscalingv2.PodsScalingPolicy, Value: 4, PeriodSeconds: 60},
				},
			},
			expectedLimit: 5,
		},
		{
			name:            "MinChange policy",
			currentReplicas: 5,
			scaleUpEvents:   []timestampedScaleEvent{},
			scaleDownEvents: []timestampedScaleEvent{},
			scalingRules: &autoscalingv2.HPAScalingRules{
				SelectPolicy: &minChangePolicy,
				Policies: []autoscalingv2.HPAScalingPolicy{
					{Type: autoscalingv2.PodsScalingPolicy, Value: 4, PeriodSeconds: 60},
					{Type: autoscalingv2.PodsScalingPolicy, Value: 2, PeriodSeconds: 60},
				},
			},
			expectedLimit: 7,
		},
		{
			name:            "Percent scaling policy",
			currentReplicas: 10,
			scaleUpEvents:   []timestampedScaleEvent{},
			scaleDownEvents: []timestampedScaleEvent{},
			scalingRules: &autoscalingv2.HPAScalingRules{
				Policies: []autoscalingv2.HPAScalingPolicy{
					{Type: autoscalingv2.PercentScalingPolicy, Value: 50, PeriodSeconds: 60},
				},
			},
			expectedLimit: 15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit := calculateScaleUpLimitWithScalingRules(tt.currentReplicas, tt.scaleUpEvents, tt.scaleDownEvents, tt.scalingRules)
			assert.Equal(t, tt.expectedLimit, limit, "Unexpected scale up limit")
		})
	}
}

// TestCalculateScaleDownLimitWithBehaviors checks scale-down limit calculation with behaviors
func TestCalculateScaleDownLimitWithBehaviors(t *testing.T) {
	baseTime := time.Now()
	disabledPolicy := autoscalingv2.DisabledPolicySelect
	minChangePolicy := autoscalingv2.MinChangePolicySelect

	tests := []struct {
		name            string
		currentReplicas int32
		scaleUpEvents   []timestampedScaleEvent
		scaleDownEvents []timestampedScaleEvent
		scalingRules    *autoscalingv2.HPAScalingRules
		expectedLimit   int32
	}{
		{
			name:            "No previous events",
			currentReplicas: 10,
			scaleUpEvents:   []timestampedScaleEvent{},
			scaleDownEvents: []timestampedScaleEvent{},
			scalingRules: &autoscalingv2.HPAScalingRules{
				Policies: []autoscalingv2.HPAScalingPolicy{
					{Type: autoscalingv2.PercentScalingPolicy, Value: 20, PeriodSeconds: 60},
				},
			},
			expectedLimit: 8,
		},
		{
			name:            "With previous scale down event",
			currentReplicas: 10,
			scaleUpEvents:   []timestampedScaleEvent{},
			scaleDownEvents: []timestampedScaleEvent{
				{replicaChange: 1, timestamp: baseTime.Add(-30 * time.Second)},
			},
			scalingRules: &autoscalingv2.HPAScalingRules{
				Policies: []autoscalingv2.HPAScalingPolicy{
					{Type: autoscalingv2.PercentScalingPolicy, Value: 20, PeriodSeconds: 60},
				},
			},
			expectedLimit: 8,
		},
		{
			name:            "Multiple policies",
			currentReplicas: 100,
			scaleUpEvents:   []timestampedScaleEvent{},
			scaleDownEvents: []timestampedScaleEvent{},
			scalingRules: &autoscalingv2.HPAScalingRules{
				Policies: []autoscalingv2.HPAScalingPolicy{
					{Type: autoscalingv2.PercentScalingPolicy, Value: 10, PeriodSeconds: 60},
					{Type: autoscalingv2.PodsScalingPolicy, Value: 5, PeriodSeconds: 60},
				},
			},
			expectedLimit: 90,
		},
		{
			name:            "Disabled policy",
			currentReplicas: 10,
			scaleUpEvents:   []timestampedScaleEvent{},
			scaleDownEvents: []timestampedScaleEvent{},
			scalingRules: &autoscalingv2.HPAScalingRules{
				SelectPolicy: &disabledPolicy,
				Policies: []autoscalingv2.HPAScalingPolicy{
					{Type: autoscalingv2.PercentScalingPolicy, Value: 20, PeriodSeconds: 60},
				},
			},
			expectedLimit: 10,
		},
		{
			name:            "MinChange policy",
			currentReplicas: 100,
			scaleUpEvents:   []timestampedScaleEvent{},
			scaleDownEvents: []timestampedScaleEvent{},
			scalingRules: &autoscalingv2.HPAScalingRules{
				SelectPolicy: &minChangePolicy,
				Policies: []autoscalingv2.HPAScalingPolicy{
					{Type: autoscalingv2.PercentScalingPolicy, Value: 10, PeriodSeconds: 60},
					{Type: autoscalingv2.PodsScalingPolicy, Value: 15, PeriodSeconds: 60},
				},
			},
			expectedLimit: 90,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit := calculateScaleDownLimitWithBehaviors(tt.currentReplicas, tt.scaleUpEvents, tt.scaleDownEvents, tt.scalingRules)
			assert.Equal(t, tt.expectedLimit, limit, "Unexpected scale down limit")
		})
	}
}

// TestSetCurrentReplicasInStatus verifies setting of current replicas in HPA status
func TestSetCurrentReplicasInStatus(t *testing.T) {
	controller := &FHPAController{}
	hpa := &autoscalingv1alpha1.FederatedHPA{
		Status: autoscalingv2.HorizontalPodAutoscalerStatus{
			DesiredReplicas: 5,
			CurrentMetrics: []autoscalingv2.MetricStatus{
				{Type: autoscalingv2.ResourceMetricSourceType},
			},
		},
	}

	controller.setCurrentReplicasInStatus(hpa, 3)

	assert.Equal(t, int32(3), hpa.Status.CurrentReplicas)
	assert.Equal(t, int32(5), hpa.Status.DesiredReplicas)
	assert.Len(t, hpa.Status.CurrentMetrics, 1)
	assert.Nil(t, hpa.Status.LastScaleTime)
}

// TestSetStatus ensures correct status setting for FederatedHPA
func TestSetStatus(t *testing.T) {
	tests := []struct {
		name             string
		currentReplicas  int32
		desiredReplicas  int32
		metricStatuses   []autoscalingv2.MetricStatus
		rescale          bool
		initialLastScale *metav1.Time
	}{
		{
			name:            "Update without rescale",
			currentReplicas: 3,
			desiredReplicas: 5,
			metricStatuses: []autoscalingv2.MetricStatus{
				{Type: autoscalingv2.ResourceMetricSourceType},
			},
			rescale:          false,
			initialLastScale: nil,
		},
		{
			name:            "Update with rescale",
			currentReplicas: 3,
			desiredReplicas: 5,
			metricStatuses: []autoscalingv2.MetricStatus{
				{Type: autoscalingv2.ResourceMetricSourceType},
			},
			rescale:          true,
			initialLastScale: &metav1.Time{Time: time.Now().Add(-1 * time.Hour)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := &FHPAController{}
			hpa := &autoscalingv1alpha1.FederatedHPA{
				Status: autoscalingv2.HorizontalPodAutoscalerStatus{
					LastScaleTime: tt.initialLastScale,
					Conditions: []autoscalingv2.HorizontalPodAutoscalerCondition{
						{Type: autoscalingv2.ScalingActive},
					},
				},
			}

			controller.setStatus(hpa, tt.currentReplicas, tt.desiredReplicas, tt.metricStatuses, tt.rescale)

			assert.Equal(t, tt.currentReplicas, hpa.Status.CurrentReplicas)
			assert.Equal(t, tt.desiredReplicas, hpa.Status.DesiredReplicas)
			assert.Equal(t, tt.metricStatuses, hpa.Status.CurrentMetrics)
			assert.Len(t, hpa.Status.Conditions, 1)

			if tt.rescale {
				assert.NotNil(t, hpa.Status.LastScaleTime)
				assert.True(t, hpa.Status.LastScaleTime.After(time.Now().Add(-1*time.Second)))
			} else {
				assert.Equal(t, tt.initialLastScale, hpa.Status.LastScaleTime)
			}
		})
	}
}

// TestSetCondition verifies proper condition setting in FederatedHPA
func TestSetCondition(t *testing.T) {
	tests := []struct {
		name           string
		initialHPA     *autoscalingv1alpha1.FederatedHPA
		conditionType  autoscalingv2.HorizontalPodAutoscalerConditionType
		status         corev1.ConditionStatus
		reason         string
		message        string
		args           []interface{}
		expectedLength int
		checkIndex     int
	}{
		{
			name:           "Add new condition",
			initialHPA:     &autoscalingv1alpha1.FederatedHPA{},
			conditionType:  autoscalingv2.ScalingActive,
			status:         corev1.ConditionTrue,
			reason:         "TestReason",
			message:        "Test message",
			expectedLength: 1,
			checkIndex:     0,
		},
		{
			name: "Update existing condition",
			initialHPA: &autoscalingv1alpha1.FederatedHPA{
				Status: autoscalingv2.HorizontalPodAutoscalerStatus{
					Conditions: []autoscalingv2.HorizontalPodAutoscalerCondition{
						{
							Type:   autoscalingv2.ScalingActive,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
			conditionType:  autoscalingv2.ScalingActive,
			status:         corev1.ConditionTrue,
			reason:         "UpdatedReason",
			message:        "Updated message",
			expectedLength: 1,
			checkIndex:     0,
		},
		{
			name:           "Add condition with formatted message",
			initialHPA:     &autoscalingv1alpha1.FederatedHPA{},
			conditionType:  autoscalingv2.AbleToScale,
			status:         corev1.ConditionTrue,
			reason:         "FormattedReason",
			message:        "Formatted message: %d",
			args:           []interface{}{42},
			expectedLength: 1,
			checkIndex:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setCondition(tt.initialHPA, tt.conditionType, tt.status, tt.reason, tt.message, tt.args...)

			assert.Len(t, tt.initialHPA.Status.Conditions, tt.expectedLength, "Unexpected number of conditions")

			condition := tt.initialHPA.Status.Conditions[tt.checkIndex]
			assert.Equal(t, tt.conditionType, condition.Type, "Unexpected condition type")
			assert.Equal(t, tt.status, condition.Status, "Unexpected condition status")
			assert.Equal(t, tt.reason, condition.Reason, "Unexpected condition reason")

			expectedMessage := tt.message
			if len(tt.args) > 0 {
				expectedMessage = fmt.Sprintf(tt.message, tt.args...)
			}
			assert.Equal(t, expectedMessage, condition.Message, "Unexpected condition message")
			assert.False(t, condition.LastTransitionTime.IsZero(), "LastTransitionTime should be set")
		})
	}
}

// TestSetConditionInList ensures proper condition setting in a list of conditions
func TestSetConditionInList(t *testing.T) {
	tests := []struct {
		name           string
		inputList      []autoscalingv2.HorizontalPodAutoscalerCondition
		conditionType  autoscalingv2.HorizontalPodAutoscalerConditionType
		status         corev1.ConditionStatus
		reason         string
		message        string
		args           []interface{}
		expectedLength int
		checkIndex     int
	}{
		{
			name:           "Add new condition",
			inputList:      []autoscalingv2.HorizontalPodAutoscalerCondition{},
			conditionType:  autoscalingv2.ScalingActive,
			status:         corev1.ConditionTrue,
			reason:         "TestReason",
			message:        "Test message",
			expectedLength: 1,
			checkIndex:     0,
		},
		{
			name: "Update existing condition",
			inputList: []autoscalingv2.HorizontalPodAutoscalerCondition{
				{
					Type:   autoscalingv2.ScalingActive,
					Status: corev1.ConditionFalse,
				},
			},
			conditionType:  autoscalingv2.ScalingActive,
			status:         corev1.ConditionTrue,
			reason:         "UpdatedReason",
			message:        "Updated message",
			expectedLength: 1,
			checkIndex:     0,
		},
		{
			name: "Add condition with formatted message",
			inputList: []autoscalingv2.HorizontalPodAutoscalerCondition{
				{
					Type:   autoscalingv2.ScalingActive,
					Status: corev1.ConditionTrue,
				},
			},
			conditionType:  autoscalingv2.AbleToScale,
			status:         corev1.ConditionTrue,
			reason:         "FormattedReason",
			message:        "Formatted message: %d",
			args:           []interface{}{42},
			expectedLength: 2,
			checkIndex:     1,
		},
		{
			name: "Update condition without changing status",
			inputList: []autoscalingv2.HorizontalPodAutoscalerCondition{
				{
					Type:               autoscalingv2.ScalingActive,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
				},
			},
			conditionType:  autoscalingv2.ScalingActive,
			status:         corev1.ConditionTrue,
			reason:         "NewReason",
			message:        "New message",
			expectedLength: 1,
			checkIndex:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := setConditionInList(tt.inputList, tt.conditionType, tt.status, tt.reason, tt.message, tt.args...)

			assert.Len(t, result, tt.expectedLength, "Unexpected length of result list")

			condition := result[tt.checkIndex]
			assert.Equal(t, tt.conditionType, condition.Type, "Unexpected condition type")
			assert.Equal(t, tt.status, condition.Status, "Unexpected condition status")
			assert.Equal(t, tt.reason, condition.Reason, "Unexpected condition reason")

			expectedMessage := tt.message
			if len(tt.args) > 0 {
				expectedMessage = fmt.Sprintf(tt.message, tt.args...)
			}
			assert.Equal(t, expectedMessage, condition.Message, "Unexpected condition message")

			if tt.name == "Update existing condition" {
				assert.False(t, condition.LastTransitionTime.IsZero(), "LastTransitionTime should be set")
			}

			if tt.name == "Update condition without changing status" {
				assert.Equal(t, tt.inputList[0].LastTransitionTime, condition.LastTransitionTime, "LastTransitionTime should not change")
			}
		})
	}
}

// Helper functions
func getCondition(conditions []autoscalingv2.HorizontalPodAutoscalerCondition, conditionType autoscalingv2.HorizontalPodAutoscalerConditionType) *autoscalingv2.HorizontalPodAutoscalerCondition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}

func createTestFederatedHPA(name, namespace string) *autoscalingv1alpha1.FederatedHPA {
	return &autoscalingv1alpha1.FederatedHPA{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func createTestHPA(minReplicas, maxReplicas int32, behavior *autoscalingv2.HorizontalPodAutoscalerBehavior) *autoscalingv1alpha1.FederatedHPA {
	return &autoscalingv1alpha1.FederatedHPA{
		Spec: autoscalingv1alpha1.FederatedHPASpec{
			MinReplicas: &minReplicas,
			MaxReplicas: maxReplicas,
			Behavior:    behavior,
		},
	}
}

func createTestScalingRules(stabilizationWindowSeconds *int32, selectPolicy *autoscalingv2.ScalingPolicySelect, policies []autoscalingv2.HPAScalingPolicy) *autoscalingv2.HPAScalingRules {
	return &autoscalingv2.HPAScalingRules{
		StabilizationWindowSeconds: stabilizationWindowSeconds,
		SelectPolicy:               selectPolicy,
		Policies:                   policies,
	}
}

func countOldRecommendations(recommendations []timestampedRecommendation, window time.Duration) int {
	count := 0
	now := time.Now()
	for _, rec := range recommendations {
		if rec.timestamp.Before(now.Add(-window)) {
			count++
		}
	}
	return count
}

func containsRecommendation(slice []timestampedRecommendation, recommendation int32) bool {
	for _, item := range slice {
		if item.recommendation == recommendation {
			return true
		}
	}
	return false
}
