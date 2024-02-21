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

package federatedresourcequota

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation/field"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/validation"
)

func Test_validateOverallAndAssignments(t *testing.T) {
	specFld := field.NewPath("spec")
	cpuParse := resource.MustParse("10")
	memoryParse := resource.MustParse("10Gi")

	type args struct {
		quotaSpec *policyv1alpha1.FederatedResourceQuotaSpec
		fld       *field.Path
	}
	tests := []struct {
		name string
		args args
		want field.ErrorList
	}{
		{
			"normal",
			args{
				quotaSpec: &policyv1alpha1.FederatedResourceQuotaSpec{
					Overall: corev1.ResourceList{
						"cpu":    cpuParse,
						"memory": memoryParse,
					},
					StaticAssignments: []policyv1alpha1.StaticClusterAssignment{
						{
							ClusterName: "m1",
							Hard: corev1.ResourceList{
								"cpu":    resource.MustParse("1"),
								"memory": resource.MustParse("1Gi"),
							},
						},
						{
							ClusterName: "m2",
							Hard: corev1.ResourceList{
								"cpu":    resource.MustParse("1.5"),
								"memory": resource.MustParse("2Gi"),
							},
						},
					},
				},
				fld: specFld,
			},
			field.ErrorList{},
		},
		{
			"overall[cpu] is less than assignments",
			args{
				quotaSpec: &policyv1alpha1.FederatedResourceQuotaSpec{
					Overall: corev1.ResourceList{
						"cpu":    cpuParse,
						"memory": memoryParse,
					},
					StaticAssignments: []policyv1alpha1.StaticClusterAssignment{
						{
							ClusterName: "m1",
							Hard: corev1.ResourceList{
								"cpu":    resource.MustParse("1"),
								"memory": resource.MustParse("1Gi"),
							},
						},
						{
							ClusterName: "m2",
							Hard: corev1.ResourceList{
								"cpu":    resource.MustParse("10"),
								"memory": resource.MustParse("2Gi"),
							},
						},
					},
				},
				fld: specFld,
			},
			field.ErrorList{
				field.Invalid(specFld.Child("overall").Key("cpu"), cpuParse.String(), "overall is less than assignments"),
			},
		},
		{
			"overall[memory] is less than assignments",
			args{
				quotaSpec: &policyv1alpha1.FederatedResourceQuotaSpec{
					Overall: corev1.ResourceList{
						"cpu":    cpuParse,
						"memory": memoryParse,
					},
					StaticAssignments: []policyv1alpha1.StaticClusterAssignment{
						{
							ClusterName: "m1",
							Hard: corev1.ResourceList{
								"cpu":    resource.MustParse("1"),
								"memory": resource.MustParse("1Gi"),
							},
						},
						{
							ClusterName: "m2",
							Hard: corev1.ResourceList{
								"cpu":    resource.MustParse("1.5"),
								"memory": resource.MustParse("10Gi"),
							},
						},
					},
				},
				fld: specFld,
			},
			field.ErrorList{
				field.Invalid(specFld.Child("overall").Key("memory"), memoryParse.String(), "overall is less than assignments"),
			},
		},
		{
			"assignment resourceName is not exist in overall",
			args{
				quotaSpec: &policyv1alpha1.FederatedResourceQuotaSpec{
					Overall: corev1.ResourceList{
						"cpu":    cpuParse,
						"memory": memoryParse,
					},
					StaticAssignments: []policyv1alpha1.StaticClusterAssignment{
						{
							ClusterName: "m1",
							Hard: corev1.ResourceList{
								"cpux":   resource.MustParse("1"),
								"memory": resource.MustParse("1Gi"),
							},
						},
						{
							ClusterName: "m2",
							Hard: corev1.ResourceList{
								"cpu":    resource.MustParse("1.5"),
								"memory": resource.MustParse("2Gi"),
							},
						},
					},
				},
				fld: specFld,
			},
			field.ErrorList{
				field.Invalid(specFld.Child("staticAssignments").Index(0).Child("hard").Key("cpux"), corev1.ResourceName("cpux"), "assignment resourceName is not exist in overall"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateOverallAndAssignments(tt.args.quotaSpec, tt.args.fld); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("validateOverallAndAssignments() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_validateFederatedResourceQuotaName(t *testing.T) {
	invalidName := strings.Repeat("a", 64)
	tests := []struct {
		name string
		want field.ErrorList
	}{
		{
			name: invalidName,
			want: field.ErrorList{
				field.Invalid(field.NewPath("metadata").Child("name"), invalidName, fmt.Sprintf("must be no more than %d characters", validation.LabelValueMaxLength)),
			},
		},
		{
			name: "name-less-than-63-characters",
			want: field.ErrorList{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateFederatedResourceQuotaName(tt.name, field.NewPath("metadata").Child("name")); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("validateFederatedResourceQuotaName() = %v, want %v", got, tt.want)
			}
		})
	}
}
