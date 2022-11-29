package configurableinterpreter

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/gclient"
)

func TestConfigurableInterpreter(t *testing.T) {
	const emptyLuaScript = "a=0"

	stopCh := make(chan struct{})
	defer close(stopCh)

	c1 := &configv1alpha1.ResourceInterpreterCustomization{
		ObjectMeta: metav1.ObjectMeta{Name: "c1"},
		Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
			Target: configv1alpha1.CustomizationTarget{
				APIVersion: appsv1.SchemeGroupVersion.String(), Kind: "Deployment",
			},
			Customizations: configv1alpha1.CustomizationRules{
				Retention:       &configv1alpha1.LocalValueRetention{LuaScript: emptyLuaScript},
				ReplicaResource: &configv1alpha1.ReplicaResourceRequirement{LuaScript: emptyLuaScript},
			},
		},
	}

	c2 := &configv1alpha1.ResourceInterpreterCustomization{
		ObjectMeta: metav1.ObjectMeta{Name: "c2"},
		Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
			Target: configv1alpha1.CustomizationTarget{
				APIVersion: appsv1.SchemeGroupVersion.String(), Kind: "Deployment",
			},
			Customizations: configv1alpha1.CustomizationRules{
				ReplicaResource: &configv1alpha1.ReplicaResourceRequirement{LuaScript: emptyLuaScript},
				ReplicaRevision: &configv1alpha1.ReplicaRevision{LuaScript: emptyLuaScript},
			},
		},
	}

	client := fake.NewSimpleDynamicClient(gclient.NewSchema(), c1, c2)
	informer := genericmanager.NewSingleClusterInformerManager(client, 0, stopCh)
	m := NewConfigurableInterpreter(informer)

	informer.Start()
	defer informer.Stop()

	type args struct {
		objGVK    schema.GroupVersionKind
		operation configv1alpha1.InterpreterOperation
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "kind is not configured",
			args: args{
				objGVK: schema.GroupVersionKind{Version: "v1", Kind: "Pod"},
			},
			want: false,
		},
		{
			name: "operation is not configured",
			args: args{
				objGVK:    appsv1.SchemeGroupVersion.WithKind("Deployment"),
				operation: "unmatched",
			},
			want: false,
		},
		{
			name: "configured in c1",
			args: args{
				objGVK:    appsv1.SchemeGroupVersion.WithKind("Deployment"),
				operation: configv1alpha1.InterpreterOperationRetain,
			},
			want: true,
		},
		{
			name: "configured in c2",
			args: args{
				objGVK:    appsv1.SchemeGroupVersion.WithKind("Deployment"),
				operation: configv1alpha1.InterpreterOperationReviseReplica,
			},
			want: true,
		},
		{
			name: "configured in c1 and c2",
			args: args{
				objGVK:    appsv1.SchemeGroupVersion.WithKind("Deployment"),
				operation: configv1alpha1.InterpreterOperationInterpretReplica,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			informer.WaitForCacheSync()
			if got := m.HookEnabled(tt.args.objGVK, tt.args.operation); got != tt.want {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfigurableInterpreter_Unloaded(t *testing.T) {
	stopCh := make(chan struct{})
	defer close(stopCh)
	client := fake.NewSimpleDynamicClient(gclient.NewSchema())
	informer := genericmanager.NewSingleClusterInformerManager(client, 0, stopCh)
	m := NewConfigurableInterpreter(informer)

	got := m.HookEnabled(appsv1.SchemeGroupVersion.WithKind("Deployment"), configv1alpha1.InterpreterOperationInterpretHealth)
	if got {
		t.Errorf("IsEnabled() got %v, want false", got)
	}
}
