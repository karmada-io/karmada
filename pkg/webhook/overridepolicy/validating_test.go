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

package overridepolicy

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

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

func TestValidatingAdmission_Handle(t *testing.T) {
	tests := []struct {
		name    string
		decoder admission.Decoder
		req     admission.Request
		want    TestResponse
	}{
		{
			name: "Handle_DecodeError_DeniesAdmission",
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
			name: "Handle_ValidationOverrideSpecFails_DeniesAdmission",
			decoder: &fakeValidationDecoder{
				obj: &policyv1alpha1.OverridePolicy{
					Spec: policyv1alpha1.OverrideSpec{
						ResourceSelectors: []policyv1alpha1.ResourceSelector{
							{APIVersion: "test-apiversion", Kind: "test"},
						},
						OverrideRules: []policyv1alpha1.RuleWithCluster{
							{
								TargetCluster: &policyv1alpha1.ClusterAffinity{
									ClusterNames: []string{"member1"},
								},
								Overriders: policyv1alpha1.Overriders{
									LabelsOverrider: []policyv1alpha1.LabelAnnotationOverrider{
										{
											Operator: policyv1alpha1.OverriderOpAdd,
											Value:    map[string]string{"testannotation~projectId": "c-m-lfx9lk92p-v86cf"},
										},
									},
								},
							},
						},
					},
				},
			},
			req: admission.Request{},
			want: TestResponse{
				Type:    Denied,
				Message: "Invalid value: \"testannotation~projectId\"",
			},
		},
		{
			name: "Handle_ValidationSucceeds_AllowsAdmission",
			decoder: &fakeValidationDecoder{
				obj: &policyv1alpha1.OverridePolicy{
					Spec: policyv1alpha1.OverrideSpec{
						ResourceSelectors: []policyv1alpha1.ResourceSelector{
							{APIVersion: "test-apiversion", Kind: "test"},
						},
						OverrideRules: []policyv1alpha1.RuleWithCluster{
							{
								TargetCluster: &policyv1alpha1.ClusterAffinity{
									ClusterNames: []string{"member1"},
								},
								Overriders: policyv1alpha1.Overriders{
									Plaintext: []policyv1alpha1.PlaintextOverrider{
										{
											Path:     "/spec/optional",
											Operator: policyv1alpha1.OverriderOpRemove,
										},
									},
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
			name: "Handle_FieldOverrider_ContainsBothYAMLAndJSON_DeniesAdmission",
			decoder: &fakeValidationDecoder{
				obj: &policyv1alpha1.OverridePolicy{
					Spec: policyv1alpha1.OverrideSpec{
						ResourceSelectors: []policyv1alpha1.ResourceSelector{
							{APIVersion: "test-apiversion", Kind: "test"},
						},
						OverrideRules: []policyv1alpha1.RuleWithCluster{
							{
								TargetCluster: &policyv1alpha1.ClusterAffinity{
									ClusterNames: []string{"member1"},
								},
								Overriders: policyv1alpha1.Overriders{
									FieldOverrider: []policyv1alpha1.FieldOverrider{
										{
											FieldPath: "/data/config",
											JSON: []policyv1alpha1.JSONPatchOperation{
												{
													SubPath:  "/db-config",
													Operator: policyv1alpha1.OverriderOpReplace,
													Value:    apiextensionsv1.JSON{Raw: []byte(`{"db": "new"}`)},
												},
											},
											YAML: []policyv1alpha1.YAMLPatchOperation{
												{
													SubPath:  "/db-config",
													Operator: policyv1alpha1.OverriderOpReplace,
													Value:    apiextensionsv1.JSON{Raw: []byte("db: new")},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			req: admission.Request{},
			want: TestResponse{
				Type:    Denied,
				Message: "FieldOverrider has both YAML and JSON set. Only one is allowed",
			},
		},
		{
			name: "Handle_InvalidFieldPathInYAML_DeniesAdmission",
			decoder: &fakeValidationDecoder{
				obj: &policyv1alpha1.OverridePolicy{
					Spec: policyv1alpha1.OverrideSpec{
						OverrideRules: []policyv1alpha1.RuleWithCluster{
							{
								Overriders: policyv1alpha1.Overriders{
									FieldOverrider: []policyv1alpha1.FieldOverrider{
										{
											FieldPath: "invalidPath",
											YAML: []policyv1alpha1.YAMLPatchOperation{
												{
													SubPath:  "/db-config",
													Operator: policyv1alpha1.OverriderOpReplace,
													Value:    apiextensionsv1.JSON{Raw: []byte("db: new")},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			req: admission.Request{},
			want: TestResponse{
				Type:    Denied,
				Message: "spec.overrideRules[0].overriders.fieldOverrider[0].fieldPath: Invalid value: \"invalidPath\": JSON pointer must be empty or start with a \"/",
			},
		},
		{
			name: "Handle_InvalidJSONSubPath_DeniesAdmission",
			decoder: &fakeValidationDecoder{
				obj: &policyv1alpha1.OverridePolicy{
					Spec: policyv1alpha1.OverrideSpec{
						OverrideRules: []policyv1alpha1.RuleWithCluster{
							{
								Overriders: policyv1alpha1.Overriders{
									FieldOverrider: []policyv1alpha1.FieldOverrider{
										{
											FieldPath: "/data/config",
											JSON: []policyv1alpha1.JSONPatchOperation{
												{
													SubPath:  "invalidSubPath",
													Operator: policyv1alpha1.OverriderOpReplace,
													Value:    apiextensionsv1.JSON{Raw: []byte(`{"db": "new"}`)},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			req: admission.Request{},
			want: TestResponse{
				Type:    Denied,
				Message: "spec.overrideRules[0].overriders.fieldOverrider[0].json[0].subPath: Invalid value: \"invalidSubPath\": JSON pointer must be empty or start with a \"/",
			},
		},
		{
			name: "Handle_InvalidYAMLSubPath_DeniesAdmission",
			decoder: &fakeValidationDecoder{
				obj: &policyv1alpha1.OverridePolicy{
					Spec: policyv1alpha1.OverrideSpec{
						OverrideRules: []policyv1alpha1.RuleWithCluster{
							{
								Overriders: policyv1alpha1.Overriders{
									FieldOverrider: []policyv1alpha1.FieldOverrider{
										{
											FieldPath: "/data/config",
											YAML: []policyv1alpha1.YAMLPatchOperation{
												{
													SubPath:  "invalidSubPath",
													Operator: policyv1alpha1.OverriderOpReplace,
													Value:    apiextensionsv1.JSON{Raw: []byte("db: new")},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			req: admission.Request{},
			want: TestResponse{
				Type:    Denied,
				Message: "spec.overrideRules[0].overriders.fieldOverrider[0].yaml[0].subPath: Invalid value: \"invalidSubPath\": JSON pointer must be empty or start with a \"/",
			},
		},
		{
			name: "Handle_InvalidJSONValue_DeniesAdmission_OverriderOpReplace",
			decoder: &fakeValidationDecoder{
				obj: &policyv1alpha1.OverridePolicy{
					Spec: policyv1alpha1.OverrideSpec{
						OverrideRules: []policyv1alpha1.RuleWithCluster{
							{
								Overriders: policyv1alpha1.Overriders{
									FieldOverrider: []policyv1alpha1.FieldOverrider{
										{
											FieldPath: "/data/config",
											JSON: []policyv1alpha1.JSONPatchOperation{
												{
													SubPath:  "/db-config",
													Operator: policyv1alpha1.OverriderOpReplace,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			req: admission.Request{},
			want: TestResponse{
				Type:    Denied,
				Message: "spec.overrideRules[0].overriders.fieldOverrider[0].json[0].value: Invalid value: v1.JSON{Raw:[]uint8(nil)}: value is required for add or replace operation",
			},
		},
		{
			name: "Handle_InvalidJSONValue_DeniesAdmission_OverriderOpAdd",
			decoder: &fakeValidationDecoder{
				obj: &policyv1alpha1.OverridePolicy{
					Spec: policyv1alpha1.OverrideSpec{
						OverrideRules: []policyv1alpha1.RuleWithCluster{
							{
								Overriders: policyv1alpha1.Overriders{
									FieldOverrider: []policyv1alpha1.FieldOverrider{
										{
											FieldPath: "/data/config",
											JSON: []policyv1alpha1.JSONPatchOperation{
												{
													SubPath:  "/db-config",
													Operator: policyv1alpha1.OverriderOpAdd,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			req: admission.Request{},
			want: TestResponse{
				Type:    Denied,
				Message: "spec.overrideRules[0].overriders.fieldOverrider[0].json[0].value: Invalid value: v1.JSON{Raw:[]uint8(nil)}: value is required for add or replace operation",
			},
		},
		{
			name: "Handle_InvalidJSONValue_DeniesAdmission_OverriderOpRemove",
			decoder: &fakeValidationDecoder{
				obj: &policyv1alpha1.OverridePolicy{
					Spec: policyv1alpha1.OverrideSpec{
						OverrideRules: []policyv1alpha1.RuleWithCluster{
							{
								Overriders: policyv1alpha1.Overriders{
									FieldOverrider: []policyv1alpha1.FieldOverrider{
										{
											FieldPath: "/data/config",
											JSON: []policyv1alpha1.JSONPatchOperation{
												{
													SubPath:  "/db-config",
													Operator: policyv1alpha1.OverriderOpRemove,
													Value:    apiextensionsv1.JSON{Raw: []byte(`{"db": "new"}`)},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			req: admission.Request{},
			want: TestResponse{
				Type:    Denied,
				Message: "spec.overrideRules[0].overriders.fieldOverrider[0].json[0].value: Invalid value: v1.JSON{Raw:[]uint8{0x7b, 0x22, 0x64, 0x62, 0x22, 0x3a, 0x20, 0x22, 0x6e, 0x65, 0x77, 0x22, 0x7d}}: value is not allowed for remove operation",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &ValidatingAdmission{
				Decoder: tt.decoder,
			}
			got := v.Handle(context.Background(), tt.req)

			// Extract type and message from the actual response.
			gotType := extractResponseType(got)
			gotMessage := extractErrorMessage(got)

			if gotType != tt.want.Type || !strings.Contains(gotMessage, tt.want.Message) {
				t.Errorf("Handle() = {Type: %v, Message: %v}, want {Type: %v, Message: %v}", gotType, gotMessage, tt.want.Type, tt.want.Message)
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
