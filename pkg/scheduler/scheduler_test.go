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
	"fmt"
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	karmadafake "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/fake"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/grpcconnection"
)

func TestCreateScheduler(t *testing.T) {
	dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	karmadaClient := karmadafake.NewSimpleClientset()
	kubeClient := fake.NewSimpleClientset()
	port := 10025
	servicePrefix := "test-service-prefix"
	schedulerName := "test-scheduler"
	timeout := metav1.Duration{Duration: 5 * time.Second}

	testcases := []struct {
		name                                string
		opts                                []Option
		enableSchedulerEstimator            bool
		schedulerEstimatorPort              int
		disableSchedulerEstimatorInPullMode bool
		schedulerEstimatorTimeout           metav1.Duration
		schedulerEstimatorServicePrefix     string
		schedulerName                       string
		schedulerEstimatorClientConfig      *grpcconnection.ClientConfig
		enableEmptyWorkloadPropagation      bool
		plugins                             []string
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

			if tc.schedulerEstimatorServicePrefix != sche.schedulerEstimatorServicePrefix {
				t.Errorf("unexpected schedulerEstimatorServicePrefix want %v, got %v", tc.schedulerEstimatorServicePrefix, sche.schedulerEstimatorServicePrefix)
			}

			if tc.schedulerName != sche.schedulerName {
				t.Errorf("unexpected schedulerName want %v, got %v", tc.schedulerName, sche.schedulerName)
			}

			if tc.enableEmptyWorkloadPropagation != sche.enableEmptyWorkloadPropagation {
				t.Errorf("unexpected enableEmptyWorkloadPropagation want %v, got %v", tc.enableEmptyWorkloadPropagation, sche.enableEmptyWorkloadPropagation)
			}
		})
	}
}
func Test_patchBindingStatusCondition(t *testing.T) {
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

func Test_patchBindingStatusWithAffinityName(t *testing.T) {
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

func Test_patchClusterBindingStatusCondition(t *testing.T) {
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

func Test_patchClusterBindingStatusWithAffinityName(t *testing.T) {
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

func Test_recordScheduleResultEventForResourceBinding(t *testing.T) {
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

func Test_recordScheduleResultEventForClusterResourceBinding(t *testing.T) {
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

func Test_targetClustersToString(t *testing.T) {
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
