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

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	"github.com/karmada-io/karmada/pkg/util/fedinformer"
)

func TestMultiClusterInformerManager(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	transforms := map[schema.GroupVersionResource]cache.TransformFunc{
		nodeGVR: fedinformer.NodeTransformFunc,
		podGVR:  fedinformer.PodTransformFunc,
	}

	manager := NewMultiClusterInformerManager(ctx, transforms)

	t.Run("ForCluster", func(_ *testing.T) {
		cluster := "test-cluster"
		client := fake.NewSimpleClientset()
		resync := 10 * time.Second

		singleManager := manager.ForCluster(cluster, client, resync)
		if singleManager == nil {
			t.Fatalf("ForCluster() returned nil")
		}

		if !manager.IsManagerExist(cluster) {
			t.Fatalf("IsManagerExist() returned false for existing cluster")
		}
	})

	t.Run("GetSingleClusterManager", func(t *testing.T) {
		cluster := "test-cluster"
		singleManager := manager.GetSingleClusterManager(cluster)
		if singleManager == nil {
			t.Fatalf("GetSingleClusterManager() returned nil for existing cluster")
		}

		nonExistentCluster := "non-existent-cluster"
		singleManager = manager.GetSingleClusterManager(nonExistentCluster)
		if singleManager != nil {
			t.Fatalf("GetSingleClusterManager() returned non-nil for non-existent cluster")
		}
	})

	t.Run("Start and Stop", func(t *testing.T) {
		cluster := "test-cluster-2"
		client := fake.NewSimpleClientset()
		resync := 10 * time.Second

		manager.ForCluster(cluster, client, resync)
		manager.Start(cluster)

		manager.Stop(cluster)

		if manager.IsManagerExist(cluster) {
			t.Fatalf("IsManagerExist() returned true after Stop()")
		}
	})

	t.Run("WaitForCacheSync", func(t *testing.T) {
		cluster := "test-cluster-3"
		client := fake.NewSimpleClientset()
		resync := 10 * time.Millisecond
		singleManager := manager.ForCluster(cluster, client, resync)
		manager.Start(cluster)

		_, _ = singleManager.Lister(podGVR)
		_, _ = singleManager.Lister(nodeGVR)

		time.Sleep(100 * time.Millisecond)

		result := manager.WaitForCacheSync(cluster)
		if result == nil {
			t.Fatalf("WaitForCacheSync() returned nil result")
		}

		for gvr, synced := range result {
			t.Logf("Resource %v synced: %v", gvr, synced)
		}

		manager.Stop(cluster)
	})

	t.Run("WaitForCacheSyncWithTimeout", func(t *testing.T) {
		cluster := "test-cluster-4"
		client := fake.NewSimpleClientset()
		resync := 10 * time.Millisecond
		singleManager := manager.ForCluster(cluster, client, resync)
		manager.Start(cluster)

		_, _ = singleManager.Lister(podGVR)
		_, _ = singleManager.Lister(nodeGVR)

		timeout := 100 * time.Millisecond
		result := manager.WaitForCacheSyncWithTimeout(cluster, timeout)
		if result == nil {
			t.Fatalf("WaitForCacheSyncWithTimeout() returned nil result")
		}

		for gvr, synced := range result {
			t.Logf("Resource %v synced: %v", gvr, synced)
		}

		manager.Stop(cluster)
	})

	t.Run("WaitForCacheSync and WaitForCacheSyncWithTimeout with non-existent cluster", func(t *testing.T) {
		nonExistentCluster := "non-existent-cluster"

		result1 := manager.WaitForCacheSync(nonExistentCluster)
		if result1 != nil {
			t.Fatalf("WaitForCacheSync() returned non-nil for non-existent cluster")
		}

		result2 := manager.WaitForCacheSyncWithTimeout(nonExistentCluster, 1*time.Second)
		if result2 != nil {
			t.Fatalf("WaitForCacheSyncWithTimeout() returned non-nil for non-existent cluster")
		}
	})
}

func TestGetInstance(t *testing.T) {
	instance1 := GetInstance()
	instance2 := GetInstance()

	if instance1 != instance2 {
		t.Fatalf("GetInstance() returned different instances")
	}
}

func TestStopInstance(_ *testing.T) {
	StopInstance()
	// Ensure StopInstance doesn't panic
	StopInstance()
}
