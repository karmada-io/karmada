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

package backendstore

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"testing"

	"github.com/opensearch-project/opensearch-go"
	"github.com/opensearch-project/opensearch-go/opensearchapi"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	fakeclientset "k8s.io/client-go/kubernetes/fake"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	searchv1alpha1 "github.com/karmada-io/karmada/pkg/apis/search/v1alpha1"
	testingutil "github.com/karmada-io/karmada/pkg/util/testing"
)

func TestInitBackendStoreManager(t *testing.T) {
	tests := []struct {
		name          string
		numGoroutines int
		client        clientset.Interface
		wrapInit      func(client clientset.Interface, numGoroutines int) error
	}{
		{
			name:   "Init_WithNoConcurrentCalls_Initialized",
			client: fakeclientset.NewSimpleClientset(),
			wrapInit: func(client clientset.Interface, _ int) error {
				Init(client)
				return nil
			},
		},
		{
			name:          "Init_WithMultipleConcurrentCalls_Initialized",
			numGoroutines: 3,
			client:        fakeclientset.NewSimpleClientset(),
			wrapInit: func(client clientset.Interface, numGoroutines int) error {
				// Use sync.WaitGroup to wait for all goroutines to finish.
				var wg sync.WaitGroup

				// Call Init concurrently from multiple goroutines.
				for i := 0; i < numGoroutines; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						Init(client)
					}()
				}

				// Wait for all goroutines to finish.
				wg.Wait()

				return nil
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.wrapInit(test.client, test.numGoroutines); err != nil {
				t.Fatalf("failed to init backend store manager, got: %v", err)
			}
			if err := verifyInitBackendStoreManager(test.client); err != nil {
				t.Errorf("failed to verify init backend store manager, got: %v", err)
			}
		})
	}
}

func TestAddBackendStore(t *testing.T) {
	secretName, namespace := "opensearch-credentials", "default"
	tests := []struct {
		name           string
		clusterName    string
		numGoroutines  int
		cfg            *searchv1alpha1.BackendStoreConfig
		client         clientset.Interface
		wrapAddBackend func(clusterName string, cfg *searchv1alpha1.BackendStoreConfig, numGoroutines int) error
		prep           func(clientset.Interface) error
	}{
		{
			name:        "DefaultBackend_DefaultBackendStoreAdded",
			clusterName: "member1",
			client:      fakeclientset.NewSimpleClientset(),
			wrapAddBackend: func(clusterName string, cfg *searchv1alpha1.BackendStoreConfig, _ int) error {
				AddBackend(clusterName, cfg)
				return nil
			},
			prep: func(clientset.Interface) error { return nil },
		},
		{
			name:          "OpenSearchBackend_OpenSearchBackendStoreAdded",
			clusterName:   "member1",
			client:        fakeclientset.NewSimpleClientset(),
			numGoroutines: 3,
			cfg: &searchv1alpha1.BackendStoreConfig{
				OpenSearch: &searchv1alpha1.OpenSearchConfig{
					Addresses: []string{"https://10.0.0.1:9200"},
					SecretRef: clusterv1alpha1.LocalSecretReference{
						Name:      secretName,
						Namespace: namespace,
					},
				},
			},
			wrapAddBackend: func(clusterName string, cfg *searchv1alpha1.BackendStoreConfig, numGoroutines int) error {
				// Use sync.WaitGroup to wait for all goroutines to finish.
				var wg sync.WaitGroup

				// Call Init concurrently from multiple goroutines.
				for i := 0; i < numGoroutines; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						AddBackend(clusterName, cfg)
					}()
				}

				// Wait for all goroutines to finish.
				wg.Wait()

				return nil
			},
			prep: func(client clientset.Interface) error {
				username, password := "opensearchuser", "opensearchpass"
				if err := createOpenSearchCredentialsSecret(client, username, password, secretName, namespace); err != nil {
					return fmt.Errorf("failed to create open search credentials secret, got: %v", err)
				}
				OpenSearchClientBuilder = func(opensearch.Config) (*opensearch.Client, error) {
					mTransport := &testingutil.MockOpenSearchTransport{}
					mTransport.PerformFunc = func(*http.Request) (*http.Response, error) {
						return &http.Response{
							StatusCode: http.StatusOK,
							Body:       http.NoBody,
						}, nil
					}
					client := &opensearch.Client{Transport: mTransport}
					client.API = opensearchapi.New(client)
					return client, nil
				}
				return nil
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			Init(test.client)
			if err := test.prep(test.client); err != nil {
				t.Fatalf("failed to prep test environment before verifying adding the backend store, got: %v", err)
			}
			if err := test.wrapAddBackend(test.clusterName, test.cfg, test.numGoroutines); err != nil {
				t.Fatalf("failed to add backend, got error: %v", err)
			}
			if err := verifyBackendStoreAdded(test.clusterName); err != nil {
				t.Errorf("failed to verify adding the backend store, got: %v", err)
			}
		})
	}
}

func TestDeleteBackendStore(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		client      clientset.Interface
		prep        func(clusterName string) error
	}{
		{
			name:        "Existent_BackendStoreDeleted",
			clusterName: "member1",
			client:      fakeclientset.NewSimpleClientset(),
			prep: func(clusterName string) error {
				AddBackend(clusterName, nil)
				return nil
			},
		},
		{
			name:        "NonExistent_NoError",
			clusterName: "nonexistent",
			client:      fakeclientset.NewSimpleClientset(),
			prep:        func(string) error { return nil },
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			Init(test.client)
			if err := test.prep(test.clusterName); err != nil {
				t.Fatalf("failed to prep test environment before deleting backend store, got: %v", err)
			}
			DeleteBackend(test.clusterName)
			if err := verifyBackendStoreDeleted(test.clusterName); err != nil {
				t.Errorf("failed to verify backend store deleted for cluster name %s, got: %v", test.clusterName, err)
			}
		})
	}
}

func TestGetBackendStore(t *testing.T) {
	tests := []struct {
		name             string
		clusterName      string
		client           clientset.Interface
		numGoroutines    int
		wrapGetBackend   func(clusterName string, numGoroutines int) BackendStore
		prep             func(clusterName string) error
		want             BackendStore
		wantBackendStore bool
	}{
		{
			name:        "ExistentBackendStore_BackendStoreRetrieved",
			clusterName: "member1",
			client:      fakeclientset.NewSimpleClientset(),
			wrapGetBackend: func(clusterName string, _ int) BackendStore {
				return GetBackend(clusterName)
			},
			prep: func(clusterName string) error {
				AddBackend(clusterName, nil)
				return nil
			},
			wantBackendStore: true,
		},
		{
			name:          "ExistentWithMultipleConcurrentCalls_BackendStoreRetrieved",
			clusterName:   "member1",
			client:        fakeclientset.NewSimpleClientset(),
			numGoroutines: 3,
			wrapGetBackend: func(clusterName string, numGoroutines int) BackendStore {
				// Use sync.WaitGroup to wait for all goroutines to finish.
				var wg sync.WaitGroup

				// Call Init concurrently from multiple goroutines.
				for i := 0; i < numGoroutines; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						_ = GetBackend(clusterName)
					}()
				}

				// Wait for all goroutines to finish.
				wg.Wait()

				return GetBackend(clusterName)
			},
			prep: func(clusterName string) error {
				AddBackend(clusterName, nil)
				return nil
			},
			wantBackendStore: true,
		},
		{
			name:        "NonExistent_NoError",
			clusterName: "nonexistent",
			client:      fakeclientset.NewSimpleClientset(),
			wrapGetBackend: func(clusterName string, _ int) BackendStore {
				return GetBackend(clusterName)
			},
			prep:             func(string) error { return nil },
			wantBackendStore: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			Init(test.client)
			if err := test.prep(test.clusterName); err != nil {
				t.Fatalf("failed to prep test environment before getting backend store, got: %v", err)
			}
			bc := test.wrapGetBackend(test.clusterName, test.numGoroutines)
			if bc == nil && test.wantBackendStore {
				t.Error("expected backend store, but got none")
			}
			if bc != nil && !test.wantBackendStore {
				t.Errorf("unexpected backend store retrieved, got: %v", bc)
			}
		})
	}
}

func verifyInitBackendStoreManager(client clientset.Interface) error {
	if backends == nil {
		return fmt.Errorf("expected backends to be initialized, but got %v", backends)
	}
	if len(backends) != 0 {
		return fmt.Errorf("expected backends to be empty, but got %d backends", len(backends))
	}
	if !reflect.DeepEqual(client, k8sClient) {
		return fmt.Errorf("expected k8s client global varible to be %v, but got %v", client, k8sClient)
	}
	return nil
}

func verifyBackendStoreAdded(clusterName string) error {
	_, ok := backends[clusterName]
	if !ok {
		return fmt.Errorf("expected cluster name %s to be in the backend store", clusterName)
	}
	return nil
}

func verifyBackendStoreDeleted(clusterName string) error {
	_, ok := backends[clusterName]
	if ok {
		return fmt.Errorf("expected backend store for cluster %s to be deleted, but it still exists", clusterName)
	}
	return nil
}

func createOpenSearchCredentialsSecret(client clientset.Interface, username, password, secretName, namespace string) error {
	userNameEncoded := base64.StdEncoding.EncodeToString([]byte(username))
	passwordEncoded := base64.StdEncoding.EncodeToString([]byte(password))
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"username": []byte(userNameEncoded),
			"password": []byte(passwordEncoded),
		},
	}
	if _, err := client.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to create secret %s in namespace %s, got error: %v", secretName, namespace, err)
	}
	return nil
}
