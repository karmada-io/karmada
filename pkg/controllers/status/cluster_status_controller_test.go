/*
Copyright 2023 The Karmada Authors.

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

package status

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/typedmanager"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

// copy from go/src/net/http/internal/testcert/testcert.go
var testCA = []byte(`-----BEGIN CERTIFICATE-----
MIIDOTCCAiGgAwIBAgIQSRJrEpBGFc7tNb1fb5pKFzANBgkqhkiG9w0BAQsFADAS
MRAwDgYDVQQKEwdBY21lIENvMCAXDTcwMDEwMTAwMDAwMFoYDzIwODQwMTI5MTYw
MDAwWjASMRAwDgYDVQQKEwdBY21lIENvMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A
MIIBCgKCAQEA6Gba5tHV1dAKouAaXO3/ebDUU4rvwCUg/CNaJ2PT5xLD4N1Vcb8r
bFSW2HXKq+MPfVdwIKR/1DczEoAGf/JWQTW7EgzlXrCd3rlajEX2D73faWJekD0U
aUgz5vtrTXZ90BQL7WvRICd7FlEZ6FPOcPlumiyNmzUqtwGhO+9ad1W5BqJaRI6P
YfouNkwR6Na4TzSj5BrqUfP0FwDizKSJ0XXmh8g8G9mtwxOSN3Ru1QFc61Xyeluk
POGKBV/q6RBNklTNe0gI8usUMlYyoC7ytppNMW7X2vodAelSu25jgx2anj9fDVZu
h7AXF5+4nJS4AAt0n1lNY7nGSsdZas8PbQIDAQABo4GIMIGFMA4GA1UdDwEB/wQE
AwICpDATBgNVHSUEDDAKBggrBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MB0GA1Ud
DgQWBBStsdjh3/JCXXYlQryOrL4Sh7BW5TAuBgNVHREEJzAlggtleGFtcGxlLmNv
bYcEfwAAAYcQAAAAAAAAAAAAAAAAAAAAATANBgkqhkiG9w0BAQsFAAOCAQEAxWGI
5NhpF3nwwy/4yB4i/CwwSpLrWUa70NyhvprUBC50PxiXav1TeDzwzLx/o5HyNwsv
cxv3HdkLW59i/0SlJSrNnWdfZ19oTcS+6PtLoVyISgtyN6DpkKpdG1cOkW3Cy2P2
+tK/tKHRP1Y/Ra0RiDpOAmqn0gCOFGz8+lqDIor/T7MTpibL3IxqWfPrvfVRHL3B
grw/ZQTTIVjjh4JBSW3WyWgNo/ikC1lrVxzl4iPUGptxT36Cr7Zk2Bsg0XqwbOvK
5d+NTDREkSnUbie4GeutujmX3Dsx88UiV6UY/4lHJa6I5leHUNOHahRbpbWeOfs/
WkBKOclmOV2xlTVuPw==
-----END CERTIFICATE-----`)

func TestClusterStatusController_Reconcile(t *testing.T) {
	tests := []struct {
		name           string
		cluster        *clusterv1alpha1.Cluster
		clusterName    string
		expectedResult controllerruntime.Result
		expectedError  bool
	}{
		{
			name:           "Cluster not found",
			clusterName:    "test-cluster",
			expectedResult: controllerruntime.Result{},
			expectedError:  false,
		},
		{
			name:        "Cluster found with finalizer",
			clusterName: "test-cluster",
			cluster: &clusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{
						util.ClusterControllerFinalizer,
					},
				},
			},
			expectedResult: controllerruntime.Result{},
			expectedError:  false,
		},
		{
			name:           "Cluster found without finalizer",
			clusterName:    "test-cluster",
			cluster:        &clusterv1alpha1.Cluster{},
			expectedResult: controllerruntime.Result{Requeue: true},
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up the test environment.
			c := &ClusterStatusController{
				Client:                 fake.NewClientBuilder().WithScheme(gclient.NewSchema()).Build(),
				GenericInformerManager: genericmanager.GetInstance(),
				TypedInformerManager:   typedmanager.GetInstance(),
				ClusterClientOption:    &util.ClientOption{},
				ClusterClientSetFunc:   util.NewClusterClientSet,
			}

			if tt.cluster != nil {
				// Add a cluster to the fake client.
				tt.cluster.ObjectMeta.Name = tt.clusterName
				c.Client = fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithStatusSubresource(tt.cluster).Build()

				if err := c.Client.Create(context.Background(), tt.cluster); err != nil {
					t.Fatalf("Failed to create cluster: %v", err)
				}
			}

			// Run the reconcile function.
			req := controllerruntime.Request{
				NamespacedName: types.NamespacedName{
					Name: tt.clusterName,
				},
			}

			result, err := c.Reconcile(context.Background(), req)

			// Check the results.
			if tt.expectedError && err == nil {
				t.Errorf("Expected an error but got nil")
			} else if !tt.expectedError && err != nil {
				t.Errorf("Expected no error but got %v", err)
			}

			if !reflect.DeepEqual(tt.expectedResult, result) {
				t.Errorf("Expected result %v but got %v", tt.expectedResult, result)
			}
		})
	}
}

func generateClusterClient(APIEndpoint string) *util.ClusterClient {
	clusterClient := &util.ClusterClient{
		ClusterName: "test",
	}
	hostClient := fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
		&clusterv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: clusterv1alpha1.ClusterSpec{
				APIEndpoint: APIEndpoint,
				SecretRef:   &clusterv1alpha1.LocalSecretReference{Namespace: "ns1", Name: "secret1"},
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "secret1"},
			Data:       map[string][]byte{clusterv1alpha1.SecretTokenKey: []byte("token"), clusterv1alpha1.SecretCADataKey: testCA},
		}).Build()
	clusterClientSet, _ := util.NewClusterClientSet("test", hostClient, nil)
	clusterClient.KubeClient = clusterClientSet.KubeClient
	return clusterClient
}

func clusterClientSetFunc(string, client.Client, *util.ClientOption) (*util.ClusterClient, error) {
	clusterClient := generateClusterClient(serverAddress)
	// cannot mock "/readyz"'s response, so create an error
	return clusterClient, nil
}

func clusterClientSetFuncWithError(string, client.Client, *util.ClientOption) (*util.ClusterClient, error) {
	clusterClient := generateClusterClient(serverAddress)
	return clusterClient, fmt.Errorf("err")
}

var serverAddress string

func TestClusterStatusController_syncClusterStatus(t *testing.T) {
	t.Run("ClusterClientSetFunc returns error", func(t *testing.T) {
		server := mockServer(http.StatusOK, false)
		defer server.Close()
		serverAddress = server.URL
		cluster := &clusterv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: clusterv1alpha1.ClusterSpec{
				APIEndpoint: server.URL,
				SecretRef:   &clusterv1alpha1.LocalSecretReference{Namespace: "ns1", Name: "secret1"},
				ProxyURL:    "http://1.1.1.1",
			},
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "secret1"},
			Data:       map[string][]byte{clusterv1alpha1.SecretTokenKey: []byte("token"), clusterv1alpha1.SecretCADataKey: testCA},
		}
		c := &ClusterStatusController{
			Client:                 fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithStatusSubresource(cluster, secret).Build(),
			GenericInformerManager: genericmanager.GetInstance(),
			TypedInformerManager:   typedmanager.GetInstance(),
			ClusterSuccessThreshold: metav1.Duration{
				Duration: time.Duration(1000),
			},
			ClusterFailureThreshold: metav1.Duration{
				Duration: time.Duration(1000),
			},
			clusterConditionCache: clusterConditionStore{},
			PredicateFunc:         helper.NewClusterPredicateOnAgent("test"),
			RateLimiterOptions: ratelimiterflag.Options{
				RateLimiterBaseDelay:  time.Duration(1000),
				RateLimiterMaxDelay:   time.Duration(1000),
				RateLimiterQPS:        10,
				RateLimiterBucketSize: 10,
			},
			ClusterClientSetFunc:        clusterClientSetFuncWithError,
			ClusterDynamicClientSetFunc: util.NewClusterDynamicClientSetForAgent,
		}
		if err := c.Client.Create(context.Background(), cluster); err != nil {
			t.Fatalf("Failed to create cluster: %v", err)
		}
		err := c.syncClusterStatus(context.Background(), cluster)
		assert.Empty(t, err)
	})
	t.Run("online is false, readyCondition.Status isn't true", func(t *testing.T) {
		server := mockServer(http.StatusNotFound, true)
		defer server.Close()
		serverAddress = server.URL
		cluster := &clusterv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: clusterv1alpha1.ClusterSpec{
				APIEndpoint: server.URL,
				SecretRef:   &clusterv1alpha1.LocalSecretReference{Namespace: "ns1", Name: "secret1"},
				ProxyURL:    "http://1.1.1.2",
			},
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "secret1"},
			Data:       map[string][]byte{clusterv1alpha1.SecretTokenKey: []byte("token"), clusterv1alpha1.SecretCADataKey: testCA},
		}
		c := &ClusterStatusController{
			Client:                 fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithStatusSubresource(cluster, secret).Build(),
			GenericInformerManager: genericmanager.GetInstance(),
			TypedInformerManager:   typedmanager.GetInstance(),
			ClusterSuccessThreshold: metav1.Duration{
				Duration: time.Duration(1000),
			},
			ClusterFailureThreshold: metav1.Duration{
				Duration: time.Duration(1000),
			},
			clusterConditionCache: clusterConditionStore{},
			PredicateFunc:         helper.NewClusterPredicateOnAgent("test"),
			RateLimiterOptions: ratelimiterflag.Options{
				RateLimiterBaseDelay:  time.Duration(1000),
				RateLimiterMaxDelay:   time.Duration(1000),
				RateLimiterQPS:        10,
				RateLimiterBucketSize: 10,
			},
			ClusterClientSetFunc:        clusterClientSetFunc,
			ClusterDynamicClientSetFunc: util.NewClusterDynamicClientSetForAgent,
		}

		if err := c.Client.Create(context.Background(), cluster); err != nil {
			t.Fatalf("Failed to create cluster: %v", err)
		}

		err := c.syncClusterStatus(context.Background(), cluster)
		assert.Empty(t, err)
	})

	t.Run("online and healthy is true", func(t *testing.T) {
		server := mockServer(http.StatusOK, false)
		defer server.Close()
		serverAddress = server.URL
		cluster := &clusterv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: clusterv1alpha1.ClusterSpec{
				APIEndpoint: server.URL,
				SecretRef:   &clusterv1alpha1.LocalSecretReference{Namespace: "ns1", Name: "secret1"},
				ProxyURL:    "http://1.1.1.1",
			},
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "secret1"},
			Data:       map[string][]byte{clusterv1alpha1.SecretTokenKey: []byte("token"), clusterv1alpha1.SecretCADataKey: testCA},
		}
		c := &ClusterStatusController{
			Client:                 fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithStatusSubresource(cluster, secret).Build(),
			GenericInformerManager: genericmanager.GetInstance(),
			TypedInformerManager:   typedmanager.GetInstance(),
			ClusterSuccessThreshold: metav1.Duration{
				Duration: time.Duration(1000),
			},
			ClusterFailureThreshold: metav1.Duration{
				Duration: time.Duration(1000),
			},
			clusterConditionCache: clusterConditionStore{},
			PredicateFunc:         helper.NewClusterPredicateOnAgent("test"),
			RateLimiterOptions: ratelimiterflag.Options{
				RateLimiterBaseDelay:  time.Duration(1000),
				RateLimiterMaxDelay:   time.Duration(1000),
				RateLimiterQPS:        10,
				RateLimiterBucketSize: 10,
			},
			ClusterClientSetFunc:        clusterClientSetFunc,
			ClusterDynamicClientSetFunc: util.NewClusterDynamicClientSetForAgent,
		}

		if err := c.Client.Create(context.Background(), cluster); err != nil {
			t.Fatalf("Failed to create cluster: %v", err)
		}
		err := c.syncClusterStatus(context.Background(), cluster)
		assert.Empty(t, err)
	})
}

func TestGetNodeSummary(t *testing.T) {
	nodes := []*corev1.Node{
		{
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
		{
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{
					{
						Type: corev1.NodeMemoryPressure,
					},
				},
			},
		},
	}

	res := getNodeSummary(nodes)
	if res.TotalNum != 2 {
		t.Errorf("TotalNum is %v, expect 2", res.TotalNum)
	}

	if res.ReadyNum != 1 {
		t.Errorf("ReadyNum is %v, expect 1", res.ReadyNum)
	}
}

func TestGenerateReadyCondition(t *testing.T) {
	online := true
	healthy := true
	con := generateReadyCondition(online, healthy)
	expect := util.NewCondition(clusterv1alpha1.ClusterConditionReady, clusterReady, clusterHealthy, metav1.ConditionTrue)
	assert.Equal(t, expect, con)

	healthy = false
	con = generateReadyCondition(online, healthy)
	expect = util.NewCondition(clusterv1alpha1.ClusterConditionReady, clusterNotReady, clusterUnhealthy, metav1.ConditionFalse)
	assert.Equal(t, expect, con)

	online = false
	con = generateReadyCondition(online, healthy)
	expect = util.NewCondition(clusterv1alpha1.ClusterConditionReady, clusterNotReachableReason, clusterNotReachableMsg, metav1.ConditionFalse)
	assert.Equal(t, expect, con)
}

func TransformFunc(interface{}) (interface{}, error) {
	return nil, nil
}

func TestListPods(t *testing.T) {
	ctx := context.Background()
	clientset := kubernetesfake.NewSimpleClientset()
	transformFuncs := map[schema.GroupVersionResource]cache.TransformFunc{
		{}: TransformFunc,
	}
	m := typedmanager.NewSingleClusterInformerManager(ctx, clientset, 0, transformFuncs)
	pods, err := listPods(m)
	assert.Equal(t, []*corev1.Pod(nil), pods)
	assert.Empty(t, err, "listPods returns error")
}

func TestListNodes(t *testing.T) {
	ctx := context.Background()
	clientset := kubernetesfake.NewSimpleClientset()
	transformFuncs := map[schema.GroupVersionResource]cache.TransformFunc{
		{}: TransformFunc,
	}
	m := typedmanager.NewSingleClusterInformerManager(ctx, clientset, 0, transformFuncs)
	nodes, err := listNodes(m)
	assert.Equal(t, []*corev1.Node(nil), nodes)
	assert.Empty(t, err, "listNodes returns error")
}

func TestGetResourceSummary(t *testing.T) {
	nodes := []*corev1.Node{
		{
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					"test": resource.Quantity{},
				},
			},
		},
		{
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					"test": resource.Quantity{},
				},
			},
		},
	}
	pods := []*corev1.Pod{
		{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								"test": resource.Quantity{},
							}}}},
			}}}

	resourceSummary := getResourceSummary(nodes, pods)
	if _, ok := resourceSummary.Allocatable["test"]; !ok {
		t.Errorf("key test isn't in resourceSummary.Allocatable")
	}

	if _, ok := resourceSummary.Allocating["pods"]; !ok {
		t.Errorf("key pods isn't in resourceSummary.Allocating")
	}

	if _, ok := resourceSummary.Allocated["test"]; ok {
		t.Errorf("key test is in resourceSummary.Allocated")
	}

	if len(resourceSummary.AllocatableModelings) != 0 {
		t.Errorf("resourceSummary.AllocatableModelings isn't 0")
	}
}

func TestGetClusterAllocatable(t *testing.T) {
	nodes := []*corev1.Node{
		{
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					"test": resource.Quantity{},
				},
			},
		},
		{
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					"test": resource.Quantity{},
				},
			},
		},
	}
	expect := make(corev1.ResourceList)
	expect["test"] = resource.Quantity{}
	actual := getClusterAllocatable(nodes)

	assert.Equal(t, expect, actual)
}

func TestGetAllocatingResource(t *testing.T) {
	podList := []*corev1.Pod{
		{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								"test": resource.Quantity{},
							}}}},
			}},
	}

	actual := getAllocatingResource(podList)
	assert.NotEmpty(t, actual["pods"])
}

func TestGetAllocatedResource(t *testing.T) {
	podList := []*corev1.Pod{
		{
			Status: corev1.PodStatus{
				Phase: corev1.PodPending,
			},
			Spec: corev1.PodSpec{
				NodeName: "node1",
				Containers: []corev1.Container{
					{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								"test": resource.Quantity{},
							}}}},
			}},
	}

	actual := getAllocatedResource(podList)
	assert.NotEmpty(t, actual["pods"])
}

func TestGetNodeAvailable(t *testing.T) {
	tests := []struct {
		name         string
		allocatable  corev1.ResourceList
		podResources *util.Resource
		expect       corev1.ResourceList
	}{
		{
			name: "resource isn't enough",
			allocatable: corev1.ResourceList{
				corev1.ResourcePods: *resource.NewQuantity(10, resource.DecimalSI),
			},
			podResources: &util.Resource{
				AllowedPodNumber: 11,
			},
			expect: corev1.ResourceList(nil),
		},
		{
			name: "resource is enough",
			allocatable: corev1.ResourceList{
				corev1.ResourcePods: *resource.NewQuantity(10, resource.DecimalSI),
			},
			podResources: &util.Resource{
				AllowedPodNumber: 9,
			},
			expect: corev1.ResourceList{
				corev1.ResourcePods: *resource.NewQuantity(1, resource.DecimalSI),
			},
		},
		{
			name: "podResources is nil",
			allocatable: corev1.ResourceList{
				corev1.ResourcePods: *resource.NewQuantity(10, resource.DecimalSI),
			},
			podResources: nil,
			expect: corev1.ResourceList{
				corev1.ResourcePods: *resource.NewQuantity(10, resource.DecimalSI),
			},
		},
		{
			name: "allocatedResourceList is nil",
			allocatable: corev1.ResourceList{
				corev1.ResourcePods: *resource.NewQuantity(10, resource.DecimalSI),
			},
			podResources: &util.Resource{
				AllowedPodNumber: 0,
			},
			expect: corev1.ResourceList{
				corev1.ResourcePods: *resource.NewQuantity(10, resource.DecimalSI),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := getNodeAvailable(tt.allocatable, tt.podResources)
			assert.Equal(t, tt.expect, actual)
		})
	}
}

func TestGetAllocatableModelings(t *testing.T) {
	tests := []struct {
		name    string
		pods    []*corev1.Pod
		nodes   []*corev1.Node
		cluster *clusterv1alpha1.Cluster
		expect  []clusterv1alpha1.AllocatableModeling
	}{
		{
			name: "suc to get AllocatableModeling",
			pods: []*corev1.Pod{
				{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										"test": resource.Quantity{},
									}}}},
					}},
			},
			nodes: []*corev1.Node{
				{
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							"test": resource.Quantity{},
						},
					},
				},
				{
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							"test": resource.Quantity{},
						},
					},
				},
			},
			cluster: &clusterv1alpha1.Cluster{
				Spec: clusterv1alpha1.ClusterSpec{
					ResourceModels: []clusterv1alpha1.ResourceModel{
						{
							Grade: 0,
							Ranges: []clusterv1alpha1.ResourceModelRange{
								{
									Name: corev1.ResourceCPU,
									Min:  *resource.NewMilliQuantity(0, resource.DecimalSI),
									Max:  *resource.NewQuantity(1, resource.DecimalSI),
								},
								{
									Name: corev1.ResourceMemory,
									Min:  *resource.NewMilliQuantity(0, resource.DecimalSI),
									Max:  *resource.NewQuantity(1024, resource.DecimalSI),
								},
							},
						},
						{
							Grade: 1,
							Ranges: []clusterv1alpha1.ResourceModelRange{
								{
									Name: corev1.ResourceCPU,
									Min:  *resource.NewMilliQuantity(1, resource.DecimalSI),
									Max:  *resource.NewQuantity(2, resource.DecimalSI),
								},
								{
									Name: corev1.ResourceMemory,
									Min:  *resource.NewMilliQuantity(1024, resource.DecimalSI),
									Max:  *resource.NewQuantity(1024*2, resource.DecimalSI),
								},
							},
						},
					},
				},
			},
			expect: []clusterv1alpha1.AllocatableModeling{
				{
					Grade: 0,
					Count: 2,
				},
				{
					Grade: 1,
					Count: 0,
				},
			},
		},
		{
			name: "cluster.Spec.ResourceModels is empty",
			pods: []*corev1.Pod{
				{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										"test": resource.Quantity{},
									}}}},
					}},
			},
			nodes: []*corev1.Node{
				{
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							"test": resource.Quantity{},
						},
					},
				},
				{
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							"test": resource.Quantity{},
						},
					},
				},
			},
			cluster: &clusterv1alpha1.Cluster{
				Spec: clusterv1alpha1.ClusterSpec{
					ResourceModels: []clusterv1alpha1.ResourceModel{},
				},
			},
			expect: nil,
		},
		{
			name: "InitSummary occurs error",
			pods: []*corev1.Pod{
				{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										"test": resource.Quantity{},
									}}}},
					}},
			},
			nodes: []*corev1.Node{
				{
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							"test": resource.Quantity{},
						},
					},
				},
				{
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							"test": resource.Quantity{},
						},
					},
				},
			},
			cluster: &clusterv1alpha1.Cluster{
				Spec: clusterv1alpha1.ClusterSpec{
					ResourceModels: []clusterv1alpha1.ResourceModel{
						{
							Grade: 0,
							Ranges: []clusterv1alpha1.ResourceModelRange{
								{
									Name: corev1.ResourceCPU,
									Min:  *resource.NewMilliQuantity(0, resource.DecimalSI),
									Max:  *resource.NewQuantity(1, resource.DecimalSI),
								},
								{
									Name: corev1.ResourceMemory,
									Min:  *resource.NewMilliQuantity(0, resource.DecimalSI),
									Max:  *resource.NewQuantity(1024, resource.DecimalSI),
								},
							},
						},
						{
							Grade: 1,
							Ranges: []clusterv1alpha1.ResourceModelRange{
								{
									Name: corev1.ResourceCPU,
									Min:  *resource.NewMilliQuantity(1, resource.DecimalSI),
									Max:  *resource.NewQuantity(2, resource.DecimalSI),
								},
							},
						},
					},
				},
			},
			expect: nil,
		},
		{
			name: "pod.Spec.NodeName isn't 0",
			pods: []*corev1.Pod{
				{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										"test": resource.Quantity{},
									}}}},
						NodeName: "node1",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				},
			},
			nodes: []*corev1.Node{
				{
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							"test": resource.Quantity{},
						},
					},
				},
				{
					Status: corev1.NodeStatus{
						Allocatable: corev1.ResourceList{
							"test": resource.Quantity{},
						},
					},
				},
			},
			cluster: &clusterv1alpha1.Cluster{
				Spec: clusterv1alpha1.ClusterSpec{
					ResourceModels: []clusterv1alpha1.ResourceModel{
						{
							Grade: 0,
							Ranges: []clusterv1alpha1.ResourceModelRange{
								{
									Name: corev1.ResourceCPU,
									Min:  *resource.NewMilliQuantity(0, resource.DecimalSI),
									Max:  *resource.NewQuantity(1, resource.DecimalSI),
								},
								{
									Name: corev1.ResourceMemory,
									Min:  *resource.NewMilliQuantity(0, resource.DecimalSI),
									Max:  *resource.NewQuantity(1024, resource.DecimalSI),
								},
							},
						},
						{
							Grade: 1,
							Ranges: []clusterv1alpha1.ResourceModelRange{
								{
									Name: corev1.ResourceCPU,
									Min:  *resource.NewMilliQuantity(1, resource.DecimalSI),
									Max:  *resource.NewQuantity(2, resource.DecimalSI),
								},
								{
									Name: corev1.ResourceMemory,
									Min:  *resource.NewMilliQuantity(1024, resource.DecimalSI),
									Max:  *resource.NewQuantity(1024*2, resource.DecimalSI),
								},
							},
						},
					},
				},
			},
			expect: []clusterv1alpha1.AllocatableModeling{
				{
					Grade: 0,
					Count: 2,
				},
				{
					Grade: 1,
					Count: 0,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := getAllocatableModelings(tt.cluster, tt.nodes, tt.pods)
			assert.Equal(t, tt.expect, actual)
		})
	}
}

func TestClusterStatusController_updateStatusIfNeeded(t *testing.T) {
	t.Run("cluster is in client", func(t *testing.T) {
		cluster := &clusterv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster1",
				Namespace: "karmada",
			},
			Status: clusterv1alpha1.ClusterStatus{
				KubernetesVersion: "v1",
			},
			Spec: clusterv1alpha1.ClusterSpec{
				ResourceModels: []clusterv1alpha1.ResourceModel{
					{
						Grade: 0,
						Ranges: []clusterv1alpha1.ResourceModelRange{
							{
								Name: corev1.ResourceCPU,
								Min:  *resource.NewMilliQuantity(0, resource.DecimalSI),
								Max:  *resource.NewQuantity(1, resource.DecimalSI),
							},
							{
								Name: corev1.ResourceMemory,
								Min:  *resource.NewMilliQuantity(0, resource.DecimalSI),
								Max:  *resource.NewQuantity(1024, resource.DecimalSI),
							},
						},
					},
					{
						Grade: 1,
						Ranges: []clusterv1alpha1.ResourceModelRange{
							{
								Name: corev1.ResourceCPU,
								Min:  *resource.NewMilliQuantity(1, resource.DecimalSI),
								Max:  *resource.NewQuantity(2, resource.DecimalSI),
							},
							{
								Name: corev1.ResourceMemory,
								Min:  *resource.NewMilliQuantity(1024, resource.DecimalSI),
								Max:  *resource.NewQuantity(1024*2, resource.DecimalSI),
							},
						},
					},
				},
			},
		}

		currentClusterStatus := clusterv1alpha1.ClusterStatus{
			KubernetesVersion: "v2",
		}

		c := &ClusterStatusController{
			Client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
				cluster,
			).WithStatusSubresource(cluster).Build(),
			GenericInformerManager: genericmanager.GetInstance(),
			TypedInformerManager:   typedmanager.GetInstance(),
			ClusterClientOption:    &util.ClientOption{},
			ClusterClientSetFunc:   util.NewClusterClientSet,
		}

		err := c.updateStatusIfNeeded(context.Background(), cluster, currentClusterStatus)
		assert.Empty(t, err, "updateStatusIfNeeded returns error")
	})

	t.Run("cluster isn't in client", func(t *testing.T) {
		cluster := &clusterv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster1",
				Namespace: "karmada",
			},
			Status: clusterv1alpha1.ClusterStatus{
				KubernetesVersion: "v1",
			},
			Spec: clusterv1alpha1.ClusterSpec{
				ResourceModels: []clusterv1alpha1.ResourceModel{
					{
						Grade: 0,
						Ranges: []clusterv1alpha1.ResourceModelRange{
							{
								Name: corev1.ResourceCPU,
								Min:  *resource.NewMilliQuantity(0, resource.DecimalSI),
								Max:  *resource.NewQuantity(1, resource.DecimalSI),
							},
							{
								Name: corev1.ResourceMemory,
								Min:  *resource.NewMilliQuantity(0, resource.DecimalSI),
								Max:  *resource.NewQuantity(1024, resource.DecimalSI),
							},
						},
					},
					{
						Grade: 1,
						Ranges: []clusterv1alpha1.ResourceModelRange{
							{
								Name: corev1.ResourceCPU,
								Min:  *resource.NewMilliQuantity(1, resource.DecimalSI),
								Max:  *resource.NewQuantity(2, resource.DecimalSI),
							},
							{
								Name: corev1.ResourceMemory,
								Min:  *resource.NewMilliQuantity(1024, resource.DecimalSI),
								Max:  *resource.NewQuantity(1024*2, resource.DecimalSI),
							},
						},
					},
				},
			},
		}

		currentClusterStatus := clusterv1alpha1.ClusterStatus{
			KubernetesVersion: "v2",
		}

		c := &ClusterStatusController{
			Client:                 fake.NewClientBuilder().WithScheme(gclient.NewSchema()).Build(),
			GenericInformerManager: genericmanager.GetInstance(),
			TypedInformerManager:   typedmanager.GetInstance(),
			ClusterClientOption:    &util.ClientOption{},
			ClusterClientSetFunc:   util.NewClusterClientSet,
		}

		err := c.updateStatusIfNeeded(context.Background(), cluster, currentClusterStatus)
		assert.NotEmpty(t, err, "updateStatusIfNeeded doesn't return error")
	})
}

func NewClusterDynamicClientSetForAgentWithError(_ string, _ client.Client, _ *util.ClientOption) (*util.DynamicClusterClient, error) {
	return nil, fmt.Errorf("err")
}

func TestClusterStatusController_initializeGenericInformerManagerForCluster(t *testing.T) {
	t.Run("failed to create dynamicClient", func(*testing.T) {
		c := &ClusterStatusController{
			Client:                      fake.NewClientBuilder().WithScheme(gclient.NewSchema()).Build(),
			GenericInformerManager:      genericmanager.GetInstance(),
			TypedInformerManager:        typedmanager.GetInstance(),
			ClusterClientOption:         &util.ClientOption{},
			ClusterClientSetFunc:        util.NewClusterClientSet,
			ClusterDynamicClientSetFunc: NewClusterDynamicClientSetForAgentWithError,
		}
		clusterClientSet := &util.ClusterClient{
			ClusterName: "test",
		}

		c.initializeGenericInformerManagerForCluster(clusterClientSet)
	})

	t.Run("suc to create dynamicClient", func(*testing.T) {
		c := &ClusterStatusController{
			Client:                      fake.NewClientBuilder().WithScheme(gclient.NewSchema()).Build(),
			GenericInformerManager:      genericmanager.GetInstance(),
			TypedInformerManager:        typedmanager.GetInstance(),
			ClusterClientOption:         &util.ClientOption{},
			ClusterClientSetFunc:        util.NewClusterClientSet,
			ClusterDynamicClientSetFunc: util.NewClusterDynamicClientSetForAgent,
		}
		clusterClientSet := &util.ClusterClient{
			ClusterName: "test",
		}

		c.initializeGenericInformerManagerForCluster(clusterClientSet)
	})
}

func TestClusterStatusController_initLeaseController(_ *testing.T) {
	c := &ClusterStatusController{
		Client:                      fake.NewClientBuilder().WithScheme(gclient.NewSchema()).Build(),
		GenericInformerManager:      genericmanager.GetInstance(),
		TypedInformerManager:        typedmanager.GetInstance(),
		ClusterClientOption:         &util.ClientOption{},
		ClusterClientSetFunc:        util.NewClusterClientSet,
		ClusterDynamicClientSetFunc: NewClusterDynamicClientSetForAgentWithError,
	}

	cluster := &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster1",
			Namespace: "karmada",
		},
		Status: clusterv1alpha1.ClusterStatus{
			KubernetesVersion: "v1",
		},
		Spec: clusterv1alpha1.ClusterSpec{
			ResourceModels: []clusterv1alpha1.ResourceModel{
				{
					Grade: 0,
					Ranges: []clusterv1alpha1.ResourceModelRange{
						{
							Name: corev1.ResourceCPU,
							Min:  *resource.NewMilliQuantity(0, resource.DecimalSI),
							Max:  *resource.NewQuantity(1, resource.DecimalSI),
						},
						{
							Name: corev1.ResourceMemory,
							Min:  *resource.NewMilliQuantity(0, resource.DecimalSI),
							Max:  *resource.NewQuantity(1024, resource.DecimalSI),
						},
					},
				},
				{
					Grade: 1,
					Ranges: []clusterv1alpha1.ResourceModelRange{
						{
							Name: corev1.ResourceCPU,
							Min:  *resource.NewMilliQuantity(1, resource.DecimalSI),
							Max:  *resource.NewQuantity(2, resource.DecimalSI),
						},
						{
							Name: corev1.ResourceMemory,
							Min:  *resource.NewMilliQuantity(1024, resource.DecimalSI),
							Max:  *resource.NewQuantity(1024*2, resource.DecimalSI),
						},
					},
				},
			},
		},
	}

	c.initLeaseController(cluster)
}

func mockServer(statusCode int, existError bool) *httptest.Server {
	respBody := "test"
	resp := &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewBufferString(respBody)),
	}
	// Create an HTTP test server to handle the request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Write the response to the client
		if existError {
			statusCode := statusCode
			errorMessage := "An error occurred"
			http.Error(w, errorMessage, statusCode)
		} else {
			w.WriteHeader(resp.StatusCode)
			_, err := io.Copy(w, resp.Body)
			if err != nil {
				fmt.Printf("failed to copy, err: %v", err)
			}
		}
	}))

	return server
}

func TestHealthEndpointCheck(t *testing.T) {
	server := mockServer(http.StatusOK, false)
	defer server.Close()
	clusterClient := generateClusterClient(server.URL)
	actual, err := healthEndpointCheck(clusterClient.KubeClient, "/readyz")
	assert.Equal(t, http.StatusOK, actual)
	assert.Empty(t, err)
}

func TestGetClusterHealthStatus(t *testing.T) {
	t.Run("healthz return error and StatusNotFound", func(t *testing.T) {
		server := mockServer(http.StatusNotFound, true)
		defer server.Close()
		clusterClient := generateClusterClient(server.URL)
		online, healthy := getClusterHealthStatus(clusterClient)
		assert.Equal(t, false, online)
		assert.Equal(t, false, healthy)
	})

	t.Run("healthz return http.StatusOK", func(t *testing.T) {
		server := mockServer(http.StatusOK, false)
		defer server.Close()
		clusterClient := generateClusterClient(server.URL)
		online, healthy := getClusterHealthStatus(clusterClient)
		assert.Equal(t, true, online)
		assert.Equal(t, true, healthy)
	})

	t.Run("healthz return http.StatusAccepted", func(t *testing.T) {
		server := mockServer(http.StatusAccepted, false)
		defer server.Close()
		clusterClient := generateClusterClient(server.URL)
		online, healthy := getClusterHealthStatus(clusterClient)
		assert.Equal(t, true, online)
		assert.Equal(t, false, healthy)
	})
}
