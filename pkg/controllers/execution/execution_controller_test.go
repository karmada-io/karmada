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
package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	clienttesting "k8s.io/client-go/testing"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/event"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/events"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/default/native"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/objectwatcher"
	testhelper "github.com/karmada-io/karmada/test/helper"
)

// withGVKInterceptor returns an interceptor that sets the GVK on objects after Get operations.
// This simulates the behavior of real API server clients, which automatically set GVK.
// The fake client doesn't do this by default (since controller-runtime v0.22.0).
func withGVKInterceptor(scheme *runtime.Scheme) interceptor.Funcs {
	return interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if err := c.Get(ctx, key, obj, opts...); err != nil {
				return err
			}
			// Set GVK from scheme if it's empty (mimicking real client behavior)
			if obj.GetObjectKind().GroupVersionKind().Empty() {
				gvks, _, _ := scheme.ObjectKinds(obj)
				if len(gvks) > 0 {
					obj.GetObjectKind().SetGroupVersionKind(gvks[0])
				}
			}
			return nil
		},
	}
}

type FakeResourceInterpreter struct {
	*native.DefaultInterpreter
}

var _ resourceinterpreter.ResourceInterpreter = &FakeResourceInterpreter{}

type blockingListDynamicClient struct {
	dynamic.Interface
	beforeList func(context.Context) error
}

func (c *blockingListDynamicClient) Resource(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return &blockingListNamespaceableResource{
		NamespaceableResourceInterface: c.Interface.Resource(resource),
		beforeList:                     c.beforeList,
	}
}

func (c *blockingListDynamicClient) IsWatchListSemanticsUnSupported() bool {
	return true
}

type blockingListNamespaceableResource struct {
	dynamic.NamespaceableResourceInterface
	beforeList func(context.Context) error
}

func (r *blockingListNamespaceableResource) Namespace(namespace string) dynamic.ResourceInterface {
	return &blockingListResource{
		ResourceInterface: r.NamespaceableResourceInterface.Namespace(namespace),
		beforeList:        r.beforeList,
	}
}

func (r *blockingListNamespaceableResource) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	if err := r.beforeList(ctx); err != nil {
		return nil, err
	}
	return r.NamespaceableResourceInterface.List(ctx, opts)
}

type blockingListResource struct {
	dynamic.ResourceInterface
	beforeList func(context.Context) error
}

func (r *blockingListResource) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	if err := r.beforeList(ctx); err != nil {
		return nil, err
	}
	return r.ResourceInterface.List(ctx, opts)
}

const (
	podNamespace = "default"
	podName      = "test"
	clusterName  = "cluster"
	testWorkNS   = "karmada-es-cluster"
	testWorkName = "work"
)

func TestExecutionController_Reconcile(t *testing.T) {
	tests := []struct {
		name               string
		work               *workv1alpha1.Work
		ns                 string
		expectRes          controllerruntime.Result
		expectCondition    *metav1.Condition
		expectEventMessage string
		existErr           bool
		resourceExists     *bool
	}{
		{
			name:      "work dispatching is suspended, no error, no apply",
			ns:        "karmada-es-cluster",
			expectRes: controllerruntime.Result{},
			existErr:  false,
			work: newWork(func(work *workv1alpha1.Work) {
				work.Spec.SuspendDispatching = new(true)
			}),
		},
		{
			name:            "work dispatching is suspended, adds false dispatching condition",
			ns:              "karmada-es-cluster",
			expectRes:       controllerruntime.Result{},
			expectCondition: &metav1.Condition{Type: workv1alpha1.WorkDispatching, Status: metav1.ConditionFalse},
			existErr:        false,

			work: newWork(func(w *workv1alpha1.Work) {
				w.Spec.SuspendDispatching = new(true)
			}),
		},
		{
			name:               "work dispatching is suspended, adds event message",
			ns:                 "karmada-es-cluster",
			expectRes:          controllerruntime.Result{},
			expectEventMessage: fmt.Sprintf("%s %s %s", corev1.EventTypeNormal, events.EventReasonWorkDispatching, WorkSuspendDispatchingConditionMessage),
			existErr:           false,
			work: newWork(func(w *workv1alpha1.Work) {
				w.Spec.SuspendDispatching = new(true)
			}),
		},
		{
			name:            "work dispatching is suspended, overwrites existing dispatching condition",
			ns:              "karmada-es-cluster",
			expectRes:       controllerruntime.Result{},
			expectCondition: &metav1.Condition{Type: workv1alpha1.WorkDispatching, Status: metav1.ConditionFalse},
			existErr:        false,
			work: newWork(func(w *workv1alpha1.Work) {
				w.Spec.SuspendDispatching = new(true)
				meta.SetStatusCondition(&w.Status.Conditions, metav1.Condition{
					Type:   workv1alpha1.WorkDispatching,
					Status: metav1.ConditionTrue,
					Reason: workDispatchingConditionReason,
				})
			}),
		},
		{
			name:      "suspend work with deletion timestamp is deleted",
			ns:        "karmada-es-cluster",
			expectRes: controllerruntime.Result{},
			existErr:  false,
			work: newWork(func(work *workv1alpha1.Work) {
				now := metav1.Now()
				work.SetDeletionTimestamp(&now)
				work.SetFinalizers([]string{util.ExecutionControllerFinalizer})
				work.Spec.SuspendDispatching = new(true)
			}),
		},
		{
			name:           "PreserveResourcesOnDeletion=true, deletion timestamp set, does not delete resource",
			ns:             "karmada-es-cluster",
			expectRes:      controllerruntime.Result{},
			existErr:       false,
			resourceExists: new(true),
			work: newWork(func(work *workv1alpha1.Work) {
				now := metav1.Now()
				work.SetDeletionTimestamp(&now)
				work.SetFinalizers([]string{util.ExecutionControllerFinalizer})
				work.Spec.PreserveResourcesOnDeletion = new(true)
			}),
		},
		{
			name:           "PreserveResourcesOnDeletion=false, deletion timestamp set, deletes resource",
			ns:             "karmada-es-cluster",
			expectRes:      controllerruntime.Result{},
			existErr:       false,
			resourceExists: new(false),
			work: newWork(func(work *workv1alpha1.Work) {
				now := metav1.Now()
				work.SetDeletionTimestamp(&now)
				work.SetFinalizers([]string{util.ExecutionControllerFinalizer})
				work.Spec.PreserveResourcesOnDeletion = new(false)
			}),
		},
		{
			name:           "PreserveResourcesOnDeletion unset, deletion timestamp set, deletes resource",
			ns:             "karmada-es-cluster",
			expectRes:      controllerruntime.Result{},
			existErr:       false,
			resourceExists: new(false),
			work: newWork(func(work *workv1alpha1.Work) {
				now := metav1.Now()
				work.SetDeletionTimestamp(&now)
				work.SetFinalizers([]string{util.ExecutionControllerFinalizer})
			}),
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

			eventRecorder := record.NewFakeRecorder(1)
			c := newController(tt.work, eventRecorder)
			res, err := c.Reconcile(context.Background(), req)
			assert.Equal(t, tt.expectRes, res)
			if tt.existErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.expectCondition != nil {
				assert.NoError(t, c.Client.Get(context.Background(), req.NamespacedName, tt.work))
				assert.True(t, meta.IsStatusConditionPresentAndEqual(tt.work.Status.Conditions, tt.expectCondition.Type, tt.expectCondition.Status))
			}

			if tt.expectEventMessage != "" {
				assert.Equal(t, 1, len(eventRecorder.Events))
				e := <-eventRecorder.Events
				assert.Equal(t, tt.expectEventMessage, e)
			}

			if tt.resourceExists != nil {
				resourceInterface := c.InformerManager.GetSingleClusterManager(clusterName).GetClient().
					Resource(corev1.SchemeGroupVersion.WithResource("pods")).Namespace(podNamespace)
				_, err = resourceInterface.Get(context.TODO(), podName, metav1.GetOptions{})
				if *tt.resourceExists {
					assert.NoErrorf(t, err, "unable to query pod (%s/%s)", podNamespace, podName)
				} else {
					assert.True(t, apierrors.IsNotFound(err), "pod (%s/%s) was not deleted", podNamespace, podName)
				}
			}
		})
	}
}

func TestExecutionController_NewGVRInformerRequeuesWorkForMemberResourceChangedBeforeSync(t *testing.T) {
	const testTimeout = 5 * time.Second

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// This test covers a Work that introduces a new GVR. Its reconciliation starts an informer
	// but does not wait for the initial LIST to finish; instead, it creates the member resource
	// through the dynamic client and records resource version 1. If that resource is changed to
	// resource version 2 before the initial LIST completes, the informer sees only version 2 and
	// reports it as an Add event. The Add handler must compare it with the recorded version and
	// requeue the Work; otherwise, the change made before the watch starts would be missed.
	workload := newManagedUnstructuredPod("", nil, nil)
	workloadJSON, err := json.Marshal(workload)
	if err != nil {
		t.Fatalf("Failed to marshal workload: %v", err)
	}
	work := testhelper.NewWork(testWorkName, testWorkNS, string(uuid.NewUUID()), workloadJSON)
	cluster := newCluster(clusterName, clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)

	restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
	restMapper.Add(corev1.SchemeGroupVersion.WithKind("Pod"), meta.RESTScopeNamespace)
	clientScheme := gclient.NewSchema()
	controlPlaneClient := fake.NewClientBuilder().
		WithScheme(clientScheme).
		WithObjects(cluster, work).
		WithStatusSubresource(work).
		WithRESTMapper(restMapper).
		WithInterceptorFuncs(withGVKInterceptor(clientScheme)).
		Build()

	fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	listStarted := make(chan struct{})
	releaseList := make(chan struct{})
	var listStartedOnce sync.Once
	var releaseListOnce sync.Once
	releaseInitialList := func() {
		releaseListOnce.Do(func() { close(releaseList) })
	}
	t.Cleanup(releaseInitialList)

	// Block the initial LIST before it reads from the fake client so that propagation and
	// the subsequent member-resource change both happen before the LIST snapshot is taken.
	dynamicClientSet := &blockingListDynamicClient{
		Interface: fakeDynamicClient,
		beforeList: func(listCtx context.Context) error {
			listStartedOnce.Do(func() { close(listStarted) })
			select {
			case <-releaseList:
				return nil
			case <-listCtx.Done():
				return listCtx.Err()
			}
		},
	}
	// Simulate the API server assigning a resource version to the propagated resource.
	fakeDynamicClient.PrependReactor("create", "pods", func(action clienttesting.Action) (bool, runtime.Object, error) {
		created := action.(clienttesting.CreateAction).GetObject().(*unstructured.Unstructured)
		created.SetResourceVersion("1")
		return false, nil, nil
	})

	informerManager := genericmanager.NewMultiClusterInformerManager(ctx)
	clusterClientSetFunc := func(string, client.Client, *util.ClientOption) (*util.DynamicClusterClient, error) {
		return &util.DynamicClusterClient{
			ClusterName:      clusterName,
			DynamicClientSet: dynamicClientSet,
		}, nil
	}
	objectWatcher := objectwatcher.NewObjectWatcher(
		controlPlaneClient,
		restMapper,
		clusterClientSetFunc,
		nil,
		FakeResourceInterpreter{DefaultInterpreter: native.NewDefaultInterpreter()},
		informerManager,
	)
	c := Controller{
		Client:               controlPlaneClient,
		EventRecorder:        record.NewFakeRecorder(10),
		RESTMapper:           restMapper,
		ObjectWatcher:        objectWatcher,
		InformerManager:      informerManager,
		eventChannel:         make(chan event.TypedGenericEvent[client.ObjectKey], 10),
		ClusterClientSetFunc: clusterClientSetFunc,
	}
	workloadGVR := corev1.SchemeGroupVersion.WithResource("pods")
	type reconcileResult struct {
		result controllerruntime.Result
		err    error
	}
	reconcileDone := make(chan reconcileResult, 1)
	go func() {
		result, reconcileErr := c.Reconcile(ctx, controllerruntime.Request{
			NamespacedName: types.NamespacedName{Namespace: testWorkNS, Name: testWorkName},
		})
		reconcileDone <- reconcileResult{result: result, err: reconcileErr}
	}()

	select {
	case <-listStarted:
	case <-time.After(testTimeout):
		t.Fatal("Work reconciliation did not start the new GVR informer's initial LIST")
	}

	// Reconciliation must complete and record version 1 while the initial LIST remains blocked.
	select {
	case got := <-reconcileDone:
		assert.Equal(t, controllerruntime.Result{}, got.result)
		assert.NoError(t, got.err)
	case <-time.After(testTimeout):
		t.Fatal("Work reconciliation blocked while the informer initial LIST was pending")
	}

	resourceClient := dynamicClientSet.Resource(workloadGVR).Namespace(podNamespace)
	memberObject, err := resourceClient.Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get the propagated member-cluster object: %v", err)
	}
	if memberObject.GetResourceVersion() != "1" {
		t.Fatalf("Unexpected resource version after propagation: %q", memberObject.GetResourceVersion())
	}
	recordedVersion, exists := c.ObjectWatcher.GetVersionRecord(clusterName, memberObject)
	if !exists || recordedVersion != "1" {
		t.Fatalf("Unexpected recorded resource version after propagation: %q, exists: %t", recordedVersion, exists)
	}

	modifiedObject := memberObject.DeepCopy()
	modifiedObject.SetResourceVersion("2")
	if err := unstructured.SetNestedField(modifiedObject.Object, "unexpected", "spec", "nodeName"); err != nil {
		t.Fatalf("Failed to modify the member-cluster object: %v", err)
	}
	if _, err := resourceClient.Update(ctx, modifiedObject, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("Failed to update the member-cluster object: %v", err)
	}
	assert.Empty(t, c.eventChannel)

	// Let the initial LIST observe version 2 and deliver the corresponding Add event.
	releaseInitialList()
	assertWorkKeyEnqueued(t, c.eventChannel)
}

func newController(work *workv1alpha1.Work, recorder *record.FakeRecorder) Controller {
	cluster := newCluster(clusterName, clusterv1alpha1.ClusterConditionReady, metav1.ConditionTrue)
	pod := testhelper.NewPod(podNamespace, podName)
	pod.SetLabels(map[string]string{util.ManagedByKarmadaLabel: util.ManagedByKarmadaLabelValue})
	restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
	restMapper.Add(corev1.SchemeGroupVersion.WithKind(pod.Kind), meta.RESTScopeNamespace)
	clientScheme := gclient.NewSchema()
	fakeClient := fake.NewClientBuilder().
		WithScheme(clientScheme).
		WithObjects(cluster, work).
		WithStatusSubresource(work).
		WithRESTMapper(restMapper).
		WithInterceptorFuncs(withGVKInterceptor(clientScheme)).
		Build()
	dynamicClientSet := dynamicfake.NewSimpleDynamicClient(scheme.Scheme, pod)
	informerManager := genericmanager.NewMultiClusterInformerManager(context.Background())
	_, _ = informerManager.ForCluster(cluster.Name, dynamicClientSet, 0).Lister(corev1.SchemeGroupVersion.WithResource("pods"))
	informerManager.Start(cluster.Name)
	informerManager.WaitForCacheSync(cluster.Name)
	clusterClientSetFunc := func(string, client.Client, *util.ClientOption) (*util.DynamicClusterClient, error) {
		return &util.DynamicClusterClient{
			ClusterName:      clusterName,
			DynamicClientSet: dynamicClientSet,
		}, nil
	}
	resourceInterpreter := FakeResourceInterpreter{DefaultInterpreter: native.NewDefaultInterpreter()}
	return Controller{
		Client:          fakeClient,
		InformerManager: informerManager,
		EventRecorder:   recorder,
		RESTMapper:      restMapper,
		ObjectWatcher:   objectwatcher.NewObjectWatcher(fakeClient, restMapper, clusterClientSetFunc, nil, resourceInterpreter, informerManager),
	}
}

func newWork(applyFunc func(work *workv1alpha1.Work)) *workv1alpha1.Work {
	pod := testhelper.NewPod(podNamespace, podName)
	bytes, _ := json.Marshal(pod)
	work := testhelper.NewWork("work", "karmada-es-cluster", string(uuid.NewUUID()), bytes)
	if applyFunc != nil {
		applyFunc(work)
	}
	return work
}

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

func (f FakeResourceInterpreter) Start(context.Context) error {
	return nil
}

func TestController_getEventHandlerIsMemoized(t *testing.T) {
	c := &Controller{}
	first := c.getEventHandler()
	second := c.getEventHandler()
	assert.NotNil(t, first)
	assert.Same(t, first, second, "getEventHandler should return the same handler instance across calls")
}

// stubObjectWatcher implements objectwatcher.ObjectWatcher for testing.
type stubObjectWatcher struct {
	versionRecord string
	recordExists  bool
}

func (s *stubObjectWatcher) Create(_ context.Context, _ string, _ *unstructured.Unstructured) error {
	return nil
}
func (s *stubObjectWatcher) Update(_ context.Context, _ string, _, _ *unstructured.Unstructured) (objectwatcher.OperationResult, error) {
	return objectwatcher.OperationResultNone, nil
}
func (s *stubObjectWatcher) Delete(_ context.Context, _ string, _ *unstructured.Unstructured) error {
	return nil
}
func (s *stubObjectWatcher) GetVersionRecord(_ string, _ client.Object) (string, bool) {
	return s.versionRecord, s.recordExists
}

func TestWorkKeyFromWorkload(t *testing.T) {
	tests := []struct {
		name        string
		labels      map[string]string
		annotations map[string]string
		want        client.ObjectKey
		wantOK      bool
	}{
		{
			name:   "non-Karmada managed resource is ignored",
			labels: nil,
		},
		{
			name:   "Karmada managed but missing both annotations is ignored",
			labels: map[string]string{util.ManagedByKarmadaLabel: util.ManagedByKarmadaLabelValue},
		},
		{
			name:   "Karmada managed but missing name annotation is ignored",
			labels: map[string]string{util.ManagedByKarmadaLabel: util.ManagedByKarmadaLabelValue},
			annotations: map[string]string{
				workv1alpha2.WorkNamespaceAnnotation: testWorkNS,
			},
		},
		{
			name:   "Karmada managed but missing namespace annotation is ignored",
			labels: map[string]string{util.ManagedByKarmadaLabel: util.ManagedByKarmadaLabelValue},
			annotations: map[string]string{
				workv1alpha2.WorkNameAnnotation: testWorkName,
			},
		},
		{
			name:   "empty Work reference is ignored",
			labels: map[string]string{util.ManagedByKarmadaLabel: util.ManagedByKarmadaLabelValue},
			annotations: map[string]string{
				workv1alpha2.WorkNamespaceAnnotation: testWorkNS,
				workv1alpha2.WorkNameAnnotation:      "",
			},
		},
		{
			name:   "both annotations present returns Work key",
			labels: map[string]string{util.ManagedByKarmadaLabel: util.ManagedByKarmadaLabelValue},
			annotations: map[string]string{
				workv1alpha2.WorkNamespaceAnnotation: testWorkNS,
				workv1alpha2.WorkNameAnnotation:      testWorkName,
			},
			want:   client.ObjectKey{Namespace: testWorkNS, Name: testWorkName},
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := testhelper.NewPod(podNamespace, podName)
			pod.SetLabels(tt.labels)
			pod.SetAnnotations(tt.annotations)
			got, ok := workKeyFromWorkload(pod)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantOK, ok)
		})
	}
}

func TestController_enqueueWork(t *testing.T) {
	workKey := client.ObjectKey{Namespace: testWorkNS, Name: testWorkName}

	t.Run("nil channel drops event without panic", func(_ *testing.T) {
		c := &Controller{}
		c.enqueueWork(workKey)
	})

	t.Run("Work key is forwarded to eventChannel", func(t *testing.T) {
		ch := make(chan event.TypedGenericEvent[client.ObjectKey], 10)
		c := &Controller{eventChannel: ch}
		c.enqueueWork(workKey)
		select {
		case ev := <-ch:
			assert.Equal(t, workKey, ev.Object)
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for enqueued event")
		}
	})
}

func assertWorkKeyEnqueued(t *testing.T, eventChannel <-chan event.TypedGenericEvent[client.ObjectKey]) {
	t.Helper()
	select {
	case ev := <-eventChannel:
		assert.Equal(t, client.ObjectKey{Namespace: testWorkNS, Name: testWorkName}, ev.Object)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for enqueued Work key")
	}
}

// newManagedUnstructuredPod creates an Unstructured Pod pre-configured with
// labels, annotations, and ResourceVersion required to pass the event-handler guards.
func newManagedUnstructuredPod(rv string, spec, status map[string]any) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("Pod")
	obj.SetNamespace(podNamespace)
	obj.SetName(podName)
	obj.SetResourceVersion(rv)
	obj.SetLabels(map[string]string{util.ManagedByKarmadaLabel: util.ManagedByKarmadaLabelValue})
	obj.SetAnnotations(map[string]string{
		workv1alpha2.WorkNamespaceAnnotation: testWorkNS,
		workv1alpha2.WorkNameAnnotation:      testWorkName,
	})
	if spec != nil {
		_ = unstructured.SetNestedMap(obj.Object, spec, "spec")
	}
	if status != nil {
		_ = unstructured.SetNestedMap(obj.Object, status, "status")
	}
	return obj
}

func TestController_onAdd(t *testing.T) {
	t.Run("regular add with changed version enqueues Work", func(t *testing.T) {
		c := &Controller{
			eventChannel:  make(chan event.TypedGenericEvent[client.ObjectKey], 10),
			ObjectWatcher: &stubObjectWatcher{versionRecord: "v1", recordExists: true},
		}
		c.onAdd(newManagedUnstructuredPod("v2", nil, nil), false)
		assertWorkKeyEnqueued(t, c.eventChannel)
	})

	t.Run("non-Unstructured object is ignored", func(t *testing.T) {
		c := &Controller{eventChannel: make(chan event.TypedGenericEvent[client.ObjectKey], 10)}
		c.onAdd(testhelper.NewPod(podNamespace, podName), false)
		assert.Empty(t, c.eventChannel)
	})

	t.Run("resource without version record is ignored", func(t *testing.T) {
		c := &Controller{
			eventChannel:  make(chan event.TypedGenericEvent[client.ObjectKey], 10),
			ObjectWatcher: &stubObjectWatcher{recordExists: false},
		}
		c.onAdd(newManagedUnstructuredPod("v2", nil, nil), false)
		assert.Empty(t, c.eventChannel)
	})

	t.Run("resource matching version record is ignored", func(t *testing.T) {
		c := &Controller{
			eventChannel:  make(chan event.TypedGenericEvent[client.ObjectKey], 10),
			ObjectWatcher: &stubObjectWatcher{versionRecord: "v2", recordExists: true},
		}
		c.onAdd(newManagedUnstructuredPod("v2", nil, nil), false)
		assert.Empty(t, c.eventChannel)
	})

	t.Run("initial-list resource with changed version enqueues Work", func(t *testing.T) {
		c := &Controller{
			eventChannel:  make(chan event.TypedGenericEvent[client.ObjectKey], 10),
			ObjectWatcher: &stubObjectWatcher{versionRecord: "v1", recordExists: true},
		}
		c.getEventHandler().OnAdd(newManagedUnstructuredPod("v2", nil, nil), true)
		assertWorkKeyEnqueued(t, c.eventChannel)
	})

	t.Run("filter transition add with changed version enqueues Work", func(t *testing.T) {
		c := &Controller{
			eventChannel:  make(chan event.TypedGenericEvent[client.ObjectKey], 10),
			ObjectWatcher: &stubObjectWatcher{versionRecord: "v1", recordExists: true},
		}
		oldObj := newManagedUnstructuredPod("v1", nil, nil)
		newObj := oldObj.DeepCopy()
		newObj.SetResourceVersion("v2")
		labels := newObj.GetLabels()
		labels["selected"] = "true"
		newObj.SetLabels(labels)
		filteringHandler := toolscache.FilteringResourceEventHandler{
			FilterFunc: func(obj any) bool {
				return obj.(*unstructured.Unstructured).GetLabels()["selected"] == "true"
			},
			Handler: c.getEventHandler(),
		}

		filteringHandler.OnUpdate(oldObj, newObj)
		assertWorkKeyEnqueued(t, c.eventChannel)
	})
}

func TestController_onUpdate(t *testing.T) {
	guardController := func() *Controller {
		return &Controller{eventChannel: make(chan event.TypedGenericEvent[client.ObjectKey], 10)}
	}

	versionedController := func(recordRV string) *Controller {
		return &Controller{
			eventChannel:  make(chan event.TypedGenericEvent[client.ObjectKey], 10),
			ObjectWatcher: &stubObjectWatcher{versionRecord: recordRV, recordExists: true},
		}
	}

	t.Run("non-Unstructured old is ignored", func(t *testing.T) {
		c := guardController()
		c.onUpdate(testhelper.NewPod(podNamespace, podName), nil)
		assert.Equal(t, 0, len(c.eventChannel))
	})

	t.Run("non-Karmada resource is skipped", func(t *testing.T) {
		c := guardController()
		obj := newManagedUnstructuredPod("v1", nil, nil)
		obj.SetLabels(nil)
		c.onUpdate(obj, obj)
		assert.Equal(t, 0, len(c.eventChannel))
	})

	t.Run("resource missing work annotations is skipped", func(t *testing.T) {
		c := guardController()
		obj := newManagedUnstructuredPod("v1", nil, nil)
		obj.SetAnnotations(nil)
		c.onUpdate(obj, obj)
		assert.Equal(t, 0, len(c.eventChannel))
	})

	t.Run("non-Unstructured cur is ignored", func(t *testing.T) {
		c := guardController()
		oldObj := newManagedUnstructuredPod("v1", nil, nil)
		c.onUpdate(oldObj, testhelper.NewPod(podNamespace, podName))
		assert.Equal(t, 0, len(c.eventChannel))
	})

	t.Run("resource without version record is skipped", func(t *testing.T) {
		c := &Controller{
			eventChannel:  make(chan event.TypedGenericEvent[client.ObjectKey], 10),
			ObjectWatcher: &stubObjectWatcher{recordExists: false},
		}
		oldObj := newManagedUnstructuredPod("v1", nil, nil)
		curObj := newManagedUnstructuredPod("v2", map[string]any{"nodeName": "n1"}, nil)
		c.onUpdate(oldObj, curObj)
		assert.Equal(t, 0, len(c.eventChannel))
	})

	t.Run("matching resource version is skipped", func(t *testing.T) {
		c := versionedController("v1")
		oldObj := newManagedUnstructuredPod("v1", nil, nil)
		curObj := newManagedUnstructuredPod("v1", map[string]any{"nodeName": "n2"}, nil)
		c.onUpdate(oldObj, curObj)
		assert.Equal(t, 0, len(c.eventChannel))
	})

	t.Run("identical old and cur is skipped", func(t *testing.T) {
		c := versionedController("v1")
		oldObj := newManagedUnstructuredPod("v2", map[string]any{"nodeName": "n1"}, nil)
		curObj := newManagedUnstructuredPod("v2", map[string]any{"nodeName": "n1"}, nil)
		c.onUpdate(oldObj, curObj)
		assert.Equal(t, 0, len(c.eventChannel))
	})

	t.Run("status-only diff is ignored", func(t *testing.T) {
		c := versionedController("v1")
		oldObj := newManagedUnstructuredPod("v2", map[string]any{"nodeName": "n1"}, map[string]any{"phase": "Pending"})
		curObj := newManagedUnstructuredPod("v3", map[string]any{"nodeName": "n1"}, map[string]any{"phase": "Running"})
		c.onUpdate(oldObj, curObj)
		assert.Equal(t, 0, len(c.eventChannel))
	})

	t.Run("non-status diff enqueues Work key from old object", func(t *testing.T) {
		c := versionedController("v1")
		oldObj := newManagedUnstructuredPod("v2", map[string]any{"nodeName": "n1"}, nil)
		curObj := newManagedUnstructuredPod("v3", map[string]any{"nodeName": "n2"}, nil)
		c.onUpdate(oldObj, curObj)
		assertWorkKeyEnqueued(t, c.eventChannel)
	})

	t.Run("removed managed label still enqueues Work key from old object", func(t *testing.T) {
		c := versionedController("v1")
		oldObj := newManagedUnstructuredPod("v2", nil, nil)
		curObj := newManagedUnstructuredPod("v3", nil, nil)
		curObj.SetLabels(nil)
		c.onUpdate(oldObj, curObj)
		assertWorkKeyEnqueued(t, c.eventChannel)
	})

	t.Run("removed Work annotations still enqueues Work key from old object", func(t *testing.T) {
		c := versionedController("v1")
		oldObj := newManagedUnstructuredPod("v2", nil, nil)
		curObj := newManagedUnstructuredPod("v3", nil, nil)
		curObj.SetAnnotations(nil)
		c.onUpdate(oldObj, curObj)
		assertWorkKeyEnqueued(t, c.eventChannel)
	})

	t.Run("modified Work annotations still enqueues Work key from old object", func(t *testing.T) {
		c := versionedController("v1")
		oldObj := newManagedUnstructuredPod("v2", nil, nil)
		curObj := newManagedUnstructuredPod("v3", nil, nil)
		curObj.SetAnnotations(map[string]string{
			workv1alpha2.WorkNamespaceAnnotation: "karmada-es-another-cluster",
			workv1alpha2.WorkNameAnnotation:      "another-work",
		})
		c.onUpdate(oldObj, curObj)
		assertWorkKeyEnqueued(t, c.eventChannel)
	})
}

func TestController_onDelete(t *testing.T) {
	t.Run("non-client.Object old is ignored", func(t *testing.T) {
		ch := make(chan event.TypedGenericEvent[client.ObjectKey], 10)
		c := &Controller{eventChannel: ch}
		c.onDelete("not a client object")
		assert.Equal(t, 0, len(ch))
	})

	t.Run("plain object enqueues associated Work key", func(t *testing.T) {
		ch := make(chan event.TypedGenericEvent[client.ObjectKey], 10)
		c := &Controller{eventChannel: ch}
		pod := testhelper.NewPod(podNamespace, podName)
		pod.SetLabels(map[string]string{util.ManagedByKarmadaLabel: util.ManagedByKarmadaLabelValue})
		pod.SetAnnotations(map[string]string{
			workv1alpha2.WorkNamespaceAnnotation: testWorkNS,
			workv1alpha2.WorkNameAnnotation:      testWorkName,
		})
		c.onDelete(pod)
		assertWorkKeyEnqueued(t, ch)
	})

	t.Run("DeletedFinalStateUnknown wrapper is unwrapped", func(t *testing.T) {
		ch := make(chan event.TypedGenericEvent[client.ObjectKey], 10)
		c := &Controller{eventChannel: ch}
		pod := testhelper.NewPod(podNamespace, podName)
		pod.SetLabels(map[string]string{util.ManagedByKarmadaLabel: util.ManagedByKarmadaLabelValue})
		pod.SetAnnotations(map[string]string{
			workv1alpha2.WorkNamespaceAnnotation: testWorkNS,
			workv1alpha2.WorkNameAnnotation:      testWorkName,
		})
		c.onDelete(toolscache.DeletedFinalStateUnknown{Key: "k", Obj: pod})
		assertWorkKeyEnqueued(t, ch)
	})

	t.Run("DeletedFinalStateUnknown with nil Obj is dropped", func(t *testing.T) {
		ch := make(chan event.TypedGenericEvent[client.ObjectKey], 10)
		c := &Controller{eventChannel: ch}
		c.onDelete(toolscache.DeletedFinalStateUnknown{Key: "k", Obj: nil})
		assert.Equal(t, 0, len(ch))
	})
}
