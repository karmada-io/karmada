package configmanager

import (
	"context"
	"reflect"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
			stopCh := ctx.Done()

			client := fake.NewSimpleDynamicClient(gclient.NewSchema(), tt.args.customizations...)
			informer := genericmanager.NewSingleClusterInformerManager(client, 0, stopCh)
			configManager := NewInterpreterConfigManager(informer)

			informer.Start()
			defer informer.Stop()

			informer.WaitForCacheSync()

			if !cache.WaitForCacheSync(stopCh, configManager.HasSynced) {
				t.Errorf("informer has not been synced")
			}

			gotAccessors := configManager.LuaScriptAccessors()
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
