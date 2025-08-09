/*
Copyright 2022 The Karmada Authors.

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
	"errors"
	"reflect"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/tools/cache"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/gclient"
)

func Test_interpreterConfigManager_LuaScriptAccessors(t *testing.T) {
	customization01 := &configv1alpha1.ResourceInterpreterCustomization{
		ObjectMeta: metav1.ObjectMeta{Name: "customization01"},
		Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
			Target: configv1alpha1.CustomizationTarget{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
			},
			Customizations: configv1alpha1.CustomizationRules{
				Retention:       &configv1alpha1.LocalValueRetention{LuaScript: "a=0"},
				ReplicaResource: &configv1alpha1.ReplicaResourceRequirement{LuaScript: "b=0"},
			},
		},
	}
	customization02 := &configv1alpha1.ResourceInterpreterCustomization{
		ObjectMeta: metav1.ObjectMeta{Name: "customization02"},
		Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
			Target: configv1alpha1.CustomizationTarget{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
			},
			Customizations: configv1alpha1.CustomizationRules{
				ReplicaRevision:  &configv1alpha1.ReplicaRevision{LuaScript: "c=0"},
				StatusReflection: &configv1alpha1.StatusReflection{LuaScript: "d=0"},
			},
		},
	}
	customization03 := &configv1alpha1.ResourceInterpreterCustomization{
		ObjectMeta: metav1.ObjectMeta{Name: "customization03"},
		Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
			Target: configv1alpha1.CustomizationTarget{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
			},
			Customizations: configv1alpha1.CustomizationRules{
				ReplicaResource: &configv1alpha1.ReplicaResourceRequirement{LuaScript: "b=1"},
				ReplicaRevision: &configv1alpha1.ReplicaRevision{LuaScript: "c=1"},
			},
		},
	}
	deploymentGVK := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}

	type args struct {
		customizations []runtime.Object
	}
	tests := []struct {
		name string
		args args
		want map[schema.GroupVersionKind]CustomAccessor
	}{
		{
			name: "single ResourceInterpreterCustomization",
			args: args{[]runtime.Object{customization01}},
			want: map[schema.GroupVersionKind]CustomAccessor{
				deploymentGVK: &resourceCustomAccessor{
					retention:       &configv1alpha1.LocalValueRetention{LuaScript: "a=0"},
					replicaResource: &configv1alpha1.ReplicaResourceRequirement{LuaScript: "b=0"},
				},
			},
		},
		{
			name: "multi ResourceInterpreterCustomization with no redundant operation",
			args: args{[]runtime.Object{customization01, customization02}},
			want: map[schema.GroupVersionKind]CustomAccessor{
				deploymentGVK: &resourceCustomAccessor{
					retention:        &configv1alpha1.LocalValueRetention{LuaScript: "a=0"},
					replicaResource:  &configv1alpha1.ReplicaResourceRequirement{LuaScript: "b=0"},
					replicaRevision:  &configv1alpha1.ReplicaRevision{LuaScript: "c=0"},
					statusReflection: &configv1alpha1.StatusReflection{LuaScript: "d=0"},
				},
			},
		},
		{
			name: "multi ResourceInterpreterCustomization with redundant operation",
			args: args{[]runtime.Object{customization03, customization02, customization01}},
			want: map[schema.GroupVersionKind]CustomAccessor{
				deploymentGVK: &resourceCustomAccessor{
					retention:        &configv1alpha1.LocalValueRetention{LuaScript: "a=0"},
					replicaResource:  &configv1alpha1.ReplicaResourceRequirement{LuaScript: "b=0"},
					replicaRevision:  &configv1alpha1.ReplicaRevision{LuaScript: "c=0"},
					statusReflection: &configv1alpha1.StatusReflection{LuaScript: "d=0"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.TODO(), time.Second*5)
			defer cancel()

			client := fake.NewSimpleDynamicClient(gclient.NewSchema(), tt.args.customizations...)
			informer := genericmanager.NewSingleClusterInformerManager(ctx, client, 0)
			configManager := NewInterpreterConfigManager(informer)

			informer.Start()
			defer informer.Stop()

			informer.WaitForCacheSync()

			if !cache.WaitForCacheSync(ctx.Done(), configManager.HasSynced) {
				t.Errorf("informer has not been synced")
			}

			gotAccessors := configManager.CustomAccessors()
			for gvk, gotAccessor := range gotAccessors {
				wantAccessor, ok := tt.want[gvk]
				if !ok {
					t.Errorf("Can not find the target gvk %v", gvk)
				}
				if !reflect.DeepEqual(gotAccessor, wantAccessor) {
					t.Errorf("LuaScriptAccessors() = %v, want %v", gotAccessor, wantAccessor)
				}
			}
		})
	}
}

func Test_interpreterConfigManager_LoadConfig(t *testing.T) {
	tests := []struct {
		name    string
		configs []*configv1alpha1.ResourceInterpreterCustomization
		want    map[schema.GroupVersionKind]CustomAccessor
	}{
		{
			name:    "empty configs",
			configs: []*configv1alpha1.ResourceInterpreterCustomization{},
			want:    make(map[schema.GroupVersionKind]CustomAccessor),
		},
		{
			name: "single config",
			configs: []*configv1alpha1.ResourceInterpreterCustomization{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "customization01"},
					Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
						Target: configv1alpha1.CustomizationTarget{
							APIVersion: appsv1.SchemeGroupVersion.String(),
							Kind:       "Deployment",
						},
						Customizations: configv1alpha1.CustomizationRules{
							Retention: &configv1alpha1.LocalValueRetention{LuaScript: "retention-script"},
						},
					},
				},
			},
			want: map[schema.GroupVersionKind]CustomAccessor{
				{Group: "apps", Version: "v1", Kind: "Deployment"}: &resourceCustomAccessor{
					retention: &configv1alpha1.LocalValueRetention{LuaScript: "retention-script"},
				},
			},
		},
		{
			name: "multiple configs for same GVK",
			configs: []*configv1alpha1.ResourceInterpreterCustomization{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "customization01"},
					Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
						Target: configv1alpha1.CustomizationTarget{
							APIVersion: appsv1.SchemeGroupVersion.String(),
							Kind:       "Deployment",
						},
						Customizations: configv1alpha1.CustomizationRules{
							Retention: &configv1alpha1.LocalValueRetention{LuaScript: "retention-script"},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "customization02"},
					Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
						Target: configv1alpha1.CustomizationTarget{
							APIVersion: appsv1.SchemeGroupVersion.String(),
							Kind:       "Deployment",
						},
						Customizations: configv1alpha1.CustomizationRules{
							ReplicaResource: &configv1alpha1.ReplicaResourceRequirement{LuaScript: "replica-script"},
						},
					},
				},
			},
			want: map[schema.GroupVersionKind]CustomAccessor{
				{Group: "apps", Version: "v1", Kind: "Deployment"}: &resourceCustomAccessor{
					retention:       &configv1alpha1.LocalValueRetention{LuaScript: "retention-script"},
					replicaResource: &configv1alpha1.ReplicaResourceRequirement{LuaScript: "replica-script"},
				},
			},
		},
		{
			name: "multiple configs for different GVKs",
			configs: []*configv1alpha1.ResourceInterpreterCustomization{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "customization01"},
					Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
						Target: configv1alpha1.CustomizationTarget{
							APIVersion: appsv1.SchemeGroupVersion.String(),
							Kind:       "Deployment",
						},
						Customizations: configv1alpha1.CustomizationRules{
							Retention: &configv1alpha1.LocalValueRetention{LuaScript: "deployment-retention"},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "customization02"},
					Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
						Target: configv1alpha1.CustomizationTarget{
							APIVersion: appsv1.SchemeGroupVersion.String(),
							Kind:       "StatefulSet",
						},
						Customizations: configv1alpha1.CustomizationRules{
							ReplicaResource: &configv1alpha1.ReplicaResourceRequirement{LuaScript: "statefulset-replica"},
						},
					},
				},
			},
			want: map[schema.GroupVersionKind]CustomAccessor{
				{Group: "apps", Version: "v1", Kind: "Deployment"}: &resourceCustomAccessor{
					retention: &configv1alpha1.LocalValueRetention{LuaScript: "deployment-retention"},
				},
				{Group: "apps", Version: "v1", Kind: "StatefulSet"}: &resourceCustomAccessor{
					replicaResource: &configv1alpha1.ReplicaResourceRequirement{LuaScript: "statefulset-replica"},
				},
			},
		},
		{
			name: "configs sorted by name",
			configs: []*configv1alpha1.ResourceInterpreterCustomization{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "customization02"},
					Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
						Target: configv1alpha1.CustomizationTarget{
							APIVersion: appsv1.SchemeGroupVersion.String(),
							Kind:       "Deployment",
						},
						Customizations: configv1alpha1.CustomizationRules{
							Retention: &configv1alpha1.LocalValueRetention{LuaScript: "second"},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "customization01"},
					Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
						Target: configv1alpha1.CustomizationTarget{
							APIVersion: appsv1.SchemeGroupVersion.String(),
							Kind:       "Deployment",
						},
						Customizations: configv1alpha1.CustomizationRules{
							Retention: &configv1alpha1.LocalValueRetention{LuaScript: "first"},
						},
					},
				},
			},
			want: map[schema.GroupVersionKind]CustomAccessor{
				{Group: "apps", Version: "v1", Kind: "Deployment"}: &resourceCustomAccessor{
					retention: &configv1alpha1.LocalValueRetention{LuaScript: "first"},
				},
			},
		},
		{
			name: "overlapping customizations for same GVK",
			configs: []*configv1alpha1.ResourceInterpreterCustomization{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "customization01"},
					Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
						Target: configv1alpha1.CustomizationTarget{
							APIVersion: appsv1.SchemeGroupVersion.String(),
							Kind:       "Deployment",
						},
						Customizations: configv1alpha1.CustomizationRules{
							Retention:       &configv1alpha1.LocalValueRetention{LuaScript: "first-retention"},
							ReplicaResource: &configv1alpha1.ReplicaResourceRequirement{LuaScript: "first-replica"},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "customization02"},
					Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
						Target: configv1alpha1.CustomizationTarget{
							APIVersion: appsv1.SchemeGroupVersion.String(),
							Kind:       "Deployment",
						},
						Customizations: configv1alpha1.CustomizationRules{
							Retention:        &configv1alpha1.LocalValueRetention{LuaScript: "second-retention"},
							StatusReflection: &configv1alpha1.StatusReflection{LuaScript: "second-status"},
						},
					},
				},
			},
			want: map[schema.GroupVersionKind]CustomAccessor{
				{Group: "apps", Version: "v1", Kind: "Deployment"}: &resourceCustomAccessor{
					retention:        &configv1alpha1.LocalValueRetention{LuaScript: "first-retention"},
					replicaResource:  &configv1alpha1.ReplicaResourceRequirement{LuaScript: "first-replica"},
					statusReflection: &configv1alpha1.StatusReflection{LuaScript: "second-status"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configManager := &interpreterConfigManager{}
			configManager.configuration.Store(make(map[schema.GroupVersionKind]CustomAccessor))

			configManager.LoadConfig(tt.configs)

			got := configManager.CustomAccessors()

			if len(got) != len(tt.want) {
				t.Errorf("LoadConfig() got %d accessors, want %d", len(got), len(tt.want))
			}

			for gvk, wantAccessor := range tt.want {
				gotAccessor, exists := got[gvk]
				if !exists {
					t.Errorf("LoadConfig() missing accessor for GVK %v", gvk)
					continue
				}

				if !reflect.DeepEqual(gotAccessor, wantAccessor) {
					t.Errorf("LoadConfig() accessor for GVK %v = %v, want %v", gvk, gotAccessor, wantAccessor)
				}
			}

			if !configManager.initialSynced.Load() {
				t.Errorf("LoadConfig() should set initialSynced to true")
			}
		})
	}
}

func Test_interpreterConfigManager_updateConfiguration(t *testing.T) {
	tests := []struct {
		name           string
		setupManager   func() *interpreterConfigManager
		wantErr        bool
		expectedErrMsg string
	}{
		{
			name: "informer not initialized",
			setupManager: func() *interpreterConfigManager {
				return &interpreterConfigManager{
					informer: nil,
				}
			},
			wantErr:        true,
			expectedErrMsg: "informer manager is not configured",
		},
		{
			name: "informer not synced",
			setupManager: func() *interpreterConfigManager {
				mockInformer := &mockSingleClusterInformerManager{
					isSynced: false,
				}
				return &interpreterConfigManager{
					informer: mockInformer,
				}
			},
			wantErr:        true,
			expectedErrMsg: "informer of ResourceInterpreterCustomization not synced",
		},
		{
			name: "lister list error",
			setupManager: func() *interpreterConfigManager {
				mockInformer := &mockSingleClusterInformerManager{
					isSynced: true,
				}
				mockLister := &mockGenericLister{
					listErr: errors.New("list error"),
				}
				return &interpreterConfigManager{
					informer: mockInformer,
					lister:   mockLister,
				}
			},
			wantErr:        true,
			expectedErrMsg: "list error",
		},
		{
			name: "successful update with empty list",
			setupManager: func() *interpreterConfigManager {
				mockInformer := &mockSingleClusterInformerManager{
					isSynced: true,
				}
				mockLister := &mockGenericLister{
					items: []runtime.Object{},
				}
				manager := &interpreterConfigManager{
					informer: mockInformer,
					lister:   mockLister,
				}
				manager.configuration.Store(make(map[schema.GroupVersionKind]CustomAccessor))
				return manager
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configManager := tt.setupManager()

			err := configManager.updateConfiguration()

			if tt.wantErr {
				if err == nil {
					t.Errorf("updateConfiguration() expected error but got nil")
					return
				}
				if tt.expectedErrMsg != "" && err.Error() != tt.expectedErrMsg {
					t.Errorf("updateConfiguration() error = %v, want %v", err.Error(), tt.expectedErrMsg)
				}
			} else {
				if err != nil {
					t.Errorf("updateConfiguration() unexpected error = %v", err)
				}
			}
		})
	}
}

// Mock implementations for testing
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

type mockGenericLister struct {
	items   []runtime.Object
	listErr error
}

func (m *mockGenericLister) List(_ labels.Selector) ([]runtime.Object, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.items, nil
}

func (m *mockGenericLister) Get(_ string) (runtime.Object, error) {
	return nil, nil
}

func (m *mockGenericLister) ByNamespace(_ string) cache.GenericNamespaceLister {
	return nil
}
