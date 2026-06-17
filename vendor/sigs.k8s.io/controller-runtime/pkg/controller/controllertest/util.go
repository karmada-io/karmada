/*
Copyright 2017 The Kubernetes Authors.

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

package controllertest

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

var _ cache.SharedIndexInformer = &FakeInformer{}

// InformerOptions contains options for NewFakeInformer.
type InformerOptions struct {
	// Synced indicates the informer is immediately synced after NewFakeInformer.
	Synced bool
}

// InformerOption is some configuration that modifies options for NewFakeInformer.
type InformerOption interface {
	// Apply applies this configuration to the given InformerOptions.
	Apply(*InformerOptions)
}

// Synced indicates the informer is immediately synced after NewFakeInformer.
var Synced = synced{}

type synced struct{}

func (synced) Apply(opts *InformerOptions) {
	opts.Synced = true
}

// FakeInformer provides fake Informer functionality for testing.
// FakeInformer must be constructed via NewFakeInformer.
type FakeInformer struct {
	// synced is used to signal that the informer is synced.
	// This field is used in various HasSynced and HasSyncedChecker methods.
	synced chan struct{}

	// RunCount is incremented each time RunInformersAndControllers is called
	RunCount int

	handlers []cache.ResourceEventHandler
}

func NewFakeInformer(opts ...InformerOption) *FakeInformer {
	informerOptions := &InformerOptions{}
	for _, opt := range opts {
		opt.Apply(informerOptions)
	}

	f := &FakeInformer{
		synced: make(chan struct{}),
	}
	if informerOptions.Synced {
		f.Synced()
	}
	return f
}

// fakeHandlerRegistration implements cache.ResourceEventHandlerRegistration for testing.
type fakeHandlerRegistration struct {
	informer *FakeInformer
}

// HasSynced implements cache.ResourceEventHandlerRegistration.
func (f *fakeHandlerRegistration) HasSynced() bool {
	return f.informer.HasSynced()
}

// HasSyncedChecker implements cache.ResourceEventHandlerRegistration.
func (f *fakeHandlerRegistration) HasSyncedChecker() cache.DoneChecker {
	return f
}

func (f *fakeHandlerRegistration) Name() string {
	return "FakeHandlerRegistration"
}

func (f *fakeHandlerRegistration) Done() <-chan struct{} {
	return f.informer.synced
}

// AddIndexers does nothing.  TODO(community): Implement this.
func (f *FakeInformer) AddIndexers(indexers cache.Indexers) error {
	return nil
}

// GetIndexer does nothing.  TODO(community): Implement this.
func (f *FakeInformer) GetIndexer() cache.Indexer {
	return cache.NewIndexer(cache.DeletionHandlingMetaNamespaceKeyFunc, nil)
}

// Informer returns the fake Informer.
func (f *FakeInformer) Informer() cache.SharedIndexInformer {
	return f
}

// Synced sets the FakeInformer state to "synced".
func (f *FakeInformer) Synced() {
	close(f.synced)
}

// HasSynced implements the Informer interface. Returns f.Synced.
func (f *FakeInformer) HasSynced() bool {
	select {
	case <-f.synced:
		return true
	default:
		return false
	}
}

// HasSyncedChecker implements the Informer interface.
func (f *FakeInformer) HasSyncedChecker() cache.DoneChecker {
	return f
}

func (f *FakeInformer) Name() string {
	return "FakeInformer"
}

func (f *FakeInformer) Done() <-chan struct{} {
	return f.synced
}

// AddEventHandler implements the Informer interface. Adds an EventHandler to the fake Informers. TODO(community): Implement Registration.
func (f *FakeInformer) AddEventHandler(handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
	f.handlers = append(f.handlers, handler)
	return &fakeHandlerRegistration{informer: f}, nil
}

// AddEventHandlerWithResyncPeriod implements the Informer interface. Adds an EventHandler to the fake Informers (ignores resyncPeriod). TODO(community): Implement Registration.
func (f *FakeInformer) AddEventHandlerWithResyncPeriod(handler cache.ResourceEventHandler, _ time.Duration) (cache.ResourceEventHandlerRegistration, error) {
	f.handlers = append(f.handlers, handler)
	return &fakeHandlerRegistration{informer: f}, nil
}

// AddEventHandlerWithOptions implements the Informer interface. Adds an EventHandler to the fake Informers (ignores options). TODO(community): Implement Registration.
func (f *FakeInformer) AddEventHandlerWithOptions(handler cache.ResourceEventHandler, _ cache.HandlerOptions) (cache.ResourceEventHandlerRegistration, error) {
	f.handlers = append(f.handlers, handler)
	return &fakeHandlerRegistration{informer: f}, nil
}

// Run implements the Informer interface.  Increments f.RunCount.
func (f *FakeInformer) Run(<-chan struct{}) {
	f.RunCount++
}

func (f *FakeInformer) RunWithContext(_ context.Context) {
	f.RunCount++
}

// Add fakes an Add event for obj.
func (f *FakeInformer) Add(obj metav1.Object) {
	for _, h := range f.handlers {
		h.OnAdd(obj, false)
	}
}

// Update fakes an Update event for obj.
func (f *FakeInformer) Update(oldObj, newObj metav1.Object) {
	for _, h := range f.handlers {
		h.OnUpdate(oldObj, newObj)
	}
}

// Delete fakes an Delete event for obj.
func (f *FakeInformer) Delete(obj metav1.Object) {
	for _, h := range f.handlers {
		h.OnDelete(obj)
	}
}

// RemoveEventHandler does nothing.  TODO(community): Implement this.
func (f *FakeInformer) RemoveEventHandler(handle cache.ResourceEventHandlerRegistration) error {
	return nil
}

// GetStore does nothing.  TODO(community): Implement this.
func (f *FakeInformer) GetStore() cache.Store {
	return nil
}

// GetController does nothing.  TODO(community): Implement this.
func (f *FakeInformer) GetController() cache.Controller {
	return nil
}

// LastSyncResourceVersion does nothing.  TODO(community): Implement this.
func (f *FakeInformer) LastSyncResourceVersion() string {
	return ""
}

// SetWatchErrorHandler does nothing.  TODO(community): Implement this.
func (f *FakeInformer) SetWatchErrorHandler(cache.WatchErrorHandler) error {
	return nil
}

// SetWatchErrorHandlerWithContext does nothing.  TODO(community): Implement this.
func (f *FakeInformer) SetWatchErrorHandlerWithContext(cache.WatchErrorHandlerWithContext) error {
	return nil
}

// SetTransform does nothing.  TODO(community): Implement this.
func (f *FakeInformer) SetTransform(t cache.TransformFunc) error {
	return nil
}

// IsStopped does nothing.  TODO(community): Implement this.
func (f *FakeInformer) IsStopped() bool {
	return false
}
