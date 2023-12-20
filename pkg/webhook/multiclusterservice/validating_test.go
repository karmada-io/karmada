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

package multiclusterservice

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	networkingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1"
)

func TestValidateMultiClusterServiceSpec(t *testing.T) {
	validator := &ValidatingAdmission{}
	specFld := field.NewPath("spec")

	tests := []struct {
		name        string
		mcs         *networkingv1alpha1.MultiClusterService
		expectedErr field.ErrorList
	}{
		{
			name: "normal mcs",
			mcs: &networkingv1alpha1.MultiClusterService{
				Spec: networkingv1alpha1.MultiClusterServiceSpec{
					Ports: []networkingv1alpha1.ExposurePort{
						{
							Name: "foo",
							Port: 16312,
						},
						{
							Name: "bar",
							Port: 16313,
						},
					},
					Types: []networkingv1alpha1.ExposureType{
						networkingv1alpha1.ExposureTypeLoadBalancer,
					},
					ProviderClusters: []networkingv1alpha1.ClusterSelector{
						{Name: "member1"},
						{Name: "member2"},
					},
					ConsumerClusters: []networkingv1alpha1.ClusterSelector{
						{Name: "member1"},
						{Name: "member2"},
					},
				},
			},
			expectedErr: field.ErrorList{},
		},
		{
			name: "multiple exposure type mcs",
			mcs: &networkingv1alpha1.MultiClusterService{
				Spec: networkingv1alpha1.MultiClusterServiceSpec{
					Ports: []networkingv1alpha1.ExposurePort{
						{
							Name: "foo",
							Port: 16312,
						},
						{
							Name: "bar",
							Port: 16313,
						},
					},
					Types: []networkingv1alpha1.ExposureType{
						networkingv1alpha1.ExposureTypeLoadBalancer,
						networkingv1alpha1.ExposureTypeCrossCluster,
					},
					ProviderClusters: []networkingv1alpha1.ClusterSelector{
						{Name: "member1"},
						{Name: "member2"},
					},
					ConsumerClusters: []networkingv1alpha1.ClusterSelector{
						{Name: "member1"},
						{Name: "member2"},
					},
				},
			},
			expectedErr: field.ErrorList{field.Invalid(specFld.Child("types"), []networkingv1alpha1.ExposureType{
				networkingv1alpha1.ExposureTypeLoadBalancer,
				networkingv1alpha1.ExposureTypeCrossCluster,
			}, "MultiClusterService types should not contain more than one type")},
		},
		{
			name: "duplicated svc name",
			mcs: &networkingv1alpha1.MultiClusterService{
				Spec: networkingv1alpha1.MultiClusterServiceSpec{
					Ports: []networkingv1alpha1.ExposurePort{
						{
							Name: "foo",
							Port: 16312,
						},
						{
							Name: "foo",
							Port: 16313,
						},
					},
					Types: []networkingv1alpha1.ExposureType{
						networkingv1alpha1.ExposureTypeLoadBalancer,
						networkingv1alpha1.ExposureTypeLoadBalancer,
					},
					ProviderClusters: []networkingv1alpha1.ClusterSelector{
						{Name: "member1"},
						{Name: "member2"},
					},
					ConsumerClusters: []networkingv1alpha1.ClusterSelector{
						{Name: "member1"},
						{Name: "member2"},
					},
				},
			},
			expectedErr: field.ErrorList{field.Duplicate(specFld.Child("ports").Index(1).Child("name"), "foo")},
		},
		{
			name: "invalid svc port",
			mcs: &networkingv1alpha1.MultiClusterService{
				Spec: networkingv1alpha1.MultiClusterServiceSpec{
					Ports: []networkingv1alpha1.ExposurePort{
						{
							Name: "foo",
							Port: 163121,
						},
					},
					Types: []networkingv1alpha1.ExposureType{
						networkingv1alpha1.ExposureTypeLoadBalancer,
					},
					ProviderClusters: []networkingv1alpha1.ClusterSelector{
						{Name: "member1"},
						{Name: "member2"},
					},
					ConsumerClusters: []networkingv1alpha1.ClusterSelector{
						{Name: "member1"},
						{Name: "member2"},
					},
				},
			},
			expectedErr: field.ErrorList{field.Invalid(specFld.Child("ports").Index(0).Child("port"), int32(163121), validation.InclusiveRangeError(1, 65535))},
		},
		{
			name: "invalid ExposureType",
			mcs: &networkingv1alpha1.MultiClusterService{
				Spec: networkingv1alpha1.MultiClusterServiceSpec{
					Ports: []networkingv1alpha1.ExposurePort{
						{
							Name: "foo",
							Port: 16312,
						},
					},
					Types: []networkingv1alpha1.ExposureType{
						"",
					},
					ProviderClusters: []networkingv1alpha1.ClusterSelector{
						{Name: "member1"},
						{Name: "member2"},
					},
					ConsumerClusters: []networkingv1alpha1.ClusterSelector{
						{Name: "member1"},
						{Name: "member2"},
					},
				},
			},
			expectedErr: field.ErrorList{field.Invalid(specFld.Child("types").Index(0), networkingv1alpha1.ExposureType(""), "ExposureType Error")},
		},
		{
			name: "invalid cluster name",
			mcs: &networkingv1alpha1.MultiClusterService{
				Spec: networkingv1alpha1.MultiClusterServiceSpec{
					Ports: []networkingv1alpha1.ExposurePort{
						{
							Name: "foo",
							Port: 16312,
						},
					},
					Types: []networkingv1alpha1.ExposureType{
						networkingv1alpha1.ExposureTypeCrossCluster,
					},
					ProviderClusters: []networkingv1alpha1.ClusterSelector{
						{Name: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
					},
					ConsumerClusters: []networkingv1alpha1.ClusterSelector{},
				},
			},
			expectedErr: field.ErrorList{field.Invalid(specFld.Child("range").Child("providerClusters").Index(0), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "must be no more than 48 characters")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validator.validateMultiClusterServiceSpec(tt.mcs); !reflect.DeepEqual(got, tt.expectedErr) {
				t.Errorf("validateMultiClusterServiceSpec() = %v, want %v", got, tt.expectedErr)
			}
		})
	}
}
