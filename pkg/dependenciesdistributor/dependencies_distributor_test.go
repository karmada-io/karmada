/*
Copyright 2023 The Karmada Authors.

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

package dependenciesdistributor

import (
	"context"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
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
						dependenciesAnnotationKey: "[{\"apiVersion\":\"example-stgzr.karmada.io/v1alpha1\",\"kind\":\"Foot5zmh\",\"namespace\":\"karmadatest-vpvll\",\"name\":\"cr-fxzq6\"}]",
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
						dependenciesAnnotationKey: "[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"namespace\":\"karmadatest-h46wh\",\"name\":\"configmap-8w426\"}]",
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
						dependenciesAnnotationKey: "[{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"namespace\":\"test\",\"labelSelector\":{\"matchExpressions\":[{\"key\":\"app\",\"operator\":\"In\",\"values\":[\"test\"]}]}}]",
					}},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesWithBindingDependencies(tt.args.objectKey, tt.args.referenceBinding)
			if got != tt.want {
				t.Errorf("matchesWithBindingDependencies() got = %v, want %v", got, tt.want)
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
			got, err := d.isOrphanAttachedBindings(context.Background(), tt.args.dependencies, tt.args.dependencyIndexes, tt.args.attachedBinding)
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

func Test_listAttachedBindings(t *testing.T) {
	Scheme := runtime.NewScheme()
	utilruntime.Must(scheme.AddToScheme(Scheme))
	utilruntime.Must(workv1alpha2.Install(Scheme))
	tests := []struct {
		name             string
		bindingID        string
		bindingNamespace string
		bindingName      string
		wantErr          bool
		wantBindings     []*workv1alpha2.ResourceBinding
		setupClient      func() client.Client
	}{
		{
			name:             "list attached bindings",
			bindingID:        "93162d3c-ee8e-4995-9034-05f4d5d2c2b9",
			bindingNamespace: "test",
			bindingName:      "test-binding",
			wantErr:          false,
			wantBindings: []*workv1alpha2.ResourceBinding{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-binding",
						Namespace:       "test",
						ResourceVersion: "1000",
						Labels: map[string]string{
							"app": "nginx",
							"resourcebinding.karmada.io/depended-by-5dbb6dc9c8": "93162d3c-ee8e-4995-9034-05f4d5d2c2b9",
						},
					},
					Spec: workv1alpha2.ResourceBindingSpec{
						Resource: workv1alpha2.ObjectReference{
							APIVersion:      "apps/v1",
							Kind:            "Deployment",
							Namespace:       "fake-ns",
							Name:            "demo-app",
							ResourceVersion: "22222",
						},
						RequiredBy: []workv1alpha2.BindingSnapshot{
							{
								Namespace: "test",
								Name:      "test-binding",
								Clusters: []workv1alpha2.TargetCluster{
									{
										Name:     "foo",
										Replicas: 1,
									},
								},
							},
							{
								Namespace: "default-1",
								Name:      "default-binding-1",
								Clusters: []workv1alpha2.TargetCluster{
									{
										Name:     "member1",
										Replicas: 2,
									},
								},
							},
							{
								Namespace: "test",
								Name:      "test-binding",
								Clusters: []workv1alpha2.TargetCluster{
									{
										Name:     "bar",
										Replicas: 1,
									},
								},
							},
						},
					},
				},
			},
			setupClient: func() client.Client {
				rb := &workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-binding",
						Namespace:       "test",
						ResourceVersion: "1000",
						Labels: map[string]string{
							"app": "nginx",
							"resourcebinding.karmada.io/depended-by-5dbb6dc9c8": "93162d3c-ee8e-4995-9034-05f4d5d2c2b9",
						},
					},
					Spec: workv1alpha2.ResourceBindingSpec{
						Resource: workv1alpha2.ObjectReference{
							APIVersion:      "apps/v1",
							Kind:            "Deployment",
							Namespace:       "fake-ns",
							Name:            "demo-app",
							ResourceVersion: "22222",
						},
						RequiredBy: []workv1alpha2.BindingSnapshot{
							{
								Namespace: "test",
								Name:      "test-binding",
								Clusters: []workv1alpha2.TargetCluster{
									{
										Name:     "foo",
										Replicas: 1,
									},
								},
							},
							{
								Namespace: "default-1",
								Name:      "default-binding-1",
								Clusters: []workv1alpha2.TargetCluster{
									{
										Name:     "member1",
										Replicas: 2,
									},
								},
							},
							{
								Namespace: "test",
								Name:      "test-binding",
								Clusters: []workv1alpha2.TargetCluster{
									{
										Name:     "bar",
										Replicas: 1,
									},
								},
							},
						},
					},
				}
				return fake.NewClientBuilder().WithScheme(Scheme).WithObjects(rb).Build()
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DependenciesDistributor{
				Client: tt.setupClient(),
			}
			gotBindings, err := d.listAttachedBindings(tt.bindingID, tt.bindingNamespace, tt.bindingName)
			if (err != nil) != tt.wantErr {
				t.Errorf("listAttachedBindings() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(gotBindings, tt.wantBindings) {
				t.Errorf("listAttachedBindings() = %v, want %v", gotBindings, tt.wantBindings)
			}
		})
	}
}

func Test_removeScheduleResultFromAttachedBindings(t *testing.T) {
	Scheme := runtime.NewScheme()
	utilruntime.Must(scheme.AddToScheme(Scheme))
	utilruntime.Must(workv1alpha2.Install(Scheme))
	tests := []struct {
		name             string
		bindingNamespace string
		bindingName      string
		attachedBindings []*workv1alpha2.ResourceBinding
		wantErr          bool
		wantBindings     *workv1alpha2.ResourceBindingList
		setupClient      func() client.Client
	}{
		{
			name:             "remove schedule result from attached bindings",
			bindingNamespace: "test",
			bindingName:      "test-binding",
			attachedBindings: []*workv1alpha2.ResourceBinding{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-binding",
						Namespace:       "test",
						ResourceVersion: "1000",
						Labels: map[string]string{
							"app": "nginx",
							"resourcebinding.karmada.io/depended-by-5dbb6dc9c8": "93162d3c-ee8e-4995-9034-05f4d5d2c2b9",
						},
					},
					Spec: workv1alpha2.ResourceBindingSpec{
						Resource: workv1alpha2.ObjectReference{
							APIVersion:      "apps/v1",
							Kind:            "Deployment",
							Namespace:       "fake-ns",
							Name:            "demo-app",
							ResourceVersion: "22222",
						},
						RequiredBy: []workv1alpha2.BindingSnapshot{
							{
								Namespace: "test",
								Name:      "test-binding",
								Clusters: []workv1alpha2.TargetCluster{
									{
										Name:     "foo",
										Replicas: 1,
									},
								},
							},
							{
								Namespace: "default-1",
								Name:      "default-binding-1",
								Clusters: []workv1alpha2.TargetCluster{
									{
										Name:     "member1",
										Replicas: 2,
									},
								},
							},
							{
								Namespace: "test",
								Name:      "test-binding",
								Clusters: []workv1alpha2.TargetCluster{
									{
										Name:     "bar",
										Replicas: 1,
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
			wantBindings: &workv1alpha2.ResourceBindingList{
				Items: []workv1alpha2.ResourceBinding{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "test-binding",
							Namespace:       "test",
							ResourceVersion: "1001",
							Labels: map[string]string{
								"app": "nginx",
							},
						},
						Spec: workv1alpha2.ResourceBindingSpec{
							Resource: workv1alpha2.ObjectReference{
								APIVersion:      "apps/v1",
								Kind:            "Deployment",
								Namespace:       "fake-ns",
								Name:            "demo-app",
								ResourceVersion: "22222",
							},
							RequiredBy: []workv1alpha2.BindingSnapshot{
								{
									Namespace: "default-1",
									Name:      "default-binding-1",
									Clusters: []workv1alpha2.TargetCluster{
										{
											Name:     "member1",
											Replicas: 2,
										},
									},
								},
							},
						},
					},
				},
			},
			setupClient: func() client.Client {
				rb := &workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-binding",
						Namespace:       "test",
						ResourceVersion: "1000",
						Labels: map[string]string{
							"app": "nginx",
							"resourcebinding.karmada.io/depended-by-5dbb6dc9c8": "93162d3c-ee8e-4995-9034-05f4d5d2c2b9",
						},
					},
					Spec: workv1alpha2.ResourceBindingSpec{
						Resource: workv1alpha2.ObjectReference{
							APIVersion:      "apps/v1",
							Kind:            "Deployment",
							Namespace:       "fake-ns",
							Name:            "demo-app",
							ResourceVersion: "22222",
						},
						RequiredBy: []workv1alpha2.BindingSnapshot{
							{
								Namespace: "test",
								Name:      "test-binding",
								Clusters: []workv1alpha2.TargetCluster{
									{
										Name:     "foo",
										Replicas: 1,
									},
								},
							},
							{
								Namespace: "default-1",
								Name:      "default-binding-1",
								Clusters: []workv1alpha2.TargetCluster{
									{
										Name:     "member1",
										Replicas: 2,
									},
								},
							},
							{
								Namespace: "test",
								Name:      "test-binding",
								Clusters: []workv1alpha2.TargetCluster{
									{
										Name:     "bar",
										Replicas: 1,
									},
								},
							},
						},
					},
				}
				return fake.NewClientBuilder().WithScheme(Scheme).WithObjects(rb).Build()
			},
		},
		{
			name:             "empty attached bindings",
			bindingNamespace: "test",
			bindingName:      "test-binding",
			attachedBindings: []*workv1alpha2.ResourceBinding{},
			wantErr:          false,
			wantBindings: &workv1alpha2.ResourceBindingList{
				Items: []workv1alpha2.ResourceBinding{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "test-binding",
							Namespace:       "test",
							ResourceVersion: "1000",
							Labels: map[string]string{
								"app": "nginx",
								"resourcebinding.karmada.io/depended-by-5dbb6dc9c8": "93162d3c-ee8e-4995-9034-05f4d5d2c2b9",
							},
						},
						Spec: workv1alpha2.ResourceBindingSpec{
							Resource: workv1alpha2.ObjectReference{
								APIVersion:      "apps/v1",
								Kind:            "Deployment",
								Namespace:       "fake-ns",
								Name:            "demo-app",
								ResourceVersion: "22222",
							},
							RequiredBy: []workv1alpha2.BindingSnapshot{
								{
									Namespace: "test",
									Name:      "test-binding",
									Clusters: []workv1alpha2.TargetCluster{
										{
											Name:     "foo",
											Replicas: 1,
										},
									},
								},
								{
									Namespace: "default-1",
									Name:      "default-binding-1",
									Clusters: []workv1alpha2.TargetCluster{
										{
											Name:     "member1",
											Replicas: 2,
										},
									},
								},
								{
									Namespace: "test",
									Name:      "test-binding",
									Clusters: []workv1alpha2.TargetCluster{
										{
											Name:     "bar",
											Replicas: 1,
										},
									},
								},
							},
						},
					},
				},
			},
			setupClient: func() client.Client {
				rb := &workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-binding",
						Namespace:       "test",
						ResourceVersion: "1000",
						Labels: map[string]string{
							"app": "nginx",
							"resourcebinding.karmada.io/depended-by-5dbb6dc9c8": "93162d3c-ee8e-4995-9034-05f4d5d2c2b9",
						},
					},
					Spec: workv1alpha2.ResourceBindingSpec{
						Resource: workv1alpha2.ObjectReference{
							APIVersion:      "apps/v1",
							Kind:            "Deployment",
							Namespace:       "fake-ns",
							Name:            "demo-app",
							ResourceVersion: "22222",
						},
						RequiredBy: []workv1alpha2.BindingSnapshot{
							{
								Namespace: "test",
								Name:      "test-binding",
								Clusters: []workv1alpha2.TargetCluster{
									{
										Name:     "foo",
										Replicas: 1,
									},
								},
							},
							{
								Namespace: "default-1",
								Name:      "default-binding-1",
								Clusters: []workv1alpha2.TargetCluster{
									{
										Name:     "member1",
										Replicas: 2,
									},
								},
							},
							{
								Namespace: "test",
								Name:      "test-binding",
								Clusters: []workv1alpha2.TargetCluster{
									{
										Name:     "bar",
										Replicas: 1,
									},
								},
							},
						},
					},
				}
				return fake.NewClientBuilder().WithScheme(Scheme).WithObjects(rb).Build()
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DependenciesDistributor{
				Client: tt.setupClient(),
			}
			err := d.removeScheduleResultFromAttachedBindings(tt.bindingNamespace, tt.bindingName, tt.attachedBindings)
			if (err != nil) != tt.wantErr {
				t.Errorf("removeScheduleResultFromAttachedBindings() error = %v, wantErr %v", err, tt.wantErr)
			}
			existBindings := &workv1alpha2.ResourceBindingList{}
			err = d.Client.List(context.TODO(), existBindings)
			if (err != nil) != tt.wantErr {
				t.Errorf("removeScheduleResultFromAttachedBindings(), Client.List() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(existBindings, tt.wantBindings) {
				t.Errorf("removeScheduleResultFromAttachedBindings(), Client.List() = %v, want %v", existBindings, tt.wantBindings)
			}
		})
	}
}

func Test_createOrUpdateAttachedBinding(t *testing.T) {
	Scheme := runtime.NewScheme()
	utilruntime.Must(scheme.AddToScheme(Scheme))
	utilruntime.Must(workv1alpha2.Install(Scheme))
	tests := []struct {
		name            string
		attachedBinding *workv1alpha2.ResourceBinding
		wantErr         bool
		wantBinding     *workv1alpha2.ResourceBinding
		setupClient     func() client.Client
	}{
		{
			name: "update attached binding",
			attachedBinding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-binding",
					Namespace:       "test",
					ResourceVersion: "1000",
					Labels:          map[string]string{"app": "nginx"},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						APIVersion:      "apps/v1",
						Kind:            "Deployment",
						Namespace:       "fake-ns",
						Name:            "demo-app",
						ResourceVersion: "22222",
					},
					RequiredBy: []workv1alpha2.BindingSnapshot{
						{
							Namespace: "test-1",
							Name:      "test-binding-1",
							Clusters: []workv1alpha2.TargetCluster{
								{
									Name:     "foo",
									Replicas: 1,
								},
							},
						},
						{
							Namespace: "default-2",
							Name:      "default-binding-2",
							Clusters: []workv1alpha2.TargetCluster{
								{
									Name:     "member2",
									Replicas: 4,
								},
							},
						},
						{
							Namespace: "test-2",
							Name:      "test-binding-2",
							Clusters: []workv1alpha2.TargetCluster{
								{
									Name:     "bar",
									Replicas: 1,
								},
							},
						},
					},
				},
			},
			wantErr: false,
			wantBinding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-binding",
					Namespace:       "test",
					ResourceVersion: "1001",
					Labels:          map[string]string{"app": "nginx", "foo": "bar"},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						APIVersion:      "apps/v1",
						Kind:            "Deployment",
						Namespace:       "fake-ns",
						Name:            "demo-app",
						ResourceVersion: "22222",
					},
					RequiredBy: []workv1alpha2.BindingSnapshot{
						{
							Namespace: "default-1",
							Name:      "default-binding-1",
							Clusters: []workv1alpha2.TargetCluster{
								{
									Name:     "member1",
									Replicas: 2,
								},
							},
						},
						{
							Namespace: "default-2",
							Name:      "default-binding-2",
							Clusters: []workv1alpha2.TargetCluster{
								{
									Name:     "member2",
									Replicas: 4,
								},
							},
						},
						{
							Namespace: "default-3",
							Name:      "default-binding-3",
							Clusters: []workv1alpha2.TargetCluster{
								{
									Name:     "member3",
									Replicas: 4,
								},
							},
						},
						{
							Namespace: "test-1",
							Name:      "test-binding-1",
							Clusters: []workv1alpha2.TargetCluster{
								{
									Name:     "foo",
									Replicas: 1,
								},
							},
						},
						{
							Namespace: "test-2",
							Name:      "test-binding-2",
							Clusters: []workv1alpha2.TargetCluster{
								{
									Name:     "bar",
									Replicas: 1,
								},
							},
						},
					},
				},
			},
			setupClient: func() client.Client {
				rb := &workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-binding",
						Namespace:       "test",
						ResourceVersion: "1000",
						Labels:          map[string]string{"foo": "bar"},
					},
					Spec: workv1alpha2.ResourceBindingSpec{
						RequiredBy: []workv1alpha2.BindingSnapshot{
							{
								Namespace: "default-1",
								Name:      "default-binding-1",
								Clusters: []workv1alpha2.TargetCluster{
									{
										Name:     "member1",
										Replicas: 2,
									},
								},
							},
							{
								Namespace: "default-2",
								Name:      "default-binding-2",
								Clusters: []workv1alpha2.TargetCluster{
									{
										Name:     "member2",
										Replicas: 3,
									},
								},
							},
							{
								Namespace: "default-3",
								Name:      "default-binding-3",
								Clusters: []workv1alpha2.TargetCluster{
									{
										Name:     "member3",
										Replicas: 4,
									},
								},
							},
						},
					},
				}
				return fake.NewClientBuilder().WithScheme(Scheme).WithObjects(rb).Build()
			},
		},
		{
			name: "create attached binding",
			attachedBinding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "test",
					Labels:    map[string]string{"app": "nginx"},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						APIVersion:      "apps/v1",
						Kind:            "Deployment",
						Namespace:       "fake-ns",
						Name:            "demo-app",
						ResourceVersion: "22222",
					},
					RequiredBy: []workv1alpha2.BindingSnapshot{
						{
							Namespace: "test-1",
							Name:      "test-binding-1",
							Clusters: []workv1alpha2.TargetCluster{
								{
									Name:     "foo",
									Replicas: 1,
								},
							},
						},
						{
							Namespace: "default-2",
							Name:      "default-binding-2",
							Clusters: []workv1alpha2.TargetCluster{
								{
									Name:     "member2",
									Replicas: 4,
								},
							},
						},
						{
							Namespace: "test-2",
							Name:      "test-binding-2",
							Clusters: []workv1alpha2.TargetCluster{
								{
									Name:     "bar",
									Replicas: 1,
								},
							},
						},
					},
				},
			},
			wantErr: false,
			wantBinding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-binding",
					Namespace:       "test",
					ResourceVersion: "1",
					Labels:          map[string]string{"app": "nginx"},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						APIVersion:      "apps/v1",
						Kind:            "Deployment",
						Namespace:       "fake-ns",
						Name:            "demo-app",
						ResourceVersion: "22222",
					},
					RequiredBy: []workv1alpha2.BindingSnapshot{
						{
							Namespace: "test-1",
							Name:      "test-binding-1",
							Clusters: []workv1alpha2.TargetCluster{
								{
									Name:     "foo",
									Replicas: 1,
								},
							},
						},
						{
							Namespace: "default-2",
							Name:      "default-binding-2",
							Clusters: []workv1alpha2.TargetCluster{
								{
									Name:     "member2",
									Replicas: 4,
								},
							},
						},
						{
							Namespace: "test-2",
							Name:      "test-binding-2",
							Clusters: []workv1alpha2.TargetCluster{
								{
									Name:     "bar",
									Replicas: 1,
								},
							},
						},
					},
				},
			},
			setupClient: func() client.Client {
				return fake.NewClientBuilder().WithScheme(Scheme).Build()
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DependenciesDistributor{
				Client: tt.setupClient(),
			}
			err := d.createOrUpdateAttachedBinding(tt.attachedBinding)
			if (err != nil) != tt.wantErr {
				t.Errorf("createOrUpdateAttachedBinding() error = %v, wantErr %v", err, tt.wantErr)
			}
			existBinding := &workv1alpha2.ResourceBinding{}
			bindingKey := client.ObjectKeyFromObject(tt.attachedBinding)
			err = d.Client.Get(context.TODO(), bindingKey, existBinding)
			if (err != nil) != tt.wantErr {
				t.Errorf("createOrUpdateAttachedBinding(), Client.Get() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(existBinding, tt.wantBinding) {
				t.Errorf("createOrUpdateAttachedBinding(), Client.Get() = %v, want %v", existBinding, tt.wantBinding)
			}
		})
	}
}

func Test_buildAttachedBinding(t *testing.T) {
	blockOwnerDeletion := true
	isController := true
	tests := []struct {
		name                string
		independentBinding  *workv1alpha2.ResourceBinding
		object              *unstructured.Unstructured
		wantAttachedBinding *workv1alpha2.ResourceBinding
	}{
		{
			name: "build attached binding",
			independentBinding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "test",
					Labels:    map[string]string{workv1alpha2.ResourceBindingPermanentIDLabel: "93162d3c-ee8e-4995-9034-05f4d5d2c2b9"},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "member1",
							Replicas: 2,
						},
					},
				},
			},
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":            "demo-app",
						"namespace":       "fake-ns",
						"resourceVersion": "22222",
						"uid":             "db56a4a6-0dff-465a-b046-2c1dea42a42b",
					},
				},
			},
			wantAttachedBinding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "demo-app-deployment",
					Namespace: "test",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "apps/v1",
							Kind:               "Deployment",
							Name:               "demo-app",
							UID:                "db56a4a6-0dff-465a-b046-2c1dea42a42b",
							BlockOwnerDeletion: &blockOwnerDeletion,
							Controller:         &isController,
						},
					},
					Labels:     map[string]string{"resourcebinding.karmada.io/depended-by-5dbb6dc9c8": "93162d3c-ee8e-4995-9034-05f4d5d2c2b9"},
					Finalizers: []string{util.BindingControllerFinalizer},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						APIVersion:      "apps/v1",
						Kind:            "Deployment",
						Namespace:       "fake-ns",
						Name:            "demo-app",
						ResourceVersion: "22222",
					},
					RequiredBy: []workv1alpha2.BindingSnapshot{
						{
							Namespace: "test",
							Name:      "test-binding",
							Clusters: []workv1alpha2.TargetCluster{
								{
									Name:     "member1",
									Replicas: 2,
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAttachedBinding := buildAttachedBinding(tt.independentBinding, tt.object)
			if !reflect.DeepEqual(gotAttachedBinding, tt.wantAttachedBinding) {
				t.Errorf("buildAttachedBinding() = %v, want %v", gotAttachedBinding, tt.wantAttachedBinding)
			}
		})
	}
}

func Test_mergeBindingSnapshot(t *testing.T) {
	tests := []struct {
		name                string
		existSnapshot       []workv1alpha2.BindingSnapshot
		newSnapshot         []workv1alpha2.BindingSnapshot
		expectExistSnapshot []workv1alpha2.BindingSnapshot
	}{
		{
			name: "merge binding snapshot",
			existSnapshot: []workv1alpha2.BindingSnapshot{
				{
					Namespace: "default-1",
					Name:      "default-binding-1",
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "member1",
							Replicas: 2,
						},
					},
				},
				{
					Namespace: "default-2",
					Name:      "default-binding-2",
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "member2",
							Replicas: 3,
						},
					},
				},
				{
					Namespace: "default-3",
					Name:      "default-binding-3",
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "member3",
							Replicas: 4,
						},
					},
				},
			},
			newSnapshot: []workv1alpha2.BindingSnapshot{
				{
					Namespace: "test-1",
					Name:      "test-binding-1",
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "foo",
							Replicas: 1,
						},
					},
				},
				{
					Namespace: "default-2",
					Name:      "default-binding-2",
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "member2",
							Replicas: 4,
						},
					},
				},
				{
					Namespace: "test-2",
					Name:      "test-binding-2",
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "bar",
							Replicas: 1,
						},
					},
				},
			},
			expectExistSnapshot: []workv1alpha2.BindingSnapshot{
				{
					Namespace: "default-1",
					Name:      "default-binding-1",
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "member1",
							Replicas: 2,
						},
					},
				},
				{
					Namespace: "default-2",
					Name:      "default-binding-2",
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "member2",
							Replicas: 4,
						},
					},
				},
				{
					Namespace: "default-3",
					Name:      "default-binding-3",
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "member3",
							Replicas: 4,
						},
					},
				},
				{
					Namespace: "test-1",
					Name:      "test-binding-1",
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "foo",
							Replicas: 1,
						},
					},
				},
				{
					Namespace: "test-2",
					Name:      "test-binding-2",
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "bar",
							Replicas: 1,
						},
					},
				},
			},
		},
		{
			name:          "no existing binding snapshot to merge",
			existSnapshot: []workv1alpha2.BindingSnapshot{},
			newSnapshot: []workv1alpha2.BindingSnapshot{
				{
					Namespace: "default-1",
					Name:      "default-binding-1",
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "member1",
							Replicas: 2,
						},
					},
				},
				{
					Namespace: "default-2",
					Name:      "default-binding-2",
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "member2",
							Replicas: 3,
						},
					},
				},
				{
					Namespace: "default-3",
					Name:      "default-binding-3",
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "member3",
							Replicas: 4,
						},
					},
				},
			},
			expectExistSnapshot: []workv1alpha2.BindingSnapshot{
				{
					Namespace: "default-1",
					Name:      "default-binding-1",
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "member1",
							Replicas: 2,
						},
					},
				},
				{
					Namespace: "default-2",
					Name:      "default-binding-2",
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "member2",
							Replicas: 3,
						},
					},
				},
				{
					Namespace: "default-3",
					Name:      "default-binding-3",
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "member3",
							Replicas: 4,
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotExistSnapshot := mergeBindingSnapshot(tt.existSnapshot, tt.newSnapshot)
			if !reflect.DeepEqual(gotExistSnapshot, tt.expectExistSnapshot) {
				t.Errorf("mergeBindingSnapshot() = %v, want %v", gotExistSnapshot, tt.expectExistSnapshot)
			}
		})
	}
}

func Test_deleteBindingFromSnapshot(t *testing.T) {
	tests := []struct {
		name                string
		bindingNamespace    string
		bindingName         string
		existSnapshot       []workv1alpha2.BindingSnapshot
		expectExistSnapshot []workv1alpha2.BindingSnapshot
	}{
		{
			name:             "delete matching binding from snapshot",
			bindingNamespace: "test",
			bindingName:      "test-binding",
			existSnapshot: []workv1alpha2.BindingSnapshot{
				{
					Namespace: "default-1",
					Name:      "default-binding-1",
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "member1",
							Replicas: 2,
						},
					},
				},
				{
					Namespace: "test",
					Name:      "test-binding",
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "foo",
							Replicas: 1,
						},
					},
				},
				{
					Namespace: "default-2",
					Name:      "default-binding-2",
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "member2",
							Replicas: 3,
						},
					},
				},
				{
					Namespace: "test",
					Name:      "test-binding",
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "bar",
							Replicas: 1,
						},
					},
				},
				{
					Namespace: "default-3",
					Name:      "default-binding-3",
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "member3",
							Replicas: 4,
						},
					},
				},
			},
			expectExistSnapshot: []workv1alpha2.BindingSnapshot{
				{
					Namespace: "default-1",
					Name:      "default-binding-1",
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "member1",
							Replicas: 2,
						},
					},
				},
				{
					Namespace: "default-2",
					Name:      "default-binding-2",
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "member2",
							Replicas: 3,
						},
					},
				},
				{
					Namespace: "default-3",
					Name:      "default-binding-3",
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "member3",
							Replicas: 4,
						},
					},
				},
			},
		},
		{
			name:             "non-matching binding from snapshot",
			bindingNamespace: "test",
			bindingName:      "test-binding",
			existSnapshot: []workv1alpha2.BindingSnapshot{
				{
					Namespace: "default-1",
					Name:      "default-binding-1",
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "member1",
							Replicas: 2,
						},
					},
				},
				{
					Namespace: "default-2",
					Name:      "default-binding-2",
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "member2",
							Replicas: 3,
						},
					},
				},
				{
					Namespace: "default-3",
					Name:      "default-binding-3",
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "member3",
							Replicas: 4,
						},
					},
				},
			},
			expectExistSnapshot: []workv1alpha2.BindingSnapshot{
				{
					Namespace: "default-1",
					Name:      "default-binding-1",
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "member1",
							Replicas: 2,
						},
					},
				},
				{
					Namespace: "default-2",
					Name:      "default-binding-2",
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "member2",
							Replicas: 3,
						},
					},
				},
				{
					Namespace: "default-3",
					Name:      "default-binding-3",
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "member3",
							Replicas: 4,
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotExistSnapshot := deleteBindingFromSnapshot(tt.bindingNamespace, tt.bindingName, tt.existSnapshot)
			if !reflect.DeepEqual(gotExistSnapshot, tt.expectExistSnapshot) {
				t.Errorf("deleteBindingFromSnapshot() = %v, want %v", gotExistSnapshot, tt.expectExistSnapshot)
			}
		})
	}
}
