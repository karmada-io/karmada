/*
Copyright 2026 The Karmada Authors.

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

package apiclient

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientset "k8s.io/client-go/kubernetes"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	coretesting "k8s.io/client-go/testing"
)

func TestCreateOrUpdatePodDisruptionBudget(t *testing.T) {
	name, namespace := "karmada-webhook", "test"
	newPodDisruptionBudget := func(maxUnavailable int32) *policyv1.PodDisruptionBudget {
		value := intstr.FromInt32(maxUnavailable)
		return &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MaxUnavailable: &value,
			},
		}
	}
	verifyMaxUnavailable := func(client clientset.Interface, want int32) error {
		pdb, err := client.PolicyV1().PodDisruptionBudgets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get pod disruption budget %s in %s namespace, got err: %v", name, namespace, err)
		}
		if pdb.Spec.MaxUnavailable == nil || pdb.Spec.MaxUnavailable.IntVal != want {
			return fmt.Errorf("expected pod disruption budget %s to have maxUnavailable %d, but got %v", name, want, pdb.Spec.MaxUnavailable)
		}
		return nil
	}

	tests := []struct {
		name    string
		client  clientset.Interface
		pdb     *policyv1.PodDisruptionBudget
		prep    func(clientset.Interface) error
		verify  func(clientset.Interface) error
		wantErr bool
		errMsg  string
	}{
		{
			name:    "CreateOrUpdatePodDisruptionBudget_NotExisting_PodDisruptionBudgetCreated",
			client:  fakeclientset.NewClientset(),
			pdb:     newPodDisruptionBudget(1),
			prep:    func(clientset.Interface) error { return nil },
			verify:  func(client clientset.Interface) error { return verifyMaxUnavailable(client, 1) },
			wantErr: false,
		},
		{
			name:   "CreateOrUpdatePodDisruptionBudget_AlreadyExisting_PodDisruptionBudgetUpdated",
			client: fakeclientset.NewClientset(),
			pdb:    newPodDisruptionBudget(2),
			prep: func(client clientset.Interface) error {
				_, err := client.PolicyV1().PodDisruptionBudgets(namespace).Create(context.TODO(), newPodDisruptionBudget(1), metav1.CreateOptions{})
				if err != nil {
					return fmt.Errorf("failed to create pod disruption budget %s in %s namespace, got err: %v", name, namespace, err)
				}
				return nil
			},
			verify:  func(client clientset.Interface) error { return verifyMaxUnavailable(client, 2) },
			wantErr: false,
		},
		{
			name:   "CreateOrUpdatePodDisruptionBudget_GotNetworkIssue_FailedToGetPodDisruptionBudget",
			client: fakeclientset.NewClientset(),
			pdb:    newPodDisruptionBudget(1),
			prep: func(client clientset.Interface) error {
				client.(*fakeclientset.Clientset).Fake.PrependReactor("get", "poddisruptionbudgets", func(coretesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("unexpected error: encountered a network issue while getting the pod disruption budget")
				})
				return nil
			},
			verify:  func(clientset.Interface) error { return nil },
			wantErr: true,
			errMsg:  "unexpected error: encountered a network issue while getting the pod disruption budget",
		},
		{
			name:   "CreateOrUpdatePodDisruptionBudget_GotNetworkIssue_FailedToCreatePodDisruptionBudget",
			client: fakeclientset.NewClientset(),
			pdb:    newPodDisruptionBudget(1),
			prep: func(client clientset.Interface) error {
				client.(*fakeclientset.Clientset).Fake.PrependReactor("create", "poddisruptionbudgets", func(coretesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("unexpected error: encountered a network issue while creating the pod disruption budget")
				})
				return nil
			},
			verify:  func(clientset.Interface) error { return nil },
			wantErr: true,
			errMsg:  "unexpected error: encountered a network issue while creating the pod disruption budget",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.client); err != nil {
				t.Fatalf("failed to prep before creating or updating pod disruption budget: %v", err)
			}
			err := CreateOrUpdatePodDisruptionBudget(test.client, test.pdb)
			if err == nil && test.wantErr {
				t.Errorf("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if err := test.verify(test.client); err != nil {
				t.Errorf("failed to verify the pod disruption budget: %v", err)
			}
		})
	}
}
