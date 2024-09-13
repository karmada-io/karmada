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

package interpreter

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"gomodules.xyz/jsonpatch/v2"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

// customError is a helper struct for simulating errors.
type customError struct {
	msg string
}

func (e *customError) Error() string {
	return e.msg
}

func TestErrored(t *testing.T) {
	err := &customError{"Test Error"}
	code := int32(500)
	response := Errored(code, err)

	expectedResponse := Response{
		ResourceInterpreterResponse: configv1alpha1.ResourceInterpreterResponse{
			Successful: false,
			Status: &configv1alpha1.RequestStatus{
				Code:    code,
				Message: err.Error(),
			},
		},
	}

	if !reflect.DeepEqual(expectedResponse, response) {
		t.Errorf("response mismatch: expected %v, got %v", expectedResponse, response)
	}
}

func TestSucceeded(t *testing.T) {
	message := "Operation succeeded"
	response := Succeeded(message)

	expectedResponse := ValidationResponse(true, message)

	if !reflect.DeepEqual(expectedResponse, response) {
		t.Errorf("response mismatch: expected %v, got %v", expectedResponse, response)
	}
}

func TestValidationResponse(t *testing.T) {
	tests := []struct {
		name         string
		msg          string
		isSuccessful bool
		want         Response
	}{
		{
			name:         "ValidationResponse_IsSuccessful_Succeeded",
			msg:          "Success",
			isSuccessful: true,
			want: Response{
				ResourceInterpreterResponse: configv1alpha1.ResourceInterpreterResponse{
					Successful: true,
					Status: &configv1alpha1.RequestStatus{
						Code:    int32(http.StatusOK),
						Message: "Success",
					},
				},
			},
		},
		{
			name:         "ValidationResponse_IsFailed_Failed",
			msg:          "Failed",
			isSuccessful: false,
			want: Response{
				ResourceInterpreterResponse: configv1alpha1.ResourceInterpreterResponse{
					Successful: false,
					Status: &configv1alpha1.RequestStatus{
						Code:    int32(http.StatusForbidden),
						Message: "Failed",
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := ValidationResponse(test.isSuccessful, test.msg)
			if !reflect.DeepEqual(response, test.want) {
				t.Errorf("expected response %v but got response %v", test.want, response)
			}
		})
	}
}

func TestPatchResponseFromRaw(t *testing.T) {
	tests := []struct {
		name              string
		original, current []byte
		expectedPatch     []jsonpatch.Operation
		res               Response
		want              Response
		prep              func(wantRes *Response) error
	}{
		{
			name:     "PatchResponseFromRaw_ReplacePatch_ReplacePatchExpected",
			original: []byte(fmt.Sprintf(`{"name": "%s"}`, "original")),
			current:  []byte(fmt.Sprintf(`{"name": "%s"}`, "current")),
			prep: func(wantRes *Response) error {
				expectedPatch := []jsonpatch.Operation{
					{
						Operation: "replace",
						Path:      "/name",
						Value:     "current",
					},
				}
				expectedPatchJSON, err := json.Marshal(expectedPatch)
				if err != nil {
					return fmt.Errorf("marshal failure: %v", err)
				}
				wantRes.ResourceInterpreterResponse.Patch = expectedPatchJSON

				return nil
			},
			want: Response{
				ResourceInterpreterResponse: configv1alpha1.ResourceInterpreterResponse{
					Successful: true,
					PatchType:  func() *configv1alpha1.PatchType { pt := configv1alpha1.PatchTypeJSONPatch; return &pt }(),
				},
			},
		},
		{
			name:     "PatchResponseFromRaw_OriginalSameAsCurrentValue_NoPatchExpected",
			original: []byte(fmt.Sprintf(`{"name": "%s"}`, "same")),
			current:  []byte(fmt.Sprintf(`{"name": "%s"}`, "same")),
			prep:     func(*Response) error { return nil },
			want:     Succeeded(""),
		},
		{
			name:     "PatchResponseFromRaw_InvalidJSONDocument_JSONDocumentIsInvalid",
			original: []byte(nil),
			current:  []byte("updated"),
			prep:     func(*Response) error { return nil },
			want:     Errored(http.StatusInternalServerError, &customError{"invalid JSON Document"}),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(&test.want); err != nil {
				t.Errorf("failed to prep for patching response from raw: %v", err)
			}
			patchResponse := PatchResponseFromRaw(test.original, test.current)
			if !reflect.DeepEqual(patchResponse, test.want) {
				t.Errorf("unexpected error, patch responses not matched; expected %v, but got %v", patchResponse, test.want)
			}
		})
	}
}
