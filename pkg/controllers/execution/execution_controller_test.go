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
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

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

const (
	podNamespace = "default"
	podName      = "test"
	clusterName  = "cluster"
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
				work.Spec.SuspendDispatching = ptr.To(true)
			}),
		},
		{
			name:            "work dispatching is suspended, adds false dispatching condition",
			ns:              "karmada-es-cluster",
			expectRes:       controllerruntime.Result{},
			expectCondition: &metav1.Condition{Type: workv1alpha1.WorkDispatching, Status: metav1.ConditionFalse},
			existErr:        false,

			work: newWork(func(w *workv1alpha1.Work) {
				w.Spec.SuspendDispatching = ptr.To(true)
			}),
		},
		{
			name:               "work dispatching is suspended, adds event message",
			ns:                 "karmada-es-cluster",
			expectRes:          controllerruntime.Result{},
			expectEventMessage: fmt.Sprintf("%s %s %s", corev1.EventTypeNormal, events.EventReasonWorkDispatching, WorkSuspendDispatchingConditionMessage),
			existErr:           false,
			work: newWork(func(w *workv1alpha1.Work) {
				w.Spec.SuspendDispatching = ptr.To(true)
			}),
		},
		{
			name:            "work dispatching is suspended, overwrites existing dispatching condition",
			ns:              "karmada-es-cluster",
			expectRes:       controllerruntime.Result{},
			expectCondition: &metav1.Condition{Type: workv1alpha1.WorkDispatching, Status: metav1.ConditionFalse},
			existErr:        false,
			work: newWork(func(w *workv1alpha1.Work) {
				w.Spec.SuspendDispatching = ptr.To(true)
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
				work.Spec.SuspendDispatching = ptr.To(true)
			}),
		},
		{
			name:           "PreserveResourcesOnDeletion=true, deletion timestamp set, does not delete resource",
			ns:             "karmada-es-cluster",
			expectRes:      controllerruntime.Result{},
			existErr:       false,
			resourceExists: ptr.To(true),
			work: newWork(func(work *workv1alpha1.Work) {
				now := metav1.Now()
				work.SetDeletionTimestamp(&now)
				work.SetFinalizers([]string{util.ExecutionControllerFinalizer})
				work.Spec.PreserveResourcesOnDeletion = ptr.To(true)
			}),
		},
		{
			name:           "PreserveResourcesOnDeletion=false, deletion timestamp set, deletes resource",
			ns:             "karmada-es-cluster",
			expectRes:      controllerruntime.Result{},
			existErr:       false,
			resourceExists: ptr.To(false),
			work: newWork(func(work *workv1alpha1.Work) {
				now := metav1.Now()
				work.SetDeletionTimestamp(&now)
				work.SetFinalizers([]string{util.ExecutionControllerFinalizer})
				work.Spec.PreserveResourcesOnDeletion = ptr.To(false)
			}),
		},
		{
			name:           "PreserveResourcesOnDeletion unset, deletion timestamp set, deletes resource",
			ns:             "karmada-es-cluster",
			expectRes:      controllerruntime.Result{},
			existErr:       false,
			resourceExists: ptr.To(false),
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
	informerManager.ForCluster(cluster.Name, dynamicClientSet, 0).Lister(corev1.SchemeGroupVersion.WithResource("pods"))
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
func (s *stubObjectWatcher) GetVersionRecord(_ string, _ *unstructured.Unstructured) (string, bool) {
	return s.versionRecord, s.recordExists
}

func TestController_mapWorkloadToWork(t *testing.T) {
	tests := []struct {
		name        string
		labels      map[string]string
		annotations map[string]string
		want        []reconcile.Request
	}{
		{
			name:   "non-Karmada managed resource is ignored",
			labels: nil,
			want:   nil,
		},
		{
			name:   "Karmada managed but missing both annotations returns nil",
			labels: map[string]string{util.ManagedByKarmadaLabel: util.ManagedByKarmadaLabelValue},
			want:   nil,
		},
		{
			name:   "Karmada managed but missing name annotation returns nil",
			labels: map[string]string{util.ManagedByKarmadaLabel: util.ManagedByKarmadaLabelValue},
			annotations: map[string]string{
				workv1alpha2.WorkNamespaceAnnotation: "karmada-es-cluster",
			},
			want: nil,
		},
		{
			name:   "Karmada managed but missing namespace annotation returns nil",
			labels: map[string]string{util.ManagedByKarmadaLabel: util.ManagedByKarmadaLabelValue},
			annotations: map[string]string{
				workv1alpha2.WorkNameAnnotation: "work",
			},
			want: nil,
		},
		{
			name:   "both annotations present returns reconcile request",
			labels: map[string]string{util.ManagedByKarmadaLabel: util.ManagedByKarmadaLabelValue},
			annotations: map[string]string{
				workv1alpha2.WorkNamespaceAnnotation: "karmada-es-cluster",
				workv1alpha2.WorkNameAnnotation:      "work",
			},
			want: []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: "karmada-es-cluster", Name: "work"}}},
		},
	}

	c := &Controller{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := testhelper.NewPod(podNamespace, podName)
			pod.SetLabels(tt.labels)
			pod.SetAnnotations(tt.annotations)
			got := c.mapWorkloadToWork(context.Background(), pod)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestController_enqueueWorkload(t *testing.T) {
	t.Run("nil channel drops event without panic", func(_ *testing.T) {
		c := &Controller{}
		c.enqueueWorkload(testhelper.NewPod(podNamespace, podName))
	})

	t.Run("nil runtime.Object is ignored", func(t *testing.T) {
		ch := make(chan event.TypedGenericEvent[client.Object], 10)
		c := &Controller{eventChannel: ch}
		c.enqueueWorkload(nil)
		assert.Equal(t, 0, len(ch))
	})

	t.Run("non-client.Object input is ignored", func(t *testing.T) {
		ch := make(chan event.TypedGenericEvent[client.Object], 10)
		c := &Controller{eventChannel: ch}
		c.enqueueWorkload(&corev1.PodList{})
		assert.Equal(t, 0, len(ch))
	})

	t.Run("client.Object is forwarded to eventChannel", func(t *testing.T) {
		ch := make(chan event.TypedGenericEvent[client.Object], 10)
		c := &Controller{eventChannel: ch}
		pod := testhelper.NewPod(podNamespace, podName)
		c.enqueueWorkload(pod)
		select {
		case ev := <-ch:
			assert.Equal(t, pod.GetName(), ev.Object.GetName())
			assert.Equal(t, pod.GetNamespace(), ev.Object.GetNamespace())
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for enqueued event")
		}
	})
}

func TestController_onUpdate(t *testing.T) {
	// newManagedUnstructuredPod creates an Unstructured Pod pre-configured with
	// labels, annotations, and ResourceVersion required to pass the guard chain.
	newManagedUnstructuredPod := func(rv string, spec, status map[string]any) *unstructured.Unstructured {
		obj := &unstructured.Unstructured{}
		obj.SetAPIVersion("v1")
		obj.SetKind("Pod")
		obj.SetNamespace(podNamespace)
		obj.SetName(podName)
		obj.SetResourceVersion(rv)
		obj.SetLabels(map[string]string{util.ManagedByKarmadaLabel: util.ManagedByKarmadaLabelValue})
		obj.SetAnnotations(map[string]string{
			workv1alpha2.WorkNamespaceAnnotation: "karmada-es-cluster",
			workv1alpha2.WorkNameAnnotation:      "work",
		})
		if spec != nil {
			_ = unstructured.SetNestedMap(obj.Object, spec, "spec")
		}
		if status != nil {
			_ = unstructured.SetNestedMap(obj.Object, status, "status")
		}
		return obj
	}

	guardController := func() *Controller {
		return &Controller{eventChannel: make(chan event.TypedGenericEvent[client.Object], 10)}
	}

	versionedController := func(recordRV string) *Controller {
		return &Controller{
			eventChannel:  make(chan event.TypedGenericEvent[client.Object], 10),
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
			eventChannel:  make(chan event.TypedGenericEvent[client.Object], 10),
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

	t.Run("non-status diff enqueues current object", func(t *testing.T) {
		c := versionedController("v1")
		oldObj := newManagedUnstructuredPod("v2", map[string]any{"nodeName": "n1"}, nil)
		curObj := newManagedUnstructuredPod("v3", map[string]any{"nodeName": "n2"}, nil)
		c.onUpdate(oldObj, curObj)
		select {
		case ev := <-c.eventChannel:
			assert.Equal(t, podName, ev.Object.GetName())
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for enqueued event")
		}
	})
}

func TestController_onDelete(t *testing.T) {
	t.Run("non-runtime.Object old is ignored", func(t *testing.T) {
		ch := make(chan event.TypedGenericEvent[client.Object], 10)
		c := &Controller{eventChannel: ch}
		c.onDelete("not a runtime object")
		assert.Equal(t, 0, len(ch))
	})

	t.Run("plain object is enqueued", func(t *testing.T) {
		ch := make(chan event.TypedGenericEvent[client.Object], 10)
		c := &Controller{eventChannel: ch}
		pod := testhelper.NewPod(podNamespace, podName)
		c.onDelete(pod)
		select {
		case ev := <-ch:
			assert.Equal(t, podName, ev.Object.GetName())
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for enqueued event")
		}
	})

	t.Run("DeletedFinalStateUnknown wrapper is unwrapped", func(t *testing.T) {
		ch := make(chan event.TypedGenericEvent[client.Object], 10)
		c := &Controller{eventChannel: ch}
		pod := testhelper.NewPod(podNamespace, podName)
		c.onDelete(toolscache.DeletedFinalStateUnknown{Key: "k", Obj: pod})
		select {
		case ev := <-ch:
			assert.Equal(t, podName, ev.Object.GetName())
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for enqueued event")
		}
	})

	t.Run("DeletedFinalStateUnknown with nil Obj is dropped", func(t *testing.T) {
		ch := make(chan event.TypedGenericEvent[client.Object], 10)
		c := &Controller{eventChannel: ch}
		c.onDelete(toolscache.DeletedFinalStateUnknown{Key: "k", Obj: nil})
		assert.Equal(t, 0, len(ch))
	})
}
