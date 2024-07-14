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

package util

import (
	"reflect"
	"testing"

	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/karmada-io/karmada/pkg/util/gclient"
)

func TestSetLeaseOwnerFunc(t *testing.T) {
	type args struct {
		c           client.Client
		clusterName string
		lease       *coordinationv1.Lease
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		want    *coordinationv1.Lease
	}{
		{
			name: "lease has no owner",
			args: args{
				c: fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					newCluster("test"),
				).Build(),
				clusterName: "test",
				lease:       &coordinationv1.Lease{},
			},
			wantErr: false,
			want: &coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{
				{Name: "test"}}}},
		},
		{
			name: "cluster not found",
			args: args{
				c:           fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects().Build(),
				clusterName: "test",
				lease:       &coordinationv1.Lease{},
			},
			wantErr: true,
			want:    &coordinationv1.Lease{},
		},
		{
			name: "lease has owner",
			args: args{
				c:           fakeclient.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects().Build(),
				clusterName: "test",
				lease: &coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{
					{APIVersion: "cluster.karmada.io/v1alpha1", Kind: "Cluster", Name: "foo", UID: "456"}}}},
			},
			wantErr: false,
			want: &coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{
				{APIVersion: "cluster.karmada.io/v1alpha1", Kind: "Cluster", Name: "foo", UID: "456"}}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := SetLeaseOwnerFunc(tt.args.c, tt.args.clusterName)
			if f == nil {
				t.Errorf("SetLeaseOwnerFunc() returns nil")
				return
			}

			err := f(tt.args.lease)
			if (err != nil) != tt.wantErr {
				t.Errorf("got error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(tt.want, tt.args.lease) {
				t.Errorf("got = %v, want %v", tt.args.lease, tt.want)
				return
			}
		})
	}
}
