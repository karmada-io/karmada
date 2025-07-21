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
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/default/native"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/objectwatcher"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

func newCluster(name string, clusterType string, clusterStatus metav1.ConditionStatus) *clusterv1alpha1.Cluster {
	return &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: clusterv1alpha1.ClusterSpec{},
		Status: clusterv1alpha1.ClusterStatus{
			Conditions: []metav1.Condition{
				{
					Type:   clusterType,
					Status: clusterStatus,
				},
			},
		},
	}
}

func TestWorkStatusController_Reconcile(t *testing.T) {
	tests := []struct {
		name      string
		c         WorkStatusController
		work      *workv1alpha1.Work
		ns        string
		expectRes controllerruntime.Result
		existErr  bool
	}{
		{
			name: "normal case",
			c: WorkStatusController{
				Client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
						Spec: clusterv1alpha1.ClusterSpec{
							APIEndpoint: "https://127.0.0.1",
							SecretRef:   &clusterv1alpha1.LocalSecretReference{Namespace: "ns1", Name: "secret1"},
						},
						Status: clusterv1alpha1.ClusterStatus{
							Conditions: []metav1.Condition{
								{
									Type:   clusterv1alpha1.ClusterConditionReady,
									Status: metav1.ConditionTrue,
								},
							},
						},
					},
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "secret1"},
						Data:       map[string][]byte{clusterv1alpha1.SecretTokenKey: []byte("token"), clusterv1alpha1.SecretCADataKey: testCA},
					}).Build(),
				InformerManager:             genericmanager.GetInstance(),
				WorkPredicateFunc:           helper.NewClusterPredicateOnAgent("test"),
				ClusterDynamicClientSetFunc: util.NewClusterDynamicClientSet,
				ClusterCacheSyncTimeout:     metav1.Duration{},
				RateLimiterOptions:          ratelimiterflag.Options{},
			},
			work: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "work",
					Namespace: "karmada-es-cluster",
				},
				Status: workv1alpha1.WorkStatus{
					Conditions: []metav1.Condition{
						{
							Type:   workv1alpha1.WorkApplied,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			ns:        "karmada-es-cluster",
			expectRes: controllerruntime.Result{},
			existErr:  false,
		},
		{
			name: "work not exists",
			c: WorkStatusController{
				Client:                      fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(newCluster("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)).Build(),
				InformerManager:             genericmanager.GetInstance(),
				WorkPredicateFunc:           helper.NewClusterPredicateOnAgent("test"),
				ClusterDynamicClientSetFunc: util.NewClusterDynamicClientSetForAgent,
				ClusterCacheSyncTimeout:     metav1.Duration{},
				RateLimiterOptions:          ratelimiterflag.Options{},
			},
			work: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "work-1",
					Namespace: "karmada-es-cluster",
				},
				Status: workv1alpha1.WorkStatus{
					Conditions: []metav1.Condition{
						{
							Type:   workv1alpha1.WorkApplied,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			ns:        "karmada-es-cluster",
			expectRes: controllerruntime.Result{},
			existErr:  false,
		},
		{
			name: "work's DeletionTimestamp isn't zero",
			c: WorkStatusController{
				Client:                      fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(newCluster("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)).Build(),
				InformerManager:             genericmanager.GetInstance(),
				WorkPredicateFunc:           helper.NewClusterPredicateOnAgent("test"),
				ClusterDynamicClientSetFunc: util.NewClusterDynamicClientSetForAgent,
				ClusterCacheSyncTimeout:     metav1.Duration{},
				RateLimiterOptions:          ratelimiterflag.Options{},
			},
			work: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "work",
					Namespace:         "karmada-es-cluster",
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
				},
				Status: workv1alpha1.WorkStatus{
					Conditions: []metav1.Condition{
						{
							Type:   workv1alpha1.WorkApplied,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			ns:        "karmada-es-cluster",
			expectRes: controllerruntime.Result{},
			existErr:  false,
		},
		{
			name: "work's status is not applied",
			c: WorkStatusController{
				Client:                      fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(newCluster("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)).Build(),
				InformerManager:             genericmanager.GetInstance(),
				WorkPredicateFunc:           helper.NewClusterPredicateOnAgent("test"),
				ClusterDynamicClientSetFunc: util.NewClusterDynamicClientSetForAgent,
				ClusterCacheSyncTimeout:     metav1.Duration{},
				RateLimiterOptions:          ratelimiterflag.Options{},
			},
			work: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "work",
					Namespace: "karmada-es-cluster",
				},
				Status: workv1alpha1.WorkStatus{
					Conditions: []metav1.Condition{
						{
							Type:   workv1alpha1.WorkAvailable,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			ns:        "karmada-es-cluster",
			expectRes: controllerruntime.Result{},
			existErr:  false,
		},
		{
			name: "failed to get cluster name",
			c: WorkStatusController{
				Client:                      fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(newCluster("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)).Build(),
				InformerManager:             genericmanager.GetInstance(),
				WorkPredicateFunc:           helper.NewClusterPredicateOnAgent("test"),
				ClusterDynamicClientSetFunc: util.NewClusterDynamicClientSetForAgent,
				ClusterCacheSyncTimeout:     metav1.Duration{},
				RateLimiterOptions:          ratelimiterflag.Options{},
			},
			work: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "work",
					Namespace: "karmada-cluster",
				},
				Status: workv1alpha1.WorkStatus{
					Conditions: []metav1.Condition{
						{
							Type:   workv1alpha1.WorkApplied,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			ns:        "karmada-cluster",
			expectRes: controllerruntime.Result{},
			existErr:  true,
		},
		{
			name: "failed to get cluster",
			c: WorkStatusController{
				Client:                      fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(newCluster("cluster1", clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)).Build(),
				InformerManager:             genericmanager.GetInstance(),
				WorkPredicateFunc:           helper.NewClusterPredicateOnAgent("test"),
				ClusterDynamicClientSetFunc: util.NewClusterDynamicClientSetForAgent,
				ClusterCacheSyncTimeout:     metav1.Duration{},
				RateLimiterOptions:          ratelimiterflag.Options{},
			},
			work: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "work",
					Namespace: "karmada-es-cluster",
				},
				Status: workv1alpha1.WorkStatus{
					Conditions: []metav1.Condition{
						{
							Type:   workv1alpha1.WorkApplied,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			ns:        "karmada-es-cluster",
			expectRes: controllerruntime.Result{},
			existErr:  true,
		},
		{
			name: "cluster is not ready",
			c: WorkStatusController{
				Client:                      fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(newCluster("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionFalse)).Build(),
				InformerManager:             genericmanager.GetInstance(),
				WorkPredicateFunc:           helper.NewClusterPredicateOnAgent("test"),
				ClusterDynamicClientSetFunc: util.NewClusterDynamicClientSetForAgent,
				ClusterCacheSyncTimeout:     metav1.Duration{},
				RateLimiterOptions:          ratelimiterflag.Options{},
			},
			work: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "work",
					Namespace: "karmada-es-cluster",
				},
				Status: workv1alpha1.WorkStatus{
					Conditions: []metav1.Condition{
						{
							Type:   workv1alpha1.WorkApplied,
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			ns:        "karmada-es-cluster",
			expectRes: controllerruntime.Result{},
			existErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := controllerruntime.Request{
				NamespacedName: types.NamespacedName{
					Name:      "work",
					Namespace: tt.ns,
				},
			}

			if err := tt.c.Client.Create(context.Background(), tt.work); err != nil {
				t.Fatalf("Failed to create cluster: %v", err)
			}

			res, err := tt.c.Reconcile(context.Background(), req)
			assert.Equal(t, tt.expectRes, res)
			if tt.existErr {
				assert.NotEmpty(t, err)
			} else {
				assert.Empty(t, err)
			}
		})
	}
}

func TestWorkStatusController_getEventHandler(t *testing.T) {
	opt := util.Options{
		Name:          "opt",
		KeyFunc:       nil,
		ReconcileFunc: nil,
	}

	c := WorkStatusController{
		Client:                      fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(newCluster("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionFalse)).Build(),
		InformerManager:             genericmanager.GetInstance(),
		WorkPredicateFunc:           helper.NewClusterPredicateOnAgent("test"),
		ClusterDynamicClientSetFunc: util.NewClusterDynamicClientSetForAgent,
		ClusterCacheSyncTimeout:     metav1.Duration{},
		RateLimiterOptions:          ratelimiterflag.Options{},
		eventHandler:                nil,
		worker:                      util.NewAsyncWorker(opt),
	}

	eventHandler := c.getEventHandler()
	assert.NotEmpty(t, eventHandler)
}

func TestWorkStatusController_RunWorkQueue(_ *testing.T) {
	c := WorkStatusController{
		Client:                      fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(newCluster("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionFalse)).Build(),
		InformerManager:             genericmanager.GetInstance(),
		WorkPredicateFunc:           helper.NewClusterPredicateOnAgent("test"),
		ClusterDynamicClientSetFunc: util.NewClusterDynamicClientSetForAgent,
		ClusterCacheSyncTimeout:     metav1.Duration{},
		RateLimiterOptions:          ratelimiterflag.Options{},
		eventHandler:                nil,
		Context:                     context.Background(),
	}

	c.RunWorkQueue()
}

func TestGenerateKey(t *testing.T) {
	tests := []struct {
		name     string
		obj      *unstructured.Unstructured
		existRes bool
		existErr bool
	}{
		{
			name: "normal case",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "test",
						"namespace": "default",
						"annotations": map[string]interface{}{
							workv1alpha2.WorkNamespaceAnnotation: "karmada-es-cluster",
						},
					},
				},
			},
			existRes: true,
			existErr: false,
		},
		{
			name: "getClusterNameFromAnnotation failed",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "test",
						"namespace": "default",
						"annotations": map[string]interface{}{
							workv1alpha2.WorkNamespaceAnnotation: "karmada-cluster",
						},
					},
				},
			},
			existRes: false,
			existErr: true,
		},
		{
			name: "cluster is empty",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "test",
						"namespace": "default",
						"annotations": map[string]interface{}{
							"test": "karmada-es-cluster",
						},
					},
				},
			},
			existRes: false,
			existErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queueKey, err := generateKey(tt.obj)
			if tt.existRes {
				assert.NotEmpty(t, queueKey)
			} else {
				assert.Empty(t, queueKey)
			}

			if tt.existErr {
				assert.NotEmpty(t, err)
			} else {
				assert.Empty(t, err)
			}
		})
	}
}

func TestGetClusterNameFromAnnotation(t *testing.T) {
	tests := []struct {
		name     string
		resource *unstructured.Unstructured
		expect   string
		existErr bool
	}{
		{
			name: "normal case",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "test",
						"namespace": "default",
						"annotations": map[string]interface{}{
							workv1alpha2.WorkNamespaceAnnotation: "karmada-es-cluster",
						},
					},
				},
			},
			expect:   "cluster",
			existErr: false,
		},
		{
			name: "workNamespace is 0",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "test",
						"namespace": "default",
						"annotations": map[string]interface{}{
							"foo": "karmada-es-cluster",
						},
					},
				},
			},
			expect:   "",
			existErr: false,
		},
		{
			name: "failed to get cluster name",
			resource: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "test",
						"namespace": "default",
						"annotations": map[string]interface{}{
							workv1alpha2.WorkNamespaceAnnotation: "karmada-cluster",
						},
					},
				},
			},
			expect:   "",
			existErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := getClusterNameFromAnnotation(tt.resource)
			assert.Equal(t, tt.expect, actual)
			if tt.existErr {
				assert.NotEmpty(t, err)
			} else {
				assert.Empty(t, err)
			}
		})
	}
}

func newPodObj(namespace string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "pod",
				"namespace": "default",
				"annotations": map[string]interface{}{
					workv1alpha2.WorkNamespaceAnnotation: namespace,
					workv1alpha2.WorkNameAnnotation:      "work-name",
				},
			},
		},
	}
	return obj
}

func newPod(workNs, workName string, wrongAnnotations ...bool) *corev1.Pod {
	var pod *corev1.Pod
	if len(wrongAnnotations) > 0 && wrongAnnotations[0] == true {
		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod",
				Namespace: "default",
				Annotations: map[string]string{
					"test":                          workNs,
					workv1alpha2.WorkNameAnnotation: workName,
				},
			},
		}
	} else {
		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod",
				Namespace: "default",
				Annotations: map[string]string{
					workv1alpha2.WorkNamespaceAnnotation: workNs,
					workv1alpha2.WorkNameAnnotation:      workName,
				},
			},
		}
	}
	return pod
}

func TestWorkStatusController_syncWorkStatus(t *testing.T) {
	cluster := newCluster("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionFalse)
	workName := "work"
	workNs := "karmada-es-cluster"
	workUID := "92345678-1234-5678-1234-567812345678"

	tests := []struct {
		name                      string
		obj                       *unstructured.Unstructured
		pod                       *corev1.Pod
		raw                       []byte
		controllerWithoutInformer bool
		expectedError             bool
		wrongWorkNS               bool
		workApplyFunc             func(work *workv1alpha1.Work)
		assertFunc                func(t *testing.T, dynamicClientSets *dynamicfake.FakeDynamicClient)
	}{
		{
			name:                      "failed to exec NeedUpdate",
			obj:                       newPodObj("karmada-es-cluster"),
			pod:                       newPod(workNs, workName),
			raw:                       []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"default"}}`),
			controllerWithoutInformer: true,
			expectedError:             true,
		},
		{
			name:                      "invalid key, wrong WorkNamespaceLabel in obj",
			obj:                       newPodObj("karmada-cluster"),
			pod:                       newPod(workNs, workName),
			raw:                       []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"default"}}`),
			controllerWithoutInformer: true,
			expectedError:             true,
		},
		{
			name:                      "failed to GetObjectFromCache, wrong InformerManager in WorkStatusController",
			obj:                       newPodObj("karmada-es-cluster"),
			pod:                       newPod(workNs, workName),
			raw:                       []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"default"}}`),
			controllerWithoutInformer: false,
			expectedError:             true,
		},
		{
			name:                      "obj not found in informer, wrong dynamicClientSet without pod",
			obj:                       newPodObj("karmada-es-cluster"),
			raw:                       []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"default"}}`),
			controllerWithoutInformer: true,
			expectedError:             false,
		},
		{
			name:                      "workNamespace is zero, set wrong label 'test' in pod",
			obj:                       newPodObj("karmada-es-cluster"),
			pod:                       newPod(workNs, workName, true),
			raw:                       []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"default"}}`),
			controllerWithoutInformer: true,
			expectedError:             false,
		},
		{
			name:                      "failed to exec Client.Get, set wrong name in work",
			obj:                       newPodObj("karmada-es-cluster"),
			pod:                       newPod(workNs, workName),
			raw:                       []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"default"}}`),
			controllerWithoutInformer: true,
			expectedError:             false,
			workApplyFunc: func(work *workv1alpha1.Work) {
				work.SetName(fmt.Sprintf("%v-test", workNs))
			},
		},
		{
			name:                      "failed to getRawManifest, wrong Manifests in work",
			obj:                       newPodObj("karmada-es-cluster"),
			pod:                       newPod(workNs, workName),
			raw:                       []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod1","namespace":"default"}}`),
			controllerWithoutInformer: true,
			expectedError:             true,
		},
		{
			name:                      "failed to exec GetClusterName, wrong workNamespace",
			obj:                       newPodObj("karmada-es-cluster"),
			pod:                       newPod(workNs, workName),
			raw:                       []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"default"}}`),
			controllerWithoutInformer: true,
			expectedError:             true,
			wrongWorkNS:               true,
		},
		{
			name:                      "skips work with suspend dispatching",
			obj:                       newPodObj("karmada-es-cluster"),
			pod:                       newPod(workNs, workName),
			raw:                       []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"default"}}`),
			controllerWithoutInformer: true,
			expectedError:             false,
			workApplyFunc: func(work *workv1alpha1.Work) {
				work.Spec.SuspendDispatching = ptr.To(true)
			},
		},
		{
			name:                      "skips work with deletion timestamp",
			obj:                       newPodObj("karmada-es-cluster"),
			pod:                       newPod(workNs, workName),
			raw:                       []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"default"}}`),
			controllerWithoutInformer: true,
			expectedError:             false,
			workApplyFunc: func(work *workv1alpha1.Work) {
				work.SetDeletionTimestamp(ptr.To(metav1.Now()))
			},
		},
		{
			name:                      "resource not found, work suspendDispatching true, should not recreate resource",
			obj:                       newPodObj("karmada-es-cluster"),
			pod:                       nil, // Simulate the resource does not exist in the member cluster
			raw:                       []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"default"}}`),
			controllerWithoutInformer: true,
			expectedError:             false,
			workApplyFunc: func(work *workv1alpha1.Work) {
				work.Spec.SuspendDispatching = ptr.To(true)
			},
			assertFunc: func(t *testing.T, dynamicClientSets *dynamicfake.FakeDynamicClient) {
				gvr := corev1.SchemeGroupVersion.WithResource("pods")
				obj, err := dynamicClientSets.Resource(gvr).Namespace("default").Get(context.Background(), "pod", metav1.GetOptions{})
				assert.True(t, apierrors.IsNotFound(err), "expected a NotFound error but got: %s", err)
				assert.Nil(t, obj)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wrongWorkNS {
				workNs = "karmada-cluster"
				tt.pod = newPod(workNs, workName)
			}

			var dynamicClientSet *dynamicfake.FakeDynamicClient
			if tt.pod != nil {
				dynamicClientSet = dynamicfake.NewSimpleDynamicClient(scheme.Scheme, tt.pod)
			} else {
				dynamicClientSet = dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
			}

			var c WorkStatusController
			if tt.controllerWithoutInformer {
				c = newWorkStatusController(cluster, dynamicClientSet)
			} else {
				c = newWorkStatusController(cluster)
			}

			work := testhelper.NewWork(workName, workNs, workUID, tt.raw)
			if tt.workApplyFunc != nil {
				tt.workApplyFunc(work)
			}

			key, _ := generateKey(tt.obj)

			if err := c.Client.Create(context.Background(), work); err != nil {
				t.Fatalf("Failed to create work: %v", err)
			}

			err := c.syncWorkStatus(key)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.assertFunc != nil {
				tt.assertFunc(t, dynamicClientSet)
			}
		})
	}
}

func newWorkStatusController(cluster *clusterv1alpha1.Cluster, dynamicClientSets ...*dynamicfake.FakeDynamicClient) WorkStatusController {
	c := WorkStatusController{
		Client:                      fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(cluster).WithStatusSubresource().Build(),
		InformerManager:             genericmanager.GetInstance(),
		WorkPredicateFunc:           helper.NewClusterPredicateOnAgent("test"),
		ClusterDynamicClientSetFunc: util.NewClusterDynamicClientSetForAgent,
		ClusterCacheSyncTimeout:     metav1.Duration{},
		RateLimiterOptions:          ratelimiterflag.Options{},
		eventHandler:                nil,
		EventRecorder:               record.NewFakeRecorder(1024),
		RESTMapper: func() meta.RESTMapper {
			m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
			m.Add(corev1.SchemeGroupVersion.WithKind("Pod"), meta.RESTScopeNamespace)
			return m
		}(),
		ResourceInterpreter: FakeResourceInterpreter{
			DefaultInterpreter: native.NewDefaultInterpreter(),
		},
	}

	if len(dynamicClientSets) > 0 {
		c.ResourceInterpreter = FakeResourceInterpreter{DefaultInterpreter: native.NewDefaultInterpreter()}
		c.ObjectWatcher = objectwatcher.NewObjectWatcher(c.Client, c.RESTMapper, util.NewClusterDynamicClientSetForAgent, nil, c.ResourceInterpreter)

		// Generate InformerManager
		clusterName := cluster.Name
		dynamicClientSet := dynamicClientSets[0]
		// Generate ResourceInterpreter and ObjectWatcher
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		m := genericmanager.NewMultiClusterInformerManager(ctx)
		m.ForCluster(clusterName, dynamicClientSet, 0).Lister(corev1.SchemeGroupVersion.WithResource("pods")) // register pod informer
		m.Start(clusterName)
		m.WaitForCacheSync(clusterName)
		c.InformerManager = m
	}

	return c
}

func TestWorkStatusController_getSingleClusterManager(t *testing.T) {
	clusterName := "cluster"
	cluster := newCluster(clusterName, clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)

	// Generate InformerManager
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dynamicClientSet := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)

	tests := []struct {
		name                             string
		rightClusterName                 bool
		expectInformer                   bool
		expectError                      bool
		wrongClusterDynamicClientSetFunc bool
	}{
		{
			name:             "normal case",
			rightClusterName: true,
			expectInformer:   true,
			expectError:      false,
		},
		{
			name:                             "failed to build dynamic cluster client",
			rightClusterName:                 false,
			expectInformer:                   false,
			expectError:                      true,
			wrongClusterDynamicClientSetFunc: true,
		},
		{
			name:             "failed to get single cluster",
			rightClusterName: false,
			expectInformer:   true,
			expectError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newWorkStatusController(cluster)
			m := genericmanager.NewMultiClusterInformerManager(ctx)
			if tt.rightClusterName {
				m.ForCluster(clusterName, dynamicClientSet, 0).Lister(corev1.SchemeGroupVersion.WithResource("pods"))
			} else {
				m.ForCluster("test", dynamicClientSet, 0).Lister(corev1.SchemeGroupVersion.WithResource("pods"))
			}
			m.Start(clusterName)
			m.WaitForCacheSync(clusterName)
			c.InformerManager = m

			if tt.wrongClusterDynamicClientSetFunc {
				c.ClusterDynamicClientSetFunc = NewClusterDynamicClientSetForAgentWithError
			} else {
				c.ClusterDynamicClientSetFunc = util.NewClusterDynamicClientSet
				c.Client = fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
						Spec: clusterv1alpha1.ClusterSpec{
							APIEndpoint: "https://127.0.0.1",
							SecretRef:   &clusterv1alpha1.LocalSecretReference{Namespace: "ns1", Name: "secret1"},
						},
						Status: clusterv1alpha1.ClusterStatus{
							Conditions: []metav1.Condition{
								{
									Type:   clusterv1alpha1.ClusterConditionReady,
									Status: metav1.ConditionTrue,
								},
							},
						},
					},
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "secret1"},
						Data:       map[string][]byte{clusterv1alpha1.SecretTokenKey: []byte("token"), clusterv1alpha1.SecretCADataKey: testCA},
					}).Build()
			}

			informerManager, err := c.getSingleClusterManager(cluster)

			if tt.expectInformer {
				assert.NotEmpty(t, informerManager)
			} else {
				assert.Empty(t, informerManager)
			}

			if tt.expectError {
				assert.NotEmpty(t, err)
			} else {
				assert.Empty(t, err)
			}
		})
	}
}

func TestWorkStatusController_recreateResourceIfNeeded(t *testing.T) {
	c := WorkStatusController{
		Client:                      fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(newCluster("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)).Build(),
		InformerManager:             genericmanager.GetInstance(),
		WorkPredicateFunc:           helper.NewClusterPredicateOnAgent("test"),
		ClusterDynamicClientSetFunc: util.NewClusterDynamicClientSetForAgent,
		ClusterCacheSyncTimeout:     metav1.Duration{},
		RateLimiterOptions:          ratelimiterflag.Options{},
	}

	workUID := "92345678-1234-5678-1234-567812345678"
	raw := []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"default"}}`)
	work := testhelper.NewWork("work", "default", workUID, raw)

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "pod1",
				"namespace": "default",
				"annotations": map[string]interface{}{
					workv1alpha2.WorkNamespaceAnnotation: "karmada-es-cluster",
				},
			},
		},
	}

	key, _ := generateKey(obj)

	fedKey, ok := key.(keys.FederatedKey)
	if !ok {
		t.Fatalf("Invalid key, key: %v", key)
	}

	t.Run("normal case", func(t *testing.T) {
		err := c.recreateResourceIfNeeded(context.Background(), work, fedKey)
		assert.Empty(t, err)
	})

	t.Run("failed to UnmarshalJSON", func(t *testing.T) {
		work.Spec.Workload.Manifests[0].RawExtension.Raw = []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"default"}},`)
		err := c.recreateResourceIfNeeded(context.Background(), work, fedKey)
		assert.NotEmpty(t, err)
	})
}

func TestWorkStatusController_buildStatusIdentifier(t *testing.T) {
	c := WorkStatusController{
		Client:                      fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(newCluster("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)).Build(),
		InformerManager:             genericmanager.GetInstance(),
		WorkPredicateFunc:           helper.NewClusterPredicateOnAgent("test"),
		ClusterDynamicClientSetFunc: util.NewClusterDynamicClientSetForAgent,
		ClusterCacheSyncTimeout:     metav1.Duration{},
		RateLimiterOptions:          ratelimiterflag.Options{},
	}

	work := &workv1alpha1.Work{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod",
			Namespace: "default",
		},
		Spec: workv1alpha1.WorkSpec{
			Workload: workv1alpha1.WorkloadTemplate{
				Manifests: []workv1alpha1.Manifest{},
			},
		},
	}
	cluster := &clusterv1alpha1.Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Cluster",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster",
			Namespace: "default",
		},
		Spec:   clusterv1alpha1.ClusterSpec{},
		Status: clusterv1alpha1.ClusterStatus{},
	}
	clusterObj, _ := helper.ToUnstructured(cluster)
	clusterJSON, _ := json.Marshal(clusterObj)

	t.Run("normal case", func(t *testing.T) {
		work.Spec.Workload.Manifests = []workv1alpha1.Manifest{
			{
				RawExtension: runtime.RawExtension{Raw: clusterJSON},
			},
		}
		idf, err := c.buildStatusIdentifier(work, clusterObj)
		assert.NotEmpty(t, idf)
		assert.Empty(t, err)
	})

	t.Run("failed to GetManifestIndex", func(t *testing.T) {
		wrongClusterObj, _ := helper.ToUnstructured(newCluster("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue))
		wrongClusterJSON, _ := json.Marshal(wrongClusterObj)
		work.Spec.Workload.Manifests = []workv1alpha1.Manifest{
			{
				RawExtension: runtime.RawExtension{Raw: wrongClusterJSON},
			},
		}
		idf, err := c.buildStatusIdentifier(work, clusterObj)
		assert.Empty(t, idf)
		assert.NotEmpty(t, err)
	})
}

func TestWorkStatusController_mergeStatus(t *testing.T) {
	c := WorkStatusController{
		Client:                      fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(newCluster("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)).Build(),
		InformerManager:             genericmanager.GetInstance(),
		WorkPredicateFunc:           helper.NewClusterPredicateOnAgent("test"),
		ClusterDynamicClientSetFunc: util.NewClusterDynamicClientSetForAgent,
		ClusterCacheSyncTimeout:     metav1.Duration{},
		RateLimiterOptions:          ratelimiterflag.Options{},
	}

	newStatus := workv1alpha1.ManifestStatus{
		Health: "health",
	}
	actual := c.mergeStatus([]workv1alpha1.ManifestStatus{}, newStatus)
	assert.Equal(t, []workv1alpha1.ManifestStatus{newStatus}, actual)
}

func TestWorkStatusController_registerInformersAndStart(t *testing.T) {
	clusterName := "cluster"
	cluster := newCluster(clusterName, clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)

	// Generate InformerManager
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dynamicClientSet := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	c := newWorkStatusController(cluster)
	opt := util.Options{
		Name:          "opt",
		KeyFunc:       nil,
		ReconcileFunc: nil,
	}
	c.worker = util.NewAsyncWorker(opt)

	workUID := "92345678-1234-5678-1234-567812345678"
	raw := []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"default"}}`)
	work := testhelper.NewWork("work", "default", workUID, raw)

	t.Run("normal case", func(t *testing.T) {
		m := genericmanager.NewMultiClusterInformerManager(ctx)
		m.ForCluster(clusterName, dynamicClientSet, 0).Lister(corev1.SchemeGroupVersion.WithResource("pods")) // register pod informer
		m.Start(clusterName)
		m.WaitForCacheSync(clusterName)
		c.InformerManager = m

		err := c.registerInformersAndStart(cluster, work)
		assert.Empty(t, err)
	})

	t.Run("failed to getSingleClusterManager", func(t *testing.T) {
		c := newWorkStatusController(cluster)
		m := genericmanager.NewMultiClusterInformerManager(ctx)
		m.ForCluster("test", dynamicClientSet, 0).Lister(corev1.SchemeGroupVersion.WithResource("pods")) // register pod informer
		m.Start(clusterName)
		m.WaitForCacheSync(clusterName)
		c.InformerManager = m
		c.ClusterDynamicClientSetFunc = NewClusterDynamicClientSetForAgentWithError

		err := c.registerInformersAndStart(cluster, work)
		assert.NotEmpty(t, err)
	})

	t.Run("failed to getGVRsFromWork", func(t *testing.T) {
		work.Spec.Workload.Manifests[0].RawExtension.Raw = []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"default"}},`)

		m := genericmanager.NewMultiClusterInformerManager(ctx)
		m.ForCluster(clusterName, dynamicClientSet, 0).Lister(corev1.SchemeGroupVersion.WithResource("pods")) // register pod informer
		m.Start(clusterName)
		m.WaitForCacheSync(clusterName)
		c.InformerManager = m

		err := c.registerInformersAndStart(cluster, work)
		assert.NotEmpty(t, err)
	})
}

func TestWorkStatusController_interpretHealth(t *testing.T) {
	tests := []struct {
		name                   string
		clusterObj             client.Object
		expectedResourceHealth workv1alpha1.ResourceHealth
		expectedEventReason    string
	}{
		{
			name:                   "deployment without status is interpreted as unhealthy",
			clusterObj:             testhelper.NewDeployment("foo", "bar"),
			expectedResourceHealth: workv1alpha1.ResourceUnhealthy,
			expectedEventReason:    events.EventReasonInterpretHealthSucceed,
		},
		{
			name:                   "cluster role without status is interpreted as healthy",
			clusterObj:             testhelper.NewClusterRole("foo", []rbacv1.PolicyRule{}),
			expectedResourceHealth: workv1alpha1.ResourceHealthy,
		},
	}

	cluster := newCluster("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)
	c := newWorkStatusController(cluster)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			work := testhelper.NewWork(tt.clusterObj.GetName(), tt.clusterObj.GetNamespace(), string(uuid.NewUUID()), []byte{})
			obj, err := helper.ToUnstructured(tt.clusterObj)
			assert.NoError(t, err)

			resourceHealth := c.interpretHealth(obj, work)
			assert.Equalf(t, tt.expectedResourceHealth, resourceHealth, "expected resource health %v, got %v", tt.expectedResourceHealth, resourceHealth)

			eventRecorder := c.EventRecorder.(*record.FakeRecorder)
			if tt.expectedEventReason == "" {
				assert.Empty(t, eventRecorder.Events, "expected no events to get recorded")
			} else {
				assert.Equal(t, 1, len(eventRecorder.Events))
				e := <-eventRecorder.Events
				assert.Containsf(t, e, tt.expectedEventReason, "expected event reason %v, got %v", tt.expectedEventReason, e)
			}
		})
	}
}
