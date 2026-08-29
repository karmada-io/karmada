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

package typedmanager

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

func TestSingleClusterInformerManagerRejectsInvalidHandler(t *testing.T) {
	informer := newRecordingTypedGenericInformer()
	factory := &recordingTypedInformerFactory{informer: informer}
	manager := newTestTypedSingleClusterInformerManager(t, factory, nil)

	if err := manager.ForResource(podGVR, &cache.ResourceEventHandlerFuncs{}); err != nil {
		t.Fatalf("ForResource() returned an unexpected error for a pointer handler: %v", err)
	}

	handler := cache.ResourceEventHandlerFuncs{}
	if manager.IsHandlerExist(podGVR, handler) {
		t.Fatal("IsHandlerExist() returned true for a non-pointer handler")
	}
	if err := manager.ForResource(podGVR, handler); err == nil {
		t.Fatal("ForResource() returned nil for a non-pointer handler")
	}

	var nilHandler *cache.ResourceEventHandlerFuncs
	if manager.IsHandlerExist(podGVR, nilHandler) {
		t.Fatal("IsHandlerExist() returned true for a nil pointer handler")
	}
	if err := manager.ForResource(podGVR, nilHandler); err == nil {
		t.Fatal("ForResource() returned nil for a nil pointer handler")
	}
}

func TestSingleClusterInformerManagerAppliesTransform(t *testing.T) {
	tests := []struct {
		name          string
		observeObject func(t *testing.T, manager SingleClusterInformerManager) *corev1.Pod
	}{
		{
			name: "ForResource",
			observeObject: func(t *testing.T, manager SingleClusterInformerManager) *corev1.Pod {
				observed := make(chan any, 1)
				handler := &cache.ResourceEventHandlerDetailedFuncs{
					AddFunc: func(obj any, _ bool) {
						observed <- obj
					},
				}
				if err := manager.ForResource(podGVR, handler); err != nil {
					t.Fatalf("ForResource() returned an unexpected error: %v", err)
				}
				manager.Start()
				waitForTypedTestInformerSync(t, manager, podGVR)

				select {
				case obj := <-observed:
					pod, ok := obj.(*corev1.Pod)
					if !ok {
						t.Fatalf("got object type %T, want *corev1.Pod", obj)
					}
					return pod
				case <-time.After(5 * time.Second):
					t.Fatal("timed out waiting for informer add event")
					return nil
				}
			},
		},
		{
			name: "Lister",
			observeObject: func(t *testing.T, manager SingleClusterInformerManager) *corev1.Pod {
				lister, err := manager.Lister(podGVR)
				if err != nil {
					t.Fatalf("Lister() returned an unexpected error: %v", err)
				}
				podLister, ok := lister.(corev1listers.PodLister)
				if !ok {
					t.Fatalf("got lister type %T, want corev1listers.PodLister", lister)
				}
				manager.Start()
				waitForTypedTestInformerSync(t, manager, podGVR)

				pod, err := podLister.Pods("default").Get("test-pod")
				if err != nil {
					t.Fatalf("failed to get Pod from informer cache: %v", err)
				}
				return pod
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientset(newTypedTransformTestPod())
			manager := NewSingleClusterInformerManager(
				t.Context(),
				client,
				0,
				map[schema.GroupVersionResource]cache.TransformFunc{podGVR: stripTypedManagedFields},
			)
			t.Cleanup(manager.Stop)

			assertTransformedTypedTestPod(t, tt.observeObject(t, manager))
		})
	}
}

func TestSingleClusterInformerManagerReturnsTypedListers(t *testing.T) {
	manager := NewSingleClusterInformerManager(t.Context(), fake.NewClientset(), 0, nil)
	t.Cleanup(manager.Stop)

	tests := []struct {
		name     string
		resource schema.GroupVersionResource
		assert   func(t *testing.T, lister any)
	}{
		{
			name:     "Pod",
			resource: podGVR,
			assert: func(t *testing.T, lister any) {
				if _, ok := lister.(corev1listers.PodLister); !ok {
					t.Fatalf("got lister type %T, want corev1listers.PodLister", lister)
				}
			},
		},
		{
			name:     "Node",
			resource: nodeGVR,
			assert: func(t *testing.T, lister any) {
				if _, ok := lister.(corev1listers.NodeLister); !ok {
					t.Fatalf("got lister type %T, want corev1listers.NodeLister", lister)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lister, err := manager.Lister(tt.resource)
			if err != nil {
				t.Fatalf("Lister() returned an unexpected error: %v", err)
			}
			tt.assert(t, lister)
		})
	}
}

func TestSingleClusterInformerManagerConcurrentHandlerRegistration(t *testing.T) {
	genericInformer := newRecordingTypedGenericInformer()
	factory := &recordingTypedInformerFactory{informer: genericInformer}
	manager := newTestTypedSingleClusterInformerManager(t, factory, nil)
	handler := &testResourceEventHandler{id: 1}

	const goroutines = 32
	start := make(chan struct{})
	errors := make(chan error, goroutines)
	var workers sync.WaitGroup
	workers.Add(goroutines)
	for range goroutines {
		go func() {
			defer workers.Done()
			<-start
			errors <- manager.ForResource(podGVR, handler)
		}()
	}
	close(start)
	workers.Wait()
	close(errors)

	for err := range errors {
		if err != nil {
			t.Fatalf("ForResource() returned an unexpected error: %v", err)
		}
	}
	if !manager.IsHandlerExist(podGVR, handler) {
		t.Fatal("handler was not recorded after registration")
	}
	if got := factory.forResourceCalls.Load(); got != 1 {
		t.Fatalf("factory.ForResource() called %d times, want 1", got)
	}
	if got := genericInformer.informer.addEventHandlerCalls.Load(); got != 1 {
		t.Fatalf("AddEventHandler() called %d times, want 1", got)
	}
}

func TestSingleClusterInformerManagerCachedReadsSkipLock(t *testing.T) {
	manager := NewSingleClusterInformerManager(t.Context(), fake.NewClientset(), 0, nil).(*singleClusterInformerManagerImpl)
	t.Cleanup(manager.Stop)
	handler := &testResourceEventHandler{id: 1}
	if err := manager.ForResource(podGVR, handler); err != nil {
		t.Fatalf("ForResource() returned an unexpected error: %v", err)
	}
	manager.syncedInformers.Store(podGVR, struct{}{})

	manager.lock.Lock()
	lockHeld := true
	defer func() {
		if lockHeld {
			manager.lock.Unlock()
		}
	}()

	lookupDone := make(chan error, 1)
	go func() {
		if _, err := manager.Lister(podGVR); err != nil {
			lookupDone <- err
			return
		}
		if !manager.IsHandlerExist(podGVR, handler) {
			lookupDone <- errors.New("registered handler was not found")
			return
		}
		if !manager.IsInformerSynced(podGVR) {
			lookupDone <- errors.New("synced informer was not found")
			return
		}
		lookupDone <- nil
	}()

	select {
	case err := <-lookupDone:
		manager.lock.Unlock()
		lockHeld = false
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		manager.lock.Unlock()
		lockHeld = false
		if err := waitForTypedTestValue(t, lookupDone, "cached reads after releasing lock"); err != nil {
			t.Fatal(err)
		}
		t.Fatal("cached reads blocked on lock")
	}
}

func TestSingleClusterInformerManagerSerializesInitializationWithStart(t *testing.T) {
	genericInformer := newRecordingTypedGenericInformer()
	factory := &recordingTypedInformerFactory{informer: genericInformer}
	manager := newTestTypedSingleClusterInformerManager(
		t,
		factory,
		map[schema.GroupVersionResource]cache.TransformFunc{podGVR: stripTypedManagedFields},
	)

	factoryEntered := make(chan struct{})
	releaseFactory := make(chan struct{})
	var releaseFactoryOnce sync.Once
	release := func() {
		releaseFactoryOnce.Do(func() { close(releaseFactory) })
	}
	t.Cleanup(release)

	factory.onForResource = func() {
		assertTypedMutexHeld(t, &manager.lock, "factory.ForResource")
		close(factoryEntered)
		<-releaseFactory
	}
	var transformInstalled atomic.Bool
	genericInformer.informer.onSetTransform = func() {
		assertTypedMutexHeld(t, &manager.lock, "SetTransform")
		transformInstalled.Store(true)
	}
	var startBeforeTransform atomic.Bool
	var startBeforeInformerPublished atomic.Bool
	factory.onStart = func() {
		if !transformInstalled.Load() {
			startBeforeTransform.Store(true)
		}
		if _, exists := manager.initializedInformers.Load(podGVR); !exists {
			startBeforeInformerPublished.Store(true)
		}
	}

	initializationDone := make(chan error, 1)
	go func() {
		_, err := manager.informerForResource(podGVR)
		initializationDone <- err
	}()
	waitForTypedTestSignal(t, factoryEntered, "factory.ForResource")

	startAttempted := make(chan struct{})
	startDone := make(chan struct{})
	go func() {
		close(startAttempted)
		manager.Start()
		close(startDone)
	}()
	waitForTypedTestSignal(t, startAttempted, "Start attempt")

	startReturnedBeforeInitialization := false
	select {
	case <-startDone:
		startReturnedBeforeInitialization = true
	case <-time.After(50 * time.Millisecond):
	}

	release()
	if err := waitForTypedTestValue(t, initializationDone, "informer initialization"); err != nil {
		t.Fatalf("informerForResource() returned an unexpected error: %v", err)
	}
	if !startReturnedBeforeInitialization {
		waitForTypedTestSignal(t, startDone, "Start")
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
}

func TestSingleClusterInformerManagerRetriesFailedTransformWithoutPublishing(t *testing.T) {
	wantErr := errors.New("failed to install transform")
	genericInformer := newRecordingTypedGenericInformer()
	genericInformer.informer.setTransformErr = wantErr
	factory := &recordingTypedInformerFactory{informer: genericInformer}
	manager := newTestTypedSingleClusterInformerManager(
		t,
		factory,
		map[schema.GroupVersionResource]cache.TransformFunc{podGVR: stripTypedManagedFields},
	)

	for attempt := 1; attempt <= 2; attempt++ {
		resourceInformer, err := manager.informerForResource(podGVR)
		if !errors.Is(err, wantErr) {
			t.Fatalf("attempt %d: informerForResource() error = %v, want %v", attempt, err, wantErr)
		}
		if resourceInformer != genericInformer {
			t.Fatalf("attempt %d: failed transform returned a different GenericInformer", attempt)
		}
		if _, exists := manager.initializedInformers.Load(podGVR); exists {
			t.Fatalf("attempt %d: failed transform published an initialized informer", attempt)
		}
	}

	genericInformer.informer.setTransformErr = nil
	resourceInformer, err := manager.informerForResource(podGVR)
	if err != nil {
		t.Fatalf("informerForResource() returned an unexpected error after recovery: %v", err)
	}
	if resourceInformer != genericInformer {
		t.Fatal("successful retry returned a different GenericInformer")
	}
	if _, exists := manager.initializedInformers.Load(podGVR); !exists {
		t.Fatal("successful retry did not publish the initialized informer")
	}
	if got := factory.forResourceCalls.Load(); got != 3 {
		t.Fatalf("factory.ForResource() called %d times, want 3", got)
	}
	if got := genericInformer.informer.setTransformCalls.Load(); got != 3 {
		t.Fatalf("SetTransform() called %d times, want 3", got)
	}
}

func TestSingleClusterInformerManagerWaitForCacheSyncSkipsLock(t *testing.T) {
	factory := &recordingTypedInformerFactory{
		informer: newRecordingTypedGenericInformer(),
		waitForCacheSyncResult: map[reflect.Type]bool{
			reflect.TypeFor[*corev1.Pod]():  true,
			reflect.TypeFor[*corev1.Node](): false,
		},
	}
	manager := newTestTypedSingleClusterInformerManager(t, factory, nil)
	factory.onWaitForCacheSync = func() {
		if !manager.lock.TryLock() {
			t.Error("WaitForCacheSync() called while lock was held")
			return
		}
		manager.lock.Unlock()
	}

	result := manager.waitForCacheSync(t.Context())
	if !result[podGVR] {
		t.Fatalf("Pod informer sync result = %v, want true", result[podGVR])
	}
	if result[nodeGVR] {
		t.Fatalf("Node informer sync result = %v, want false", result[nodeGVR])
	}
	if !manager.IsInformerSynced(podGVR) {
		t.Fatal("Pod informer was not recorded as synced")
	}
	if manager.IsInformerSynced(nodeGVR) {
		t.Fatal("Node informer was recorded as synced")
	}
}

func newTypedTransformTestPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "test-pod",
			ManagedFields: []metav1.ManagedFieldsEntry{
				{
					Manager:    "test-manager",
					Operation:  metav1.ManagedFieldsOperationUpdate,
					APIVersion: "v1",
					FieldsType: "FieldsV1",
				},
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "test-container", Image: "test-image"}},
		},
	}
}

func stripTypedManagedFields(obj any) (any, error) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}
	accessor.SetManagedFields(nil)
	return obj, nil
}

func assertTransformedTypedTestPod(t *testing.T, pod *corev1.Pod) {
	t.Helper()
	if len(pod.GetManagedFields()) != 0 {
		t.Fatalf("managedFields were not stripped: %v", pod.GetManagedFields())
	}
	if len(pod.Spec.Containers) != 1 || pod.Spec.Containers[0].Image != "test-image" {
		t.Fatalf("Pod spec was not preserved: %#v", pod.Spec)
	}
}

func waitForTypedTestInformerSync(t *testing.T, manager SingleClusterInformerManager, resource schema.GroupVersionResource) {
	t.Helper()
	if synced := manager.WaitForCacheSyncWithTimeout(5 * time.Second); !synced[resource] {
		t.Fatalf("informer failed to sync: %v", synced)
	}
}

func newTestTypedSingleClusterInformerManager(
	t *testing.T,
	factory informers.SharedInformerFactory,
	transforms map[schema.GroupVersionResource]cache.TransformFunc,
) *singleClusterInformerManagerImpl {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	return &singleClusterInformerManagerImpl{
		ctx:             ctx,
		cancel:          cancel,
		informerFactory: factory,
		transformFuncs:  transforms,
	}
}

func assertTypedMutexHeld(t *testing.T, mutex *sync.Mutex, operation string) {
	t.Helper()
	if mutex.TryLock() {
		mutex.Unlock()
		t.Errorf("lock is not held during %s", operation)
	}
}

type recordingTypedInformerFactory struct {
	informers.SharedInformerFactory

	informer informers.GenericInformer

	forResourceCalls       atomic.Int64
	startCalls             atomic.Int64
	onForResource          func()
	onStart                func()
	onWaitForCacheSync     func()
	waitForCacheSyncResult map[reflect.Type]bool
}

func (f *recordingTypedInformerFactory) ForResource(_ schema.GroupVersionResource) (informers.GenericInformer, error) {
	f.forResourceCalls.Add(1)
	if f.onForResource != nil {
		f.onForResource()
	}
	return f.informer, nil
}

func (f *recordingTypedInformerFactory) Start(_ <-chan struct{}) {
	f.startCalls.Add(1)
	if f.onStart != nil {
		f.onStart()
	}
}

func (f *recordingTypedInformerFactory) WaitForCacheSync(_ <-chan struct{}) map[reflect.Type]bool {
	if f.onWaitForCacheSync != nil {
		f.onWaitForCacheSync()
	}
	return f.waitForCacheSyncResult
}

type recordingTypedGenericInformer struct {
	informer *recordingTypedSharedIndexInformer
	lister   cache.GenericLister
}

func (i *recordingTypedGenericInformer) Informer() cache.SharedIndexInformer {
	return i.informer
}

func (i *recordingTypedGenericInformer) Lister() cache.GenericLister {
	return i.lister
}

type recordingTypedSharedIndexInformer struct {
	cache.SharedIndexInformer

	indexer              cache.Indexer
	setTransformCalls    atomic.Int64
	addEventHandlerCalls atomic.Int64
	setTransformErr      error
	onSetTransform       func()
}

func (i *recordingTypedSharedIndexInformer) SetTransform(_ cache.TransformFunc) error {
	i.setTransformCalls.Add(1)
	if i.onSetTransform != nil {
		i.onSetTransform()
	}
	return i.setTransformErr
}

func (i *recordingTypedSharedIndexInformer) AddEventHandler(_ cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
	i.addEventHandlerCalls.Add(1)
	return nil, nil
}

func (i *recordingTypedSharedIndexInformer) GetIndexer() cache.Indexer {
	return i.indexer
}

func newRecordingTypedGenericInformer() *recordingTypedGenericInformer {
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	return &recordingTypedGenericInformer{
		informer: &recordingTypedSharedIndexInformer{indexer: indexer},
		lister:   cache.NewGenericLister(indexer, podGVR.GroupResource()),
	}
}

func waitForTypedTestSignal(t *testing.T, signal <-chan struct{}, operation string) {
	t.Helper()
	waitForTypedTestValue(t, signal, operation)
}

func waitForTypedTestValue[T any](t *testing.T, values <-chan T, operation string) T {
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

type testResourceEventHandler struct {
	id int
}

func (t *testResourceEventHandler) OnAdd(_ any, _ bool) {}
func (t *testResourceEventHandler) OnUpdate(_, _ any)   {}
func (t *testResourceEventHandler) OnDelete(_ any)      {}

var _ informers.SharedInformerFactory = (*recordingTypedInformerFactory)(nil)
var _ informers.GenericInformer = (*recordingTypedGenericInformer)(nil)
