package scheduler

import (
	"context"
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	karmadafake "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/fake"
	"github.com/karmada-io/karmada/pkg/util"
)

func TestCreateScheduler(t *testing.T) {
	dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	karmadaClient := karmadafake.NewSimpleClientset()
	kubeClient := fake.NewSimpleClientset()
	port := 10025

	testcases := []struct {
		name                     string
		opts                     []Option
		enableSchedulerEstimator bool
		schedulerEstimatorPort   int
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
				WithSchedulerEstimatorPort(port),
			},
			enableSchedulerEstimator: true,
			schedulerEstimatorPort:   port,
		},
		{
			name: "scheduler with enableSchedulerEstimator disabled, WithSchedulerEstimatorPort enabled",
			opts: []Option{
				WithEnableSchedulerEstimator(false),
				WithSchedulerEstimatorPort(port),
			},
			enableSchedulerEstimator: false,
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

			if tc.schedulerEstimatorPort != sche.schedulerEstimatorPort {
				t.Errorf("unexpected schedulerEstimatorPort want %v, got %v", tc.schedulerEstimatorPort, sche.schedulerEstimatorPort)
			}
		})
	}
}

func Test_patchBindingScheduleStatus(t *testing.T) {
	oneHourBefore := time.Now().Add(-1 * time.Hour).Round(time.Second)
	oneHourAfter := time.Now().Add(1 * time.Hour).Round(time.Second)

	successCondition := util.NewCondition(workv1alpha2.Scheduled, scheduleSuccessReason, scheduleSuccessMessage, metav1.ConditionTrue)
	failureCondition := util.NewCondition(workv1alpha2.Scheduled, scheduleFailedReason, "schedule error", metav1.ConditionFalse)

	successCondition.LastTransitionTime = metav1.Time{Time: oneHourBefore}
	failureCondition.LastTransitionTime = metav1.Time{Time: oneHourAfter}

	dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	karmadaClient := karmadafake.NewSimpleClientset()
	kubeClient := fake.NewSimpleClientset()

	scheduler, err := NewScheduler(dynamicClient, karmadaClient, kubeClient)
	if err != nil {
		t.Error(err)
	}

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
			name: "replace to success condition",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-3", Namespace: "default", Generation: 1},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{failureCondition}, SchedulerObservedGeneration: 2},
			},
			newScheduledCondition: successCondition,
			expected: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-3", Namespace: "default"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{successCondition}, SchedulerObservedGeneration: 1},
			},
		},
		{
			name: "replace failure condition",
			binding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-4", Namespace: "default"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{successCondition}},
			},
			newScheduledCondition: failureCondition,
			expected: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-4", Namespace: "default"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{failureCondition}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := karmadaClient.WorkV1alpha2().ResourceBindings(test.binding.Namespace).Create(context.TODO(), test.binding, metav1.CreateOptions{})
			if err != nil {
				t.Fatal(err)
			}
			err = scheduler.patchBindingScheduleStatus(test.binding, test.newScheduledCondition)
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

func Test_patchClusterBindingScheduleStatus(t *testing.T) {
	oneHourBefore := time.Now().Add(-1 * time.Hour).Round(time.Second)
	oneHourAfter := time.Now().Add(1 * time.Hour).Round(time.Second)

	successCondition := util.NewCondition(workv1alpha2.Scheduled, scheduleSuccessReason, scheduleSuccessMessage, metav1.ConditionTrue)
	failureCondition := util.NewCondition(workv1alpha2.Scheduled, scheduleFailedReason, "schedule error", metav1.ConditionFalse)

	successCondition.LastTransitionTime = metav1.Time{Time: oneHourBefore}
	failureCondition.LastTransitionTime = metav1.Time{Time: oneHourAfter}

	dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	karmadaClient := karmadafake.NewSimpleClientset()
	kubeClient := fake.NewSimpleClientset()

	scheduler, err := NewScheduler(dynamicClient, karmadaClient, kubeClient)
	if err != nil {
		t.Error(err)
	}

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
			name: "replace to success condition",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-3", Generation: 1},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{failureCondition}, SchedulerObservedGeneration: 2},
			},
			newScheduledCondition: successCondition,
			expected: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-3"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{successCondition}, SchedulerObservedGeneration: 1},
			},
		},
		{
			name: "replace failure condition",
			binding: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-4"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{successCondition}},
			},
			newScheduledCondition: failureCondition,
			expected: &workv1alpha2.ClusterResourceBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "rb-4"},
				Spec:       workv1alpha2.ResourceBindingSpec{},
				Status:     workv1alpha2.ResourceBindingStatus{Conditions: []metav1.Condition{failureCondition}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := karmadaClient.WorkV1alpha2().ClusterResourceBindings().Create(context.TODO(), test.binding, metav1.CreateOptions{})
			if err != nil {
				t.Fatal(err)
			}
			err = scheduler.patchClusterBindingScheduleStatus(test.binding, test.newScheduledCondition)
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
