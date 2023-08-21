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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/helper"
	"github.com/karmada-io/karmada/pkg/util/memberclusterinformer"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

func TestWorkStatusController_Reconcile(t *testing.T) {
	tests := []struct {
		name      string
		c         WorkStatusController
		dFunc     func(clusterName string, client client.Client) (*util.DynamicClusterClient, error)
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
				PredicateFunc:      helper.NewClusterPredicateOnAgent("test"),
				RateLimiterOptions: ratelimiterflag.Options{},
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
			dFunc:     util.NewClusterDynamicClientSet,
			ns:        "karmada-es-cluster",
			expectRes: controllerruntime.Result{},
			existErr:  false,
		},
		{
			name: "work not exists",
			c: WorkStatusController{
				Client:             fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(testhelper.NewClusterWithTypeAndStatus("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)).Build(),
				PredicateFunc:      helper.NewClusterPredicateOnAgent("test"),
				RateLimiterOptions: ratelimiterflag.Options{},
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
			dFunc:     util.NewClusterDynamicClientSetForAgent,
			ns:        "karmada-es-cluster",
			expectRes: controllerruntime.Result{},
			existErr:  false,
		},
		{
			name: "work's DeletionTimestamp isn't zero",
			c: WorkStatusController{
				Client:             fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(testhelper.NewClusterWithTypeAndStatus("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)).Build(),
				PredicateFunc:      helper.NewClusterPredicateOnAgent("test"),
				RateLimiterOptions: ratelimiterflag.Options{},
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
			dFunc:     util.NewClusterDynamicClientSetForAgent,
			ns:        "karmada-es-cluster",
			expectRes: controllerruntime.Result{},
			existErr:  false,
		},
		{
			name: "work's status is not applied",
			c: WorkStatusController{
				Client:             fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(testhelper.NewClusterWithTypeAndStatus("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)).Build(),
				PredicateFunc:      helper.NewClusterPredicateOnAgent("test"),
				RateLimiterOptions: ratelimiterflag.Options{},
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
			dFunc:     util.NewClusterDynamicClientSetForAgent,
			ns:        "karmada-es-cluster",
			expectRes: controllerruntime.Result{},
			existErr:  false,
		},
		{
			name: "failed to get cluster name",
			c: WorkStatusController{
				Client:             fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(testhelper.NewClusterWithTypeAndStatus("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)).Build(),
				PredicateFunc:      helper.NewClusterPredicateOnAgent("test"),
				RateLimiterOptions: ratelimiterflag.Options{},
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
			dFunc:     util.NewClusterDynamicClientSetForAgent,
			ns:        "karmada-cluster",
			expectRes: controllerruntime.Result{Requeue: true},
			existErr:  true,
		},
		{
			name: "failed to get cluster",
			c: WorkStatusController{
				Client:             fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(testhelper.NewClusterWithTypeAndStatus("cluster1", clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)).Build(),
				PredicateFunc:      helper.NewClusterPredicateOnAgent("test"),
				RateLimiterOptions: ratelimiterflag.Options{},
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
			dFunc:     util.NewClusterDynamicClientSetForAgent,
			ns:        "karmada-es-cluster",
			expectRes: controllerruntime.Result{Requeue: true},
			existErr:  true,
		},
		{
			name: "cluster is not ready",
			c: WorkStatusController{
				Client:             fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(testhelper.NewClusterWithTypeAndStatus("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionFalse)).Build(),
				PredicateFunc:      helper.NewClusterPredicateOnAgent("test"),
				RateLimiterOptions: ratelimiterflag.Options{},
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
			dFunc:     util.NewClusterDynamicClientSetForAgent,
			ns:        "karmada-es-cluster",
			expectRes: controllerruntime.Result{Requeue: true},
			existErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			injectMemberClusterInformer(&tt.c, genericmanager.GetInstance(), tt.dFunc)

			req := controllerruntime.Request{
				NamespacedName: types.NamespacedName{
					Name:      "work",
					Namespace: tt.ns,
				},
			}

			if err := tt.c.Create(context.Background(), tt.work); err != nil {
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

func TestWorkStatusController_RunWorkQueue(t *testing.T) {
	c := WorkStatusController{
		Client:             fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(testhelper.NewClusterWithTypeAndStatus("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionFalse)).Build(),
		PredicateFunc:      helper.NewClusterPredicateOnAgent("test"),
		RateLimiterOptions: ratelimiterflag.Options{},
		eventHandler:       nil,
	}

	injectMemberClusterInformer(&c, genericmanager.GetInstance(), util.NewClusterDynamicClientSetForAgent)

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
	cluster := testhelper.NewClusterWithTypeAndStatus("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionFalse)
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
		wrongWorkNS               bool
	}{
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
				work = testhelper.NewWork(workName, workNs, tt.raw)
			} else {
				work = testhelper.NewWork(workName, fmt.Sprintf("%v-test", workNs), tt.raw)
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

func injectMemberClusterInformer(c *WorkStatusController,
	informerManager genericmanager.MultiClusterInformerManager,
	clusterDynamicClientSetFunc func(clusterName string, client client.Client) (*util.DynamicClusterClient, error),
) {
	r := memberclusterinformer.NewMemberClusterInformer(c.Client, newRESTMapper(), informerManager, metav1.Duration{}, clusterDynamicClientSetFunc)
	c.MemberClusterInformer = r
}

func newRESTMapper() meta.RESTMapper {
	m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
	m.Add(corev1.SchemeGroupVersion.WithKind("Pod"), meta.RESTScopeNamespace)
	return m
}

func newWorkStatusController(cluster *clusterv1alpha1.Cluster, dynamicClientSets ...*dynamicfake.FakeDynamicClient) WorkStatusController {
	c := WorkStatusController{
		Client:             fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(cluster).Build(),
		PredicateFunc:      helper.NewClusterPredicateOnAgent("test"),
		RateLimiterOptions: ratelimiterflag.Options{},
		eventHandler:       nil,
	}

	informerManager := genericmanager.GetInstance()

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

		// Generate InformerManager
		m := genericmanager.NewMultiClusterInformerManager(stopCh)
		m.ForCluster(clusterName, dynamicClientSet, 0).Lister(corev1.SchemeGroupVersion.WithResource("pods")) // register pod informer
		m.Start(clusterName)
		m.WaitForCacheSync(clusterName)
		informerManager = m
	}

	injectMemberClusterInformer(&c, informerManager, util.NewClusterDynamicClientSetForAgent)

	return c
}

func TestWorkStatusController_buildStatusIdentifier(t *testing.T) {
	c := WorkStatusController{}

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
		wrongClusterObj, _ := helper.ToUnstructured(testhelper.NewClusterWithTypeAndStatus("cluster", clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue))
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
	c := WorkStatusController{}

	newStatus := workv1alpha1.ManifestStatus{
		Health: "health",
	}
	actual := c.mergeStatus([]workv1alpha1.ManifestStatus{}, newStatus)
	assert.Equal(t, []workv1alpha1.ManifestStatus{newStatus}, actual)
}
