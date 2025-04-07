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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

func TestSingleClusterInformerManager(t *testing.T) {
	client := fake.NewSimpleClientset()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := NewSingleClusterInformerManager(ctx, client, 0, nil)

	t.Run("ForResource", func(t *testing.T) {
		handler := &testResourceEventHandler{}
		err := manager.ForResource(podGVR, handler)
		require.NoError(t, err, "ForResource failed")

		assert.True(t, manager.IsHandlerExist(podGVR, handler), "Handler should exist for podGVR")
	})

	t.Run("Lister", func(t *testing.T) {
		lister, err := manager.Lister(podGVR)
		require.NoError(t, err, "Lister failed")
		assert.NotNil(t, lister, "Lister should not be nil")
	})

	t.Run("Start and Stop", func(_ *testing.T) {
		manager.Start()
		// Sleep to allow informers to start
		time.Sleep(100 * time.Millisecond)
		manager.Stop()
	})

	t.Run("WaitForCacheSync", func(t *testing.T) {
		manager.Start()
		defer manager.Stop()

		synced := manager.WaitForCacheSync()
		assert.NotEmpty(t, synced, "WaitForCacheSync should return non-empty map")
	})

	t.Run("WaitForCacheSyncWithTimeout", func(t *testing.T) {
		manager.Start()
		defer manager.Stop()

		synced := manager.WaitForCacheSyncWithTimeout(5 * time.Second)
		assert.NotEmpty(t, synced, "WaitForCacheSyncWithTimeout should return non-empty map")
	})

	t.Run("Context", func(t *testing.T) {
		ctx := manager.Context()
		assert.NotNil(t, ctx, "Context should not be nil")
	})

	t.Run("GetClient", func(t *testing.T) {
		c := manager.GetClient()
		assert.NotNil(t, c, "GetClient should not return nil")
	})
}

func TestSingleClusterInformerManagerWithTransformFunc(t *testing.T) {
	client := fake.NewSimpleClientset()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	transformFunc := func(i interface{}) (interface{}, error) {
		return i, nil
	}

	transformFuncs := map[schema.GroupVersionResource]cache.TransformFunc{
		podGVR: transformFunc,
	}

	manager := NewSingleClusterInformerManager(ctx, client, 0, transformFuncs)

	t.Run("ForResourceWithTransform", func(t *testing.T) {
		handler := &testResourceEventHandler{}
		err := manager.ForResource(podGVR, handler)
		require.NoError(t, err, "ForResource with transform failed")
	})
}

func TestSingleClusterInformerManagerMultipleHandlers(t *testing.T) {
	client := fake.NewSimpleClientset()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := NewSingleClusterInformerManager(ctx, client, 0, nil)

	handler1 := &testResourceEventHandler{}
	handler2 := &testResourceEventHandler{}

	t.Run("MultipleHandlers", func(t *testing.T) {
		err := manager.ForResource(podGVR, handler1)
		require.NoError(t, err, "ForResource failed for handler1")

		err = manager.ForResource(podGVR, handler2)
		require.NoError(t, err, "ForResource failed for handler2")

		assert.True(t, manager.IsHandlerExist(podGVR, handler1), "Handler1 should exist for podGVR")
		assert.True(t, manager.IsHandlerExist(podGVR, handler2), "Handler2 should exist for podGVR")
	})
}

func TestSingleClusterInformerManagerDifferentResources(t *testing.T) {
	client := fake.NewSimpleClientset()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := NewSingleClusterInformerManager(ctx, client, 0, nil)

	t.Run("DifferentResources", func(t *testing.T) {
		podHandler := &testResourceEventHandler{}
		err := manager.ForResource(podGVR, podHandler)
		require.NoError(t, err, "ForResource failed for podGVR")

		nodeHandler := &testResourceEventHandler{}
		err = manager.ForResource(nodeGVR, nodeHandler)
		require.NoError(t, err, "ForResource failed for nodeGVR")

		assert.True(t, manager.IsHandlerExist(podGVR, podHandler), "PodHandler should exist for podGVR")
		assert.True(t, manager.IsHandlerExist(nodeGVR, nodeHandler), "NodeHandler should exist for nodeGVR")
	})
}

func TestIsInformerSynced(t *testing.T) {
	client := fake.NewSimpleClientset()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager := NewSingleClusterInformerManager(ctx, client, 0, nil)

	assert.False(t, manager.IsInformerSynced(podGVR))
	assert.False(t, manager.IsInformerSynced(nodeGVR))

	handler := &testResourceEventHandler{}
	err := manager.ForResource(podGVR, handler)
	require.NoError(t, err)
	err = manager.ForResource(nodeGVR, handler)
	require.NoError(t, err)

	manager.Start()
	defer manager.Stop()

	synced := manager.WaitForCacheSyncWithTimeout(5 * time.Second)

	assert.True(t, synced[podGVR], "Pod informer should be synced")
	assert.True(t, synced[nodeGVR], "Node informer should be synced")

	time.Sleep(100 * time.Millisecond)

	assert.True(t, manager.IsInformerSynced(podGVR), "Pod informer should be reported as synced")
	assert.True(t, manager.IsInformerSynced(nodeGVR), "Node informer should be reported as synced")
}

type testResourceEventHandler struct{}

func (t *testResourceEventHandler) OnAdd(_ interface{}, _ bool) {}
func (t *testResourceEventHandler) OnUpdate(_, _ interface{})   {}
func (t *testResourceEventHandler) OnDelete(_ interface{})      {}
