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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
		})
	}
}

func TestHasSynced(t *testing.T) {
	tests := []struct {
		name           string
		initialSynced  bool
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
			name:           "not synced but empty list",
			initialSynced:  false,
			listResult:     []runtime.Object{},
			expectedSynced: true,
		},
		{
			name:          "not synced with items",
			initialSynced: false,
			listResult: []runtime.Object{
				&configv1alpha1.ResourceInterpreterWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
				},
			},
			expectedSynced: false,
		},
		{
			name:           "list error",
			initialSynced:  false,
			listErr:        fmt.Errorf("test error"),
			expectedSynced: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &interpreterConfigManager{
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
		expectedCount int
		wantSynced    bool
	}{
		{
			name:          "empty configuration",
			configs:       []runtime.Object{},
			expectedCount: 0,
			wantSynced:    true,
		},
		{
			name: "valid configurations",
			configs: []runtime.Object{
				&configv1alpha1.ResourceInterpreterWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{Name: "config1"},
					Webhooks: []configv1alpha1.ResourceInterpreterWebhook{
						{Name: "webhook1"},
					},
				},
			},
			expectedCount: 1,
			wantSynced:    true,
		},
		{
			name:          "list error",
			configs:       []runtime.Object{},
			listErr:       fmt.Errorf("test error"),
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
			}
			manager.configuration.Store([]WebhookAccessor{})
			manager.initialSynced.Store(false)

			manager.updateConfiguration()

			accessors := manager.HookAccessors()
			assert.Equal(t, tt.expectedCount, len(accessors))
			assert.Equal(t, tt.wantSynced, manager.HasSynced())
		})
	}
}

// Mock Implementations

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
