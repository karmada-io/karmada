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

package configmanager

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
)

func TestNewExploreConfigManager(t *testing.T) {
	tests := []struct {
		name     string
		initObjs []runtime.Object
	}{
		{
			name:     "empty initial objects",
			initObjs: []runtime.Object{},
		},
		{
			name: "with initial configurations",
			initObjs: []runtime.Object{
				&configv1alpha1.ResourceInterpreterWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{Name: "test-config"},
					Webhooks: []configv1alpha1.ResourceInterpreterWebhook{
						{Name: "webhook1"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			informerManager := &mockInformerManager{
				lister: &mockLister{items: tt.initObjs},
			}
			manager := NewExploreConfigManager(informerManager)

			assert.NotNil(t, manager, "Manager should not be nil")
			assert.NotNil(t, manager.HookAccessors(), "Accessors should be initialized")

			internalManager, ok := manager.(*interpreterConfigManager)
			assert.True(t, ok)
			assert.Equal(t, informerManager, internalManager.informer)
		})
	}
}

func TestHasSynced(t *testing.T) {
	tests := []struct {
		name           string
		initialSynced  bool
		informer       genericmanager.SingleClusterInformerManager
		listErr        error
		listResult     []runtime.Object
		expectedSynced bool
	}{
		{
			name:           "already synced",
			initialSynced:  true,
			expectedSynced: true,
		},
		{
			name:           "informer not configured",
			initialSynced:  false,
			informer:       nil,
			expectedSynced: false,
		},
		{
			name:          "informer not synced",
			initialSynced: false,
			informer: &mockSingleClusterInformerManager{
				isSynced: false,
			},
			expectedSynced: false,
		},
		{
			name:          "sync with empty list",
			initialSynced: false,
			informer: &mockSingleClusterInformerManager{
				isSynced: true,
			},
			listResult:     []runtime.Object{},
			expectedSynced: true,
		},
		{
			name:          "sync with items",
			initialSynced: false,
			informer: &mockSingleClusterInformerManager{
				isSynced: true,
			},
			listResult: []runtime.Object{
				&configv1alpha1.ResourceInterpreterWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
				},
			},
			expectedSynced: true,
		},
		{
			name:          "list error",
			initialSynced: false,
			informer: &mockSingleClusterInformerManager{
				isSynced: true,
			},
			listErr:        fmt.Errorf("test error"),
			expectedSynced: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &interpreterConfigManager{
				informer: tt.informer,
				lister: &mockLister{
					err:   tt.listErr,
					items: tt.listResult,
				},
			}
			manager.initialSynced.Store(tt.initialSynced)
			assert.Equal(t, tt.expectedSynced, manager.HasSynced())
		})
	}
}

func TestMergeResourceExploreWebhookConfigurations(t *testing.T) {
	tests := []struct {
		name           string
		configurations []*configv1alpha1.ResourceInterpreterWebhookConfiguration
		expectedLen    int
	}{
		{
			name:           "empty configurations",
			configurations: []*configv1alpha1.ResourceInterpreterWebhookConfiguration{},
			expectedLen:    0,
		},
		{
			name: "single configuration with one webhook",
			configurations: []*configv1alpha1.ResourceInterpreterWebhookConfiguration{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "config1"},
					Webhooks: []configv1alpha1.ResourceInterpreterWebhook{
						{Name: "webhook1"},
					},
				},
			},
			expectedLen: 1,
		},
		{
			name: "multiple configurations with multiple webhooks",
			configurations: []*configv1alpha1.ResourceInterpreterWebhookConfiguration{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "config1"},
					Webhooks: []configv1alpha1.ResourceInterpreterWebhook{
						{Name: "webhook1"},
						{Name: "webhook2"},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "config2"},
					Webhooks: []configv1alpha1.ResourceInterpreterWebhook{
						{Name: "webhook3"},
					},
				},
			},
			expectedLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeResourceExploreWebhookConfigurations(tt.configurations)
			assert.Equal(t, tt.expectedLen, len(result))

			if len(result) > 1 {
				for i := 1; i < len(result); i++ {
					assert.True(t, result[i-1].GetUID() <= result[i].GetUID(),
						"Webhooks should be sorted by UID")
				}
			}
		})
	}
}

func TestUpdateConfiguration(t *testing.T) {
	tests := []struct {
		name          string
		configs       []runtime.Object
		listErr       error
		informer      genericmanager.SingleClusterInformerManager
		expectedCount int
		wantSynced    bool
	}{
		{
			name:          "informer not configured",
			informer:      nil,
			expectedCount: 0,
			wantSynced:    false,
		},
		{
			name: "informer not synced",
			informer: &mockSingleClusterInformerManager{
				isSynced: false,
			},
			expectedCount: 0,
			wantSynced:    false,
		},
		{
			name:    "empty configuration",
			configs: []runtime.Object{},
			informer: &mockSingleClusterInformerManager{
				isSynced: true,
			},
			expectedCount: 0,
			wantSynced:    true,
		},
		{
			name: "valid configurations",
			configs: []runtime.Object{
				&configv1alpha1.ResourceInterpreterWebhookConfiguration{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ResourceInterpreterWebhookConfiguration",
						APIVersion: configv1alpha1.GroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{Name: "config1"},
					Webhooks: []configv1alpha1.ResourceInterpreterWebhook{
						{Name: "webhook1"},
					},
				},
			},
			informer: &mockSingleClusterInformerManager{
				isSynced: true,
			},
			expectedCount: 1,
			wantSynced:    true,
		},
		{
			name:    "list error",
			configs: []runtime.Object{},
			listErr: fmt.Errorf("test error"),
			informer: &mockSingleClusterInformerManager{
				isSynced: true,
			},
			expectedCount: 0,
			wantSynced:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &interpreterConfigManager{
				lister: &mockLister{
					items: tt.configs,
					err:   tt.listErr,
				},
				informer: tt.informer,
			}
			manager.configuration.Store([]WebhookAccessor{})
			manager.initialSynced.Store(false)

			synced := manager.HasSynced()
			assert.Equal(t, tt.wantSynced, synced)

			accessors := manager.HookAccessors()
			assert.Equal(t, tt.expectedCount, len(accessors))
		})
	}
}

// Mock Implementations

type mockSingleClusterInformerManager struct {
	isSynced bool
}

func (m *mockSingleClusterInformerManager) IsInformerSynced(_ schema.GroupVersionResource) bool {
	return m.isSynced
}

func (m *mockSingleClusterInformerManager) Lister(_ schema.GroupVersionResource) cache.GenericLister {
	return nil
}

func (m *mockSingleClusterInformerManager) ForResource(_ schema.GroupVersionResource, _ cache.ResourceEventHandler) {
}

func (m *mockSingleClusterInformerManager) Start() {
}

func (m *mockSingleClusterInformerManager) Stop() {
}

func (m *mockSingleClusterInformerManager) WaitForCacheSync() map[schema.GroupVersionResource]bool {
	return nil
}

func (m *mockSingleClusterInformerManager) WaitForCacheSyncWithTimeout(_ time.Duration) map[schema.GroupVersionResource]bool {
	return nil
}

func (m *mockSingleClusterInformerManager) Context() context.Context {
	return context.Background()
}

func (m *mockSingleClusterInformerManager) GetClient() dynamic.Interface {
	return nil
}

func (m *mockSingleClusterInformerManager) IsHandlerExist(_ schema.GroupVersionResource, _ cache.ResourceEventHandler) bool {
	return false
}

type mockLister struct {
	items []runtime.Object
	err   error
}

func (m *mockLister) List(_ labels.Selector) ([]runtime.Object, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.items, nil
}

func (m *mockLister) Get(_ string) (runtime.Object, error) {
	return nil, nil
}

func (m *mockLister) ByNamespace(_ string) cache.GenericNamespaceLister {
	return nil
}

type mockInformerManager struct {
	genericmanager.SingleClusterInformerManager
	lister cache.GenericLister
}

func (m *mockInformerManager) Lister(_ schema.GroupVersionResource) cache.GenericLister {
	return m.lister
}

func (m *mockInformerManager) ForResource(_ schema.GroupVersionResource, _ cache.ResourceEventHandler) {
}
