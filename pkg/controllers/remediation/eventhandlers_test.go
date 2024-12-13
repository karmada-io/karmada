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

package remediation

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllertest"
	"sigs.k8s.io/controller-runtime/pkg/event"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	remedyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/remedy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/gclient"
)

func Test_clusterEventHandler(t *testing.T) {
	type args struct {
		operation string
		q         workqueue.TypedRateLimitingInterface[controllerruntime.Request]
		obj       *clusterv1alpha1.Cluster
		oldObj    *clusterv1alpha1.Cluster
	}
	tests := []struct {
		name     string
		args     args
		wantQLen int
	}{
		{
			name: "create event",
			args: args{
				operation: "Create",
				q:         &controllertest.TypedQueue[controllerruntime.Request]{TypedInterface: workqueue.NewTyped[controllerruntime.Request]()},
				obj: &clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{Name: "member1"},
				},
			},
			wantQLen: 0,
		},
		{
			name: "delete event",
			args: args{
				operation: "Delete",
				q:         &controllertest.TypedQueue[controllerruntime.Request]{TypedInterface: workqueue.NewTyped[controllerruntime.Request]()},
				obj: &clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{Name: "member1"},
				},
			},
			wantQLen: 0,
		},
		{
			name: "update event: equal cluster condition",
			args: args{
				operation: "Update",
				q:         &controllertest.TypedQueue[controllerruntime.Request]{TypedInterface: workqueue.NewTyped[controllerruntime.Request]()},
				obj: &clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{Name: "member1"},
					Status: clusterv1alpha1.ClusterStatus{
						Conditions: []metav1.Condition{
							{
								Type:   "Ready",
								Status: metav1.ConditionFalse,
							},
						},
					},
				},
				oldObj: &clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{Name: "member1"},
					Status: clusterv1alpha1.ClusterStatus{
						Conditions: []metav1.Condition{
							{
								Type:   "Ready",
								Status: metav1.ConditionFalse,
							},
						},
					},
				},
			},
			wantQLen: 0,
		},
		{
			name: "update event: not equal cluster condition",
			args: args{
				operation: "Update",
				q:         &controllertest.TypedQueue[controllerruntime.Request]{TypedInterface: workqueue.NewTyped[controllerruntime.Request]()},
				obj: &clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{Name: "member1"},
					Status: clusterv1alpha1.ClusterStatus{
						Conditions: []metav1.Condition{
							{
								Type:   "Ready",
								Status: metav1.ConditionFalse,
							},
						},
					},
				},
				oldObj: &clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{Name: "member1"},
					Status: clusterv1alpha1.ClusterStatus{
						Conditions: []metav1.Condition{
							{
								Type:   "Ready",
								Status: metav1.ConditionTrue,
							},
						},
					},
				},
			},
			wantQLen: 1,
		},
		{
			name: "generic event",
			args: args{
				operation: "Generic",
				q:         &controllertest.TypedQueue[controllerruntime.Request]{TypedInterface: workqueue.NewTyped[controllerruntime.Request]()},
				obj: &clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{Name: "member1"},
				},
			},
			wantQLen: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queue := tt.args.q
			h := newClusterEventHandler()
			switch tt.args.operation {
			case "Create":
				createEvent := event.TypedCreateEvent[*clusterv1alpha1.Cluster]{Object: tt.args.obj}
				h.Create(context.TODO(), createEvent, queue)
			case "Delete":
				deleteEvent := event.TypedDeleteEvent[*clusterv1alpha1.Cluster]{Object: tt.args.obj}
				h.Delete(context.TODO(), deleteEvent, queue)
			case "Update":
				updateEvent := event.TypedUpdateEvent[*clusterv1alpha1.Cluster]{ObjectNew: tt.args.obj, ObjectOld: tt.args.oldObj}
				h.Update(context.TODO(), updateEvent, queue)
			case "Generic":
				genericEvent := event.TypedGenericEvent[*clusterv1alpha1.Cluster]{Object: tt.args.obj}
				h.Generic(context.TODO(), genericEvent, queue)
			default:
				t.Errorf("no support operation %v", tt.args.operation)
				return
			}

			if got := queue.Len(); got != tt.wantQLen {
				t.Errorf("clusterEventHandler process queue length = %v, want %v", got, tt.wantQLen)
			}
		})
	}
}

func Test_remedyEventHandler(t *testing.T) {
	type args struct {
		operation string
		obj       *remedyv1alpha1.Remedy
		oldObj    *remedyv1alpha1.Remedy
		client    client.Client
	}
	tests := []struct {
		name        string
		args        args
		wantChanLen int
	}{
		{
			name: "create event: remedy with clusterAffinity",
			args: args{
				operation: "Create",
				obj: &remedyv1alpha1.Remedy{
					ObjectMeta: metav1.ObjectMeta{Name: "foo-01"},
					Spec: remedyv1alpha1.RemedySpec{
						ClusterAffinity: &remedyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"member1", "member2"}},
					},
				},
				client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).Build(),
			},
			wantChanLen: 2,
		},
		{
			name: "create event: remedy with nil clusterAffinity",
			args: args{
				operation: "Create",
				obj: &remedyv1alpha1.Remedy{
					ObjectMeta: metav1.ObjectMeta{Name: "foo-01"},
					Spec:       remedyv1alpha1.RemedySpec{},
				},
				client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "member1"}},
					&clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "member2"}},
					&clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "member3"}},
				).Build(),
			},
			wantChanLen: 3,
		},
		{
			name: "update event: old and new remedy all have nil clusterAffinity",
			args: args{
				operation: "Update",
				obj: &remedyv1alpha1.Remedy{
					ObjectMeta: metav1.ObjectMeta{Name: "foo-01"},
					Spec:       remedyv1alpha1.RemedySpec{},
				},
				oldObj: &remedyv1alpha1.Remedy{
					ObjectMeta: metav1.ObjectMeta{Name: "foo-01"},
					Spec:       remedyv1alpha1.RemedySpec{},
				},
				client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "member1"}},
					&clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "member2"}},
					&clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "member3"}},
				).Build(),
			},
			wantChanLen: 3,
		},
		{
			name: "update event: one of the old and new remedy have nil clusterAffinity",
			args: args{
				operation: "Update",
				obj: &remedyv1alpha1.Remedy{
					ObjectMeta: metav1.ObjectMeta{Name: "foo-01"},
					Spec:       remedyv1alpha1.RemedySpec{},
				},
				oldObj: &remedyv1alpha1.Remedy{
					ObjectMeta: metav1.ObjectMeta{Name: "foo-01"},
					Spec: remedyv1alpha1.RemedySpec{
						ClusterAffinity: &remedyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"member1"}},
					},
				},
				client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "member1"}},
					&clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "member2"}},
					&clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "member3"}},
				).Build(),
			},
			wantChanLen: 3,
		},
		{
			name: "update event: the old and new remedy clusterAffinity changed",
			args: args{
				operation: "Update",
				obj: &remedyv1alpha1.Remedy{
					ObjectMeta: metav1.ObjectMeta{Name: "foo-01"},
					Spec: remedyv1alpha1.RemedySpec{
						ClusterAffinity: &remedyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"member1", "member2"}},
					},
				},
				oldObj: &remedyv1alpha1.Remedy{
					ObjectMeta: metav1.ObjectMeta{Name: "foo-01"},
					Spec: remedyv1alpha1.RemedySpec{
						ClusterAffinity: &remedyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"member1", "member3"}},
					},
				},
				client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).Build(),
			},
			wantChanLen: 3,
		},
		{
			name: "delete event: the remedy with clusterAffinity",
			args: args{
				operation: "Delete",
				obj: &remedyv1alpha1.Remedy{
					ObjectMeta: metav1.ObjectMeta{Name: "foo-01"},
					Spec: remedyv1alpha1.RemedySpec{
						ClusterAffinity: &remedyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"member1", "member2"}},
					},
				},
				client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).Build(),
			},
			wantChanLen: 2,
		},
		{
			name: "delete event: the remedy with nil clusterAffinity",
			args: args{
				operation: "Delete",
				obj: &remedyv1alpha1.Remedy{
					ObjectMeta: metav1.ObjectMeta{Name: "foo-01"},
					Spec:       remedyv1alpha1.RemedySpec{},
				},
				client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "member1"}},
					&clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "member2"}},
					&clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "member3"}},
				).Build(),
			},
			wantChanLen: 3,
		},
		{
			name: "generic event",
			args: args{
				operation: "Generic",
				obj: &remedyv1alpha1.Remedy{
					ObjectMeta: metav1.ObjectMeta{Name: "foo-01"},
					Spec: remedyv1alpha1.RemedySpec{
						ClusterAffinity: &remedyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"member1", "member2"}},
					},
				},
				client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "member1"}},
					&clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "member2"}},
					&clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "member3"}},
				).Build(),
			},
			wantChanLen: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clusterChan := make(chan event.TypedGenericEvent[*clusterv1alpha1.Cluster])
			h := newRemedyEventHandler(clusterChan, tt.args.client)
			switch tt.args.operation {
			case "Create":
				go func() {
					createEvent := event.TypedCreateEvent[*remedyv1alpha1.Remedy]{Object: tt.args.obj}
					h.Create(context.TODO(), createEvent, nil)
					close(clusterChan)
				}()
			case "Delete":
				go func() {
					deleteEvent := event.TypedDeleteEvent[*remedyv1alpha1.Remedy]{Object: tt.args.obj}
					h.Delete(context.TODO(), deleteEvent, nil)
					close(clusterChan)
				}()
			case "Update":
				go func() {
					updateEvent := event.TypedUpdateEvent[*remedyv1alpha1.Remedy]{ObjectNew: tt.args.obj, ObjectOld: tt.args.oldObj}
					h.Update(context.TODO(), updateEvent, nil)
					close(clusterChan)
				}()
			case "Generic":
				go func() {
					genericEvent := event.TypedGenericEvent[*remedyv1alpha1.Remedy]{Object: tt.args.obj}
					h.Generic(context.TODO(), genericEvent, nil)
					close(clusterChan)
				}()
			default:
				t.Errorf("no support operation %v", tt.args.operation)
				return
			}

			got := 0
			for range clusterChan {
				got++
			}
			if got != tt.wantChanLen {
				t.Errorf("remedyEventHandler process chan length = %v, want %v", got, tt.wantChanLen)
			}
		})
	}
}
