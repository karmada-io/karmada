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
			name:          "synced with items",
			initialSynced: false,
			listResult: []runtime.Object{
				&configv1alpha1.ResourceInterpreterWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
				},
			},
			expectedSynced: true,
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
				informer: &mockInformerManager{},
			}
			manager.initialSynced.Store(tt.initialSynced)
			assert.Equal(t, tt.expectedSynced, manager.HasSynced())
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
				informer: &mockInformerManager{},
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

func TestInterpreterConfigManager_LoadConfig(t *testing.T) {
	tests := []struct {
		name          string
		input         []*configv1alpha1.ResourceInterpreterWebhookConfiguration
		wantAccessors []string // The expected list of UIDs, and the order should be the order of UIDs generated after sorting by Name
		wantSynced    bool
	}{
		{
			name:          "Empty configuration",
			input:         []*configv1alpha1.ResourceInterpreterWebhookConfiguration{},
			wantAccessors: []string{},
			wantSynced:    true,
		},
		{
			name: "Single configuration with a single hook",
			input: []*configv1alpha1.ResourceInterpreterWebhookConfiguration{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "configA"},
					Webhooks: []configv1alpha1.ResourceInterpreterWebhook{
						{Name: "hook1"},
					},
				},
			},
			wantAccessors: []string{"configA/hook1"},
			wantSynced:    true,
		},
		{
			name: "Multiple configurations and multiple hooks, testing sorting and UID generation",
			input: []*configv1alpha1.ResourceInterpreterWebhookConfiguration{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "configB"},
					Webhooks: []configv1alpha1.ResourceInterpreterWebhook{
						{Name: "hook2"},
						{Name: "hook1"},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "configA"},
					Webhooks: []configv1alpha1.ResourceInterpreterWebhook{
						{Name: "hook3"},
					},
				},
			},
			wantAccessors: []string{
				"configA/hook3", // configA is ranked ahead of configB
				"configB/hook2",
				"configB/hook1",
			},
			wantSynced: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &interpreterConfigManager{}
			m.LoadConfig(tt.input)

			gotSynced := m.initialSynced.Load()
			if gotSynced != tt.wantSynced {
				t.Errorf("initialSynced = %v, want %v", gotSynced, tt.wantSynced)
			}

			gotAccessors, ok := m.configuration.Load().([]WebhookAccessor)
			if !ok {
				t.Fatalf("configuration	type assertion failed")
			}

			if len(gotAccessors) != len(tt.wantAccessors) {
				t.Fatalf("accessors length = %d, want %d", len(gotAccessors), len(tt.wantAccessors))
			}

			for i, wantUID := range tt.wantAccessors {
				if gotAccessors[i].GetUID() != wantUID {
					t.Errorf("accessors[%d].UID() = %q, want %q", i, gotAccessors[i].GetUID(), wantUID)
				}
			}
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

func (m *mockInformerManager) IsInformerSynced(_ schema.GroupVersionResource) bool {
	return true
}
