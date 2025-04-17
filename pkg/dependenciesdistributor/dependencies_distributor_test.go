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
	"fmt"
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/keys"
)

type MockAsyncWorker struct {
	queue []interface{}
}

// Note: This is a dummy implementation of Add for testing purposes.
func (m *MockAsyncWorker) Add(item interface{}) {
	// No actual work is done in the mock; we just simulate running
	m.queue = append(m.queue, item)
}

// Note: This is a dummy implementation of AddAfter for testing purposes.
func (m *MockAsyncWorker) AddAfter(item interface{}, duration time.Duration) {
	// No actual work is done in the mock; we just simulate running
	fmt.Printf("%v", duration)
	m.queue = append(m.queue, item)
}

// Note: This is a dummy implementation of Enqueue for testing purposes.
func (m *MockAsyncWorker) Enqueue(obj interface{}) {
	// Assuming KeyFunc is used to generate a key; for simplicity, we use obj directly
	m.queue = append(m.queue, obj)
}

// Note: This is a dummy implementation of Run for testing purposes.
func (m *MockAsyncWorker) Run(ctx context.Context, workerNumber int) {
	// No actual work is done in the mock; we just simulate running
	fmt.Printf("%v", workerNumber)
	fmt.Printf("%v", <-ctx.Done())
}

// GetQueue returns the current state of the queue
func (m *MockAsyncWorker) GetQueue() []interface{} {
	return m.queue
}

func Test_OnUpdate(t *testing.T) {
	type args struct {
		oldObj interface{}
		newObj interface{}
	}
	tests := []struct {
		name          string
		args          args
		wantQueueSize int
	}{
		{
			name: "update the object, specification changed",
			args: args{
				oldObj: &corev1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "bar",
					},
				},
				newObj: &corev1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
			},
			wantQueueSize: 1,
		},
		{
			name: "do not update the object, no specification changed",
			args: args{
				oldObj: &corev1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "bar",
					},
				},
				newObj: &corev1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "bar",
					},
				},
			},
			wantQueueSize: 0,
		},
		{
			name: "no specification changed, labels changed",
			args: args{
				oldObj: &corev1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "bar",
						Labels: map[string]string{
							"app": "test",
						},
					},
				},
				newObj: &corev1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "bar",
						Labels: map[string]string{
							"app": "new-test",
						},
					},
				},
			},
			wantQueueSize: 2,
		},
		{
			name: "specification changed, labels not changed",
			args: args{
				oldObj: &corev1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "bar",
						Labels: map[string]string{
							"app": "test",
						},
					},
				},
				newObj: &corev1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
						Labels: map[string]string{
							"app": "test",
						},
					},
				},
			},
			wantQueueSize: 1,
		},
		{
			name: "specification changed,  more than two elements in labels and not changed",
			args: args{
				oldObj: &corev1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "bar",
						Labels: map[string]string{
							"app":    "test",
							"online": "svc",
						},
					},
				},
				newObj: &corev1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
						Labels: map[string]string{
							"online": "svc",
							"app":    "test",
						},
					},
				},
			},
			wantQueueSize: 1,
		},
		{
			name: "specification changed,  more than two elements in labels and changed",
			args: args{
				oldObj: &corev1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "bar",
						Labels: map[string]string{
							"app":    "test",
							"online": "svc",
						},
					},
				},
				newObj: &corev1.Node{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Node",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
						Labels: map[string]string{
							"online": "svc1",
							"app":    "test",
						},
					},
				},
			},
			wantQueueSize: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWorker := &MockAsyncWorker{}
			d := &DependenciesDistributor{
				resourceProcessor: mockWorker,
			}

			d.OnUpdate(tt.args.oldObj, tt.args.newObj)

			gotQueueSize := len(mockWorker.GetQueue())
			if gotQueueSize != tt.wantQueueSize {
				t.Errorf("OnUpdate() want queue size %v, got %v", tt.wantQueueSize, gotQueueSize)
			}
		})
	}
}

func Test_reconcileResourceTemplate(t *testing.T) {
	type args struct {
		key util.QueueKey
	}
	type fields struct {
		Client client.Client
	}
	tests := []struct {
		name                   string
		args                   args
		fields                 fields
		wantGenericEventLength int
		wantErr                bool
	}{
		{
			name: "reconcile resource template",
			args: args{
				key: &LabelsKey{
					ClusterWideKey: keys.ClusterWideKey{
						Group:     "apps",
						Version:   "v1",
						Kind:      "Deployment",
						Name:      "demo-app",
						Namespace: "test",
					},
					Labels: map[string]string{
						"app": "test",
					},
				},
			},
			fields: fields{
				Client: func() client.Client {
					Scheme := runtime.NewScheme()
					utilruntime.Must(scheme.AddToScheme(Scheme))
					utilruntime.Must(workv1alpha2.Install(Scheme))
					rb := &workv1alpha2.ResourceBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "test-binding",
							Namespace:       "test",
							ResourceVersion: "1000",
							Labels: map[string]string{
								"app": "test",
							},
							Annotations: map[string]string{
								dependenciesAnnotationKey: "[{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"namespace\":\"test\",\"name\":\"demo-app\"}]",
							},
						},
						Spec: workv1alpha2.ResourceBindingSpec{
							Resource: workv1alpha2.ObjectReference{
								APIVersion:      "apps/v1",
								Kind:            "Deployment",
								Namespace:       "test",
								Name:            "demo-app",
								ResourceVersion: "22222",
							},
						},
					}
					return fake.NewClientBuilder().WithScheme(Scheme).WithObjects(rb).Build()
				}(),
			},
			wantGenericEventLength: 1,
			wantErr:                false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DependenciesDistributor{
				Client:       tt.fields.Client,
				genericEvent: make(chan event.TypedGenericEvent[*workv1alpha2.ResourceBinding], 1),
			}

			err := d.reconcileResourceTemplate(tt.args.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("reconcileResourceTemplate() error = %v, wantErr %v", err, tt.wantErr)
			}

			gotGenericEventLength := len(d.genericEvent)
			if gotGenericEventLength != tt.wantGenericEventLength {
				t.Errorf("reconcileResourceTemplate() length of genericEvent = %v, want length %v", gotGenericEventLength, tt.wantGenericEventLength)
			}
		})
	}
}

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

func Test_addFinalizer(t *testing.T) {
	type fields struct {
		Client client.Client
	}
	type args struct {
		independentBinding *workv1alpha2.ResourceBinding
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *workv1alpha2.ResourceBinding
		wantErr bool
	}{
		{
			name: "add finalizer",
			fields: fields{
				Client: func() client.Client {
					Scheme := runtime.NewScheme()
					utilruntime.Must(scheme.AddToScheme(Scheme))
					utilruntime.Must(workv1alpha2.Install(Scheme))
					rb := &workv1alpha2.ResourceBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "test-binding",
							Namespace:       "test",
							ResourceVersion: "1000",
						},
						Spec: workv1alpha2.ResourceBindingSpec{
							Resource: workv1alpha2.ObjectReference{
								APIVersion:      "apps/v1",
								Kind:            "Deployment",
								Namespace:       "fake-ns",
								Name:            "demo-app",
								ResourceVersion: "22222",
							},
						},
					}
					return fake.NewClientBuilder().WithScheme(Scheme).WithObjects(rb).Build()
				}(),
			},
			args: args{
				independentBinding: &workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-binding",
						Namespace:       "test",
						ResourceVersion: "1000",
					},
					Spec: workv1alpha2.ResourceBindingSpec{
						Resource: workv1alpha2.ObjectReference{
							APIVersion:      "apps/v1",
							Kind:            "Deployment",
							Namespace:       "fake-ns",
							Name:            "demo-app",
							ResourceVersion: "22222",
						},
					},
				},
			},
			want: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-binding",
					Namespace:       "test",
					ResourceVersion: "1001",
					Finalizers:      []string{util.BindingDependenciesDistributorFinalizer},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						APIVersion:      "apps/v1",
						Kind:            "Deployment",
						Namespace:       "fake-ns",
						Name:            "demo-app",
						ResourceVersion: "22222",
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DependenciesDistributor{
				Client: tt.fields.Client,
			}
			err := d.addFinalizer(context.Background(), tt.args.independentBinding)
			if (err != nil) != tt.wantErr {
				t.Errorf("addFinalizer() error = %v, wantErr %v", err, tt.wantErr)
			}

			bindingKey := client.ObjectKey{Namespace: tt.args.independentBinding.Namespace, Name: tt.args.independentBinding.Name}
			got := &workv1alpha2.ResourceBinding{}
			err = d.Client.Get(context.Background(), bindingKey, got)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.Get() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Client.Get() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_removeFinalizer(t *testing.T) {
	type fields struct {
		Client client.Client
	}
	type args struct {
		independentBinding *workv1alpha2.ResourceBinding
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *workv1alpha2.ResourceBinding
		wantErr bool
	}{
		{
			name: "remove non-empty finalizer",
			fields: fields{
				Client: func() client.Client {
					Scheme := runtime.NewScheme()
					utilruntime.Must(scheme.AddToScheme(Scheme))
					utilruntime.Must(workv1alpha2.Install(Scheme))
					rb := &workv1alpha2.ResourceBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "test-binding",
							Namespace:       "test",
							ResourceVersion: "1000",
							Finalizers:      []string{util.BindingDependenciesDistributorFinalizer},
						},
						Spec: workv1alpha2.ResourceBindingSpec{
							Resource: workv1alpha2.ObjectReference{
								APIVersion:      "apps/v1",
								Kind:            "Deployment",
								Namespace:       "fake-ns",
								Name:            "demo-app",
								ResourceVersion: "22222",
							},
						},
					}
					return fake.NewClientBuilder().WithScheme(Scheme).WithObjects(rb).Build()
				}(),
			},
			args: args{
				independentBinding: &workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-binding",
						Namespace:       "test",
						ResourceVersion: "1000",
						Finalizers:      []string{util.BindingDependenciesDistributorFinalizer},
					},
					Spec: workv1alpha2.ResourceBindingSpec{
						Resource: workv1alpha2.ObjectReference{
							APIVersion:      "apps/v1",
							Kind:            "Deployment",
							Namespace:       "fake-ns",
							Name:            "demo-app",
							ResourceVersion: "22222",
						},
					},
				},
			},
			want: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-binding",
					Namespace:       "test",
					ResourceVersion: "1001",
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						APIVersion:      "apps/v1",
						Kind:            "Deployment",
						Namespace:       "fake-ns",
						Name:            "demo-app",
						ResourceVersion: "22222",
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DependenciesDistributor{
				Client: tt.fields.Client,
			}

			err := d.removeFinalizer(context.Background(), tt.args.independentBinding)
			if (err != nil) != tt.wantErr {
				t.Errorf("removeFinalizer() error = %v, wantErr %v", err, tt.wantErr)
			}

			bindingKey := client.ObjectKey{Namespace: tt.args.independentBinding.Namespace, Name: tt.args.independentBinding.Name}
			got := &workv1alpha2.ResourceBinding{}
			err = d.Client.Get(context.Background(), bindingKey, got)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.Get() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Client.Get() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_handleIndependentBindingDeletion(t *testing.T) {
	type fields struct {
		Client client.Client
	}
	type args struct {
		id        string
		namespace string
		name      string
	}
	tests := []struct {
		name         string
		fields       fields
		args         args
		wantBindings *workv1alpha2.ResourceBindingList
		wantErr      bool
	}{
		{
			name: "handle independent binding deletion",
			fields: fields{
				Client: func() client.Client {
					Scheme := runtime.NewScheme()
					utilruntime.Must(scheme.AddToScheme(Scheme))
					utilruntime.Must(workv1alpha2.Install(Scheme))
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
				}(),
			},
			args: args{
				id:        "93162d3c-ee8e-4995-9034-05f4d5d2c2b9",
				namespace: "test",
				name:      "test-binding",
			},
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
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DependenciesDistributor{
				Client: tt.fields.Client,
			}
			err := d.handleIndependentBindingDeletion(tt.args.id, tt.args.namespace, tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleIndependentBindingDeletion() error = %v, wantErr %v", err, tt.wantErr)
			}

			existBindings := &workv1alpha2.ResourceBindingList{}
			err = d.Client.List(context.TODO(), existBindings)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleIndependentBindingDeletion(), Client.List() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(existBindings, tt.wantBindings) {
				t.Errorf("handleIndependentBindingDeletion(), Client.List() = %v, want %v", existBindings, tt.wantBindings)
			}
		})
	}
}

func Test_removeOrphanAttachedBindings(t *testing.T) {
	type fields struct {
		Client          client.Client
		DynamicClient   dynamic.Interface
		InformerManager genericmanager.SingleClusterInformerManager
		RESTMapper      meta.RESTMapper
	}
	type args struct {
		independentBinding *workv1alpha2.ResourceBinding
		dependencies       []configv1alpha1.DependentObjectReference
	}
	tests := []struct {
		name         string
		fields       fields
		args         args
		wantBindings *workv1alpha2.ResourceBindingList
		wantErr      bool
	}{
		{
			name: "remove orphan attached bindings",
			fields: fields{
				Client: func() client.Client {
					Scheme := runtime.NewScheme()
					utilruntime.Must(scheme.AddToScheme(Scheme))
					utilruntime.Must(workv1alpha2.Install(Scheme))
					rb := &workv1alpha2.ResourceBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "test-binding-1",
							Namespace:       "test",
							ResourceVersion: "1000",
							Labels: map[string]string{
								"resourcebinding.karmada.io/depended-by-5dbb6dc9c8": "93162d3c-ee8e-4995-9034-05f4d5d2c2b9",
							},
						},
						Spec: workv1alpha2.ResourceBindingSpec{
							Resource: workv1alpha2.ObjectReference{
								APIVersion:      "v1",
								Kind:            "Pod",
								Namespace:       "default",
								Name:            "pod",
								ResourceVersion: "22222",
							},
						},
					}
					return fake.NewClientBuilder().WithScheme(Scheme).WithObjects(rb).Build()
				}(),
				DynamicClient: dynamicfake.NewSimpleDynamicClient(scheme.Scheme),
				InformerManager: func() genericmanager.SingleClusterInformerManager {
					c := dynamicfake.NewSimpleDynamicClient(scheme.Scheme,
						&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default", Labels: map[string]string{"resourcebinding.karmada.io/depended-by-5dbb6dc9c8": "93162d3c-ee8e-4995-9034-05f4d5d2c2b9"}}})
					m := genericmanager.NewSingleClusterInformerManager(context.TODO(), c, 0)
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
				independentBinding: &workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-binding",
						Namespace:       "test",
						ResourceVersion: "1000",
						Labels: map[string]string{
							workv1alpha2.ResourceBindingPermanentIDLabel: "93162d3c-ee8e-4995-9034-05f4d5d2c2b9",
						},
					},
					Spec: workv1alpha2.ResourceBindingSpec{
						Resource: workv1alpha2.ObjectReference{
							APIVersion:      "v1",
							Kind:            "Pod",
							Namespace:       "default",
							Name:            "pod",
							ResourceVersion: "22222",
						},
					},
				},
				dependencies: []configv1alpha1.DependentObjectReference{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  "test",
						Name:       "pod-test",
					},
				},
			},
			wantBindings: &workv1alpha2.ResourceBindingList{
				Items: []workv1alpha2.ResourceBinding{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "test-binding-1",
							Namespace:       "test",
							ResourceVersion: "1001",
						},
						Spec: workv1alpha2.ResourceBindingSpec{
							Resource: workv1alpha2.ObjectReference{
								APIVersion:      "v1",
								Kind:            "Pod",
								Namespace:       "default",
								Name:            "pod",
								ResourceVersion: "22222",
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DependenciesDistributor{
				Client:          tt.fields.Client,
				DynamicClient:   tt.fields.DynamicClient,
				InformerManager: tt.fields.InformerManager,
				RESTMapper:      tt.fields.RESTMapper,
			}
			err := d.removeOrphanAttachedBindings(context.Background(), tt.args.independentBinding, tt.args.dependencies)
			if (err != nil) != tt.wantErr {
				t.Errorf("removeOrphanAttachedBindings() error = %v, wantErr %v", err, tt.wantErr)
			}

			existBindings := &workv1alpha2.ResourceBindingList{}
			err = d.Client.List(context.TODO(), existBindings)
			if (err != nil) != tt.wantErr {
				t.Errorf("removeOrphanAttachedBindings(), Client.List() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(existBindings, tt.wantBindings) {
				t.Errorf("removeOrphanAttachedBindings(), Client.List() = %v, want %v", existBindings, tt.wantBindings)
			}
		})
	}
}

func Test_handleDependentResource(t *testing.T) {
	type fields struct {
		Client          client.Client
		DynamicClient   dynamic.Interface
		InformerManager genericmanager.SingleClusterInformerManager
		RESTMapper      meta.RESTMapper
	}
	type args struct {
		independentBinding *workv1alpha2.ResourceBinding
		dependencies       configv1alpha1.DependentObjectReference
	}
	tests := []struct {
		name        string
		fields      fields
		args        args
		wantBinding *workv1alpha2.ResourceBinding
		wantErr     bool
	}{
		{
			name: "nil label selector, non-empty name",
			fields: fields{
				Client: func() client.Client {
					Scheme := runtime.NewScheme()
					utilruntime.Must(scheme.AddToScheme(Scheme))
					utilruntime.Must(workv1alpha2.Install(Scheme))
					rb := &workv1alpha2.ResourceBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "test-binding",
							Namespace:       "test",
							ResourceVersion: "1000",
							Labels: map[string]string{
								"resourcebinding.karmada.io/depended-by-5dbb6dc9c8": "93162d3c-ee8e-4995-9034-05f4d5d2c2b9",
							},
						},
						Spec: workv1alpha2.ResourceBindingSpec{
							Resource: workv1alpha2.ObjectReference{
								APIVersion:      "v1",
								Kind:            "Pod",
								Namespace:       "default",
								Name:            "pod",
								ResourceVersion: "22222",
								UID:             types.UID("db56a4a6-0dff-465a-b046-2c1dea42a42b"),
							},
							Clusters: []workv1alpha2.TargetCluster{
								{
									Name:     "member1",
									Replicas: 2,
								},
							},
						},
					}
					return fake.NewClientBuilder().WithScheme(Scheme).WithObjects(rb).Build()
				}(),
				DynamicClient: dynamicfake.NewSimpleDynamicClient(scheme.Scheme),
				InformerManager: func() genericmanager.SingleClusterInformerManager {
					c := dynamicfake.NewSimpleDynamicClient(scheme.Scheme,
						&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default", Labels: map[string]string{"resourcebinding.karmada.io/depended-by-5dbb6dc9c8": "93162d3c-ee8e-4995-9034-05f4d5d2c2b9"}}})
					m := genericmanager.NewSingleClusterInformerManager(context.TODO(), c, 0)
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
				independentBinding: &workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-binding",
						Namespace: "test",
						Labels:    map[string]string{workv1alpha2.ResourceBindingPermanentIDLabel: "93162d3c-ee8e-4995-9034-05f4d5d2c2b9"},
					},
					Spec: workv1alpha2.ResourceBindingSpec{
						Resource: workv1alpha2.ObjectReference{
							APIVersion:      "v1",
							Kind:            "Pod",
							Namespace:       "default",
							Name:            "pod",
							ResourceVersion: "22222",
							UID:             types.UID("db56a4a6-0dff-465a-b046-2c1dea42a42b"),
						},
						Clusters: []workv1alpha2.TargetCluster{
							{
								Name:     "member1",
								Replicas: 2,
							},
						},
					},
				},
				dependencies: configv1alpha1.DependentObjectReference{
					APIVersion: "v1",
					Kind:       "Pod",
					Namespace:  "default",
					Name:       "pod",
				},
			},
			wantBinding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-binding",
					Namespace:       "test",
					ResourceVersion: "1000",
					Labels:          map[string]string{"resourcebinding.karmada.io/depended-by-5dbb6dc9c8": "93162d3c-ee8e-4995-9034-05f4d5d2c2b9"},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						APIVersion:      "v1",
						Kind:            "Pod",
						Namespace:       "default",
						Name:            "pod",
						ResourceVersion: "22222",
						UID:             types.UID("db56a4a6-0dff-465a-b046-2c1dea42a42b"),
					},
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "member1",
							Replicas: 2,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty name, non-nil label selector",
			fields: fields{
				Client: func() client.Client {
					Scheme := runtime.NewScheme()
					utilruntime.Must(scheme.AddToScheme(Scheme))
					utilruntime.Must(workv1alpha2.Install(Scheme))
					rb := &workv1alpha2.ResourceBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "test-binding",
							Namespace:       "test",
							ResourceVersion: "1000",
							Labels: map[string]string{
								"resourcebinding.karmada.io/depended-by-5dbb6dc9c8": "93162d3c-ee8e-4995-9034-05f4d5d2c2b9",
							},
						},
						Spec: workv1alpha2.ResourceBindingSpec{
							Resource: workv1alpha2.ObjectReference{
								APIVersion:      "v1",
								Kind:            "Pod",
								Namespace:       "default",
								Name:            "pod",
								ResourceVersion: "22222",
								UID:             types.UID("db56a4a6-0dff-465a-b046-2c1dea42a42b"),
							},
							Clusters: []workv1alpha2.TargetCluster{
								{
									Name:     "member1",
									Replicas: 2,
								},
							},
						},
					}
					return fake.NewClientBuilder().WithScheme(Scheme).WithObjects(rb).Build()
				}(),
				DynamicClient: dynamicfake.NewSimpleDynamicClient(scheme.Scheme),
				InformerManager: func() genericmanager.SingleClusterInformerManager {
					c := dynamicfake.NewSimpleDynamicClient(scheme.Scheme,
						&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default", Labels: map[string]string{"resourcebinding.karmada.io/depended-by-5dbb6dc9c8": "93162d3c-ee8e-4995-9034-05f4d5d2c2b9"}}})
					m := genericmanager.NewSingleClusterInformerManager(context.TODO(), c, 0)
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
				independentBinding: &workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-binding",
						Namespace: "test",
						Labels:    map[string]string{workv1alpha2.ResourceBindingPermanentIDLabel: "93162d3c-ee8e-4995-9034-05f4d5d2c2b9"},
					},
					Spec: workv1alpha2.ResourceBindingSpec{
						Resource: workv1alpha2.ObjectReference{
							APIVersion:      "v1",
							Kind:            "Pod",
							Namespace:       "default",
							Name:            "pod",
							ResourceVersion: "22222",
							UID:             types.UID("db56a4a6-0dff-465a-b046-2c1dea42a42b"),
						},
						Clusters: []workv1alpha2.TargetCluster{
							{
								Name:     "member1",
								Replicas: 2,
							},
						},
					},
				},
				dependencies: configv1alpha1.DependentObjectReference{
					APIVersion:    "v1",
					Kind:          "Pod",
					Namespace:     "default",
					LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"resourcebinding.karmada.io/depended-by-5dbb6dc9c8": "93162d3c-ee8e-4995-9034-05f4d5d2c2b9"}},
				},
			},
			wantBinding: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-binding",
					Namespace:       "test",
					ResourceVersion: "1000",
					Labels:          map[string]string{"resourcebinding.karmada.io/depended-by-5dbb6dc9c8": "93162d3c-ee8e-4995-9034-05f4d5d2c2b9"},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						APIVersion:      "v1",
						Kind:            "Pod",
						Namespace:       "default",
						Name:            "pod",
						ResourceVersion: "22222",
						UID:             types.UID("db56a4a6-0dff-465a-b046-2c1dea42a42b"),
					},
					Clusters: []workv1alpha2.TargetCluster{
						{
							Name:     "member1",
							Replicas: 2,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "nil label selector, empty name",
			fields: fields{
				Client: func() client.Client {
					return fake.NewClientBuilder().Build()
				}(),
				DynamicClient: dynamicfake.NewSimpleDynamicClient(scheme.Scheme),
				InformerManager: func() genericmanager.SingleClusterInformerManager {
					c := dynamicfake.NewSimpleDynamicClient(scheme.Scheme,
						&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default", Labels: map[string]string{"resourcebinding.karmada.io/depended-by-5dbb6dc9c8": "93162d3c-ee8e-4995-9034-05f4d5d2c2b9"}}})
					m := genericmanager.NewSingleClusterInformerManager(context.TODO(), c, 0)
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
				dependencies: configv1alpha1.DependentObjectReference{
					APIVersion: "v1",
					Kind:       "Pod",
					Namespace:  "default",
				},
			},
			wantBinding: &workv1alpha2.ResourceBinding{},
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DependenciesDistributor{
				Client:          tt.fields.Client,
				DynamicClient:   tt.fields.DynamicClient,
				InformerManager: tt.fields.InformerManager,
				RESTMapper:      tt.fields.RESTMapper,
			}
			err := d.handleDependentResource(context.Background(), tt.args.independentBinding, tt.args.dependencies)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleDependentResource() error = %v, wantErr %v", err, tt.wantErr)
			}

			existBinding := &workv1alpha2.ResourceBinding{}
			bindingKey := client.ObjectKeyFromObject(tt.args.independentBinding)
			err = d.Client.Get(context.TODO(), bindingKey, existBinding)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleDependentResource(), Client.Get() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(existBinding, tt.wantBinding) {
				t.Errorf("handleDependentResource(), Client.Get() = %v, want %v", existBinding, tt.wantBinding)
			}
		})
	}
}

func Test_recordDependencies(t *testing.T) {
	type fields struct {
		Client client.Client
	}
	type args struct {
		independentBinding *workv1alpha2.ResourceBinding
		dependencies       []configv1alpha1.DependentObjectReference
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *workv1alpha2.ResourceBinding
		wantErr bool
	}{
		{
			name: "record updated dependencies",
			fields: fields{
				Client: func() client.Client {
					Scheme := runtime.NewScheme()
					utilruntime.Must(scheme.AddToScheme(Scheme))
					utilruntime.Must(workv1alpha2.Install(Scheme))
					rb := &workv1alpha2.ResourceBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "test-binding",
							Namespace:       "test",
							ResourceVersion: "1000",
						},
						Spec: workv1alpha2.ResourceBindingSpec{
							Resource: workv1alpha2.ObjectReference{
								APIVersion:      "v1",
								Kind:            "Pod",
								Namespace:       "default",
								Name:            "pod",
								ResourceVersion: "22222",
							},
						},
					}
					return fake.NewClientBuilder().WithScheme(Scheme).WithObjects(rb).Build()
				}(),
			},
			args: args{
				independentBinding: &workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-binding",
						Namespace:       "test",
						ResourceVersion: "1000",
					},
					Spec: workv1alpha2.ResourceBindingSpec{
						Resource: workv1alpha2.ObjectReference{
							APIVersion:      "v1",
							Kind:            "Pod",
							Namespace:       "default",
							Name:            "pod",
							ResourceVersion: "22222",
						},
					},
				},
				dependencies: []configv1alpha1.DependentObjectReference{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  "default",
						Name:       "pod",
					},
				},
			},
			want: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-binding",
					Namespace:       "test",
					ResourceVersion: "1001",
					Annotations: map[string]string{
						dependenciesAnnotationKey: "[{\"apiVersion\":\"v1\",\"kind\":\"Pod\",\"namespace\":\"default\",\"name\":\"pod\"}]",
					},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						APIVersion:      "v1",
						Kind:            "Pod",
						Namespace:       "default",
						Name:            "pod",
						ResourceVersion: "22222",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "no need to record non-updated dependencies",
			fields: fields{
				Client: func() client.Client {
					Scheme := runtime.NewScheme()
					utilruntime.Must(scheme.AddToScheme(Scheme))
					utilruntime.Must(workv1alpha2.Install(Scheme))
					rb := &workv1alpha2.ResourceBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "test-binding",
							Namespace:       "test",
							ResourceVersion: "1000",
							Annotations: map[string]string{
								dependenciesAnnotationKey: "[{\"apiVersion\":\"v1\",\"kind\":\"Pod\",\"namespace\":\"default\",\"name\":\"pod\"}]",
							},
						},
						Spec: workv1alpha2.ResourceBindingSpec{
							Resource: workv1alpha2.ObjectReference{
								APIVersion:      "v1",
								Kind:            "Pod",
								Namespace:       "default",
								Name:            "pod",
								ResourceVersion: "22222",
							},
						},
					}
					return fake.NewClientBuilder().WithScheme(Scheme).WithObjects(rb).Build()
				}(),
			},
			args: args{
				independentBinding: &workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-binding",
						Namespace:       "test",
						ResourceVersion: "1000",
						Annotations: map[string]string{
							dependenciesAnnotationKey: "[{\"apiVersion\":\"v1\",\"kind\":\"Pod\",\"namespace\":\"default\",\"name\":\"pod\"}]",
						},
					},
					Spec: workv1alpha2.ResourceBindingSpec{
						Resource: workv1alpha2.ObjectReference{
							APIVersion:      "v1",
							Kind:            "Pod",
							Namespace:       "default",
							Name:            "pod",
							ResourceVersion: "22222",
						},
					},
				},
				dependencies: []configv1alpha1.DependentObjectReference{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  "default",
						Name:       "pod",
					},
				},
			},
			want: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-binding",
					Namespace:       "test",
					ResourceVersion: "1000",
					Annotations: map[string]string{
						dependenciesAnnotationKey: "[{\"apiVersion\":\"v1\",\"kind\":\"Pod\",\"namespace\":\"default\",\"name\":\"pod\"}]",
					},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						APIVersion:      "v1",
						Kind:            "Pod",
						Namespace:       "default",
						Name:            "pod",
						ResourceVersion: "22222",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "non-matching independent binding",
			fields: fields{
				Client: func() client.Client {
					Scheme := runtime.NewScheme()
					utilruntime.Must(scheme.AddToScheme(Scheme))
					utilruntime.Must(workv1alpha2.Install(Scheme))
					rb := &workv1alpha2.ResourceBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "test-binding",
							Namespace:       "test",
							ResourceVersion: "1000",
							Annotations: map[string]string{
								dependenciesAnnotationKey: "[{\"apiVersion\":\"v1\",\"kind\":\"Pod\",\"namespace\":\"default\",\"name\":\"pod\"}]",
							},
						},
						Spec: workv1alpha2.ResourceBindingSpec{
							Resource: workv1alpha2.ObjectReference{
								APIVersion:      "v1",
								Kind:            "Pod",
								Namespace:       "default",
								Name:            "pod",
								ResourceVersion: "22222",
							},
						},
					}
					return fake.NewClientBuilder().WithScheme(Scheme).WithObjects(rb).Build()
				}(),
			},
			args: args{
				independentBinding: &workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-binding",
						Namespace:       "test",
						ResourceVersion: "999",
					},
					Spec: workv1alpha2.ResourceBindingSpec{
						Resource: workv1alpha2.ObjectReference{
							APIVersion:      "v1",
							Kind:            "Pod",
							Namespace:       "default",
							Name:            "pod",
							ResourceVersion: "22222",
						},
					},
				},
				dependencies: []configv1alpha1.DependentObjectReference{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  "default",
						Name:       "pod",
					},
				},
			},
			want: &workv1alpha2.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-binding",
					Namespace:       "test",
					ResourceVersion: "1001",
					Annotations: map[string]string{
						dependenciesAnnotationKey: "[{\"apiVersion\":\"v1\",\"kind\":\"Pod\",\"namespace\":\"default\",\"name\":\"pod\"}]",
					},
				},
				Spec: workv1alpha2.ResourceBindingSpec{
					Resource: workv1alpha2.ObjectReference{
						APIVersion:      "v1",
						Kind:            "Pod",
						Namespace:       "default",
						Name:            "pod",
						ResourceVersion: "22222",
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DependenciesDistributor{
				Client: tt.fields.Client,
			}
			err := d.recordDependencies(context.Background(), tt.args.independentBinding, tt.args.dependencies)
			if (err != nil) != tt.wantErr {
				t.Errorf("recordDependencies() error = %v, wantErr %v", err, tt.wantErr)
			}

			bindingKey := client.ObjectKey{Namespace: tt.args.independentBinding.Namespace, Name: tt.args.independentBinding.Name}
			got := &workv1alpha2.ResourceBinding{}
			err = d.Client.Get(context.Background(), bindingKey, got)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.Get() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Client.Get() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_findOrphanAttachedBindings(t *testing.T) {
	type fields struct {
		Client          client.Client
		DynamicClient   dynamic.Interface
		InformerManager genericmanager.SingleClusterInformerManager
		RESTMapper      meta.RESTMapper
	}
	type args struct {
		independentBinding *workv1alpha2.ResourceBinding
		dependencies       []configv1alpha1.DependentObjectReference
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []*workv1alpha2.ResourceBinding
		wantErr bool
	}{
		{
			name: "find orphan attached bindings - matching dependency",
			fields: fields{
				Client: func() client.Client {
					Scheme := runtime.NewScheme()
					utilruntime.Must(scheme.AddToScheme(Scheme))
					utilruntime.Must(workv1alpha2.Install(Scheme))
					rb := &workv1alpha2.ResourceBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "test-binding-1",
							Namespace:       "test",
							ResourceVersion: "1000",
							Labels: map[string]string{
								"resourcebinding.karmada.io/depended-by-5dbb6dc9c8": "93162d3c-ee8e-4995-9034-05f4d5d2c2b9",
							},
						},
						Spec: workv1alpha2.ResourceBindingSpec{
							Resource: workv1alpha2.ObjectReference{
								APIVersion:      "v1",
								Kind:            "Pod",
								Namespace:       "default",
								Name:            "pod",
								ResourceVersion: "22222",
							},
						},
					}
					return fake.NewClientBuilder().WithScheme(Scheme).WithObjects(rb).Build()
				}(),
				DynamicClient: dynamicfake.NewSimpleDynamicClient(scheme.Scheme),
				InformerManager: func() genericmanager.SingleClusterInformerManager {
					c := dynamicfake.NewSimpleDynamicClient(scheme.Scheme,
						&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default", Labels: map[string]string{"resourcebinding.karmada.io/depended-by-5dbb6dc9c8": "93162d3c-ee8e-4995-9034-05f4d5d2c2b9"}}})
					m := genericmanager.NewSingleClusterInformerManager(context.TODO(), c, 0)
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
				independentBinding: &workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-binding",
						Namespace:       "test",
						ResourceVersion: "1000",
						Labels: map[string]string{
							workv1alpha2.ResourceBindingPermanentIDLabel: "93162d3c-ee8e-4995-9034-05f4d5d2c2b9",
						},
					},
					Spec: workv1alpha2.ResourceBindingSpec{
						Resource: workv1alpha2.ObjectReference{
							APIVersion:      "v1",
							Kind:            "Pod",
							Namespace:       "default",
							Name:            "pod",
							ResourceVersion: "22222",
						},
					},
				},
				dependencies: []configv1alpha1.DependentObjectReference{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  "default",
						Name:       "pod-test",
					},
				},
			},
			want: []*workv1alpha2.ResourceBinding{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-binding-1",
						Namespace:       "test",
						ResourceVersion: "1000",
						Labels: map[string]string{
							"resourcebinding.karmada.io/depended-by-5dbb6dc9c8": "93162d3c-ee8e-4995-9034-05f4d5d2c2b9",
						},
					},
					Spec: workv1alpha2.ResourceBindingSpec{
						Resource: workv1alpha2.ObjectReference{
							APIVersion:      "v1",
							Kind:            "Pod",
							Namespace:       "default",
							Name:            "pod",
							ResourceVersion: "22222",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "find orphan attached bindings - non matching dependency",
			fields: fields{
				Client: func() client.Client {
					Scheme := runtime.NewScheme()
					utilruntime.Must(scheme.AddToScheme(Scheme))
					utilruntime.Must(workv1alpha2.Install(Scheme))
					rb := &workv1alpha2.ResourceBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "test-binding-1",
							Namespace:       "test",
							ResourceVersion: "1000",
							Labels: map[string]string{
								"resourcebinding.karmada.io/depended-by-5dbb6dc9c8": "93162d3c-ee8e-4995-9034-05f4d5d2c2b9",
							},
						},
						Spec: workv1alpha2.ResourceBindingSpec{
							Resource: workv1alpha2.ObjectReference{
								APIVersion:      "v1",
								Kind:            "Pod",
								Namespace:       "default",
								Name:            "pod",
								ResourceVersion: "22222",
							},
						},
					}

					return fake.NewClientBuilder().WithScheme(Scheme).WithObjects(rb).Build()
				}(),
				DynamicClient: dynamicfake.NewSimpleDynamicClient(scheme.Scheme),
				InformerManager: func() genericmanager.SingleClusterInformerManager {
					c := dynamicfake.NewSimpleDynamicClient(scheme.Scheme,
						&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default", Labels: map[string]string{"resourcebinding.karmada.io/depended-by-5dbb6dc9c8": "93162d3c-ee8e-4995-9034-05f4d5d2c2b9"}}})
					m := genericmanager.NewSingleClusterInformerManager(context.TODO(), c, 0)
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
				independentBinding: &workv1alpha2.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-binding",
						Namespace:       "test",
						ResourceVersion: "1000",
						Labels: map[string]string{
							workv1alpha2.ResourceBindingPermanentIDLabel: "93162d3c-ee8e-4995-9034-05f4d5d2c2b9",
						},
					},
					Spec: workv1alpha2.ResourceBindingSpec{
						Resource: workv1alpha2.ObjectReference{
							APIVersion:      "v1",
							Kind:            "Pod",
							Namespace:       "default",
							Name:            "pod",
							ResourceVersion: "22222",
						},
					},
				},
				dependencies: []configv1alpha1.DependentObjectReference{
					{
						APIVersion: "v1",
						Kind:       "Pod",
						Namespace:  "test",
						Name:       "pod-test",
					},
				},
			},
			want: []*workv1alpha2.ResourceBinding{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "test-binding-1",
						Namespace:       "test",
						ResourceVersion: "1000",
						Labels: map[string]string{
							"resourcebinding.karmada.io/depended-by-5dbb6dc9c8": "93162d3c-ee8e-4995-9034-05f4d5d2c2b9",
						},
					},
					Spec: workv1alpha2.ResourceBindingSpec{
						Resource: workv1alpha2.ObjectReference{
							APIVersion:      "v1",
							Kind:            "Pod",
							Namespace:       "default",
							Name:            "pod",
							ResourceVersion: "22222",
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DependenciesDistributor{
				Client:          tt.fields.Client,
				DynamicClient:   tt.fields.DynamicClient,
				InformerManager: tt.fields.InformerManager,
				RESTMapper:      tt.fields.RESTMapper,
			}
			got, err := d.findOrphanAttachedBindings(context.Background(), tt.args.independentBinding, tt.args.dependencies)
			if (err != nil) != tt.wantErr {
				t.Errorf("findOrphanAttachedBindings() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findOrphanAttachedBindings() got = %v, want %v", got, tt.want)
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
					m := genericmanager.NewSingleClusterInformerManager(context.TODO(), c, 0)
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
					m := genericmanager.NewSingleClusterInformerManager(context.TODO(), c, 0)
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
