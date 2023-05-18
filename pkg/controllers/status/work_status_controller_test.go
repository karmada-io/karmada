package status

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/informers"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/objectwatcher"
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
							APIEndpoint:                 "https://127.0.0.1",
							SecretRef:                   &clusterv1alpha1.LocalSecretReference{Namespace: "ns1", Name: "secret1"},
							InsecureSkipTLSVerification: true,
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
						Data:       map[string][]byte{clusterv1alpha1.SecretTokenKey: []byte("token")},
					}).Build(),
				InformerManager:             genericmanager.GetInstance(),
				PredicateFunc:               helper.NewClusterPredicateOnAgent("test"),
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
				PredicateFunc:               helper.NewClusterPredicateOnAgent("test"),
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
				PredicateFunc:               helper.NewClusterPredicateOnAgent("test"),
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
				PredicateFunc:               helper.NewClusterPredicateOnAgent("test"),
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
				PredicateFunc:               helper.NewClusterPredicateOnAgent("test"),
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
			expectRes: controllerruntime.Result{Requeue: true},
			existErr:  true,
		},
		{
			name: "failed to get cluster",
			c: WorkStatusController{
				Client:                      fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(newCluster("cluster1", clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)).Build(),
				InformerManager:             genericmanager.GetInstance(),
				PredicateFunc:               helper.NewClusterPredicateOnAgent("test"),
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
			expectRes: controllerruntime.Result{Requeue: true},
			existErr:  true,
		},
		{
			name: "cluster is not ready",
			c: WorkStatusController{
				Client:                      fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(newCluster("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionFalse)).Build(),
				InformerManager:             genericmanager.GetInstance(),
				PredicateFunc:               helper.NewClusterPredicateOnAgent("test"),
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
			expectRes: controllerruntime.Result{Requeue: true},
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
		PredicateFunc:               helper.NewClusterPredicateOnAgent("test"),
		ClusterDynamicClientSetFunc: util.NewClusterDynamicClientSetForAgent,
		ClusterCacheSyncTimeout:     metav1.Duration{},
		RateLimiterOptions:          ratelimiterflag.Options{},
		eventHandler:                nil,
		worker:                      util.NewAsyncWorker(opt),
	}

	eventHandler := c.getEventHandler()
	assert.NotEmpty(t, eventHandler)
}

func TestWorkStatusController_RunWorkQueue(t *testing.T) {
	c := WorkStatusController{
		Client:                      fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(newCluster("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionFalse)).Build(),
		InformerManager:             genericmanager.GetInstance(),
		PredicateFunc:               helper.NewClusterPredicateOnAgent("test"),
		ClusterDynamicClientSetFunc: util.NewClusterDynamicClientSetForAgent,
		ClusterCacheSyncTimeout:     metav1.Duration{},
		RateLimiterOptions:          ratelimiterflag.Options{},
		eventHandler:                nil,
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
						"labels": map[string]interface{}{
							workv1alpha1.WorkNamespaceLabel: "karmada-es-cluster",
						},
					},
				},
			},
			existRes: true,
			existErr: false,
		},
		{
			name: "getClusterNameFromLabel failed",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "test",
						"namespace": "default",
						"labels": map[string]interface{}{
							workv1alpha1.WorkNamespaceLabel: "karmada-cluster",
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
						"labels": map[string]interface{}{
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

func TestGetClusterNameFromLabel(t *testing.T) {
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
						"labels": map[string]interface{}{
							workv1alpha1.WorkNamespaceLabel: "karmada-es-cluster",
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
						"labels": map[string]interface{}{
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
						"labels": map[string]interface{}{
							workv1alpha1.WorkNamespaceLabel: "karmada-cluster",
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
			actual, err := getClusterNameFromLabel(tt.resource)
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
				"labels": map[string]interface{}{
					workv1alpha1.WorkNamespaceLabel: namespace,
					workv1alpha1.WorkNameLabel:      "work-name",
				},
			},
		},
	}
	return obj
}

func newPod(workNs, workName string, wrongLabel ...bool) *corev1.Pod {
	var pod *corev1.Pod
	if len(wrongLabel) > 0 && wrongLabel[0] == true {
		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod",
				Namespace: "default",
				Labels: map[string]string{
					"test":                     workNs,
					workv1alpha1.WorkNameLabel: workName,
				},
			},
		}
	} else {
		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod",
				Namespace: "default",
				Labels: map[string]string{
					workv1alpha1.WorkNamespaceLabel: workNs,
					workv1alpha1.WorkNameLabel:      workName,
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

	tests := []struct {
		name                      string
		obj                       *unstructured.Unstructured
		pod                       *corev1.Pod
		raw                       []byte
		controllerWithoutInformer bool
		workWithRigntNS           bool
		expectedError             bool
		workWithDeletionTimestamp bool
		wrongWorkNS               bool
	}{
		{
			name:                      "failed to exec NeedUpdate",
			obj:                       newPodObj("karmada-es-cluster"),
			pod:                       newPod(workNs, workName),
			raw:                       []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"default"}}`),
			controllerWithoutInformer: true,
			workWithRigntNS:           true,
			expectedError:             true,
		},
		{
			name:                      "invalid key, wrong WorkNamespaceLabel in obj",
			obj:                       newPodObj("karmada-cluster"),
			pod:                       newPod(workNs, workName),
			raw:                       []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"default"}}`),
			controllerWithoutInformer: true,
			workWithRigntNS:           true,
			expectedError:             true,
		},
		{
			name:                      "failed to GetObjectFromCache, wrong InformerManager in WorkStatusController",
			obj:                       newPodObj("karmada-es-cluster"),
			pod:                       newPod(workNs, workName),
			raw:                       []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"default"}}`),
			controllerWithoutInformer: false,
			workWithRigntNS:           true,
			expectedError:             true,
		},
		{
			name:                      "obj not found in informer, wrong dynamicClientSet without pod",
			obj:                       newPodObj("karmada-es-cluster"),
			raw:                       []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"default"}}`),
			controllerWithoutInformer: true,
			workWithRigntNS:           true,
			expectedError:             false,
		},
		{
			name:                      "workNamespace is zero, set wrong label 'test' in pod",
			obj:                       newPodObj("karmada-es-cluster"),
			pod:                       newPod(workNs, workName, true),
			raw:                       []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"default"}}`),
			controllerWithoutInformer: true,
			workWithRigntNS:           true,
			expectedError:             false,
		},
		{
			name:                      "failed to exec Client.Get, set wrong name in work",
			obj:                       newPodObj("karmada-es-cluster"),
			pod:                       newPod(workNs, workName),
			raw:                       []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"default"}}`),
			controllerWithoutInformer: true,
			workWithRigntNS:           false,
			expectedError:             false,
		},
		{
			name:                      "set DeletionTimestamp in work",
			obj:                       newPodObj("karmada-es-cluster"),
			pod:                       newPod(workNs, workName),
			raw:                       []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"default"}}`),
			controllerWithoutInformer: true,
			workWithRigntNS:           true,
			workWithDeletionTimestamp: true,
			expectedError:             false,
		},
		{
			name:                      "failed to getRawManifest, wrong Manifests in work",
			obj:                       newPodObj("karmada-es-cluster"),
			pod:                       newPod(workNs, workName),
			raw:                       []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod1","namespace":"default"}}`),
			controllerWithoutInformer: true,
			workWithRigntNS:           true,
			expectedError:             true,
		},
		{
			name:                      "failed to exec GetClusterName, wrong workNamespace",
			obj:                       newPodObj("karmada-es-cluster"),
			pod:                       newPod(workNs, workName),
			raw:                       []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"default"}}`),
			controllerWithoutInformer: true,
			workWithRigntNS:           true,
			expectedError:             true,
			wrongWorkNS:               true,
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

			var work *workv1alpha1.Work
			if tt.workWithRigntNS {
				work = newWork(workName, workNs, tt.raw)
			} else {
				work = newWork(workName, fmt.Sprintf("%v-test", workNs), tt.raw)
			}

			if tt.workWithDeletionTimestamp {
				work = newWork(workName, workNs, tt.raw, false)
			}

			key, _ := generateKey(tt.obj)

			if err := c.Client.Create(context.Background(), work); err != nil {
				t.Fatalf("Failed to create work: %v", err)
			}

			err := c.syncWorkStatus(key)
			if tt.expectedError {
				assert.NotEmpty(t, err)
			} else {
				assert.Empty(t, err)
			}
		})
	}
}

func newWorkStatusController(cluster *clusterv1alpha1.Cluster, dynamicClientSets ...*dynamicfake.FakeDynamicClient) WorkStatusController {
	c := WorkStatusController{
		Client:                      fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(cluster).Build(),
		InformerManager:             genericmanager.GetInstance(),
		PredicateFunc:               helper.NewClusterPredicateOnAgent("test"),
		ClusterDynamicClientSetFunc: util.NewClusterDynamicClientSetForAgent,
		ClusterCacheSyncTimeout:     metav1.Duration{},
		RateLimiterOptions:          ratelimiterflag.Options{},
		eventHandler:                nil,
		RESTMapper: func() meta.RESTMapper {
			m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
			m.Add(corev1.SchemeGroupVersion.WithKind("Pod"), meta.RESTScopeNamespace)
			return m
		}(),
	}

	if len(dynamicClientSets) > 0 {
		clusterName := cluster.Name
		dynamicClientSet := dynamicClientSets[0]
		// Generate ResourceInterpreter and ObjectWatcher
		stopCh := make(chan struct{})
		defer close(stopCh)

		controlPlaneInformerManager := genericmanager.NewSingleClusterInformerManager(dynamicClientSet, 0, stopCh)
		controlPlaneKubeClientSet := kubernetesfake.NewSimpleClientset()
		sharedFactory := informers.NewSharedInformerFactory(controlPlaneKubeClientSet, 0)
		serviceLister := sharedFactory.Core().V1().Services().Lister()

		c.ResourceInterpreter = resourceinterpreter.NewResourceInterpreter(controlPlaneInformerManager, serviceLister)
		c.ObjectWatcher = objectwatcher.NewObjectWatcher(c.Client, c.RESTMapper, util.NewClusterDynamicClientSetForAgent, c.ResourceInterpreter)

		// Generate InformerManager
		m := genericmanager.NewMultiClusterInformerManager(stopCh)
		m.ForCluster(clusterName, dynamicClientSet, 0).Lister(corev1.SchemeGroupVersion.WithResource("pods")) // register pod informer
		m.Start(clusterName)
		m.WaitForCacheSync(clusterName)
		c.InformerManager = m
	}

	return c
}

func newWork(workName, workNs string, raw []byte, deletionTimestampIsZero ...bool) *workv1alpha1.Work {
	work := &workv1alpha1.Work{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workName,
			Namespace: workNs,
		},
		Spec: workv1alpha1.WorkSpec{
			Workload: workv1alpha1.WorkloadTemplate{
				Manifests: []workv1alpha1.Manifest{
					{RawExtension: runtime.RawExtension{
						Raw: raw,
					},
					},
				},
			},
		},
	}

	if len(deletionTimestampIsZero) > 0 && !deletionTimestampIsZero[0] {
		work.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	}

	return work
}

func TestWorkStatusController_getSingleClusterManager(t *testing.T) {
	clusterName := "cluster"
	cluster := newCluster(clusterName, clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)

	// Generate InformerManager
	stopCh := make(chan struct{})
	defer close(stopCh)

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
			m := genericmanager.NewMultiClusterInformerManager(stopCh)
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
							APIEndpoint:                 "https://127.0.0.1",
							SecretRef:                   &clusterv1alpha1.LocalSecretReference{Namespace: "ns1", Name: "secret1"},
							InsecureSkipTLSVerification: true,
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
						Data:       map[string][]byte{clusterv1alpha1.SecretTokenKey: []byte("token")},
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
		PredicateFunc:               helper.NewClusterPredicateOnAgent("test"),
		ClusterDynamicClientSetFunc: util.NewClusterDynamicClientSetForAgent,
		ClusterCacheSyncTimeout:     metav1.Duration{},
		RateLimiterOptions:          ratelimiterflag.Options{},
	}

	raw := []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"default"}}`)
	work := newWork("work", "default", raw)

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "pod1",
				"namespace": "default",
				"labels": map[string]interface{}{
					workv1alpha1.WorkNamespaceLabel: "karmada-es-cluster",
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
		err := c.recreateResourceIfNeeded(work, fedKey)
		assert.Empty(t, err)
	})

	t.Run("failed to UnmarshalJSON", func(t *testing.T) {
		work.Spec.Workload.Manifests[0].RawExtension.Raw = []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"default"}},`)
		err := c.recreateResourceIfNeeded(work, fedKey)
		assert.NotEmpty(t, err)
	})
}

func TestWorkStatusController_buildStatusIdentifier(t *testing.T) {
	c := WorkStatusController{
		Client:                      fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(newCluster("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)).Build(),
		InformerManager:             genericmanager.GetInstance(),
		PredicateFunc:               helper.NewClusterPredicateOnAgent("test"),
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
	clusterJson, _ := json.Marshal(clusterObj)

	t.Run("normal case", func(t *testing.T) {
		work.Spec.Workload.Manifests = []workv1alpha1.Manifest{
			{
				RawExtension: runtime.RawExtension{Raw: clusterJson},
			},
		}
		idf, err := c.buildStatusIdentifier(work, clusterObj)
		assert.NotEmpty(t, idf)
		assert.Empty(t, err)
	})

	t.Run("failed to GetManifestIndex", func(t *testing.T) {
		wrongClusterObj, _ := helper.ToUnstructured(newCluster("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue))
		wrongClusterJson, _ := json.Marshal(wrongClusterObj)
		work.Spec.Workload.Manifests = []workv1alpha1.Manifest{
			{
				RawExtension: runtime.RawExtension{Raw: wrongClusterJson},
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
		PredicateFunc:               helper.NewClusterPredicateOnAgent("test"),
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
	stopCh := make(chan struct{})
	defer close(stopCh)
	dynamicClientSet := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	c := newWorkStatusController(cluster)
	opt := util.Options{
		Name:          "opt",
		KeyFunc:       nil,
		ReconcileFunc: nil,
	}
	c.worker = util.NewAsyncWorker(opt)

	raw := []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod","namespace":"default"}}`)
	work := newWork("work", "default", raw)

	t.Run("normal case", func(t *testing.T) {
		m := genericmanager.NewMultiClusterInformerManager(stopCh)
		m.ForCluster(clusterName, dynamicClientSet, 0).Lister(corev1.SchemeGroupVersion.WithResource("pods")) // register pod informer
		m.Start(clusterName)
		m.WaitForCacheSync(clusterName)
		c.InformerManager = m

		err := c.registerInformersAndStart(cluster, work)
		assert.Empty(t, err)
	})

	t.Run("failed to getSingleClusterManager", func(t *testing.T) {
		c := newWorkStatusController(cluster)
		m := genericmanager.NewMultiClusterInformerManager(stopCh)
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

		m := genericmanager.NewMultiClusterInformerManager(stopCh)
		m.ForCluster(clusterName, dynamicClientSet, 0).Lister(corev1.SchemeGroupVersion.WithResource("pods")) // register pod informer
		m.Start(clusterName)
		m.WaitForCacheSync(clusterName)
		c.InformerManager = m

		err := c.registerInformersAndStart(cluster, work)
		assert.NotEmpty(t, err)
	})
}
