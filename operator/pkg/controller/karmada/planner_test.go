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
	"fmt"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

func TestNewPlannerFor(t *testing.T) {
	name := "karmada-demo"
	tests := []struct {
		name       string
		karmada    *operatorv1alpha1.Karmada
		client     client.Client
		config     *rest.Config
		wantAction Action
		wantErr    bool
	}{
		{
			name: "NewPlannerFor_WithInitAction_PlannerConstructed",
			karmada: &operatorv1alpha1.Karmada{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Spec: operatorv1alpha1.KarmadaSpec{
					Components: &operatorv1alpha1.KarmadaComponents{
						Etcd: &operatorv1alpha1.Etcd{
							Local: &operatorv1alpha1.LocalEtcd{},
						},
					},
				},
			},
			client:     fake.NewFakeClient(),
			config:     &rest.Config{},
			wantAction: InitAction,
			wantErr:    false,
		},
		{
			name: "NewPlannerFor_WithDeInitAction_PlannerConstructed",
			karmada: &operatorv1alpha1.Karmada{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
					DeletionTimestamp: &metav1.Time{
						Time: time.Now().Add(-5 * time.Minute),
					},

					Finalizers: []string{ControllerFinalizerName},
				},
				Spec: operatorv1alpha1.KarmadaSpec{
					Components: &operatorv1alpha1.KarmadaComponents{
						Etcd: &operatorv1alpha1.Etcd{
							Local: &operatorv1alpha1.LocalEtcd{},
						},
					},
				},
			},
			client:     fake.NewFakeClient(),
			config:     &rest.Config{},
			wantAction: DeInitAction,
			wantErr:    false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			planner, err := NewPlannerFor(test.karmada, test.client, test.config)
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if planner.action != test.wantAction {
				t.Errorf("expected planner action to be %s, but got %s", test.wantAction, planner.action)
			}
		})
	}
}

func TestPreRunJob(t *testing.T) {
	name, namespace := "karmada-demo", names.NamespaceDefault
	tests := []struct {
		name    string
		karmada *operatorv1alpha1.Karmada
		config  *rest.Config
		action  Action
		verify  func(p *Planner, conditionStatus metav1.ConditionStatus, conditionMsg, conditionReason string) error
		wantErr bool
	}{
		{
			name: "PreRunJob_WithInitActionPlanned_PreRunJobCompleted",
			karmada: &operatorv1alpha1.Karmada{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: operatorv1alpha1.KarmadaSpec{
					Components: &operatorv1alpha1.KarmadaComponents{
						Etcd: &operatorv1alpha1.Etcd{
							Local: &operatorv1alpha1.LocalEtcd{},
						},
					},
				},
			},
			config:  &rest.Config{},
			action:  InitAction,
			verify:  verifyJobInCommon,
			wantErr: false,
		},
		{
			name: "PreRunJob_WithDeInitActionPlanned_PreRunJobCompleted",
			karmada: &operatorv1alpha1.Karmada{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					DeletionTimestamp: &metav1.Time{
						Time: time.Now().Add(-5 * time.Minute),
					},
					Finalizers: []string{ControllerFinalizerName},
				},
				Spec: operatorv1alpha1.KarmadaSpec{
					Components: &operatorv1alpha1.KarmadaComponents{
						Etcd: &operatorv1alpha1.Etcd{
							Local: &operatorv1alpha1.LocalEtcd{},
						},
					},
				},
			},
			config:  &rest.Config{},
			action:  DeInitAction,
			verify:  verifyJobInCommon,
			wantErr: false,
		},
		{
			name: "PreRunJob_WithUnknownActionPlanned_PreRunJobCompleted",
			karmada: &operatorv1alpha1.Karmada{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: operatorv1alpha1.KarmadaSpec{
					Components: &operatorv1alpha1.KarmadaComponents{
						Etcd: &operatorv1alpha1.Etcd{
							Local: &operatorv1alpha1.LocalEtcd{},
						},
					},
				},
			},
			config: &rest.Config{},
			action: "UnknownAction",
			verify: func(planner *Planner, _ metav1.ConditionStatus, _, _ string) error {
				// Check the status conditions.
				conditions := planner.karmada.Status.Conditions
				if len(conditions) != 0 {
					return fmt.Errorf("expected no conditions, but got %d conditions", len(conditions))
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, err := prepJobInCommon(test.karmada)
			if err != nil {
				t.Fatalf("failed to prep before creating planner, got error: %v", err)
			}

			planner, err := NewPlannerFor(test.karmada, client, test.config)
			if err != nil {
				t.Fatalf("failed to create planner, got error: %v", err)
			}
			planner.action = test.action

			err = planner.preRunJob()
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}

			conditionMsg := fmt.Sprintf("karmada %s job is in progressing", strings.ToLower(string(test.action)))
			if err := test.verify(planner, metav1.ConditionFalse, conditionMsg, "Progressing"); err != nil {
				t.Errorf("failed to verify the pre running job, got error: %v", err)
			}
		})
	}
}

func TestAfterRunJob(t *testing.T) {
	name, namespace := "karmada-demo", names.NamespaceDefault
	tests := []struct {
		name    string
		karmada *operatorv1alpha1.Karmada
		config  *rest.Config
		action  Action
		verify  func(*operatorv1alpha1.Karmada, *Planner, Action) error
		wantErr bool
	}{
		{
			name: "AfterRunJob_WithInitActionPlannedAndHostClusterIsLocal_AfterRunJobCompleted",
			karmada: &operatorv1alpha1.Karmada{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: operatorv1alpha1.KarmadaSpec{
					Components: &operatorv1alpha1.KarmadaComponents{
						Etcd: &operatorv1alpha1.Etcd{
							Local: &operatorv1alpha1.LocalEtcd{},
						},
					},
				},
			},
			config: &rest.Config{},
			action: InitAction,
			verify: func(karmada *operatorv1alpha1.Karmada, planner *Planner, action Action) error {
				secretRefNameExpected := util.AdminKarmadaConfigSecretName(karmada.GetName())
				if planner.karmada.Status.SecretRef == nil {
					return fmt.Errorf("expected SecretRef to be set, but got nil")
				}
				if planner.karmada.Status.SecretRef.Name != secretRefNameExpected {
					return fmt.Errorf("expected SecretRef Name to be %s, but got %s", secretRefNameExpected, planner.karmada.Status.SecretRef.Name)
				}
				if planner.karmada.Status.SecretRef.Namespace != names.NamespaceDefault {
					return fmt.Errorf("expected SecretRef Namespace to be %s, but got %s", names.NamespaceDefault, planner.karmada.Status.SecretRef.Namespace)
				}

				conditionMsg := fmt.Sprintf("karmada %s job is completed", strings.ToLower(string(action)))
				if err := verifyJobInCommon(planner, metav1.ConditionTrue, conditionMsg, "Completed"); err != nil {
					return fmt.Errorf("failed to verify after run job, got error: %v", err)
				}
				if planner.karmada.Status.APIServerService == nil {
					return fmt.Errorf("expected API Server service ref to be set, but got nil")
				}
				expectedAPIServerName := util.KarmadaAPIServerName(karmada.GetName())
				if planner.karmada.Status.APIServerService.Name != expectedAPIServerName {
					return fmt.Errorf("expected API Server service Name to be %s, but got %s", expectedAPIServerName, planner.karmada.Status.APIServerService.Name)
				}

				return nil
			},
			wantErr: false,
		},
		{
			name: "AfterRunJob_WithDeInitActionPlanned_AfterRunJobCompleted",
			karmada: &operatorv1alpha1.Karmada{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					DeletionTimestamp: &metav1.Time{
						Time: time.Now().Add(-5 * time.Minute),
					},
					Finalizers: []string{ControllerFinalizerName},
				},
				Spec: operatorv1alpha1.KarmadaSpec{
					Components: &operatorv1alpha1.KarmadaComponents{
						Etcd: &operatorv1alpha1.Etcd{
							Local: &operatorv1alpha1.LocalEtcd{},
						},
					},
				},
			},
			config: &rest.Config{},
			action: DeInitAction,
			verify: func(_ *operatorv1alpha1.Karmada, planner *Planner, _ Action) error {
				conditions := planner.karmada.Status.Conditions
				if len(conditions) != 0 {
					t.Errorf("expected no conditions, but got %d conditions", len(conditions))
				}
				if planner.karmada.Status.SecretRef != nil {
					t.Errorf("expected SecretRef to be nil, but got %v", planner.karmada.Status.SecretRef)
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, err := prepJobInCommon(test.karmada)
			if err != nil {
				t.Fatalf("failed to prep before creating planner, got error: %v", err)
			}

			planner, err := NewPlannerFor(test.karmada, client, test.config)
			if err != nil {
				t.Fatalf("failed to create planner, got error: %v", err)
			}
			planner.action = test.action

			err = planner.afterRunJob()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if err := test.verify(test.karmada, planner, test.action); err != nil {
				t.Errorf("failed to verify the after running job, got error: %v", err)
			}
		})
	}
}
func TestRunJobErr(t *testing.T) {
	name, namespace := "karmada-demo", names.NamespaceDefault
	tests := []struct {
		name    string
		karmada *operatorv1alpha1.Karmada
		config  *rest.Config
		jobErr  error
		wantErr bool
	}{
		{
			name: "RunJobErr_WithInitActionPlannedAndHostClusterIsLocal_AfterRunJobCompleted",
			karmada: &operatorv1alpha1.Karmada{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: operatorv1alpha1.KarmadaSpec{
					Components: &operatorv1alpha1.KarmadaComponents{
						Etcd: &operatorv1alpha1.Etcd{
							Local: &operatorv1alpha1.LocalEtcd{},
						},
					},
				},
			},
			config:  &rest.Config{},
			jobErr:  errors.New("test error"),
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, err := prepJobInCommon(test.karmada)
			if err != nil {
				t.Fatalf("failed to prep before creating planner, got error: %v", err)
			}

			planner, err := NewPlannerFor(test.karmada, client, test.config)
			if err != nil {
				t.Fatalf("failed to create planner, got error: %v", err)
			}

			err = planner.runJobErr(test.jobErr)
			if err == nil && test.wantErr {
				t.Fatalf("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error: %v", err)
			}
			if !containsError(err, test.jobErr) {
				t.Errorf("expected job error to contain: %v, but got: %v", test.jobErr, err)
			}
			if err := verifyJobInCommon(planner, metav1.ConditionFalse, test.jobErr.Error(), "Failed"); err != nil {
				t.Errorf("failed to verify run job err, got error: %v", err)
			}
		})
	}
}

// prepJobInCommon prepares a fake Kubernetes client for testing purposes.
// It creates a new scheme and adds the operatorv1alpha1 types to it.
// A fake client is then built using the provided Karmada object.
func prepJobInCommon(karmada *operatorv1alpha1.Karmada) (client.Client, error) {
	// Create a scheme and add operatorv1alpha1 type to it.
	scheme := runtime.NewScheme()
	err := operatorv1alpha1.AddToScheme(scheme)
	if err != nil {
		return nil, fmt.Errorf("error adding operatorv1alpha1 to k8s scheme %v", err)
	}

	// Create a fake client with the scheme.
	client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(karmada).WithStatusSubresource(karmada).Build()
	return client, nil
}

// verifyJobInCommon verifies the conditions of a Karmada job by checking its status conditions.
// It ensures that the job has at least one condition and that its type, status, reason, and message
// match the expected values provided as arguments.
func verifyJobInCommon(planner *Planner, conditionStatus metav1.ConditionStatus, conditionMsg, conditionReason string) error {
	conditions := planner.karmada.Status.Conditions
	if len(conditions) < 1 {
		return fmt.Errorf("expected at least one condition, but got %d", len(conditions))
	}
	if conditions[0].Type != string(operatorv1alpha1.Ready) {
		return fmt.Errorf("expected condition type to be %s, but got %s", operatorv1alpha1.Ready, conditions[0].Type)
	}
	if conditions[0].Status != conditionStatus {
		return fmt.Errorf("expected condition status to be %s, but got %s", conditionStatus, conditions[0].Status)
	}
	if conditions[0].Reason != conditionReason {
		return fmt.Errorf("expected condition reason to be %s, but got %s", conditionReason, conditions[0].Reason)
	}
	if conditions[0].Message != conditionMsg {
		return fmt.Errorf("expected condition message to be %s, but got %s", conditionMsg, conditions[0].Message)
	}

	return nil
}

// containsError checks if a target error is present within an aggregated error.
// It verifies if the aggregated error is non-nil and if it contains the specified target error.
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
