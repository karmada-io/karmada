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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

var testResourceGVR = schema.GroupVersionResource{
	Group:    "example.io",
	Version:  "v1",
	Resource: "widgets",
}

func TestSingleClusterInformerManagerCreateInformerRechecksInitializedInformer(t *testing.T) {
	cachedInformer := newRecordingGenericInformer()
	factoryInformer := newRecordingGenericInformer()
	factory := &recordingDynamicInformerFactory{informer: factoryInformer}
	manager := newTestSingleClusterInformerManager(t, factory)
	manager.initializedInformers.Store(testResourceGVR, cachedInformer)

	got := manager.createInformer(testResourceGVR)
	if got != cachedInformer {
		t.Fatal("createInformer did not reuse the informer initialized by another caller")
	}
	if got := factory.forResourceCalls.Load(); got != 0 {
		t.Fatalf("factory.ForResource() called %d times, want 0", got)
	}
}

func TestSingleClusterInformerManagerCachedLookupSkipsLock(t *testing.T) {
	genericInformer := newRecordingGenericInformer()
	factory := &recordingDynamicInformerFactory{informer: genericInformer}
	manager := newTestSingleClusterInformerManager(t, factory)

	got := manager.Lister(testResourceGVR)
	if got != genericInformer.lister {
		t.Fatal("Lister() returned a different lister during initialization")
	}

	manager.lock.Lock()
	lockHeld := true
	defer func() {
		if lockHeld {
			manager.lock.Unlock()
		}
	}()

	lookupResult := make(chan cache.GenericLister, 1)
	go func() {
		lookupResult <- manager.Lister(testResourceGVR)
	}()

	select {
	case got := <-lookupResult:
		manager.lock.Unlock()
		lockHeld = false
		if got != genericInformer.lister {
			t.Fatal("cached lookup returned a different lister")
		}
	case <-time.After(time.Second):
		manager.lock.Unlock()
		lockHeld = false
		waitForTestValue(t, lookupResult, "cached lookup after releasing lock")
		t.Fatal("cached lookup blocked on lock")
	}

	if got := factory.forResourceCalls.Load(); got != 1 {
		t.Fatalf("factory.ForResource() called %d times, want 1", got)
	}
}

func TestSingleClusterInformerManagerConcurrentHandlerRegistration(t *testing.T) {
	genericInformer := newRecordingGenericInformer()
	factory := &recordingDynamicInformerFactory{informer: genericInformer}
	manager := newTestSingleClusterInformerManager(t, factory)
	handler := &testResourceEventHandler{}

	const goroutines = 32
	start := make(chan struct{})
	var workers sync.WaitGroup
	workers.Add(goroutines)
	for range goroutines {
		go func() {
			defer workers.Done()
			<-start
			manager.ForResource(testResourceGVR, handler)
		}()
	}
	close(start)
	workers.Wait()

	if !manager.IsHandlerExist(testResourceGVR, handler) {
		t.Fatal("handler was not recorded after registration")
	}
	handlers, ok := manager.handlers.Load(testResourceGVR)
	if !ok {
		t.Fatal("handler slice was not stored")
	}
	if got := len(handlers.([]cache.ResourceEventHandler)); got != 1 {
		t.Fatalf("stored %d handlers, want 1", got)
	}
	if got := factory.forResourceCalls.Load(); got != 1 {
		t.Fatalf("factory.ForResource() called %d times, want 1", got)
	}
	if got := genericInformer.informer.addEventHandlerCalls.Load(); got != 1 {
		t.Fatalf("AddEventHandler() called %d times, want 1", got)
	}
}

func TestSingleClusterInformerManagerSerializesInitializationWithStart(t *testing.T) {
	genericInformer := newRecordingGenericInformer()
	factory := &recordingDynamicInformerFactory{informer: genericInformer}
	manager := newTestSingleClusterInformerManager(t, factory)

	factoryEntered := make(chan struct{})
	releaseFactory := make(chan struct{})
	var releaseFactoryOnce sync.Once
	release := func() {
		releaseFactoryOnce.Do(func() { close(releaseFactory) })
	}
	t.Cleanup(release)

	factory.onForResource = func() {
		assertMutexHeld(t, &manager.lock, "factory.ForResource")
		close(factoryEntered)
		<-releaseFactory
	}
	genericInformer.onInformer = func() {
		assertMutexHeld(t, &manager.lock, "GenericInformer.Informer")
	}
	var startBeforeInformerPublished atomic.Bool
	factory.onStart = func() {
		if _, exists := manager.initializedInformers.Load(testResourceGVR); !exists {
			startBeforeInformerPublished.Store(true)
		}
	}

	initializationDone := make(chan struct{})
	go func() {
		manager.getOrCreateInformer(testResourceGVR)
		close(initializationDone)
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
	waitForTestSignal(t, initializationDone, "informer initialization")
	if !startReturnedBeforeInitialization {
		waitForTestSignal(t, startDone, "Start")
	}

	if startReturnedBeforeInitialization {
		t.Fatal("Start returned before informer initialization completed")
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
	manager := newTestSingleClusterInformerManager(t, factory)
	factory.onStart = func() {
		assertMutexHeld(t, &manager.lock, "factory.Start")
	}

	manager.Start()

	if got := factory.startCalls.Load(); got != 1 {
		t.Fatalf("factory.Start() called %d times, want 1", got)
	}
}

func newTestSingleClusterInformerManager(
	t *testing.T,
	factory dynamicinformer.DynamicSharedInformerFactory,
) *singleClusterInformerManagerImpl {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	return &singleClusterInformerManagerImpl{
		ctx:             ctx,
		cancel:          cancel,
		informerFactory: factory,
	}
}

func assertMutexHeld(t *testing.T, mutex *sync.Mutex, operation string) {
	t.Helper()
	if mutex.TryLock() {
		mutex.Unlock()
		t.Errorf("lock is not held during %s", operation)
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

	addEventHandlerCalls atomic.Int64
}

type testResourceEventHandler struct{}

func (*testResourceEventHandler) OnAdd(_ any, _ bool) {}
func (*testResourceEventHandler) OnUpdate(_, _ any)   {}
func (*testResourceEventHandler) OnDelete(_ any)      {}

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
