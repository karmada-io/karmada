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
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	clustervalidation "github.com/karmada-io/karmada/pkg/apis/cluster/validation"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
)

// ResponseType represents the type of admission response.
type ResponseType string

const (
	Denied  ResponseType = "Denied"
	Allowed ResponseType = "Allowed"
	Errored ResponseType = "Errored"
)

// TestResponse is used to define expected response in a test case.
type TestResponse struct {
	Type    ResponseType
	Message string
}

type fakeValidationDecoder struct {
	err error
	obj runtime.Object
}

// Decode mocks the Decode method of admission.Decoder.
func (f *fakeValidationDecoder) Decode(_ admission.Request, obj runtime.Object) error {
	if f.err != nil {
		return f.err
	}
	if f.obj != nil {
		reflect.ValueOf(obj).Elem().Set(reflect.ValueOf(f.obj).Elem())
	}
	return nil
}

// DecodeRaw mocks the DecodeRaw method of admission.Decoder.
func (f *fakeValidationDecoder) DecodeRaw(_ runtime.RawExtension, obj runtime.Object) error {
	if f.err != nil {
		return f.err
	}
	if f.obj != nil {
		reflect.ValueOf(obj).Elem().Set(reflect.ValueOf(f.obj).Elem())
	}
	return nil
}

// normalizedMessages trim the square brackets, and sorted error message parts to ensure consistent ordering.
// This prevents test flakiness caused by varying error message order.
func normalizedMessages(message string) string {
	message = strings.TrimLeft(message, "[")
	message = strings.TrimRight(message, "]")
	parts := strings.Split(message, ", ")
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
			"validateOverallAndAssignments_ValidValues_NoErrors",
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
			"validateOverallAndAssignments_LowerOverallCPUThanAssignments_OverallCPULessThanAssignments",
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
			"validateOverallAndAssignments_LowerOverallMemoryThanAssignments_OverallMemoryLessThanAssignments",
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
			"validateOverallAndAssignments_InvalidAssignmentResource_AssignmentResourceNotInOverall",
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
		want    TestResponse
	}{
		{
			name: "Handle_DecodeErrorHandling_ErrorReturned",
			decoder: &fakeValidationDecoder{
				err: errors.New("decode error"),
			},
			req: admission.Request{},
			want: TestResponse{
				Type:    Errored,
				Message: "decode error",
			},
		},
		{
			name: "Handle_ResourceLimitsMatch_NoErrors",
			decoder: &fakeValidationDecoder{
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
			req: admission.Request{},
			want: TestResponse{
				Type:    Allowed,
				Message: "",
			},
		},
		{
			name: "Handle_ExceedResourceLimits_ResourceLimitsExceeded",
			decoder: &fakeValidationDecoder{
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
			req: admission.Request{},
			want: TestResponse{
				Type:    Denied,
				Message: fmt.Sprintf("spec.overall[cpu]: Invalid value: \"%s\": overall is less than assignments, spec.overall[memory]: Invalid value: \"%s\": overall is less than assignments", "5", "5Gi"),
			},
		},
		{
			name: "Handle_ExceedCPUAllocationOverallLimit_CPUAllocationExceedsOverallLimit",
			decoder: &fakeValidationDecoder{
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
			req: admission.Request{},
			want: TestResponse{
				Type:    Denied,
				Message: fmt.Sprintf("spec.overall[cpu]: Invalid value: \"%s\": overall is less than assignments", "5"),
			},
		},
		{
			name: "Handle_ExceedMemoryAllocationOverallLimit_MemoryAllocationExceedsOverallLimit",
			decoder: &fakeValidationDecoder{
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
			req: admission.Request{},
			want: TestResponse{
				Type:    Denied,
				Message: fmt.Sprintf("spec.overall[memory]: Invalid value: \"%s\": overall is less than assignments", "5Gi"),
			},
		},
		{
			name: "Handle_InvalidClusterName_ClusterNameInvalid",
			decoder: &fakeValidationDecoder{
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
			req: admission.Request{},
			want: TestResponse{
				Type:    Denied,
				Message: "Invalid value: \"invalid cluster name\"",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &ValidatingAdmission{
				Decoder: tt.decoder,
			}
			got := v.Handle(context.Background(), tt.req)
			got.Result.Message = normalizedMessages(got.Result.Message)
			tt.want.Message = normalizedMessages(tt.want.Message)

			// Extract type and message from the actual response.
			gotType := extractResponseType(got)
			gotMessage := extractErrorMessage(got)

			if gotType != tt.want.Type || !strings.Contains(gotMessage, tt.want.Message) {
				t.Errorf("Handle() = {Type: %v, Message: %v}, want {Type: %v, Message: %v}", gotType, gotMessage, tt.want.Type, tt.want.Message)
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
			name: "validateFederatedResourceQuotaStatus_ValidStatus_NoErrors",
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
			name: "validateFederatedResourceQuotaStatus_InvalidOverallResourceList_ResourceTypeInvalid",
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
			name: "validateFederatedResourceQuotaStatus_InvalidAggregatedStatusResourceList_ResourceTypeInvalid",
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
			name: "validateClusterQuotaStatus_ValidClusterQuotaStatus_NoErrors",
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
			name: "validateClusterQuotaStatus_InvalidClusterName_ClusterNameInvalid",
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
			name: "validateClusterQuotaStatus_InvalidResourceList_HardResourceInvalid",
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
			name: "validateClusterQuotaStatus_InvalidResourceList_UsedResourceInvalid",
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

// extractResponseType extracts the type of admission response.
func extractResponseType(resp admission.Response) ResponseType {
	if resp.Allowed {
		return Allowed
	}
	if resp.Result != nil {
		if resp.Result.Code == http.StatusBadRequest {
			return Errored
		}
	}
	return Denied
}

// extractErrorMessage extracts the error message from a Denied/Errored response.
func extractErrorMessage(resp admission.Response) string {
	if !resp.Allowed && resp.Result != nil {
		return resp.Result.Message
	}
	return ""
}
