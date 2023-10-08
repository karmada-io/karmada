package dependenciesdistributor

import (
	"context"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
)

func Test_dependentObjectReferenceMatches(t *testing.T) {
	type args struct {
		objectKey        *LabelsKey
		referenceBinding *workv1alpha2.ResourceBinding
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test custom resource",
			args: args{
				objectKey: &LabelsKey{
					ClusterWideKey: keys.ClusterWideKey{
						Group:     "example-stgzr.karmada.io",
						Version:   "v1alpha1",
						Kind:      "Foot5zmh",
						Namespace: "karmadatest-vpvll",
						Name:      "cr-fxzq6",
					},
					Labels: nil,
				},
				referenceBinding: &workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
						bindingDependenciesAnnotationKey: "[{\"apiVersion\":\"example-stgzr.karmada.io/v1alpha1\",\"kind\":\"Foot5zmh\",\"namespace\":\"karmadatest-vpvll\",\"name\":\"cr-fxzq6\"}]",
					}},
				},
			},
			want: true,
		},
		{
			name: "test configmap",
			args: args{
				objectKey: &LabelsKey{
					ClusterWideKey: keys.ClusterWideKey{
						Group:     "",
						Version:   "v1",
						Kind:      "ConfigMap",
						Namespace: "karmadatest-h46wh",
						Name:      "configmap-8w426",
					},
					Labels: nil,
				},
				referenceBinding: &workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
						bindingDependenciesAnnotationKey: "[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"namespace\":\"karmadatest-h46wh\",\"name\":\"configmap-8w426\"}]",
					}},
				},
			},
			want: true,
		},
		{
			name: "test labels",
			args: args{
				objectKey: &LabelsKey{
					ClusterWideKey: keys.ClusterWideKey{
						Group:     "",
						Version:   "v1",
						Kind:      "ConfigMap",
						Namespace: "test",
					},
					Labels: map[string]string{
						"app": "test",
					},
				},
				referenceBinding: &workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
						bindingDependenciesAnnotationKey: "[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"namespace\":\"test\",\"labelSelector\":{\"matchExpressions\":[{\"key\":\"app\",\"operator\":\"In\",\"values\":[\"test\"]}]}}]",
					}},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dependentObjectReferenceMatches(tt.args.objectKey, tt.args.referenceBinding)
			if got != tt.want {
				t.Errorf("dependentObjectReferenceMatches() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDependenciesDistributor_findOrphanAttachedBindingsByDependencies(t *testing.T) {
	type fields struct {
		DynamicClient   dynamic.Interface
		InformerManager genericmanager.SingleClusterInformerManager
		RESTMapper      meta.RESTMapper
	}
	type args struct {
		dependencies      []configv1alpha1.DependentObjectReference
		dependencyIndexes []int
		attachedBinding   *workv1alpha2.ResourceBinding
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "can't match labels",
			fields: fields{
				DynamicClient: dynamicfake.NewSimpleDynamicClient(scheme.Scheme),
				InformerManager: func() genericmanager.SingleClusterInformerManager {
					c := dynamicfake.NewSimpleDynamicClient(scheme.Scheme,
						&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default", Labels: map[string]string{"bar": "bar"}}})
					m := genericmanager.NewSingleClusterInformerManager(c, 0, context.TODO().Done())
					m.Lister(corev1.SchemeGroupVersion.WithResource("pods"))
					m.Start()
					m.WaitForCacheSync()
					return m
				}(),
				RESTMapper: func() meta.RESTMapper {
					m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
					m.Add(corev1.SchemeGroupVersion.WithKind("Pod"), meta.RESTScopeNamespace)
					return m
				}(),
			},
			args: args{
				dependencies: []configv1alpha1.DependentObjectReference{
					{
						APIVersion: "v1",
						Kind:       "Secret",
						Namespace:  "default",
						Name:       "test",
					},
					{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  "default",
						LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
							"bar": "foo",
						}},
					},
				},
				dependencyIndexes: []int{1},
				attachedBinding: &workv1alpha2.ResourceBinding{
					Spec: workv1alpha2.ResourceBindingSpec{
						Resource: workv1alpha2.ObjectReference{
							APIVersion: "v1",
							Kind:       "Pod",
							Namespace:  "default",
							Name:       "pod",
						},
					},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "can't match name",
			fields: fields{
				DynamicClient: dynamicfake.NewSimpleDynamicClient(scheme.Scheme),
				InformerManager: func() genericmanager.SingleClusterInformerManager {
					c := dynamicfake.NewSimpleDynamicClient(scheme.Scheme,
						&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default", Labels: map[string]string{"bar": "foo"}}})
					m := genericmanager.NewSingleClusterInformerManager(c, 0, context.TODO().Done())
					m.Lister(corev1.SchemeGroupVersion.WithResource("pods"))
					m.Start()
					m.WaitForCacheSync()
					return m
				}(),
				RESTMapper: func() meta.RESTMapper {
					m := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
					m.Add(corev1.SchemeGroupVersion.WithKind("Pod"), meta.RESTScopeNamespace)
					return m
				}(),
			},
			args: args{
				dependencies: []configv1alpha1.DependentObjectReference{
					{
						APIVersion: "v1",
						Kind:       "Secret",
						Namespace:  "default",
						Name:       "test",
					},
					{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  "default",
						Name:       "pod",
					},
				},
				dependencyIndexes: []int{1},
				attachedBinding: &workv1alpha2.ResourceBinding{
					Spec: workv1alpha2.ResourceBindingSpec{
						Resource: workv1alpha2.ObjectReference{
							APIVersion: "v1",
							Kind:       "Pod",
							Namespace:  "default",
							Name:       "test2",
						},
					},
				},
			},
			want:    true,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DependenciesDistributor{
				DynamicClient:   tt.fields.DynamicClient,
				InformerManager: tt.fields.InformerManager,
				RESTMapper:      tt.fields.RESTMapper,
			}
			got, err := d.isOrphanAttachedBindings(tt.args.dependencies, tt.args.dependencyIndexes, tt.args.attachedBinding)
			if (err != nil) != tt.wantErr {
				t.Errorf("findOrphanAttachedBindingsByDependencies() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findOrphanAttachedBindingsByDependencies() got = %v, want %v", got, tt.want)
			}
		})
	}
}
