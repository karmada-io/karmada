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
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	clustervalidation "github.com/karmada-io/karmada/pkg/apis/cluster/validation"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

type fakeDecoder struct {
	err error
	obj runtime.Object
}

// Decode mocks the Decode method of admission.Decoder.
func (f *fakeDecoder) Decode(_ admission.Request, obj runtime.Object) error {
	if f.err != nil {
		return f.err
	}
	if f.obj != nil {
		reflect.ValueOf(obj).Elem().Set(reflect.ValueOf(f.obj).Elem())
	}
	return nil
}

// DecodeRaw mocks the DecodeRaw method of admission.Decoder.
func (f *fakeDecoder) DecodeRaw(_ runtime.RawExtension, obj runtime.Object) error {
	if f.err != nil {
		return f.err
	}
	if f.obj != nil {
		reflect.ValueOf(obj).Elem().Set(reflect.ValueOf(f.obj).Elem())
	}
	return nil
}

// sortAndJoinMessages sorts and joins error message parts to ensure consistent ordering.
// This prevents test flakiness caused by varying error message order.
func sortAndJoinMessages(message string) string {
	parts := strings.Split(strings.Trim(message, "[]"), ", ")
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

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

func TestValidatingAdmission_Handle(t *testing.T) {
	tests := []struct {
		name    string
		decoder admission.Decoder
		req     admission.Request
		want    admission.Response
	}{
		{
			name: "Decode Error Handling",
			decoder: &fakeDecoder{
				err: errors.New("decode error"),
			},
			req:  admission.Request{},
			want: admission.Errored(http.StatusBadRequest, errors.New("decode error")),
		},
		{
			name: "Validation Success - Resource Limits Match",
			decoder: &fakeDecoder{
				obj: &policyv1alpha1.FederatedResourceQuota{
					Spec: policyv1alpha1.FederatedResourceQuotaSpec{
						Overall: corev1.ResourceList{
							"cpu":    resource.MustParse("10"),
							"memory": resource.MustParse("10Gi"),
						},
						StaticAssignments: []policyv1alpha1.StaticClusterAssignment{
							{
								ClusterName: "m1",
								Hard: corev1.ResourceList{
									"cpu":    resource.MustParse("5"),
									"memory": resource.MustParse("5Gi"),
								},
							},
							{
								ClusterName: "m2",
								Hard: corev1.ResourceList{
									"cpu":    resource.MustParse("5"),
									"memory": resource.MustParse("5Gi"),
								},
							},
						},
					},
				},
			},
			req:  admission.Request{},
			want: admission.Allowed(""),
		},
		{
			name: "Validation Error - Resource Limits Exceeded",
			decoder: &fakeDecoder{
				obj: &policyv1alpha1.FederatedResourceQuota{
					Spec: policyv1alpha1.FederatedResourceQuotaSpec{
						Overall: corev1.ResourceList{
							"cpu":    resource.MustParse("5"),
							"memory": resource.MustParse("5Gi"),
						},
						StaticAssignments: []policyv1alpha1.StaticClusterAssignment{
							{
								ClusterName: "m1",
								Hard: corev1.ResourceList{
									"cpu":    resource.MustParse("5"),
									"memory": resource.MustParse("5Gi"),
								},
							},
							{
								ClusterName: "m2",
								Hard: corev1.ResourceList{
									"cpu":    resource.MustParse("5"),
									"memory": resource.MustParse("5Gi"),
								},
							},
						},
					},
				},
			},
			req:  admission.Request{},
			want: admission.Denied(fmt.Sprintf("[spec.overall[cpu]: Invalid value: \"%s\": overall is less than assignments, spec.overall[memory]: Invalid value: \"%s\": overall is less than assignments]", "5", "5Gi")),
		},
		{
			name: "Validation Error - CPU Allocation Exceeds Overall Limit",
			decoder: &fakeDecoder{
				obj: &policyv1alpha1.FederatedResourceQuota{
					Spec: policyv1alpha1.FederatedResourceQuotaSpec{
						Overall: corev1.ResourceList{
							"cpu":    resource.MustParse("5"),
							"memory": resource.MustParse("10Gi"),
						},
						StaticAssignments: []policyv1alpha1.StaticClusterAssignment{
							{
								ClusterName: "m1",
								Hard: corev1.ResourceList{
									"cpu":    resource.MustParse("10"),
									"memory": resource.MustParse("5Gi"),
								},
							},
							{
								ClusterName: "m2",
								Hard: corev1.ResourceList{
									"cpu":    resource.MustParse("5"),
									"memory": resource.MustParse("5Gi"),
								},
							},
						},
					},
				},
			},
			req:  admission.Request{},
			want: admission.Denied(fmt.Sprintf("spec.overall[cpu]: Invalid value: \"%s\": overall is less than assignments", "5")),
		},
		{
			name: "Validation Error - Memory Allocation Exceeds Overall Limit",
			decoder: &fakeDecoder{
				obj: &policyv1alpha1.FederatedResourceQuota{
					Spec: policyv1alpha1.FederatedResourceQuotaSpec{
						Overall: corev1.ResourceList{
							"cpu":    resource.MustParse("10"),
							"memory": resource.MustParse("5Gi"),
						},
						StaticAssignments: []policyv1alpha1.StaticClusterAssignment{
							{
								ClusterName: "m1",
								Hard: corev1.ResourceList{
									"cpu":    resource.MustParse("5"),
									"memory": resource.MustParse("5Gi"),
								},
							},
							{
								ClusterName: "m2",
								Hard: corev1.ResourceList{
									"cpu":    resource.MustParse("5"),
									"memory": resource.MustParse("10Gi"),
								},
							},
						},
					},
				},
			},
			req:  admission.Request{},
			want: admission.Denied(fmt.Sprintf("spec.overall[memory]: Invalid value: \"%s\": overall is less than assignments", "5Gi")),
		},
		{
			name: "Invalid Cluster Name",
			decoder: &fakeDecoder{
				obj: &policyv1alpha1.FederatedResourceQuota{
					Spec: policyv1alpha1.FederatedResourceQuotaSpec{
						Overall: corev1.ResourceList{
							"cpu":    resource.MustParse("10"),
							"memory": resource.MustParse("10Gi"),
						},
						StaticAssignments: []policyv1alpha1.StaticClusterAssignment{
							{
								ClusterName: "invalid cluster name",
								Hard: corev1.ResourceList{
									"cpu":    resource.MustParse("5"),
									"memory": resource.MustParse("5Gi"),
								},
							},
						},
					},
				},
			},
			req:  admission.Request{},
			want: admission.Denied("[spec.staticAssignments[0].clusterName: Invalid value: \"invalid cluster name\": a lowercase RFC 1123 label must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character (e.g. 'my-name',  or '123-abc', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?')]"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &ValidatingAdmission{
				Decoder: tt.decoder,
			}
			got := v.Handle(context.Background(), tt.req)
			got.Result.Message = sortAndJoinMessages(got.Result.Message)
			tt.want.Result.Message = sortAndJoinMessages(tt.want.Result.Message)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Handle() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_validateFederatedResourceQuotaStatus(t *testing.T) {
	fld := field.NewPath("status")

	tests := []struct {
		name     string
		status   *policyv1alpha1.FederatedResourceQuotaStatus
		expected field.ErrorList
	}{
		{
			name: "Valid FederatedResourceQuotaStatus",
			status: &policyv1alpha1.FederatedResourceQuotaStatus{
				Overall:     corev1.ResourceList{"cpu": resource.MustParse("10")},
				OverallUsed: corev1.ResourceList{"cpu": resource.MustParse("5")},
				AggregatedStatus: []policyv1alpha1.ClusterQuotaStatus{
					{
						ClusterName: "valid-cluster",
						ResourceQuotaStatus: corev1.ResourceQuotaStatus{
							Hard: corev1.ResourceList{"cpu": resource.MustParse("10")},
							Used: corev1.ResourceList{"cpu": resource.MustParse("5")},
						},
					},
				},
			},
			expected: field.ErrorList{},
		},
		{
			name: "Invalid Overall Resource List",
			status: &policyv1alpha1.FederatedResourceQuotaStatus{
				Overall:     corev1.ResourceList{"invalid-resource": resource.MustParse("10")},
				OverallUsed: corev1.ResourceList{"cpu": resource.MustParse("5")},
				AggregatedStatus: []policyv1alpha1.ClusterQuotaStatus{
					{
						ClusterName: "valid-cluster",
						ResourceQuotaStatus: corev1.ResourceQuotaStatus{
							Hard: corev1.ResourceList{"cpu": resource.MustParse("10")},
							Used: corev1.ResourceList{"cpu": resource.MustParse("5")},
						},
					},
				},
			},
			expected: field.ErrorList{
				field.Invalid(fld.Child("overall").Key("invalid-resource"), "invalid-resource", "must be a standard resource type or fully qualified"),
				field.Invalid(fld.Child("overall").Key("invalid-resource"), "invalid-resource", "must be a standard resource for quota"),
			},
		},
		{
			name: "Invalid AggregatedStatus Resource List",
			status: &policyv1alpha1.FederatedResourceQuotaStatus{
				Overall:     corev1.ResourceList{"cpu": resource.MustParse("10")},
				OverallUsed: corev1.ResourceList{"cpu": resource.MustParse("5")},
				AggregatedStatus: []policyv1alpha1.ClusterQuotaStatus{
					{
						ClusterName: "valid-cluster",
						ResourceQuotaStatus: corev1.ResourceQuotaStatus{
							Hard: corev1.ResourceList{"invalid-resource": resource.MustParse("10")},
							Used: corev1.ResourceList{"cpu": resource.MustParse("5")},
						},
					},
				},
			},
			expected: field.ErrorList{
				field.Invalid(fld.Child("aggregatedStatus").Index(0).Child("hard").Key("invalid-resource"), "invalid-resource", "must be a standard resource type or fully qualified"),
				field.Invalid(fld.Child("aggregatedStatus").Index(0).Child("hard").Key("invalid-resource"), "invalid-resource", "must be a standard resource for quota"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateFederatedResourceQuotaStatus(tt.status, fld); !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("validateFederatedResourceQuotaStatus() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func Test_validateClusterQuotaStatus(t *testing.T) {
	fld := field.NewPath("status").Child("aggregatedStatus")

	tests := []struct {
		name     string
		status   *policyv1alpha1.ClusterQuotaStatus
		expected field.ErrorList
	}{
		{
			name: "Valid ClusterQuotaStatus",
			status: &policyv1alpha1.ClusterQuotaStatus{
				ClusterName: "valid-cluster",
				ResourceQuotaStatus: corev1.ResourceQuotaStatus{
					Hard: corev1.ResourceList{"cpu": resource.MustParse("10")},
					Used: corev1.ResourceList{"cpu": resource.MustParse("5")},
				},
			},
			expected: field.ErrorList{},
		},
		{
			name: "Invalid Cluster Name",
			status: &policyv1alpha1.ClusterQuotaStatus{
				ClusterName: "invalid cluster name",
				ResourceQuotaStatus: corev1.ResourceQuotaStatus{
					Hard: corev1.ResourceList{"cpu": resource.MustParse("10")},
					Used: corev1.ResourceList{"cpu": resource.MustParse("5")},
				},
			},
			expected: field.ErrorList{
				field.Invalid(fld.Child("clusterName"), "invalid cluster name", strings.Join(clustervalidation.ValidateClusterName("invalid cluster name"), ",")),
			},
		},
		{
			name: "Invalid Resource List - Hard",
			status: &policyv1alpha1.ClusterQuotaStatus{
				ClusterName: "valid-cluster",
				ResourceQuotaStatus: corev1.ResourceQuotaStatus{
					Hard: corev1.ResourceList{"invalid-resource": resource.MustParse("10")},
					Used: corev1.ResourceList{"cpu": resource.MustParse("5")},
				},
			},
			expected: field.ErrorList{
				field.Invalid(fld.Child("hard").Key("invalid-resource"), "invalid-resource", "must be a standard resource type or fully qualified"),
				field.Invalid(fld.Child("hard").Key("invalid-resource"), "invalid-resource", "must be a standard resource for quota"),
			},
		},
		{
			name: "Invalid Resource List - Used",
			status: &policyv1alpha1.ClusterQuotaStatus{
				ClusterName: "valid-cluster",
				ResourceQuotaStatus: corev1.ResourceQuotaStatus{
					Hard: corev1.ResourceList{"cpu": resource.MustParse("10")},
					Used: corev1.ResourceList{"invalid-resource": resource.MustParse("5")},
				},
			},
			expected: field.ErrorList{
				field.Invalid(fld.Child("used").Key("invalid-resource"), "invalid-resource", "must be a standard resource type or fully qualified"),
				field.Invalid(fld.Child("used").Key("invalid-resource"), "invalid-resource", "must be a standard resource for quota"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateClusterQuotaStatus(tt.status, fld); !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("validateClusterQuotaStatus() = %v, want %v", got, tt.expected)
			}
		})
	}
}
