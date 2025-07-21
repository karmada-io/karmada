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

package helper

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/gclient"
	"github.com/karmada-io/karmada/pkg/util/names"
)

func TestNewClusterPredicateOnAgent(t *testing.T) {
	type want struct {
		create, update, delete, generic bool
	}
	tests := []struct {
		name string
		obj  client.Object
		want want
	}{
		{
			name: "not matched",
			obj:  &clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "unmatched"}},
			want: want{
				create:  false,
				update:  false,
				delete:  false,
				generic: false,
			},
		},
		{
			name: "matched",
			obj:  &clusterv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "test"}},
			want: want{
				create:  true,
				update:  true,
				delete:  true,
				generic: false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pred := NewClusterPredicateOnAgent("test")
			if got := pred.Create(event.CreateEvent{Object: tt.obj}); got != tt.want.create {
				t.Errorf("Create() got = %v, want %v", got, tt.want.create)
				return
			}
			if got := pred.Update(event.UpdateEvent{ObjectOld: tt.obj, ObjectNew: tt.obj}); got != tt.want.update {
				t.Errorf("Update() got = %v, want %v", got, tt.want.update)
				return
			}
			if got := pred.Delete(event.DeleteEvent{Object: tt.obj}); got != tt.want.delete {
				t.Errorf("Delete() got = %v, want %v", got, tt.want.delete)
				return
			}
			if got := pred.Generic(event.GenericEvent{Object: tt.obj}); got != tt.want.generic {
				t.Errorf("Generic() got = %v, want %v", got, tt.want.generic)
				return
			}
		})
	}
}

func TestWorkWithinPushClusterPredicate(t *testing.T) {
	type args struct {
		mgr controllerruntime.Manager
		obj client.Object
	}
	type want struct {
		create, update, delete, generic bool
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "get cluster name error",
			args: args{
				mgr: &fakeManager{client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
						Spec:       clusterv1alpha1.ClusterSpec{SyncMode: clusterv1alpha1.Push},
					},
				).Build()},
				obj: &workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{Name: "work", Namespace: "cluster"},
				},
			},
			want: want{
				create:  false,
				update:  false,
				delete:  false,
				generic: false,
			},
		},
		{
			name: "cluster not found",
			args: args{
				mgr: &fakeManager{client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects().Build()},
				obj: &workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{Name: "work", Namespace: names.ExecutionSpacePrefix + "cluster"},
				},
			},
			want: want{
				create:  false,
				update:  false,
				delete:  false,
				generic: false,
			},
		},
		{
			name: "cluster is pull mode",
			args: args{
				mgr: &fakeManager{client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
						Spec:       clusterv1alpha1.ClusterSpec{SyncMode: clusterv1alpha1.Pull},
					},
				).Build()},
				obj: &workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{Name: "work", Namespace: names.ExecutionSpacePrefix + "cluster"},
				},
			},
			want: want{
				create:  false,
				update:  false,
				delete:  false,
				generic: false,
			},
		},
		{
			name: "matched",
			args: args{
				mgr: &fakeManager{client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
						Spec:       clusterv1alpha1.ClusterSpec{SyncMode: clusterv1alpha1.Push},
					},
				).Build()},
				obj: &workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{Name: "work", Namespace: names.ExecutionSpacePrefix + "cluster"},
				},
			},
			want: want{
				create:  true,
				update:  true,
				delete:  true,
				generic: false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pred := WorkWithinPushClusterPredicate(tt.args.mgr)
			if got := pred.Create(event.CreateEvent{Object: tt.args.obj}); got != tt.want.create {
				t.Errorf("Create() got = %v, want %v", got, tt.want.create)
				return
			}
			if got := pred.Update(event.UpdateEvent{ObjectOld: tt.args.obj, ObjectNew: tt.args.obj}); got != tt.want.update {
				t.Errorf("Update() got = %v, want %v", got, tt.want.update)
				return
			}
			if got := pred.Delete(event.DeleteEvent{Object: tt.args.obj}); got != tt.want.delete {
				t.Errorf("Delete() got = %v, want %v", got, tt.want.delete)
				return
			}
			if got := pred.Generic(event.GenericEvent{Object: tt.args.obj}); got != tt.want.generic {
				t.Errorf("Generic() got = %v, want %v", got, tt.want.generic)
				return
			}
		})
	}
}

func TestWorkWithinPushClusterPredicate_Update(t *testing.T) {
	mgr := &fakeManager{client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
		&clusterv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "cluster1"},
			Spec:       clusterv1alpha1.ClusterSpec{SyncMode: clusterv1alpha1.Pull},
		},
		&clusterv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "cluster2"},
			Spec:       clusterv1alpha1.ClusterSpec{SyncMode: clusterv1alpha1.Push},
		},
	).Build()}
	unmatched := &workv1alpha1.Work{
		ObjectMeta: metav1.ObjectMeta{
			Name: "work", Namespace: names.ExecutionSpacePrefix + "cluster1",
		},
	}
	matched := &workv1alpha1.Work{
		ObjectMeta: metav1.ObjectMeta{
			Name: "work", Namespace: names.ExecutionSpacePrefix + "cluster2",
		},
	}

	type args struct {
		event event.UpdateEvent
	}

	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "both old and new are unmatched",
			args: args{
				event: event.UpdateEvent{ObjectOld: unmatched, ObjectNew: unmatched},
			},
			want: false,
		},
		{
			name: "old is unmatched, new is matched",
			args: args{
				event: event.UpdateEvent{ObjectOld: unmatched, ObjectNew: matched},
			},
			want: true,
		},
		{
			name: "old is matched, new is unmatched",
			args: args{
				event: event.UpdateEvent{ObjectOld: matched, ObjectNew: unmatched},
			},
			want: true,
		},
		{
			name: "both old and new are matched",
			args: args{
				event: event.UpdateEvent{ObjectOld: matched, ObjectNew: matched},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pred := WorkWithinPushClusterPredicate(mgr)
			if got := pred.Update(tt.args.event); got != tt.want {
				t.Errorf("Update() got = %v, want %v", got, tt.want)
				return
			}
		})
	}
}

func TestNewPredicateForServiceExportController(t *testing.T) {
	type args struct {
		mgr controllerruntime.Manager
		obj client.Object
	}
	type want struct {
		create, update, delete, generic bool
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "object is suppressed",
			args: args{
				mgr: &fakeManager{client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
						Spec:       clusterv1alpha1.ClusterSpec{SyncMode: clusterv1alpha1.Push},
					},
				).Build()},
				obj: &workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{
						Name: "work", Namespace: names.ExecutionSpacePrefix + "cluster",
					},
					Spec: workv1alpha1.WorkSpec{
						SuspendDispatching: ptr.To(true),
					},
				},
			},
			want: want{
				create:  false,
				update:  false,
				delete:  false,
				generic: false,
			},
		},
		{
			name: "get cluster name error",
			args: args{
				mgr: &fakeManager{client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
						Spec:       clusterv1alpha1.ClusterSpec{SyncMode: clusterv1alpha1.Push},
					},
				).Build()},
				obj: &workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{Name: "work", Namespace: "cluster"},
				},
			},
			want: want{
				create:  false,
				update:  false,
				delete:  false,
				generic: false,
			},
		},
		{
			name: "cluster not found",
			args: args{
				mgr: &fakeManager{client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects().Build()},
				obj: &workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{Name: "work", Namespace: names.ExecutionSpacePrefix + "cluster"},
				},
			},
			want: want{
				create:  false,
				update:  false,
				delete:  false,
				generic: false,
			},
		},
		{
			name: "cluster is pull mode",
			args: args{
				mgr: &fakeManager{client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
						Spec:       clusterv1alpha1.ClusterSpec{SyncMode: clusterv1alpha1.Pull},
					},
				).Build()},
				obj: &workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{Name: "work", Namespace: names.ExecutionSpacePrefix + "cluster"},
				},
			},
			want: want{
				create:  false,
				update:  false,
				delete:  false,
				generic: false,
			},
		},
		{
			name: "matched",
			args: args{
				mgr: &fakeManager{client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&clusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
						Spec:       clusterv1alpha1.ClusterSpec{SyncMode: clusterv1alpha1.Push},
					},
				).Build()},
				obj: &workv1alpha1.Work{
					ObjectMeta: metav1.ObjectMeta{Name: "work", Namespace: names.ExecutionSpacePrefix + "cluster"},
				},
			},
			want: want{
				create:  false,
				update:  true,
				delete:  false,
				generic: false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pred := NewPredicateForServiceExportController(tt.args.mgr)
			if got := pred.Create(event.CreateEvent{Object: tt.args.obj}); got != tt.want.create {
				t.Errorf("Create() got = %v, F %v", got, tt.want.create)
				return
			}
			if got := pred.Update(event.UpdateEvent{ObjectOld: tt.args.obj, ObjectNew: tt.args.obj}); got != tt.want.update {
				t.Errorf("Update() got = %v, want %v", got, tt.want.update)
				return
			}
			if got := pred.Delete(event.DeleteEvent{Object: tt.args.obj}); got != tt.want.delete {
				t.Errorf("Delete() got = %v, want %v", got, tt.want.delete)
				return
			}
			if got := pred.Generic(event.GenericEvent{Object: tt.args.obj}); got != tt.want.generic {
				t.Errorf("Generic() got = %v, want %v", got, tt.want.generic)
				return
			}
		})
	}
}

func TestNewPredicateForServiceExportController_Update(t *testing.T) {
	mgr := &fakeManager{client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
		&clusterv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
			Spec:       clusterv1alpha1.ClusterSpec{SyncMode: clusterv1alpha1.Push},
		},
	).Build()}
	unmatched := &workv1alpha1.Work{
		ObjectMeta: metav1.ObjectMeta{
			Name: "work", Namespace: names.ExecutionSpacePrefix + "cluster",
		},
		Spec: workv1alpha1.WorkSpec{
			SuspendDispatching: ptr.To(true),
		},
	}
	matched := &workv1alpha1.Work{
		ObjectMeta: metav1.ObjectMeta{
			Name: "work", Namespace: names.ExecutionSpacePrefix + "cluster",
		},
	}

	type args struct {
		event event.UpdateEvent
	}

	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "both old and new are unmatched",
			args: args{
				event: event.UpdateEvent{ObjectOld: unmatched, ObjectNew: unmatched},
			},
			want: false,
		},
		{
			name: "old is unmatched, new is matched",
			args: args{
				event: event.UpdateEvent{ObjectOld: unmatched, ObjectNew: matched},
			},
			want: true,
		},
		{
			name: "old is matched, new is unmatched",
			args: args{
				event: event.UpdateEvent{ObjectOld: matched, ObjectNew: unmatched},
			},
			want: true,
		},
		{
			name: "both old and new are matched",
			args: args{
				event: event.UpdateEvent{ObjectOld: matched, ObjectNew: matched},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pred := NewPredicateForServiceExportController(mgr)
			if got := pred.Update(tt.args.event); got != tt.want {
				t.Errorf("Update() got = %v, want %v", got, tt.want)
				return
			}
		})
	}
}

func TestNewPredicateForServiceExportControllerOnAgent(t *testing.T) {
	pred := NewPredicateForServiceExportControllerOnAgent("cluster")
	type want struct {
		create, update, delete, generic bool
	}
	tests := []struct {
		name string
		obj  client.Object
		want want
	}{
		{
			name: "object is suppressed",
			obj: &workv1alpha1.Work{
				ObjectMeta: metav1.ObjectMeta{
					Name: "work", Namespace: names.ExecutionSpacePrefix + "cluster",
				},
				Spec: workv1alpha1.WorkSpec{
					SuspendDispatching: ptr.To(true),
				},
			},
			want: want{
				create:  false,
				update:  false,
				delete:  false,
				generic: false,
			},
		},
		{
			name: "get cluster name error",
			obj: &workv1alpha1.Work{ObjectMeta: metav1.ObjectMeta{
				Name: "work", Namespace: "cluster",
			}},
			want: want{
				create:  false,
				update:  false,
				delete:  false,
				generic: false,
			},
		},
		{
			name: "cluster name unmatched",
			obj: &workv1alpha1.Work{ObjectMeta: metav1.ObjectMeta{
				Name: "work", Namespace: names.ExecutionSpacePrefix + "unmatched",
			}},
			want: want{
				create:  false,
				update:  false,
				delete:  false,
				generic: false,
			},
		},
		{
			name: "matched",
			obj: &workv1alpha1.Work{ObjectMeta: metav1.ObjectMeta{
				Name: "work", Namespace: names.ExecutionSpacePrefix + "cluster",
			}},
			want: want{
				create:  false,
				update:  true,
				delete:  false,
				generic: false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pred.Create(event.CreateEvent{Object: tt.obj}); got != tt.want.create {
				t.Errorf("Create() got = %v, want %v", got, tt.want.create)
				return
			}
			if got := pred.Update(event.UpdateEvent{ObjectOld: tt.obj, ObjectNew: tt.obj}); got != tt.want.update {
				t.Errorf("Update() got = %v, want %v", got, tt.want.update)
				return
			}
			if got := pred.Delete(event.DeleteEvent{Object: tt.obj}); got != tt.want.delete {
				t.Errorf("Delete() got = %v, want %v", got, tt.want.delete)
				return
			}
			if got := pred.Generic(event.GenericEvent{Object: tt.obj}); got != tt.want.generic {
				t.Errorf("Generic() got = %v, want %v", got, tt.want.generic)
				return
			}
		})
	}
}

func TestNewPredicateForServiceExportControllerOnAgent_Update(t *testing.T) {
	unmatched := &workv1alpha1.Work{
		ObjectMeta: metav1.ObjectMeta{
			Name: "work", Namespace: names.ExecutionSpacePrefix + "cluster",
		},
		Spec: workv1alpha1.WorkSpec{
			SuspendDispatching: ptr.To(true),
		},
	}
	matched := &workv1alpha1.Work{
		ObjectMeta: metav1.ObjectMeta{
			Name: "work", Namespace: names.ExecutionSpacePrefix + "cluster",
		},
	}

	type args struct {
		event event.UpdateEvent
	}

	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "both old and new are unmatched",
			args: args{
				event: event.UpdateEvent{ObjectOld: unmatched, ObjectNew: unmatched},
			},
			want: false,
		},
		{
			name: "old is unmatched, new is matched",
			args: args{
				event: event.UpdateEvent{ObjectOld: unmatched, ObjectNew: matched},
			},
			want: true,
		},
		{
			name: "old is matched, new is unmatched",
			args: args{
				event: event.UpdateEvent{ObjectOld: matched, ObjectNew: unmatched},
			},
			want: true,
		},
		{
			name: "both old and new are matched",
			args: args{
				event: event.UpdateEvent{ObjectOld: matched, ObjectNew: matched},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pred := NewPredicateForServiceExportControllerOnAgent("cluster")
			if got := pred.Update(tt.args.event); got != tt.want {
				t.Errorf("Update() got = %v, want %v", got, tt.want)
				return
			}
		})
	}
}

type fakeManager struct {
	controllerruntime.Manager
	client client.Client
}

func (f *fakeManager) GetClient() client.Client {
	return f.client
}
