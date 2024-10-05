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

package karmada

import (
	"errors"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

func TestNewPlannerFor_InitAction(t *testing.T) {
	karmada := &operatorv1alpha1.Karmada{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-karmada",
		},
	}
	cl := fake.NewFakeClient()
	config := &rest.Config{}

	planner, err := NewPlannerFor(karmada, cl, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if planner.action != InitAction {
		t.Errorf("expected %s but got %s", InitAction, planner.action)
	}
	if planner.job == nil {
		t.Errorf("expected planner job not to be nil")
	}
}

func TestNewPlannerFor_DeInitAction(t *testing.T) {
	karmada := &operatorv1alpha1.Karmada{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-karmada",
			DeletionTimestamp: &metav1.Time{
				Time: time.Now().Add(-5 * time.Minute),
			},
			Finalizers: []string{ControllerFinalizerName},
		},
	}
	cl := fake.NewFakeClient()
	config := &rest.Config{}

	planner, err := NewPlannerFor(karmada, cl, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if planner.action != DeInitAction {
		t.Errorf("expected %s but got %s", DeInitAction, planner.action)
	}
	if planner.job == nil {
		t.Errorf("expected planner job not to be nil")
	}
}

func TestPreRunJob_InitAction(t *testing.T) {
	// Create a scheme and add operatorv1alpha1 type to it.
	scheme := runtime.NewScheme()
	err := operatorv1alpha1.AddToScheme(scheme)
	if err != nil {
		t.Fatalf("error adding operatorv1alpha1 to k8s scheme %v", err)
	}

	// Create a fake Karmada object.
	karmada := &operatorv1alpha1.Karmada{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-karmada",
			Namespace: names.NamespaceDefault,
		},
	}

	// Create a fake client with the scheme.
	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(karmada).WithStatusSubresource(karmada).Build()
	config := &rest.Config{}

	// Create a planner with InitAction.
	planner, err := NewPlannerFor(karmada, cl, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Run the preRunJob method.
	err = planner.preRunJob()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check the status conditions.
	if len(planner.karmada.Status.Conditions) == 0 {
		t.Fatalf("expected at least one condition, but got none")
	}

	expectedConditionMsg := "karmada init job is in progressing"
	condition := planner.karmada.Status.Conditions[0]
	if condition.Type != string(operatorv1alpha1.Ready) {
		t.Errorf("expected condition type to be %s, but got %s", operatorv1alpha1.Ready, condition.Type)
	}
	if condition.Status != metav1.ConditionFalse {
		t.Errorf("expected condition status to be %s, but got %s", metav1.ConditionFalse, condition.Status)
	}
	if condition.Reason != "Progressing" {
		t.Errorf("expected condition reason to be 'Progressing', but got %s", condition.Reason)
	}
	if condition.Message != expectedConditionMsg {
		t.Errorf("expected condition message to be %s, but got %s", expectedConditionMsg, condition.Message)
	}
}

func TestPreRunJob_DeInitAction(t *testing.T) {
	// Create a scheme and add operatorv1alpha1 type to it.
	scheme := runtime.NewScheme()
	err := operatorv1alpha1.AddToScheme(scheme)
	if err != nil {
		t.Fatalf("error adding operatorv1alpha1 to k8s scheme %v", err)
	}

	// Create a fake Karmada object.
	karmada := &operatorv1alpha1.Karmada{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-karmada",
			Namespace: names.NamespaceDefault,
			DeletionTimestamp: &metav1.Time{
				Time: time.Now().Add(-5 * time.Minute),
			},
			Finalizers: []string{ControllerFinalizerName},
		},
	}

	// Create a fake client with the scheme.
	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(karmada).WithStatusSubresource(karmada).Build()
	config := &rest.Config{}

	// Create a planner with DeInitAction.
	planner, err := NewPlannerFor(karmada, cl, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Run the preRunJob method.
	err = planner.preRunJob()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check the status conditions.
	if len(planner.karmada.Status.Conditions) == 0 {
		t.Fatalf("expected at least one condition, but got none")
	}

	expectedConditionMsg := "karmada deinit job is in progressing"
	condition := planner.karmada.Status.Conditions[0]
	if condition.Type != string(operatorv1alpha1.Ready) {
		t.Errorf("expected condition type to be %s, but got %s", operatorv1alpha1.Ready, condition.Type)
	}
	if condition.Status != metav1.ConditionFalse {
		t.Errorf("expected condition status to be %s, but got %s", metav1.ConditionFalse, condition.Status)
	}
	if condition.Reason != "Progressing" {
		t.Errorf("expected condition reason to be 'Progressing', but got %s", condition.Reason)
	}
	if condition.Message != expectedConditionMsg {
		t.Errorf("expected condition message to be %s, but got %s", expectedConditionMsg, condition.Message)
	}
}

func TestPreRunJob_UnknownAction(t *testing.T) {
	// Create a scheme and add operatorv1alpha1 type to it.
	scheme := runtime.NewScheme()
	err := operatorv1alpha1.AddToScheme(scheme)
	if err != nil {
		t.Fatalf("error adding operatorv1alpha1 to k8s scheme %v", err)
	}

	// Create a fake Karmada object.
	karmada := &operatorv1alpha1.Karmada{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-karmada",
			Namespace: names.NamespaceDefault,
		},
	}

	// Create a fake client with the scheme.
	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(karmada).WithStatusSubresource(karmada).Build()
	config := &rest.Config{}

	// Create a planner with UnknownAction.
	planner, err := NewPlannerFor(karmada, cl, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	planner.action = "UnknownAction"

	// Run the preRunJob method.
	err = planner.preRunJob()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check the status conditions.
	if len(planner.karmada.Status.Conditions) != 0 {
		t.Errorf("expected no conditions, but got %d conditions", len(planner.karmada.Status.Conditions))
	}
}

func TestAfterRunJobWith_InitAction_HostClusterIsLocal(t *testing.T) {
	// Create a scheme and add operatorv1alpha1 type to it.
	scheme := runtime.NewScheme()
	err := operatorv1alpha1.AddToScheme(scheme)
	if err != nil {
		t.Fatalf("error adding operatorv1alpha1 to k8s scheme %v", err)
	}

	// Create a fake Karmada object.
	karmada := &operatorv1alpha1.Karmada{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-karmada",
			Namespace: names.NamespaceDefault,
		},
		Spec: operatorv1alpha1.KarmadaSpec{},
	}

	// Create a fake client with the scheme.
	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(karmada).WithStatusSubresource(karmada).Build()
	config := &rest.Config{}

	// Create a planner with InitAction.
	planner, err := NewPlannerFor(karmada, cl, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Run the afterRunJob method.
	err = planner.afterRunJob()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check the status update.
	expectedSecretRefName := util.AdminKubeconfigSecretName(karmada.GetName())
	if planner.karmada.Status.SecretRef == nil {
		t.Fatalf("expected SecretRef to be set, but got nil")
	}
	if planner.karmada.Status.SecretRef.Name != expectedSecretRefName {
		t.Errorf("expected SecretRef Name to be %s, but got %s", expectedSecretRefName, planner.karmada.Status.SecretRef.Name)
	}
	if planner.karmada.Status.SecretRef.Namespace != names.NamespaceDefault {
		t.Errorf("expected SecretRef Namespace to be %s, but got %s", names.NamespaceDefault, planner.karmada.Status.SecretRef.Namespace)
	}

	// Check the status conditions.
	if len(planner.karmada.Status.Conditions) == 0 {
		t.Fatalf("expected at least one condition, but got none")
	}

	expectedConditionMsg := "karmada init job is completed"
	condition := planner.karmada.Status.Conditions[0]
	if condition.Type != string(operatorv1alpha1.Ready) {
		t.Errorf("expected condition type to be %s, but got %s", operatorv1alpha1.Ready, condition.Type)
	}
	if condition.Status != metav1.ConditionTrue {
		t.Errorf("expected condition status to be %s, but got %s", metav1.ConditionTrue, condition.Status)
	}
	if condition.Reason != "Completed" {
		t.Errorf("expected condition reason to be 'Completed', but got %s", condition.Reason)
	}
	if condition.Message != expectedConditionMsg {
		t.Errorf("expected condition message to be %s, but got %s", expectedConditionMsg, condition.Message)
	}
}

func TestAfterRunJobWith_DeInitAction(t *testing.T) {
	// Create a scheme and add operatorv1alpha1 type to it.
	scheme := runtime.NewScheme()
	err := operatorv1alpha1.AddToScheme(scheme)
	if err != nil {
		t.Fatalf("error adding operatorv1alpha1 to k8s scheme %v", err)
	}

	// Create a fake Karmada object.
	karmada := &operatorv1alpha1.Karmada{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-karmada",
			Namespace: names.NamespaceDefault,
			DeletionTimestamp: &metav1.Time{
				Time: time.Now().Add(-5 * time.Minute),
			},
			Finalizers: []string{ControllerFinalizerName},
		},
	}

	// Create a fake client with the scheme.
	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(karmada).WithStatusSubresource(karmada).Build()
	config := &rest.Config{}

	// Create a planner with DeInitAction.
	planner, err := NewPlannerFor(karmada, cl, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	planner.action = DeInitAction

	// Run the afterRunJob method.
	err = planner.afterRunJob()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check no status updates and conditions happened.
	if planner.karmada.Status.SecretRef != nil {
		t.Errorf("expected SecretRef to be nil, but got %v", planner.karmada.Status.SecretRef)
	}
	if len(planner.karmada.Status.Conditions) != 0 {
		t.Errorf("expected no conditions, but got %d conditions", len(planner.karmada.Status.Conditions))
	}
}

func TestRunJobErr(t *testing.T) {
	// Create a scheme and add operatorv1alpha1 type to it.
	scheme := runtime.NewScheme()
	err := operatorv1alpha1.AddToScheme(scheme)
	if err != nil {
		t.Fatalf("error adding operatorv1alpha1 to k8s scheme %v", err)
	}

	// Create a fake Karmada object.
	karmada := &operatorv1alpha1.Karmada{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-karmada",
			Namespace: names.NamespaceDefault,
		},
	}

	// Create a fake client with the scheme.
	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(karmada).WithStatusSubresource(karmada).Build()
	config := &rest.Config{}

	// Create a planner with InitAction.
	planner, err := NewPlannerFor(karmada, cl, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Create a test error to pass into runJobErr.
	testError := errors.New("test error")

	// Run the runJobErr method and check the returned error.
	aggErr := planner.runJobErr(testError)
	if aggErr == nil {
		t.Fatalf("expected error but got nil")
	}

	// Use the helper function to check if the aggregated error contains the test error.
	if !containsError(aggErr, testError) {
		t.Errorf("expected aggregated error to contain: %v, but got: %v", testError, aggErr)
	}

	// Check that the Karmada condition was updated to Failed.
	if len(karmada.Status.Conditions) == 0 {
		t.Fatalf("expected at least one condition, but got none")
	}

	condition := karmada.Status.Conditions[0]
	if condition.Type != string(operatorv1alpha1.Ready) {
		t.Errorf("expected condition type to be %s, but got %s", string(operatorv1alpha1.Ready), condition.Type)
	}
	if condition.Status != metav1.ConditionFalse {
		t.Errorf("expected condition status to be %s, but got '%s'", metav1.ConditionFalse, condition.Status)
	}
	if condition.Reason != "Failed" {
		t.Errorf("expected condition reason to be 'Failed', but got %s", condition.Reason)
	}
	if condition.Message != testError.Error() {
		t.Errorf("expected condition message to be %s, but got %s", testError.Error(), condition.Message)
	}
}

func containsError(aggErr error, targetErr error) bool {
	if aggErr != nil {
		if agg, ok := aggErr.(utilerrors.Aggregate); ok {
			for _, err := range agg.Errors() {
				if err.Error() == targetErr.Error() {
					return true
				}
			}
		}
	}
	return false
}
