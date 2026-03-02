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

package clusterapi

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest"
	clusterapiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	secretutil "sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/karmada-io/karmada/pkg/karmadactl/join"
	"github.com/karmada-io/karmada/pkg/karmadactl/unjoin"
	"github.com/karmada-io/karmada/pkg/search/backendstore"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
)

// MyTestData is a struct that implements the TestInterface.
type MyTestData struct {
	Data string
}

// Get returns the data stored in the MyTestData struct.
func (m *MyTestData) Get() string {
	return m.Data
}

func TestStartClusterDetector(t *testing.T) {
	clusterName := "test-cluster"
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: clusterapiv1beta1.GroupVersion.Group, Version: clusterapiv1beta1.GroupVersion.Version, Resource: resourceCluster}: "ClusterList",
	}
	tests := []struct {
		name            string
		clusterDetector *ClusterDetector
		dynamicClient   dynamic.Interface
		prep            func(*ClusterDetector, dynamic.Interface, context.Context) error
	}{
		{
			name: "StartClusterDetector_RunningReceivesStopSignal_ShouldStop",
			clusterDetector: &ClusterDetector{
				EventHandler: backendstore.NewDefaultBackend(clusterName).ResourceEventHandlerFuncs(),
			},
			dynamicClient: dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrToListKind),
			prep: func(clusterDetector *ClusterDetector, dynamicClient dynamic.Interface, ctx context.Context) error {
				clusterDetector.InformerManager = genericmanager.NewSingleClusterInformerManager(ctx, dynamicClient, 0)
				return nil
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*10)
			defer cancel()
			if err := test.prep(test.clusterDetector, test.dynamicClient, ctx); err != nil {
				t.Fatalf("failed to prep before starting cluster detector, got: %v", err)
			}
			err := test.clusterDetector.Start(ctx)
			if err != nil {
				t.Fatalf("failed to start cluster detector, got: %v", err)
			}
		})
	}
}

func TestGetUnstructuredObject(t *testing.T) {
	clusterName, namespace := "test-cluster", "test"
	tests := []struct {
		name            string
		clusterDetector *ClusterDetector
		objectKey       *keys.ClusterWideKey
		prep            func() error
		wantErr         bool
		errMsg          string
	}{
		{
			name:            "GetUnstructuredObject_WithNetworkIssue_FailedToGetResource",
			clusterDetector: &ClusterDetector{},
			objectKey: &keys.ClusterWideKey{
				Group:     clusterapiv1beta1.GroupVersion.Group,
				Version:   clusterapiv1beta1.GroupVersion.Version,
				Namespace: namespace,
				Name:      resourceCluster,
				Kind:      "cluster",
			},
			prep: func() error {
				cacheGenericListerBuilder = func(*ClusterDetector, schema.GroupVersionResource, keys.ClusterWideKey) (runtime.Object, error) {
					return nil, errors.New("unexpected error, got network issue")
				}
				return nil
			},
			wantErr: true,
			errMsg:  "unexpected error, got network issue",
		},
		{
			name: "GetUnstructuredObject_InvalidRuntimeObject_FailedToTransformObject",
			clusterDetector: &ClusterDetector{
				EventHandler: backendstore.NewDefaultBackend(clusterName).ResourceEventHandlerFuncs(),
			},
			objectKey: &keys.ClusterWideKey{
				Group:     clusterapiv1beta1.GroupVersion.Group,
				Version:   clusterapiv1beta1.GroupVersion.Version,
				Name:      resourceCluster,
				Namespace: namespace,
				Kind:      "cluster",
			},
			prep: func() error {
				cacheGenericListerBuilder = func(*ClusterDetector, schema.GroupVersionResource, keys.ClusterWideKey) (runtime.Object, error) {
					return nil, errors.New("failed to transform object")
				}
				return nil
			},
			wantErr: true,
			errMsg:  "failed to transform object",
		},
		{
			name: "GetUnstructuredObject_ValidRuntimeObject_TransformedToUnstructuredObject",
			clusterDetector: &ClusterDetector{
				EventHandler: backendstore.NewDefaultBackend(clusterName).ResourceEventHandlerFuncs(),
			},
			objectKey: &keys.ClusterWideKey{
				Group:     clusterapiv1beta1.GroupVersion.Group,
				Version:   clusterapiv1beta1.GroupVersion.Version,
				Name:      resourceCluster,
				Namespace: namespace,
				Kind:      "cluster",
			},
			prep: func() error {
				cacheGenericListerBuilder = func(*ClusterDetector, schema.GroupVersionResource, keys.ClusterWideKey) (runtime.Object, error) {
					return &appsv1.Deployment{}, nil
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(); err != nil {
				t.Fatalf("failed to prep before getting unstructured object, got: %v", err)
			}
			_, err := test.clusterDetector.GetUnstructuredObject(*test.objectKey)
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
		})
	}
}

func TestJoinClusterAPICluster(t *testing.T) {
	_, namespace := "test-cluster", "test"
	tests := []struct {
		name            string
		clusterDetector *ClusterDetector
		clusterWideKey  *keys.ClusterWideKey
		prep            func(client.Client, *keys.ClusterWideKey) error
		wantErr         bool
		errMsg          string
	}{
		{
			name: "JoinClusterAPICluster_WithoutClusterAPIClusterSecret_FailedToGetSecret",
			clusterDetector: &ClusterDetector{
				ClusterAPIClient: fakeclient.NewFakeClient(),
			},
			clusterWideKey: &keys.ClusterWideKey{
				Group:     clusterapiv1beta1.GroupVersion.Group,
				Version:   clusterapiv1beta1.GroupVersion.Version,
				Name:      resourceCluster,
				Namespace: namespace,
				Kind:      "cluster",
			},
			prep:    func(client.Client, *keys.ClusterWideKey) error { return nil },
			wantErr: true,
			errMsg:  `secrets "clusters-kubeconfig" not found`,
		},
		{
			name: "JoinClusterAPICluster_WritingKubeConfigFile_FailedToWriteKubeConfigFile",
			clusterDetector: &ClusterDetector{
				ClusterAPIClient: fakeclient.NewFakeClient(),
			},
			clusterWideKey: &keys.ClusterWideKey{
				Group:     clusterapiv1beta1.GroupVersion.Group,
				Version:   clusterapiv1beta1.GroupVersion.Version,
				Name:      resourceCluster,
				Namespace: namespace,
				Kind:      "cluster",
			},
			prep: func(client client.Client, clusterWideKey *keys.ClusterWideKey) error {
				// Create Kubeconfig secret.
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretutil.Name(clusterWideKey.Name, secretutil.Kubeconfig),
						Namespace: clusterWideKey.Namespace,
					},
				}
				if err := client.Create(context.TODO(), secret); err != nil {
					return fmt.Errorf("failed to create secret %s in namespace %s, got: %v", secret.Name, secret.Namespace, err)
				}

				// Mock Kubeconfig file generator.
				kubeconfigFileGenerator = func(string, []byte) (string, error) {
					return "", errors.New("failed to write kubeconfig file")
				}
				return nil
			},
			wantErr: true,
			errMsg:  "failed to write kubeconfig file",
		},
		{
			name: "JoinClusterAPICluster_WithNetworkIssueWhileJoiningCluster_FailedToJoinClusterAPICluster",
			clusterDetector: &ClusterDetector{
				ClusterAPIClient:      fakeclient.NewFakeClient(),
				ControllerPlaneConfig: &rest.Config{},
			},
			clusterWideKey: &keys.ClusterWideKey{
				Group:     clusterapiv1beta1.GroupVersion.Group,
				Version:   clusterapiv1beta1.GroupVersion.Version,
				Name:      resourceCluster,
				Namespace: namespace,
				Kind:      "cluster",
			},
			prep: func(client client.Client, clusterWideKey *keys.ClusterWideKey) error {
				// Create kubeconfig secret.
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretutil.Name(clusterWideKey.Name, secretutil.Kubeconfig),
						Namespace: clusterWideKey.Namespace,
					},
				}
				if err := client.Create(context.TODO(), secret); err != nil {
					return fmt.Errorf("failed to create secret %s in namespace %s, got: %v", secret.Name, secret.Namespace, err)
				}

				// Mock kubeconfig file generator.
				kubeconfigFileGenerator = func(string, []byte) (string, error) {
					return "", nil
				}

				// Mock join clusterAPICluster.
				joinClusterRunner = func(*join.CommandJoinOption, *rest.Config, *rest.Config) error {
					return errors.New("unexpected error got network error while joining clusterAPICluster")
				}

				return nil
			},
			wantErr: true,
			errMsg:  "unexpected error got network error while joining clusterAPICluster",
		},
		{
			name: "JoinClusterAPICluster_JoiningClusterAPICluster_ClusterAPIClusterJoined",
			clusterDetector: &ClusterDetector{
				ClusterAPIClient:      fakeclient.NewFakeClient(),
				ControllerPlaneConfig: &rest.Config{},
			},
			clusterWideKey: &keys.ClusterWideKey{
				Group:     clusterapiv1beta1.GroupVersion.Group,
				Version:   clusterapiv1beta1.GroupVersion.Version,
				Name:      resourceCluster,
				Namespace: namespace,
				Kind:      "cluster",
			},
			prep: func(client client.Client, clusterWideKey *keys.ClusterWideKey) error {
				// Create kubeconfig secret.
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretutil.Name(clusterWideKey.Name, secretutil.Kubeconfig),
						Namespace: clusterWideKey.Namespace,
					},
				}
				if err := client.Create(context.TODO(), secret); err != nil {
					return fmt.Errorf("failed to create secret %s in namespace %s, got: %v", secret.Name, secret.Namespace, err)
				}

				// Mock kubeconfig file generator.
				kubeconfigFileGenerator = func(string, []byte) (string, error) {
					return "", nil
				}

				// Mock join clusterAPICluster.
				joinClusterRunner = func(*join.CommandJoinOption, *rest.Config, *rest.Config) error {
					return nil
				}

				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.clusterDetector.ClusterAPIClient, test.clusterWideKey); err != nil {
				t.Fatalf("failed to prep test environment before joining clusterAPICluster, got: %v", err)
			}
			err := test.clusterDetector.joinClusterAPICluster(*test.clusterWideKey)
			if err == nil && test.wantErr {
				t.Fatal("expected an error, got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
		})
	}
}

func TestUnjoinClusterAPICluster(t *testing.T) {
	clusterName := "test-cluster"
	tests := []struct {
		name            string
		clusterDetector *ClusterDetector
		prep            func() error
		wantErr         bool
		errMsg          string
	}{
		{
			name: "UnjoinClusterAPICluster_WithNetworkIssue_FailedToUnjoinClusterAPIClustter",
			clusterDetector: &ClusterDetector{
				EventHandler: backendstore.NewDefaultBackend(clusterName).ResourceEventHandlerFuncs(),
			},
			prep: func() error {
				unjoinClusterRunner = func(*unjoin.CommandUnjoinOption, *rest.Config, *rest.Config) error {
					return errors.New("unexpected error got network error while unjoining clusterAPICluster")
				}
				return nil
			},
			wantErr: true,
			errMsg:  "unexpected error got network error while unjoining clusterAPICluster",
		},
		{
			name: "UnjoinClusterAPICluster_UnjoiningClusterAPICluster_ClusterAPIClusterUnjoined",
			clusterDetector: &ClusterDetector{
				EventHandler: backendstore.NewDefaultBackend(clusterName).ResourceEventHandlerFuncs(),
			},
			prep: func() error {
				unjoinClusterRunner = func(*unjoin.CommandUnjoinOption, *rest.Config, *rest.Config) error {
					return nil
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(); err != nil {
				t.Fatalf("failed to prep before unjoining cluster API cluster, got: %v", err)
			}
			err := test.clusterDetector.unJoinClusterAPICluster(clusterName)
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
		})
	}
}

func TestReconcileClusterAPICluster(t *testing.T) {
	clusterName, namespace := "test-cluster", "test"
	tests := []struct {
		name            string
		clusterDetector *ClusterDetector
		key             util.QueueKey
		prep            func() error
		wantErr         bool
		errMsg          string
	}{
		{
			name: "Reconcile_InvalidTypeAssertion_TypeAssertionIsInvalid",
			clusterDetector: &ClusterDetector{
				EventHandler: backendstore.NewDefaultBackend(clusterName).ResourceEventHandlerFuncs(),
			},
			key:     MyTestData{Data: "test"},
			prep:    func() error { return nil },
			wantErr: true,
			errMsg:  "expected keys.ClusterWideKey, but got clusterapi.MyTestData type",
		},
		{
			name: "Reconcile_InvalidRuntimeObject_FailedTGetUnstructuredObject",
			clusterDetector: &ClusterDetector{
				EventHandler: backendstore.NewDefaultBackend(clusterName).ResourceEventHandlerFuncs(),
			},
			key: keys.ClusterWideKey{
				Group:     clusterapiv1beta1.GroupVersion.Group,
				Version:   clusterapiv1beta1.GroupVersion.Version,
				Name:      resourceCluster,
				Namespace: namespace,
				Kind:      "cluster",
			},
			prep: func() error {
				cacheGenericListerBuilder = func(*ClusterDetector, schema.GroupVersionResource, keys.ClusterWideKey) (runtime.Object, error) {
					return nil, errors.New("failed to get unstructured object")
				}
				return nil
			},
			wantErr: true,
			errMsg:  "failed to get unstructured object",
		},
		{
			name: "Reconcile_ValidRuntimeObject_Reconciled",
			clusterDetector: &ClusterDetector{
				EventHandler: backendstore.NewDefaultBackend(clusterName).ResourceEventHandlerFuncs(),
			},
			key: keys.ClusterWideKey{
				Group:     clusterapiv1beta1.GroupVersion.Group,
				Version:   clusterapiv1beta1.GroupVersion.Version,
				Name:      resourceCluster,
				Namespace: namespace,
				Kind:      "cluster",
			},
			prep: func() error {
				cacheGenericListerBuilder = func(*ClusterDetector, schema.GroupVersionResource, keys.ClusterWideKey) (runtime.Object, error) {
					return &corev1.Pod{
						Status: corev1.PodStatus{
							Phase: corev1.PodRunning,
						},
					}, nil
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(); err != nil {
				t.Fatalf("failed to prep test environment before reconiliation, got: %v", err)
			}
			err := test.clusterDetector.Reconcile(test.key)
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
		})
	}
}
