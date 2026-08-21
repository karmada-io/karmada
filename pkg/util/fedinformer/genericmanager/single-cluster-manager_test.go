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

package genericmanager

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

var testResourceGVR = schema.GroupVersionResource{
	Group:    "example.io",
	Version:  "v1",
	Resource: "widgets",
}

func TestSingleClusterInformerManagerAppliesTransform(t *testing.T) {
	tests := []struct {
		name          string
		observeObject func(t *testing.T, manager SingleClusterInformerManager) *unstructured.Unstructured
	}{
		{
			name: "ForResource",
			observeObject: func(t *testing.T, manager SingleClusterInformerManager) *unstructured.Unstructured {
				observed := make(chan any, 1)
				manager.ForResource(testResourceGVR, cache.ResourceEventHandlerFuncs{
					AddFunc: func(obj any) {
						observed <- obj
					},
				})
				manager.Start()
				waitForTestInformerSync(t, manager)

				select {
				case obj := <-observed:
					resource, ok := obj.(*unstructured.Unstructured)
					if !ok {
						t.Fatalf("got object type %T, want *unstructured.Unstructured", obj)
					}
					return resource
				case <-time.After(5 * time.Second):
					t.Fatal("timed out waiting for informer add event")
					return nil
				}
			},
		},
		{
			name: "Lister",
			observeObject: func(t *testing.T, manager SingleClusterInformerManager) *unstructured.Unstructured {
				lister := manager.Lister(testResourceGVR)
				if lister == nil {
					t.Fatal("expected a lister")
				}
				manager.Start()
				waitForTestInformerSync(t, manager)

				obj, err := lister.ByNamespace("default").Get("widget")
				if err != nil {
					t.Fatalf("failed to get object from informer cache: %v", err)
				}
				resource, ok := obj.(*unstructured.Unstructured)
				if !ok {
					t.Fatalf("got object type %T, want *unstructured.Unstructured", obj)
				}
				return resource
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
				runtime.NewScheme(),
				map[schema.GroupVersionResource]string{testResourceGVR: "WidgetList"},
				newTransformTestObject(),
			)
			manager := NewSingleClusterInformerManager(t.Context(), client, 0, stripManagedFields)
			t.Cleanup(manager.Stop)

			assertTransformedTestObject(t, tt.observeObject(t, manager))
		})
	}
}

func TestSingleClusterInformerManagerSlowPathRechecksInitializedInformer(t *testing.T) {
	cachedInformer := newRecordingGenericInformer()
	factoryInformer := newRecordingGenericInformer()
	factory := &recordingDynamicInformerFactory{informer: factoryInformer}
	manager := newTestSingleClusterInformerManager(t, factory, stripManagedFields)
	manager.initializedInformers.Store(testResourceGVR, cachedInformer)

	got, err := manager.informerForResourceSlowPath(testResourceGVR)
	if err != nil {
		t.Fatalf("informerForResourceSlowPath() returned an unexpected error: %v", err)
	}
	if got != cachedInformer {
		t.Fatal("slow path did not reuse the informer initialized by another caller")
	}
	if got := factory.forResourceCalls.Load(); got != 0 {
		t.Fatalf("factory.ForResource() called %d times, want 0", got)
	}
	if got := factoryInformer.informer.setTransformCalls.Load(); got != 0 {
		t.Fatalf("SetTransform() called %d times, want 0", got)
	}
}

func TestSingleClusterInformerManagerCachedLookupSkipsLifecycleLock(t *testing.T) {
	genericInformer := newRecordingGenericInformer()
	factory := &recordingDynamicInformerFactory{informer: genericInformer}
	manager := newTestSingleClusterInformerManager(t, factory, stripManagedFields)

	if got := manager.Lister(testResourceGVR); got != genericInformer.lister {
		t.Fatal("Lister() returned a different lister during initialization")
	}

	manager.informerLifecycleLock.Lock()
	lockHeld := true
	defer func() {
		if lockHeld {
			manager.informerLifecycleLock.Unlock()
		}
	}()

	lookupResult := make(chan cache.GenericLister, 1)
	go func() {
		lookupResult <- manager.Lister(testResourceGVR)
	}()

	select {
	case got := <-lookupResult:
		manager.informerLifecycleLock.Unlock()
		lockHeld = false
		if got != genericInformer.lister {
			t.Fatal("cached lookup returned a different lister")
		}
	case <-time.After(time.Second):
		manager.informerLifecycleLock.Unlock()
		lockHeld = false
		waitForTestValue(t, lookupResult, "cached lookup after releasing informerLifecycleLock")
		t.Fatal("cached lookup blocked on informerLifecycleLock")
	}

	if got := factory.forResourceCalls.Load(); got != 1 {
		t.Fatalf("factory.ForResource() called %d times, want 1", got)
	}
	if got := genericInformer.informer.setTransformCalls.Load(); got != 1 {
		t.Fatalf("SetTransform() called %d times, want 1", got)
	}
}

func TestSingleClusterInformerManagerSerializesInitializationWithStart(t *testing.T) {
	genericInformer := newRecordingGenericInformer()
	factory := &recordingDynamicInformerFactory{informer: genericInformer}
	manager := newTestSingleClusterInformerManager(t, factory, stripManagedFields)

	factoryEntered := make(chan struct{})
	releaseFactory := make(chan struct{})
	var releaseFactoryOnce sync.Once
	release := func() {
		releaseFactoryOnce.Do(func() { close(releaseFactory) })
	}
	t.Cleanup(release)

	factory.onForResource = func() {
		assertMutexHeld(t, &manager.informerLifecycleLock, "factory.ForResource")
		close(factoryEntered)
		<-releaseFactory
	}
	genericInformer.onInformer = func() {
		assertMutexHeld(t, &manager.informerLifecycleLock, "GenericInformer.Informer")
	}
	var transformInstalled atomic.Bool
	genericInformer.informer.onSetTransform = func() {
		assertMutexHeld(t, &manager.informerLifecycleLock, "SetTransform")
		transformInstalled.Store(true)
	}
	var startBeforeTransform atomic.Bool
	var startBeforeInformerPublished atomic.Bool
	factory.onStart = func() {
		if !transformInstalled.Load() {
			startBeforeTransform.Store(true)
		}
		if _, exists := manager.initializedInformers.Load(testResourceGVR); !exists {
			startBeforeInformerPublished.Store(true)
		}
	}

	initializationDone := make(chan error, 1)
	go func() {
		_, err := manager.informerForResource(testResourceGVR)
		initializationDone <- err
	}()
	waitForTestSignal(t, factoryEntered, "factory.ForResource")

	startAttempted := make(chan struct{})
	startDone := make(chan struct{})
	go func() {
		close(startAttempted)
		manager.Start()
		close(startDone)
	}()
	waitForTestSignal(t, startAttempted, "Start attempt")

	startReturnedBeforeInitialization := false
	select {
	case <-startDone:
		startReturnedBeforeInitialization = true
	case <-time.After(50 * time.Millisecond):
	}

	release()
	if err := waitForTestValue(t, initializationDone, "informer initialization"); err != nil {
		t.Fatalf("informerForResource() returned an unexpected error: %v", err)
	}
	if !startReturnedBeforeInitialization {
		waitForTestSignal(t, startDone, "Start")
	}

	if startReturnedBeforeInitialization {
		t.Fatal("Start returned before informer initialization completed")
	}
	if startBeforeTransform.Load() {
		t.Fatal("factory.Start() ran before SetTransform() completed")
	}
	if startBeforeInformerPublished.Load() {
		t.Fatal("factory.Start() ran before the initialized informer was published")
	}
	if got := factory.startCalls.Load(); got != 1 {
		t.Fatalf("factory.Start() called %d times, want 1", got)
	}
}

func TestSingleClusterInformerManagerStartUsesLifecycleLock(t *testing.T) {
	factory := &recordingDynamicInformerFactory{informer: newRecordingGenericInformer()}
	manager := newTestSingleClusterInformerManager(t, factory, stripManagedFields)
	factory.onStart = func() {
		assertMutexHeld(t, &manager.informerLifecycleLock, "factory.Start")
	}

	manager.Start()

	if got := factory.startCalls.Load(); got != 1 {
		t.Fatalf("factory.Start() called %d times, want 1", got)
	}
}

func TestSingleClusterInformerManagerPublishesInformerAfterTransform(t *testing.T) {
	genericInformer := newRecordingGenericInformer()
	factory := &recordingDynamicInformerFactory{informer: genericInformer}
	manager := newTestSingleClusterInformerManager(t, factory, stripManagedFields)

	transformEntered := make(chan struct{})
	releaseTransform := make(chan struct{})
	var releaseTransformOnce sync.Once
	release := func() {
		releaseTransformOnce.Do(func() { close(releaseTransform) })
	}
	t.Cleanup(release)
	genericInformer.informer.onSetTransform = func() {
		close(transformEntered)
		<-releaseTransform
	}

	type initializationResult struct {
		informer informers.GenericInformer
		err      error
	}
	initializationDone := make(chan initializationResult, 1)
	go func() {
		informer, err := manager.informerForResource(testResourceGVR)
		initializationDone <- initializationResult{informer: informer, err: err}
	}()
	waitForTestSignal(t, transformEntered, "SetTransform")

	_, publishedBeforeTransform := manager.initializedInformers.Load(testResourceGVR)
	release()
	result := waitForTestValue(t, initializationDone, "informer initialization")
	if result.err != nil {
		t.Fatalf("informerForResource() returned an unexpected error: %v", result.err)
	}
	if publishedBeforeTransform {
		t.Fatal("informer was published before SetTransform() completed")
	}
	if result.informer != genericInformer {
		t.Fatal("informerForResource() returned a different GenericInformer")
	}
	initializedInformer, exists := manager.initializedInformers.Load(testResourceGVR)
	if !exists {
		t.Fatal("informer was not published after SetTransform() completed")
	}
	if initializedInformer != genericInformer {
		t.Fatal("initializedInformers contains a different GenericInformer")
	}
}

func TestSingleClusterInformerManagerRetriesFailedTransformWithoutPublishing(t *testing.T) {
	wantErr := errors.New("failed to install transform")
	genericInformer := newRecordingGenericInformer()
	genericInformer.informer.setTransformErr = wantErr
	factory := &recordingDynamicInformerFactory{informer: genericInformer}
	manager := newTestSingleClusterInformerManager(t, factory, stripManagedFields)

	resourceInformer, err := manager.informerForResource(testResourceGVR)
	if !errors.Is(err, wantErr) {
		t.Fatalf("informerForResource() error = %v, want %v", err, wantErr)
	}
	if resourceInformer != genericInformer {
		t.Fatal("failed transform should still return the underlying GenericInformer")
	}
	if _, exists := manager.initializedInformers.Load(testResourceGVR); exists {
		t.Fatal("failed transform must not publish an initialized informer")
	}

	if got := manager.Lister(testResourceGVR); got != genericInformer.lister {
		t.Fatal("Lister() should preserve access to the underlying informer after a transform failure")
	}
	handler := &cache.ResourceEventHandlerFuncs{}
	manager.ForResource(testResourceGVR, handler)
	if !manager.IsHandlerExist(testResourceGVR, handler) {
		t.Fatal("ForResource() should still register the handler after a transform failure")
	}
	if got := factory.forResourceCalls.Load(); got != 3 {
		t.Fatalf("factory.ForResource() called %d times, want 3", got)
	}
	if got := genericInformer.informer.setTransformCalls.Load(); got != 3 {
		t.Fatalf("SetTransform() called %d times, want 3", got)
	}
	if got := genericInformer.informer.addEventHandlerCalls.Load(); got != 1 {
		t.Fatalf("AddEventHandler() called %d times, want 1", got)
	}
	if _, exists := manager.initializedInformers.Load(testResourceGVR); exists {
		t.Fatal("failed transform must not publish an initialized informer after retries")
	}
}

func newTransformTestObject() *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "example.io/v1",
		"kind":       "Widget",
		"metadata": map[string]any{
			"namespace": "default",
			"name":      "widget",
		},
		"spec":   map[string]any{"value": "keep-spec"},
		"status": map[string]any{"value": "keep-status"},
	}}
	obj.SetManagedFields([]metav1.ManagedFieldsEntry{{
		Manager:    "test-manager",
		Operation:  metav1.ManagedFieldsOperationUpdate,
		APIVersion: "example.io/v1",
		FieldsType: "FieldsV1",
	}})
	return obj
}

func stripManagedFields(obj any) (any, error) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}
	accessor.SetManagedFields(nil)
	return obj, nil
}

func assertTransformedTestObject(t *testing.T, obj *unstructured.Unstructured) {
	t.Helper()
	if len(obj.GetManagedFields()) != 0 {
		t.Fatalf("managedFields were not stripped: %v", obj.GetManagedFields())
	}
	if got, _, _ := unstructured.NestedString(obj.Object, "spec", "value"); got != "keep-spec" {
		t.Fatalf("spec value = %q, want %q", got, "keep-spec")
	}
	if got, _, _ := unstructured.NestedString(obj.Object, "status", "value"); got != "keep-status" {
		t.Fatalf("status value = %q, want %q", got, "keep-status")
	}
}

func waitForTestInformerSync(t *testing.T, manager SingleClusterInformerManager) {
	t.Helper()
	if synced := manager.WaitForCacheSyncWithTimeout(5 * time.Second); !synced[testResourceGVR] {
		t.Fatalf("informer failed to sync: %v", synced)
	}
}

func newTestSingleClusterInformerManager(
	t *testing.T,
	factory dynamicinformer.DynamicSharedInformerFactory,
	transform cache.TransformFunc,
) *singleClusterInformerManagerImpl {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	return &singleClusterInformerManagerImpl{
		ctx:             ctx,
		cancel:          cancel,
		informerFactory: factory,
		syncedInformers: make(map[schema.GroupVersionResource]struct{}),
		handlers:        make(map[schema.GroupVersionResource][]cache.ResourceEventHandler),
		transformFunc:   transform,
	}
}

func assertMutexHeld(t *testing.T, mutex *sync.Mutex, operation string) {
	t.Helper()
	if mutex.TryLock() {
		mutex.Unlock()
		t.Errorf("informerLifecycleLock is not held during %s", operation)
	}
}

type recordingDynamicInformerFactory struct {
	informer informers.GenericInformer

	forResourceCalls atomic.Int64
	startCalls       atomic.Int64
	onForResource    func()
	onStart          func()
}

func (f *recordingDynamicInformerFactory) ForResource(_ schema.GroupVersionResource) informers.GenericInformer {
	f.forResourceCalls.Add(1)
	if f.onForResource != nil {
		f.onForResource()
	}
	return f.informer
}

func (f *recordingDynamicInformerFactory) Start(_ <-chan struct{}) {
	f.startCalls.Add(1)
	if f.onStart != nil {
		f.onStart()
	}
}

func (f *recordingDynamicInformerFactory) WaitForCacheSync(_ <-chan struct{}) map[schema.GroupVersionResource]bool {
	return nil
}

func (f *recordingDynamicInformerFactory) Shutdown() {}

type recordingGenericInformer struct {
	informer   *recordingSharedIndexInformer
	lister     cache.GenericLister
	onInformer func()
}

func (i *recordingGenericInformer) Informer() cache.SharedIndexInformer {
	if i.onInformer != nil {
		i.onInformer()
	}
	return i.informer
}

func (i *recordingGenericInformer) Lister() cache.GenericLister {
	return i.lister
}

// recordingSharedIndexInformer implements only the SharedIndexInformer methods exercised by these tests.
type recordingSharedIndexInformer struct {
	cache.SharedIndexInformer

	setTransformCalls    atomic.Int64
	addEventHandlerCalls atomic.Int64
	setTransformErr      error
	onSetTransform       func()
}

func (i *recordingSharedIndexInformer) SetTransform(_ cache.TransformFunc) error {
	i.setTransformCalls.Add(1)
	if i.onSetTransform != nil {
		i.onSetTransform()
	}
	return i.setTransformErr
}

func (i *recordingSharedIndexInformer) AddEventHandler(_ cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
	i.addEventHandlerCalls.Add(1)
	return nil, nil
}

func newRecordingGenericInformer() *recordingGenericInformer {
	return &recordingGenericInformer{
		informer: &recordingSharedIndexInformer{},
		lister: cache.NewGenericLister(
			cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}),
			testResourceGVR.GroupResource(),
		),
	}
}

func waitForTestSignal(t *testing.T, signal <-chan struct{}, operation string) {
	t.Helper()
	waitForTestValue(t, signal, operation)
}

func waitForTestValue[T any](t *testing.T, values <-chan T, operation string) T {
	t.Helper()
	select {
	case value := <-values:
		return value
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", operation)
		var zero T
		return zero
	}
}

var _ dynamicinformer.DynamicSharedInformerFactory = (*recordingDynamicInformerFactory)(nil)
var _ informers.GenericInformer = (*recordingGenericInformer)(nil)
