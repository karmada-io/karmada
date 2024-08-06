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
	"context"
	"reflect"
	"testing"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/karmada-io/karmada/pkg/util/gclient"
)

func TestCreateOrUpdateEndpointSlice(t *testing.T) {
	type args struct {
		client        client.Client
		endpointSlice *discoveryv1.EndpointSlice
	}
	tests := []struct {
		name    string
		args    args
		want    *discoveryv1.EndpointSlice
		wantErr bool
	}{
		{
			name: "not exist and create",
			args: args{
				client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).Build(),
				endpointSlice: &discoveryv1.EndpointSlice{
					ObjectMeta: metav1.ObjectMeta{
						Name: "eps", Namespace: "ns",
						Labels: map[string]string{"foo": "foo1"},
					},
					AddressType: discoveryv1.AddressTypeIPv4,
					Endpoints: []discoveryv1.Endpoint{{
						Addresses: []string{"1.1.1.1"},
					}},
					Ports: []discoveryv1.EndpointPort{{
						Port: ptr.To[int32](80),
					}},
				},
			},
			want: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name: "eps", Namespace: "ns",
					Labels: map[string]string{"foo": "foo1"},
				},
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints: []discoveryv1.Endpoint{{
					Addresses:  []string{"1.1.1.1"},
					Conditions: discoveryv1.EndpointConditions{},
				}},
				Ports: []discoveryv1.EndpointPort{{
					Port: ptr.To[int32](80),
				}},
			},
			wantErr: false,
		},
		{
			name: "exist and update",
			args: args{
				client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&discoveryv1.EndpointSlice{ObjectMeta: metav1.ObjectMeta{Name: "eps", Namespace: "ns"}}).Build(),
				endpointSlice: &discoveryv1.EndpointSlice{
					ObjectMeta: metav1.ObjectMeta{
						Name: "eps", Namespace: "ns",
						Labels: map[string]string{"foo": "foo1"},
					},
					AddressType: discoveryv1.AddressTypeIPv4,
					Endpoints: []discoveryv1.Endpoint{{
						Addresses: []string{"1.1.1.1"},
					}},
					Ports: []discoveryv1.EndpointPort{{
						Port: ptr.To[int32](80),
					}},
				},
			},
			want: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name: "eps", Namespace: "ns",
					Labels: map[string]string{"foo": "foo1"},
				},
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints: []discoveryv1.Endpoint{{
					Addresses:  []string{"1.1.1.1"},
					Conditions: discoveryv1.EndpointConditions{},
				}},
				Ports: []discoveryv1.EndpointPort{{
					Port: ptr.To[int32](80),
				}},
			},
			wantErr: false,
		},
		{
			name: "exist and not update",
			args: args{
				client: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&discoveryv1.EndpointSlice{
						ObjectMeta: metav1.ObjectMeta{
							Name: "eps", Namespace: "ns",
							Labels: map[string]string{"foo": "foo1"},
						},
						AddressType: discoveryv1.AddressTypeIPv4,
						Endpoints: []discoveryv1.Endpoint{{
							Addresses: []string{"1.1.1.1"},
						}},
						Ports: []discoveryv1.EndpointPort{{
							Port: ptr.To[int32](80),
						}},
					}).Build(),
				endpointSlice: &discoveryv1.EndpointSlice{
					ObjectMeta: metav1.ObjectMeta{
						Name: "eps", Namespace: "ns",
						Labels: map[string]string{"foo": "foo1"},
					},
					AddressType: discoveryv1.AddressTypeIPv4,
					Endpoints: []discoveryv1.Endpoint{{
						Addresses: []string{"1.1.1.1"},
					}},
					Ports: []discoveryv1.EndpointPort{{
						Port: ptr.To[int32](80),
					}},
				},
			},
			want: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name: "eps", Namespace: "ns",
					Labels: map[string]string{"foo": "foo1"},
				},
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints: []discoveryv1.Endpoint{{
					Addresses:  []string{"1.1.1.1"},
					Conditions: discoveryv1.EndpointConditions{},
				}},
				Ports: []discoveryv1.EndpointPort{{
					Port: ptr.To[int32](80),
				}},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CreateOrUpdateEndpointSlice(context.Background(), tt.args.client, tt.args.endpointSlice); (err != nil) != tt.wantErr {
				t.Errorf("CreateOrUpdateEndpointSlice() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			got := &discoveryv1.EndpointSlice{}
			if err := tt.args.client.Get(context.TODO(), client.ObjectKey{Namespace: "ns", Name: "eps"}, got); err != nil {
				t.Error(err)
				return
			}
			got.ResourceVersion = ""
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateOrUpdateEndpointSlice() got = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestDeleteEndpointSlice(t *testing.T) {
	type args struct {
		c        client.Client
		selector labels.Set
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		want    []discoveryv1.EndpointSlice
	}{
		{
			name: "delete",
			args: args{
				c: fake.NewClientBuilder().WithScheme(gclient.NewSchema()).WithObjects(
					&discoveryv1.EndpointSlice{ObjectMeta: metav1.ObjectMeta{Name: "unmatched", Namespace: "ns"}},
					&discoveryv1.EndpointSlice{ObjectMeta: metav1.ObjectMeta{Name: "no_", Namespace: "ns", Labels: map[string]string{"foo": "foo1"}}},
				).Build(),
				selector: map[string]string{"foo": "foo1"},
			},
			wantErr: false,
			want: []discoveryv1.EndpointSlice{
				{ObjectMeta: metav1.ObjectMeta{Name: "unmatched", Namespace: "ns"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := DeleteEndpointSlice(context.Background(), tt.args.c, tt.args.selector); (err != nil) != tt.wantErr {
				t.Errorf("DeleteEndpointSlice() error = %v, wantErr %v", err, tt.wantErr)
			}
			list := &discoveryv1.EndpointSliceList{}
			if err := tt.args.c.List(context.TODO(), list); err != nil {
				t.Error(err)
				return
			}
			for i := range list.Items {
				list.Items[i].ResourceVersion = ""
			}
			if got := list.Items; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DeleteEndpointSlice() got = %#v, want %#v", got, tt.want)
			}
		})
	}
}
