/*
Copyright 2025 The Karmada Authors.

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

package pdb

import (
	"context"
	"fmt"
	"testing"

	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	coretesting "k8s.io/client-go/testing"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/constants"
)

func TestEnsurePodDisruptionBudget(t *testing.T) {
	name := "karmada-demo"
	namespace := "test"
	component := constants.KarmadaAPIServer

	tests := []struct {
		name           string
		commonSettings *operatorv1alpha1.CommonSettings
		existingPDB    *policyv1.PodDisruptionBudget
		expectedError  bool
		expectedAction string
	}{
		{
			name: "Create PDB with MinAvailable",
			commonSettings: &operatorv1alpha1.CommonSettings{
				PodDisruptionBudgetConfig: &operatorv1alpha1.PodDisruptionBudgetConfig{
					MinAvailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
				},
			},
			existingPDB:    nil,
			expectedError:  false,
			expectedAction: "create",
		},
		{
			name: "Create PDB with MaxUnavailable",
			commonSettings: &operatorv1alpha1.CommonSettings{
				PodDisruptionBudgetConfig: &operatorv1alpha1.PodDisruptionBudgetConfig{
					MaxUnavailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
				},
			},
			existingPDB:    nil,
			expectedError:  false,
			expectedAction: "create",
		},
		{
			name: "Update existing PDB",
			commonSettings: &operatorv1alpha1.CommonSettings{
				PodDisruptionBudgetConfig: &operatorv1alpha1.PodDisruptionBudgetConfig{
					MinAvailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 2},
				},
			},
			existingPDB: &policyv1.PodDisruptionBudget{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "karmada-demo-karmada-apiserver",
					Namespace:       namespace,
					ResourceVersion: "1",
				},
				Spec: policyv1.PodDisruptionBudgetSpec{
					MinAvailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
				},
			},
			expectedError:  false,
			expectedAction: "update",
		},
		{
			name:           "Delete PDB when config is nil",
			commonSettings: &operatorv1alpha1.CommonSettings{},
			existingPDB: &policyv1.PodDisruptionBudget{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "karmada-demo-karmada-apiserver",
					Namespace: namespace,
				},
			},
			expectedError:  false,
			expectedAction: "delete",
		},
		{
			name:           "No-op when config is nil and no PDB exists",
			commonSettings: &operatorv1alpha1.CommonSettings{},
			existingPDB:    nil,
			expectedError:  false,
			expectedAction: "none",
		},
		{
			name:           "Delete PDB when commonSettings is nil",
			commonSettings: nil,
			existingPDB: &policyv1.PodDisruptionBudget{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "karmada-demo-karmada-apiserver",
					Namespace: namespace,
				},
			},
			expectedError:  false,
			expectedAction: "delete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objs []runtime.Object
			if tt.existingPDB != nil {
				objs = append(objs, tt.existingPDB)
			}
			fakeClient := fakeclientset.NewSimpleClientset(objs...)

			err := EnsurePodDisruptionBudget(component, name, namespace, tt.commonSettings, fakeClient)

			if (err != nil) != tt.expectedError {
				t.Errorf("EnsurePodDisruptionBudget() error = %v, expectedError %v", err, tt.expectedError)
				return
			}

			actions := fakeClient.Actions()
			switch tt.expectedAction {
			case "create":
				if len(actions) != 1 {
					t.Errorf("expected 1 action for create, got %d", len(actions))
					return
				}
				if !actions[0].Matches("create", "poddisruptionbudgets") {
					t.Errorf("expected create action, got %v", actions[0])
				}
			case "update":
				if len(actions) < 2 {
					t.Errorf("expected at least 2 actions for update, got %d", len(actions))
					return
				}
			case "delete":
				if len(actions) != 1 {
					t.Errorf("expected 1 action for delete, got %d", len(actions))
					return
				}
				if !actions[0].Matches("delete", "poddisruptionbudgets") {
					t.Errorf("expected delete action, got %v", actions[0])
				}
			case "none":
				if len(actions) != 1 {
					t.Errorf("expected 1 action (delete check), got %d", len(actions))
				}
			}
		})
	}
}

func TestCreatePodDisruptionBudget(t *testing.T) {
	karmadaName := "karmada-demo"
	namespace := "test"
	component := constants.KarmadaAPIServer

	tests := []struct {
		name           string
		commonSettings *operatorv1alpha1.CommonSettings
		validateFunc   func(*testing.T, *policyv1.PodDisruptionBudget)
	}{
		{
			name: "Create PDB with MinAvailable",
			commonSettings: &operatorv1alpha1.CommonSettings{
				PodDisruptionBudgetConfig: &operatorv1alpha1.PodDisruptionBudgetConfig{
					MinAvailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
				},
			},
			validateFunc: func(t *testing.T, pdb *policyv1.PodDisruptionBudget) {
				if pdb.Name != "karmada-demo-karmada-apiserver" {
					t.Errorf("expected name 'karmada-demo-karmada-apiserver', got %s", pdb.Name)
				}
				if pdb.Namespace != namespace {
					t.Errorf("expected namespace '%s', got %s", namespace, pdb.Namespace)
				}
				if pdb.Spec.MinAvailable == nil || pdb.Spec.MinAvailable.IntVal != 1 {
					t.Errorf("expected MinAvailable=1, got %v", pdb.Spec.MinAvailable)
				}
				if pdb.Spec.MaxUnavailable != nil {
					t.Errorf("expected MaxUnavailable to be nil, got %v", pdb.Spec.MaxUnavailable)
				}
			},
		},
		{
			name: "Create PDB with MaxUnavailable",
			commonSettings: &operatorv1alpha1.CommonSettings{
				PodDisruptionBudgetConfig: &operatorv1alpha1.PodDisruptionBudgetConfig{
					MaxUnavailable: &intstr.IntOrString{Type: intstr.String, StrVal: "50%"},
				},
			},
			validateFunc: func(t *testing.T, pdb *policyv1.PodDisruptionBudget) {
				if pdb.Spec.MaxUnavailable == nil || pdb.Spec.MaxUnavailable.StrVal != "50%" {
					t.Errorf("expected MaxUnavailable='50%%', got %v", pdb.Spec.MaxUnavailable)
				}
				if pdb.Spec.MinAvailable != nil {
					t.Errorf("expected MinAvailable to be nil, got %v", pdb.Spec.MinAvailable)
				}
			},
		},
		{
			name: "Verify labels and selector",
			commonSettings: &operatorv1alpha1.CommonSettings{
				PodDisruptionBudgetConfig: &operatorv1alpha1.PodDisruptionBudgetConfig{
					MinAvailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
				},
			},
			validateFunc: func(t *testing.T, pdb *policyv1.PodDisruptionBudget) {
				expectedLabels := map[string]string{
					constants.AppNameLabel:     component,
					constants.AppInstanceLabel: karmadaName,
				}

				for k, v := range expectedLabels {
					if pdb.Labels[k] != v {
						t.Errorf("expected label %s=%s, got %s", k, v, pdb.Labels[k])
					}
				}

				if pdb.Spec.Selector == nil {
					t.Fatal("expected selector to be set")
				}
				for k, v := range expectedLabels {
					if pdb.Spec.Selector.MatchLabels[k] != v {
						t.Errorf("expected selector %s=%s, got %s", k, v, pdb.Spec.Selector.MatchLabels[k])
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdb, err := createPodDisruptionBudget(karmadaName, namespace, component, tt.commonSettings)
			if err != nil {
				t.Fatalf("createPodDisruptionBudget() error = %v", err)
			}

			tt.validateFunc(t, pdb)
		})
	}
}

func TestGetPDBName(t *testing.T) {
	tests := []struct {
		karmadaName string
		component   string
		expected    string
	}{
		{
			karmadaName: "karmada-demo",
			component:   constants.KarmadaAPIServer,
			expected:    "karmada-demo-karmada-apiserver",
		},
		{
			karmadaName: "my-cluster",
			component:   constants.KarmadaWebhook,
			expected:    "my-cluster-karmada-webhook",
		},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := getPDBName(tt.karmadaName, tt.component)
			if result != tt.expected {
				t.Errorf("getPDBName() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestDeletePodDisruptionBudget(t *testing.T) {
	namespace := "test"
	pdbName := "test-pdb"

	tests := []struct {
		name          string
		existingPDB   *policyv1.PodDisruptionBudget
		expectedError bool
	}{
		{
			name: "Delete existing PDB",
			existingPDB: &policyv1.PodDisruptionBudget{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pdbName,
					Namespace: namespace,
				},
			},
			expectedError: false,
		},
		{
			name:          "Delete non-existing PDB (should not error)",
			existingPDB:   nil,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objs []runtime.Object
			if tt.existingPDB != nil {
				objs = append(objs, tt.existingPDB)
			}
			fakeClient := fakeclientset.NewSimpleClientset(objs...)

			err := deletePodDisruptionBudget(fakeClient, namespace, pdbName)
			if (err != nil) != tt.expectedError {
				t.Errorf("deletePodDisruptionBudget() error = %v, expectedError %v", err, tt.expectedError)
			}

			_, err = fakeClient.PolicyV1().PodDisruptionBudgets(namespace).Get(context.TODO(), pdbName, metav1.GetOptions{})
			if !apierrors.IsNotFound(err) {
				t.Errorf("expected PDB to be deleted, but still exists or got error: %v", err)
			}
		})
	}
}

func TestEnsurePodDisruptionBudget_WithReactor(t *testing.T) {
	name := "karmada-demo"
	namespace := "test"
	component := constants.KarmadaAPIServer

	t.Run("Handle update with ResourceVersion", func(t *testing.T) {
		existingPDB := &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "karmada-demo-karmada-apiserver",
				Namespace:       namespace,
				ResourceVersion: "100",
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MinAvailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						constants.AppNameLabel:     component,
						constants.AppInstanceLabel: name,
					},
				},
			},
		}

		fakeClient := fakeclientset.NewSimpleClientset(existingPDB)

		fakeClient.PrependReactor("update", "poddisruptionbudgets", func(action coretesting.Action) (bool, runtime.Object, error) {
			updateAction := action.(coretesting.UpdateAction)
			pdb := updateAction.GetObject().(*policyv1.PodDisruptionBudget)

			if pdb.ResourceVersion == "" {
				t.Error("ResourceVersion should be set during update")
			}
			if pdb.ResourceVersion != "100" {
				t.Errorf("expected ResourceVersion '100', got '%s'", pdb.ResourceVersion)
			}

			return false, nil, nil
		})

		commonSettings := &operatorv1alpha1.CommonSettings{
			PodDisruptionBudgetConfig: &operatorv1alpha1.PodDisruptionBudgetConfig{
				MinAvailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 2},
			},
		}

		err := EnsurePodDisruptionBudget(component, name, namespace, commonSettings, fakeClient)
		if err != nil {
			t.Fatalf("EnsurePodDisruptionBudget() error = %v", err)
		}
	})

	t.Run("Simulate update failure without ResourceVersion", func(t *testing.T) {
		existingPDB := &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "karmada-demo-karmada-apiserver",
				Namespace:       namespace,
				ResourceVersion: "100",
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MinAvailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
			},
		}

		fakeClient := fakeclientset.NewSimpleClientset(existingPDB)

		// Simulate the old buggy behavior: update without ResourceVersion
		updateCallCount := 0
		fakeClient.PrependReactor("update", "poddisruptionbudgets", func(action coretesting.Action) (bool, runtime.Object, error) {
			updateCallCount++
			updateAction := action.(coretesting.UpdateAction)
			pdb := updateAction.GetObject().(*policyv1.PodDisruptionBudget)

			// If ResourceVersion is missing or wrong, return Conflict error (simulating the bug)
			if pdb.ResourceVersion == "" || pdb.ResourceVersion != "100" {
				return true, nil, apierrors.NewConflict(
					policyv1.Resource("poddisruptionbudget"),
					pdb.Name,
					fmt.Errorf("the object has been modified; please apply your changes to the latest version"),
				)
			}

			return false, nil, nil
		})

		commonSettings := &operatorv1alpha1.CommonSettings{
			PodDisruptionBudgetConfig: &operatorv1alpha1.PodDisruptionBudgetConfig{
				MinAvailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 2},
			},
		}

		// With the fix, this should succeed because we Get first and set ResourceVersion
		err := EnsurePodDisruptionBudget(component, name, namespace, commonSettings, fakeClient)
		if err != nil {
			t.Errorf("EnsurePodDisruptionBudget() should succeed with fix, but got error: %v", err)
		}

		// Verify update was called (after the create attempt fails)
		if updateCallCount == 0 {
			t.Error("Expected update to be called but it wasn't")
		}
	})

	t.Run("Simulate API server timeout on create", func(t *testing.T) {
		fakeClient := fakeclientset.NewSimpleClientset()

		// Simulate timeout error
		fakeClient.PrependReactor("create", "poddisruptionbudgets", func(action coretesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewTimeoutError("request timeout", 30)
		})

		commonSettings := &operatorv1alpha1.CommonSettings{
			PodDisruptionBudgetConfig: &operatorv1alpha1.PodDisruptionBudgetConfig{
				MinAvailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
			},
		}

		err := EnsurePodDisruptionBudget(component, name, namespace, commonSettings, fakeClient)
		if err == nil {
			t.Error("Expected timeout error but got nil")
		}
		if !apierrors.IsTimeout(err) {
			t.Errorf("Expected timeout error, got: %v", err)
		}
	})

	t.Run("Simulate API server timeout on update", func(t *testing.T) {
		existingPDB := &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "karmada-demo-karmada-apiserver",
				Namespace:       namespace,
				ResourceVersion: "100",
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MinAvailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
			},
		}

		fakeClient := fakeclientset.NewSimpleClientset(existingPDB)

		// Simulate timeout on update
		fakeClient.PrependReactor("update", "poddisruptionbudgets", func(action coretesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewTimeoutError("update timeout", 30)
		})

		commonSettings := &operatorv1alpha1.CommonSettings{
			PodDisruptionBudgetConfig: &operatorv1alpha1.PodDisruptionBudgetConfig{
				MinAvailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 2},
			},
		}

		err := EnsurePodDisruptionBudget(component, name, namespace, commonSettings, fakeClient)
		if err == nil {
			t.Error("Expected timeout error but got nil")
		}
		if !apierrors.IsTimeout(err) {
			t.Errorf("Expected timeout error, got: %v", err)
		}
	})

	t.Run("Simulate continuous conflict errors (infinite loop scenario)", func(t *testing.T) {
		existingPDB := &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "karmada-demo-karmada-apiserver",
				Namespace:       namespace,
				ResourceVersion: "100",
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MinAvailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
			},
		}

		fakeClient := fakeclientset.NewSimpleClientset(existingPDB)

		updateAttempts := 0
		// Simulate the scenario where update always fails with conflict
		// This would cause timeout in real reconcile loop
		fakeClient.PrependReactor("update", "poddisruptionbudgets", func(action coretesting.Action) (bool, runtime.Object, error) {
			updateAttempts++
			// Always return conflict error (simulating the bug without fix)
			return true, nil, apierrors.NewConflict(
				policyv1.Resource("poddisruptionbudget"),
				"karmada-demo-karmada-apiserver",
				fmt.Errorf("persistent conflict"),
			)
		})

		commonSettings := &operatorv1alpha1.CommonSettings{
			PodDisruptionBudgetConfig: &operatorv1alpha1.PodDisruptionBudgetConfig{
				MinAvailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 2},
			},
		}

		err := EnsurePodDisruptionBudget(component, name, namespace, commonSettings, fakeClient)
		if err == nil {
			t.Error("Expected conflict error but got nil")
		}
		if !apierrors.IsConflict(err) {
			t.Errorf("Expected conflict error, got: %v", err)
		}

		// Verify that update was attempted
		if updateAttempts == 0 {
			t.Error("Expected at least one update attempt")
		}
	})

	t.Run("Simulate Get failure during update flow", func(t *testing.T) {
		existingPDB := &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "karmada-demo-karmada-apiserver",
				Namespace:       namespace,
				ResourceVersion: "100",
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MinAvailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
			},
		}

		fakeClient := fakeclientset.NewSimpleClientset(existingPDB)

		// Simulate Get failure (would happen after create fails)
		fakeClient.PrependReactor("get", "poddisruptionbudgets", func(action coretesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewInternalError(fmt.Errorf("etcd unavailable"))
		})

		commonSettings := &operatorv1alpha1.CommonSettings{
			PodDisruptionBudgetConfig: &operatorv1alpha1.PodDisruptionBudgetConfig{
				MinAvailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 2},
			},
		}

		err := EnsurePodDisruptionBudget(component, name, namespace, commonSettings, fakeClient)
		if err == nil {
			t.Error("Expected error but got nil")
		}
		if !apierrors.IsInternalError(err) {
			t.Errorf("Expected internal error, got: %v", err)
		}
	})
}
