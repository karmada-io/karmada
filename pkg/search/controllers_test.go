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

package search

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	searchv1alpha1 "github.com/karmada-io/karmada/pkg/apis/search/v1alpha1"
	"github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	fakekarmadaclient "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/fake"
	informerfactory "github.com/karmada-io/karmada/pkg/generated/informers/externalversions"
	"github.com/karmada-io/karmada/pkg/search/backendstore"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
)

var apiGroupResources = []*restmapper.APIGroupResources{
	{
		Group: metav1.APIGroup{
			Name: "apps",
			Versions: []metav1.GroupVersionForDiscovery{
				{GroupVersion: "apps/v1", Version: "v1"},
			},
			PreferredVersion: metav1.GroupVersionForDiscovery{
				GroupVersion: "apps/v1", Version: "v1",
			},
		},
		VersionedResources: map[string][]metav1.APIResource{
			"v1": {
				{Name: "deployments", SingularName: "deployment", Namespaced: true, Kind: "Deployment"},
			},
		},
	},
	{
		Group: metav1.APIGroup{
			Name: "",
			Versions: []metav1.GroupVersionForDiscovery{
				{GroupVersion: "v1", Version: "v1"},
			},
			PreferredVersion: metav1.GroupVersionForDiscovery{
				GroupVersion: "v1", Version: "v1",
			},
		},
		VersionedResources: map[string][]metav1.APIResource{
			"v1": {
				{Name: "pods", SingularName: "pod", Namespaced: true, Kind: "Pod"},
			},
		},
	},
}

func TestNewKarmadaSearchController(t *testing.T) {
	tests := []struct {
		name       string
		restConfig *rest.Config
		factory    informerfactory.SharedInformerFactory
		restMapper meta.RESTMapper
		client     versioned.Interface
		prep       func(*informerfactory.SharedInformerFactory, versioned.Interface) error
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "NewKarmadaSearchController",
			restConfig: &rest.Config{},
			restMapper: meta.NewDefaultRESTMapper(nil),
			client:     fakekarmadaclient.NewSimpleClientset(),
			factory:    informerfactory.NewSharedInformerFactory(fakekarmadaclient.NewSimpleClientset(), 0),
			prep:       func(*informerfactory.SharedInformerFactory, versioned.Interface) error { return nil },
			wantErr:    false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(&test.factory, test.client); err != nil {
				t.Fatalf("failed to prep test environment before creating new controller, got: %v", err)
			}
			_, err := NewController(test.restConfig, test.factory, test.restMapper)
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

func TestAddClusterEventHandler(t *testing.T) {
	tests := []struct {
		name       string
		restConfig *rest.Config
		client     *fakekarmadaclient.Clientset
		restMapper meta.RESTMapper
		prep       func(context.Context, *fakekarmadaclient.Clientset, *rest.Config, meta.RESTMapper) (*Controller, informerfactory.SharedInformerFactory, error)
		verify     func(*fakekarmadaclient.Clientset, *Controller) error
	}{
		{
			name:       "AddAllEventHandlers_TriggerAddClusterEvent_ClusterAddedToWorkQueue",
			restConfig: &rest.Config{},
			client:     fakekarmadaclient.NewSimpleClientset(),
			restMapper: meta.NewDefaultRESTMapper(nil),
			prep: func(ctx context.Context, clientConnector *fakekarmadaclient.Clientset, restConfig *rest.Config, restMapper meta.RESTMapper) (*Controller, informerfactory.SharedInformerFactory, error) {
				factory := informerfactory.NewSharedInformerFactory(clientConnector, time.Second)
				controller, err := createController(ctx, restConfig, factory, restMapper)
				return controller, factory, err
			},
			verify: func(clientConnector *fakekarmadaclient.Clientset, controller *Controller) error {
				var (
					clusterName, resourceVersion = "test-cluster", "1000"
					apiEndpoint, labels          = "10.0.0.1", map[string]string{}
				)

				if err := upsertCluster(clientConnector, labels, apiEndpoint, clusterName, resourceVersion); err != nil {
					return err
				}
				if err := cacheNextWrapper(controller); err != nil {
					return err
				}

				return nil
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())

			controller, informer, err := test.prep(ctx, test.client, test.restConfig, test.restMapper)
			if err != nil {
				t.Fatalf("failed to prepare test environment for event handler setup, got: %v", err)
			}
			informer.Start(ctx.Done())
			informer.WaitForCacheSync(ctx.Done())
			defer informer.Shutdown()
			defer cancel()

			if err := test.verify(test.client, controller); err != nil {
				t.Errorf("failed to verify controller, got: %v", err)
			}
		})
	}
}

func TestUpdateClusterEventHandler(t *testing.T) {
	tests := []struct {
		name       string
		restConfig *rest.Config
		client     *fakekarmadaclient.Clientset
		restMapper meta.RESTMapper
		prep       func(context.Context, *fakekarmadaclient.Clientset, *rest.Config, meta.RESTMapper) (*Controller, informerfactory.SharedInformerFactory, error)
		verify     func(*fakekarmadaclient.Clientset, *Controller) error
	}{
		{
			name:       "AddAllEventHandlers_TriggerUpdateClusterEvent_UpdatedClusterAddedToWorkQueue",
			restConfig: &rest.Config{},
			client:     fakekarmadaclient.NewSimpleClientset(),
			restMapper: meta.NewDefaultRESTMapper(nil),
			prep: func(ctx context.Context, clientConnector *fakekarmadaclient.Clientset, restConfig *rest.Config, restMapper meta.RESTMapper) (*Controller, informerfactory.SharedInformerFactory, error) {
				factory := informerfactory.NewSharedInformerFactory(clientConnector, time.Second)
				controller, err := createController(ctx, restConfig, factory, restMapper)
				return controller, factory, err
			},
			verify: func(clientConnector *fakekarmadaclient.Clientset, controller *Controller) error {
				var (
					clusterName, resourceVersion, updatedResourceVersion = "test-cluster", "1000", "1001"
					apiEndpoint, oldLabels, newLabels                    = "10.0.0.1", map[string]string{"status": "old"}, map[string]string{"status": "new"}
				)

				if err := upsertCluster(clientConnector, oldLabels, apiEndpoint, clusterName, resourceVersion); err != nil {
					return err
				}
				if err := cacheNextWrapper(controller); err != nil {
					return err
				}

				if err := upsertCluster(clientConnector, newLabels, apiEndpoint, clusterName, updatedResourceVersion); err != nil {
					return err
				}
				if err := cacheNextWrapper(controller); err != nil {
					return err
				}

				return nil
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())

			controller, informer, err := test.prep(ctx, test.client, test.restConfig, test.restMapper)
			if err != nil {
				t.Fatalf("failed to prepare test environment for event handler setup, got: %v", err)
			}
			informer.Start(ctx.Done())
			informer.WaitForCacheSync(ctx.Done())
			defer informer.Shutdown()
			defer cancel()

			if err := test.verify(test.client, controller); err != nil {
				t.Errorf("failed to verify controller, got: %v", err)
			}
		})
	}
}

func TestDeleteClusterEventHandler(t *testing.T) {
	tests := []struct {
		name       string
		restConfig *rest.Config
		client     *fakekarmadaclient.Clientset
		restMapper meta.RESTMapper
		prep       func(context.Context, *fakekarmadaclient.Clientset, *rest.Config, meta.RESTMapper) (*Controller, informerfactory.SharedInformerFactory, error)
		verify     func(*fakekarmadaclient.Clientset, *Controller) error
	}{
		{
			name:       "AddAllEventHandlers_TriggerDeleteClusterEvent_DeletedClusterAddedToWorkQueue",
			restConfig: &rest.Config{},
			client:     fakekarmadaclient.NewSimpleClientset(),
			restMapper: restmapper.NewDiscoveryRESTMapper(apiGroupResources),
			prep: func(ctx context.Context, clientConnector *fakekarmadaclient.Clientset, restConfig *rest.Config, restMapper meta.RESTMapper) (*Controller, informerfactory.SharedInformerFactory, error) {
				factory := informerfactory.NewSharedInformerFactory(clientConnector, time.Second)
				controller, err := createController(ctx, restConfig, factory, restMapper)
				return controller, factory, err
			},
			verify: func(clientConnector *fakekarmadaclient.Clientset, controller *Controller) error {
				var (
					registryName, clusterName    = "test-registry", "test-cluster"
					resourceVersion, apiEndpoint = "1000", "10.0.0.1"
					labels                       = map[string]string{}
					resourceSelectors            = []searchv1alpha1.ResourceSelector{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
						},
					}
				)

				clusterDynamicClientBuilder = func(string, client.Client) (*util.DynamicClusterClient, error) {
					return &util.DynamicClusterClient{
						DynamicClientSet: fakedynamic.NewSimpleDynamicClient(scheme.Scheme),
						ClusterName:      clusterName,
					}, nil
				}

				if err := upsertCluster(clientConnector, labels, apiEndpoint, clusterName, resourceVersion); err != nil {
					return err
				}
				if err := cacheNextWrapper(controller); err != nil {
					return err
				}

				if err := upsertResourceRegistry(clientConnector, resourceSelectors, registryName, resourceVersion, []string{clusterName}); err != nil {
					return err
				}
				if err := cacheNextWrapper(controller); err != nil {
					return err
				}

				if err := deleteCluster(clientConnector, clusterName); err != nil {
					return err
				}
				if err := cacheNextWrapper(controller); err != nil {
					return err
				}

				// Verify no backend store for this deleted cluster.
				if backend := backendstore.GetBackend(clusterName); backend != nil {
					return fmt.Errorf("expected backend store for cluster %s to be deleted, but got: %v", clusterName, backend)
				}

				return nil
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())

			controller, informer, err := test.prep(ctx, test.client, test.restConfig, test.restMapper)
			if err != nil {
				t.Fatalf("failed to prepare test environment for event handler setup, got: %v", err)
			}
			informer.Start(ctx.Done())
			informer.WaitForCacheSync(ctx.Done())
			defer informer.Shutdown()
			defer cancel()

			if err := test.verify(test.client, controller); err != nil {
				t.Errorf("failed to verify controller, got: %v", err)
			}
		})
	}
}

func TestAddResourceRegistryEventHandler(t *testing.T) {
	tests := []struct {
		name       string
		restConfig *rest.Config
		client     *fakekarmadaclient.Clientset
		restMapper meta.RESTMapper
		prep       func(context.Context, *fakekarmadaclient.Clientset, *rest.Config, meta.RESTMapper) (*Controller, informerfactory.SharedInformerFactory, error)
		verify     func(*fakekarmadaclient.Clientset, *Controller) error
	}{
		{
			name:       "AddAllEventHandlers_TriggerAddResourceRegistryEvent_ResourceRegistryAddedToWorkQueue",
			restConfig: &rest.Config{},
			client:     fakekarmadaclient.NewSimpleClientset(),
			restMapper: restmapper.NewDiscoveryRESTMapper(apiGroupResources),
			prep: func(ctx context.Context, clientConnector *fakekarmadaclient.Clientset, restConfig *rest.Config, restMapper meta.RESTMapper) (*Controller, informerfactory.SharedInformerFactory, error) {
				factory := informerfactory.NewSharedInformerFactory(clientConnector, time.Second)
				controller, err := createController(ctx, restConfig, factory, restMapper)
				return controller, factory, err
			},
			verify: func(clientConnector *fakekarmadaclient.Clientset, controller *Controller) error {
				var (
					registryName, clusterName    = "test-registry", "test-cluster"
					resourceVersion, apiEndpoint = "1000", "10.0.0.1"
					labels                       = map[string]string{}
					resourceSelectors            = []searchv1alpha1.ResourceSelector{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
						},
					}
				)

				clusterDynamicClientBuilder = func(string, client.Client) (*util.DynamicClusterClient, error) {
					return &util.DynamicClusterClient{
						DynamicClientSet: fakedynamic.NewSimpleDynamicClient(scheme.Scheme),
						ClusterName:      clusterName,
					}, nil
				}

				if err := upsertCluster(clientConnector, labels, apiEndpoint, clusterName, resourceVersion); err != nil {
					return err
				}
				if err := cacheNextWrapper(controller); err != nil {
					return err
				}

				if err := upsertResourceRegistry(clientConnector, resourceSelectors, registryName, resourceVersion, []string{clusterName}); err != nil {
					return err
				}
				if err := cacheNextWrapper(controller); err != nil {
					return err
				}

				return nil
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())

			controller, informer, err := test.prep(ctx, test.client, test.restConfig, test.restMapper)
			if err != nil {
				t.Fatalf("failed to prepare test environment for event handler setup, got: %v", err)
			}
			informer.Start(ctx.Done())
			informer.WaitForCacheSync(ctx.Done())
			defer informer.Shutdown()
			defer cancel()

			if err := test.verify(test.client, controller); err != nil {
				t.Errorf("failed to verify controller, got: %v", err)
			}
		})
	}
}

func TestUpdateResourceRegistryEventHandler(t *testing.T) {
	tests := []struct {
		name       string
		restConfig *rest.Config
		client     *fakekarmadaclient.Clientset
		restMapper meta.RESTMapper
		prep       func(context.Context, *fakekarmadaclient.Clientset, *rest.Config, meta.RESTMapper) (*Controller, informerfactory.SharedInformerFactory, error)
		verify     func(*fakekarmadaclient.Clientset, *Controller) error
	}{
		{
			name:       "AddAllEventHandlers_TriggerUpdateResourceRegistryEvent_UpdatedResourceRegistryAddedToWorkQueue",
			restConfig: &rest.Config{},
			client:     fakekarmadaclient.NewSimpleClientset(),
			restMapper: restmapper.NewDiscoveryRESTMapper(apiGroupResources),
			prep: func(ctx context.Context, clientConnector *fakekarmadaclient.Clientset, restConfig *rest.Config, restMapper meta.RESTMapper) (*Controller, informerfactory.SharedInformerFactory, error) {
				factory := informerfactory.NewSharedInformerFactory(clientConnector, time.Second)
				controller, err := createController(ctx, restConfig, factory, restMapper)
				return controller, factory, err
			},
			verify: func(clientConnector *fakekarmadaclient.Clientset, controller *Controller) error {
				var (
					registryName, clusterName, resourceVersion = "test-registry", "test-cluster", "1000"
					apiEndpoint, labels                        = "10.0.0.1", map[string]string{}
					resourceSelectors                          = []searchv1alpha1.ResourceSelector{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
						},
					}
					resourceSelectorsUpdated = []searchv1alpha1.ResourceSelector{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
						},
						{
							APIVersion: "v1",
							Kind:       "Pod",
						},
					}
				)

				clusterDynamicClientBuilder = func(string, client.Client) (*util.DynamicClusterClient, error) {
					return &util.DynamicClusterClient{
						DynamicClientSet: fakedynamic.NewSimpleDynamicClient(scheme.Scheme),
						ClusterName:      clusterName,
					}, nil
				}

				if err := upsertCluster(clientConnector, labels, apiEndpoint, clusterName, resourceVersion); err != nil {
					return err
				}
				if err := cacheNextWrapper(controller); err != nil {
					return err
				}

				if err := upsertResourceRegistry(clientConnector, resourceSelectors, registryName, resourceVersion, []string{clusterName}); err != nil {
					return err
				}
				if err := cacheNextWrapper(controller); err != nil {
					return err
				}

				if err := upsertResourceRegistry(clientConnector, resourceSelectorsUpdated, registryName, resourceVersion, []string{clusterName}); err != nil {
					return err
				}
				if err := cacheNextWrapper(controller); err != nil {
					return err
				}

				return nil
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())

			controller, informer, err := test.prep(ctx, test.client, test.restConfig, test.restMapper)
			if err != nil {
				t.Fatalf("failed to prepare test environment for event handler setup, got: %v", err)
			}
			informer.Start(ctx.Done())
			informer.WaitForCacheSync(ctx.Done())
			defer informer.Shutdown()
			defer cancel()

			if err := test.verify(test.client, controller); err != nil {
				t.Errorf("failed to verify controller, got: %v", err)
			}
		})
	}
}

func TestDeleteResourceRegistryEventHandler(t *testing.T) {
	tests := []struct {
		name       string
		restConfig *rest.Config
		client     *fakekarmadaclient.Clientset
		restMapper meta.RESTMapper
		prep       func(context.Context, *fakekarmadaclient.Clientset, *rest.Config, meta.RESTMapper) (*Controller, informerfactory.SharedInformerFactory, error)
		verify     func(*fakekarmadaclient.Clientset, *Controller) error
	}{
		{
			name:       "AddAllEventHandlers_TriggerDeleteResourceRegistryEvent_DeletedResourceRegistryAddedToWorkQueue",
			restConfig: &rest.Config{},
			client:     fakekarmadaclient.NewSimpleClientset(),
			restMapper: restmapper.NewDiscoveryRESTMapper(apiGroupResources),
			prep: func(ctx context.Context, clientConnector *fakekarmadaclient.Clientset, restConfig *rest.Config, restMapper meta.RESTMapper) (*Controller, informerfactory.SharedInformerFactory, error) {
				factory := informerfactory.NewSharedInformerFactory(clientConnector, time.Second)
				controller, err := createController(ctx, restConfig, factory, restMapper)
				return controller, factory, err
			},
			verify: func(clientConnector *fakekarmadaclient.Clientset, controller *Controller) error {
				var (
					registryName, clusterName    = "test-registry", "test-cluster"
					resourceVersion, apiEndpoint = "1000", "10.0.0.1"
					labels                       = map[string]string{}
					resourceSelectors            = []searchv1alpha1.ResourceSelector{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
						},
					}
				)

				clusterDynamicClientBuilder = func(string, client.Client) (*util.DynamicClusterClient, error) {
					return &util.DynamicClusterClient{
						DynamicClientSet: fakedynamic.NewSimpleDynamicClient(scheme.Scheme),
						ClusterName:      clusterName,
					}, nil
				}

				if err := upsertCluster(clientConnector, labels, apiEndpoint, clusterName, resourceVersion); err != nil {
					return err
				}
				if err := cacheNextWrapper(controller); err != nil {
					return err
				}

				if err := upsertResourceRegistry(clientConnector, resourceSelectors, registryName, resourceVersion, []string{clusterName}); err != nil {
					return err
				}
				if err := cacheNextWrapper(controller); err != nil {
					return err
				}

				if err := deleteResourceRegistry(clientConnector, registryName); err != nil {
					return err
				}
				if err := cacheNextWrapper(controller); err != nil {
					return err
				}

				return nil
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())

			controller, informer, err := test.prep(ctx, test.client, test.restConfig, test.restMapper)
			if err != nil {
				t.Fatalf("failed to prepare test environment for event handler setup, got: %v", err)
			}
			informer.Start(ctx.Done())
			informer.WaitForCacheSync(ctx.Done())
			defer informer.Shutdown()
			defer cancel()

			if err := test.verify(test.client, controller); err != nil {
				t.Errorf("failed to verify controller, got: %v", err)
			}
		})
	}
}

// createController initializes a new Controller instance using the provided
// Kubernetes REST configuration, shared informer factory, and REST mapper.
// It returns the created Controller or an error if initialization fails.
func createController(ctx context.Context, restConfig *rest.Config, factory informerfactory.SharedInformerFactory, restMapper meta.RESTMapper) (*Controller, error) {
	newController, err := NewController(restConfig, factory, restMapper)
	if err != nil {
		return nil, fmt.Errorf("failed to create new controller, got: %v", err)
	}
	newController.InformerManager = genericmanager.NewMultiClusterInformerManager(ctx)
	return newController, nil
}

// upsertCluster creates or updates a Cluster resource in the Kubernetes API using the provided
// client, labels, API endpoint, cluster name, and resource version.
func upsertCluster(client *fakekarmadaclient.Clientset, labels map[string]string, apiEndpoint, clusterName, resourceVersion string) error {
	cluster := &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:            clusterName,
			ResourceVersion: resourceVersion,
			Labels:          labels,
		},
		Spec: clusterv1alpha1.ClusterSpec{
			APIEndpoint: apiEndpoint,
		},
		Status: clusterv1alpha1.ClusterStatus{
			Conditions: []metav1.Condition{
				{
					Type:   clusterv1alpha1.ClusterConditionReady,
					Status: metav1.ConditionTrue,
				},
			},
			APIEnablements: []clusterv1alpha1.APIEnablement{
				{
					GroupVersion: "apps/v1",
					Resources: []clusterv1alpha1.APIResource{
						{
							Name: "deployments",
							Kind: "Deployment",
						},
					},
				},
				{
					GroupVersion: "v1",
					Resources: []clusterv1alpha1.APIResource{
						{
							Name: "pods",
							Kind: "Pod",
						},
					},
				},
			},
		},
	}

	// Try to create the Cluster.
	_, err := client.ClusterV1alpha1().Clusters().Create(context.TODO(), cluster, metav1.CreateOptions{})
	if err == nil {
		// Successfully created the Cluster.
		return nil
	}

	// If the Cluster already exists, update it.
	if apierrors.IsAlreadyExists(err) {
		_, updateErr := client.ClusterV1alpha1().Clusters().Update(context.TODO(), cluster, metav1.UpdateOptions{})
		if updateErr != nil {
			return fmt.Errorf("failed to update cluster: %v", updateErr)
		}
		return nil
	}

	// Return any other errors encountered.
	return err
}

// upsertResourceRegistry creates or updates a ResourceRegistry resource in the Kubernetes API.
// It uses the provided client, resource selectors, registry name, resource version, and target cluster names.
func upsertResourceRegistry(client *fakekarmadaclient.Clientset, resourceSelectors []searchv1alpha1.ResourceSelector, registryName, resourceVersion string, clusterNames []string) error {
	resourceRegistry := &searchv1alpha1.ResourceRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name:            registryName,
			ResourceVersion: resourceVersion,
		},
		Spec: searchv1alpha1.ResourceRegistrySpec{
			TargetCluster: policyv1alpha1.ClusterAffinity{
				ClusterNames: clusterNames,
			},
			ResourceSelectors: resourceSelectors,
		},
	}

	// Try to create the ResourceRegistry.
	_, err := client.SearchV1alpha1().ResourceRegistries().Create(context.TODO(), resourceRegistry, metav1.CreateOptions{})
	if err == nil {
		// Successfully created the ResourceRegistry.
		return nil
	}

	// If the ResourceRegistry already exists, update it.
	if apierrors.IsAlreadyExists(err) {
		_, updateErr := client.SearchV1alpha1().ResourceRegistries().Update(context.TODO(), resourceRegistry, metav1.UpdateOptions{})
		if updateErr != nil {
			return fmt.Errorf("failed to update ResourceRegistry: %v", updateErr)
		}
		return nil
	}

	// Return any other errors encountered.
	return err
}

// deleteCluster deletes a Cluster resource by name from the Kubernetes API using the provided client.
func deleteCluster(client *fakekarmadaclient.Clientset, clusterName string) error {
	if err := client.ClusterV1alpha1().Clusters().Delete(context.TODO(), clusterName, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("failed to delete cluster, got: %v", err)
	}
	return nil
}

// deleteResourceRegistry deletes a ResourceRegistry resource by name from the Kubernetes API
// using the provided client.
func deleteResourceRegistry(client *fakekarmadaclient.Clientset, resourceRegistryName string) error {
	if err := client.SearchV1alpha1().ResourceRegistries().Delete(context.TODO(), resourceRegistryName, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("failed to delete resource registry, got: %v", err)
	}
	return nil
}

// cacheNextWrapper calls the cacheNext method on the provided Controller instance.
// If the cacheNext method fails, it returns an error.
func cacheNextWrapper(controller *Controller) error {
	if !controller.cacheNext() {
		return errors.New("failed to cache next object")
	}
	return nil
}
