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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
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
					OwnerReferences: []metav1.OwnerReference{
						{
							UID:        "foo-bar",
							Controller: ptr.To[bool](true),
						},
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
					OwnerReferences: []metav1.OwnerReference{
						{
							UID:        "foo-bar",
							Controller: ptr.To[bool](true),
						},
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
						OwnerReferences: []metav1.OwnerReference{
							{
								UID:        "foo-bar",
								Controller: ptr.To[bool](true),
							},
						},
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
					OwnerReferences: []metav1.OwnerReference{
						{
							UID:        "foo-bar",
							Controller: ptr.To[bool](true),
						},
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
					OwnerReferences: []metav1.OwnerReference{
						{
							UID:        "foo-bar",
							Controller: ptr.To[bool](true),
						},
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
		{
			name: "update attached binding with ConflictResolution",
			attachedBinding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-binding",
					Namespace:       "test",
					ResourceVersion: "1000",
					Labels:          map[string]string{"app": "nginx"},
					OwnerReferences: []metav1.OwnerReference{
						{
							UID:        "foo-bar",
							Controller: ptr.To[bool](true),
						},
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
					ConflictResolution: policyv1alpha1.ConflictOverwrite,
				},
			},
			wantErr: false,
			wantBinding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-binding",
					Namespace:       "test",
					ResourceVersion: "1001",
					Labels:          map[string]string{"app": "nginx", "foo": "bar"},
					OwnerReferences: []metav1.OwnerReference{
						{
							UID:        "foo-bar",
							Controller: ptr.To[bool](true),
						},
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
					ConflictResolution: policyv1alpha1.ConflictOverwrite,
				},
			},
			setupClient: func() client.Client {
				rb := &workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-binding",
						Namespace:       "test",
						ResourceVersion: "1000",
						Labels:          map[string]string{"foo": "bar"},
						OwnerReferences: []metav1.OwnerReference{
							{
								UID:        "foo-bar",
								Controller: ptr.To[bool](true),
							},
						},
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
			name: "update attached binding which is being deleted",
			attachedBinding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-binding",
					Namespace:       "test",
					ResourceVersion: "1000",
					Labels:          map[string]string{"app": "nginx"},
					OwnerReferences: []metav1.OwnerReference{
						{
							UID:        "foo-bar",
							Controller: ptr.To[bool](true),
						},
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
							Namespace: "test-1",
							Name:      "test-binding-1",
							Clusters: []workv1alpha2.TargetCluster{
								{
									Name:     "foo",
									Replicas: 1,
								},
							},
						},
					},
					ConflictResolution: policyv1alpha1.ConflictOverwrite,
				},
			},
			wantErr: true,
			wantBinding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-binding",
					Namespace:       "test",
					ResourceVersion: "1001",
					Labels:          map[string]string{"app": "nginx", "foo": "bar"},
					OwnerReferences: []metav1.OwnerReference{
						{
							UID:        "foo-bar",
							Controller: ptr.To[bool](true),
						},
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
					ConflictResolution: policyv1alpha1.ConflictOverwrite,
				},
			},
			setupClient: func() client.Client {
				rb := &workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-binding",
						Namespace:       "test",
						ResourceVersion: "1000",
						Labels:          map[string]string{"foo": "bar"},
						OwnerReferences: []metav1.OwnerReference{
							{
								UID:        "bar-foo",
								Controller: ptr.To[bool](true),
							},
						},
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
			err := d.createOrUpdateAttachedBinding(tt.attachedBinding)
			if (err != nil) != tt.wantErr {
				t.Errorf("createOrUpdateAttachedBinding() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			existBinding := &workv1alpha2.ResourceBinding{}
			bindingKey := client.ObjectKeyFromObject(tt.attachedBinding)
			err = d.Client.Get(context.TODO(), bindingKey, existBinding)
			if err != nil {
				t.Errorf("createOrUpdateAttachedBinding(), Client.Get() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(existBinding, tt.wantBinding) {
				t.Errorf("createOrUpdateAttachedBinding(), Client.Get() = %v, want %v", existBinding, tt.wantBinding)
			}
		})
	}
}
