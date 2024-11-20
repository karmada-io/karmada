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

package request

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

func TestCreateResourceInterpreterContext(t *testing.T) {
	const (
		testAPIVersion = "apps/v1"
		testKind       = "Deployment"
		testName       = "test-deployment"
		testNamespace  = "default"
	)

	attributes := &Attributes{
		Object: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": testAPIVersion,
				"kind":       testKind,
				"metadata": map[string]interface{}{
					"name":      testName,
					"namespace": testNamespace,
				},
			},
		},
		Operation: configv1alpha1.InterpreterOperationInterpretReplica,
	}

	tests := []struct {
		name          string
		versions      []string
		wantError     bool
		errorContains string
	}{
		{
			name:      "valid v1alpha1 version",
			versions:  []string{"v1alpha1"},
			wantError: false,
		},
		{
			name:      "multiple versions with v1alpha1",
			versions:  []string{"v2", "v1alpha1"},
			wantError: false,
		},
		{
			name:          "unsupported version",
			versions:      []string{"v2"},
			wantError:     true,
			errorContains: "does not accept known ResourceInterpreterContext versions",
		},
		{
			name:          "empty versions",
			versions:      []string{},
			wantError:     true,
			errorContains: "does not accept known ResourceInterpreterContext versions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uid, request, err := CreateResourceInterpreterContext(tt.versions, attributes)

			if tt.wantError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				assert.Empty(t, uid)
				assert.Nil(t, request)
				return
			}

			assert.NoError(t, err)
			assert.NotEmpty(t, uid)
			assert.NotNil(t, request)

			ctx, ok := request.(*configv1alpha1.ResourceInterpreterContext)
			assert.True(t, ok, "request should be of type *ResourceInterpreterContext")

			// Verify the object is not nil
			assert.NotNil(t, ctx)
			assert.Equal(t, testName, ctx.Request.Name)
			assert.Equal(t, testNamespace, ctx.Request.Namespace)
			assert.Equal(t, testAPIVersion, ctx.Request.Kind.Group+"/"+ctx.Request.Kind.Version)
			assert.Equal(t, testKind, ctx.Request.Kind.Kind)
		})
	}
}

func TestCreateV1alpha1ResourceInterpreterContext(t *testing.T) {
	const (
		testUID       = "test-uid"
		testName      = "test-deployment"
		testNamespace = "default"
	)

	tests := []struct {
		name       string
		attributes *Attributes
		checkFunc  func(*testing.T, *configv1alpha1.ResourceInterpreterContext)
	}{
		{
			name: "basic deployment object",
			attributes: &Attributes{
				Object: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name":      testName,
							"namespace": testNamespace,
						},
					},
				},
				Operation:   configv1alpha1.InterpreterOperationInterpretReplica,
				ReplicasSet: 3,
			},
			checkFunc: func(t *testing.T, ctx *configv1alpha1.ResourceInterpreterContext) {
				assert.Equal(t, "apps", ctx.Request.Kind.Group)
				assert.Equal(t, "v1", ctx.Request.Kind.Version)
				assert.Equal(t, "Deployment", ctx.Request.Kind.Kind)
				assert.Equal(t, testName, ctx.Request.Name)
				assert.Equal(t, testNamespace, ctx.Request.Namespace)
				assert.Equal(t, int32(3), *ctx.Request.DesiredReplicas)
				assert.Equal(t, configv1alpha1.InterpreterOperationInterpretReplica, ctx.Request.Operation)
				assert.NotNil(t, ctx.Request.Object.Object)
				assert.Nil(t, ctx.Request.ObservedObject)
			},
		},
		{
			name: "with observed object",
			attributes: &Attributes{
				Object: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Pod",
						"metadata": map[string]interface{}{
							"name": "test-pod",
						},
					},
				},
				ObservedObj: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Pod",
						"metadata": map[string]interface{}{
							"name": "test-pod-observed",
						},
					},
				},
				Operation: configv1alpha1.InterpreterOperationInterpretStatus,
			},
			checkFunc: func(t *testing.T, ctx *configv1alpha1.ResourceInterpreterContext) {
				assert.NotNil(t, ctx.Request.ObservedObject)
				assert.Equal(t, "test-pod-observed", ctx.Request.ObservedObject.Object.(*unstructured.Unstructured).GetName())
				assert.Equal(t, configv1alpha1.InterpreterOperationInterpretStatus, ctx.Request.Operation)
			},
		},
		{
			name: "nil observed object",
			attributes: &Attributes{
				Object: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]interface{}{
							"name": "test-configmap",
						},
					},
				},
				Operation: configv1alpha1.InterpreterOperationInterpretDependency,
			},
			checkFunc: func(t *testing.T, ctx *configv1alpha1.ResourceInterpreterContext) {
				assert.Nil(t, ctx.Request.ObservedObject)
				assert.NotNil(t, ctx.Request.Object.Object)
				assert.Equal(t, "ConfigMap", ctx.Request.Kind.Kind)
				assert.Equal(t, configv1alpha1.InterpreterOperationInterpretDependency, ctx.Request.Operation)
			},
		},
		{
			name: "zero values in attributes",
			attributes: &Attributes{
				Object: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Service",
					},
				},
			},
			checkFunc: func(t *testing.T, ctx *configv1alpha1.ResourceInterpreterContext) {
				assert.NotNil(t, ctx.Request)
				assert.Empty(t, ctx.Request.Name)
				assert.Empty(t, ctx.Request.Namespace)
				assert.Nil(t, ctx.Request.ObservedObject)
				assert.Empty(t, ctx.Request.Operation)
				assert.Equal(t, int32(0), *ctx.Request.DesiredReplicas)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uid := types.UID(testUID)
			ctx := CreateV1alpha1ResourceInterpreterContext(uid, tt.attributes)

			assert.NotNil(t, ctx)
			assert.NotNil(t, ctx.Request)
			assert.Equal(t, uid, ctx.Request.UID)

			tt.checkFunc(t, ctx)
		})
	}
}

func TestVerifyResourceInterpreterContext(t *testing.T) {
	uid := types.UID("test-uid")
	operation := configv1alpha1.InterpreterOperationInterpretReplica

	tests := []struct {
		name        string
		context     runtime.Object
		wantError   bool
		wantSuccess bool
	}{
		{
			name: "valid response with replicas",
			context: &configv1alpha1.ResourceInterpreterContext{
				Response: &configv1alpha1.ResourceInterpreterResponse{
					UID:        uid,
					Successful: true,
					Replicas:   ptr.To(int32(3)),
				},
			},
			wantError:   false,
			wantSuccess: true,
		},
		{
			name: "missing response",
			context: &configv1alpha1.ResourceInterpreterContext{
				Response: nil,
			},
			wantError: true,
		},
		{
			name: "mismatched UID",
			context: &configv1alpha1.ResourceInterpreterContext{
				Response: &configv1alpha1.ResourceInterpreterResponse{
					UID:        "wrong-uid",
					Successful: true,
					Replicas:   ptr.To(int32(3)),
				},
			},
			wantError: true,
		},
		{
			name: "unsuccessful response without status",
			context: &configv1alpha1.ResourceInterpreterContext{
				Response: &configv1alpha1.ResourceInterpreterResponse{
					UID:        uid,
					Successful: false,
				},
			},
			wantError: true,
		},
		{
			name: "invalid context type",
			context: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "InvalidType",
				},
			},
			wantError:   true,
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := VerifyResourceInterpreterContext(uid, operation, tt.context)
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, response)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, response)
				assert.Equal(t, tt.wantSuccess, response.Successful)
			}
		})
	}
}

func TestVerifyResourceInterpreterContextByOperation(t *testing.T) {
	const (
		testUID        = "test-uid"
		testSecretName = "test-secret"
		testSecretAPI  = "v1"
		testSecretKind = "Secret"
		replicaNum     = int32(3)
		invalidOpType  = "invalid-operation"
	)

	tests := []struct {
		name          string
		operation     configv1alpha1.InterpreterOperation
		response      *configv1alpha1.ResourceInterpreterResponse
		wantError     bool
		errorContains string
		checkFunc     func(*testing.T, *ResponseAttributes)
	}{
		{
			name:      "interpret replica with valid response",
			operation: configv1alpha1.InterpreterOperationInterpretReplica,
			response: &configv1alpha1.ResourceInterpreterResponse{
				UID:        types.UID(testUID),
				Successful: true,
				Replicas:   ptr.To(replicaNum),
			},
			checkFunc: func(t *testing.T, attr *ResponseAttributes) {
				assert.Equal(t, replicaNum, attr.Replicas)
			},
		},
		{
			name:      "interpret replica with missing replicas",
			operation: configv1alpha1.InterpreterOperationInterpretReplica,
			response: &configv1alpha1.ResourceInterpreterResponse{
				UID:        types.UID(testUID),
				Successful: true,
			},
			wantError:     true,
			errorContains: "nil response.replicas",
		},
		{
			name:      "interpret dependency with valid response",
			operation: configv1alpha1.InterpreterOperationInterpretDependency,
			response: &configv1alpha1.ResourceInterpreterResponse{
				UID:        types.UID(testUID),
				Successful: true,
				Dependencies: []configv1alpha1.DependentObjectReference{
					{
						APIVersion: testSecretAPI,
						Kind:       testSecretKind,
						Name:       testSecretName,
					},
				},
			},
			checkFunc: func(t *testing.T, attr *ResponseAttributes) {
				require.Len(t, attr.Dependencies, 1)
				assert.Equal(t, testSecretName, attr.Dependencies[0].Name)
				assert.Equal(t, testSecretAPI, attr.Dependencies[0].APIVersion)
				assert.Equal(t, testSecretKind, attr.Dependencies[0].Kind)
			},
		},
		{
			name:      "interpret status with valid response",
			operation: configv1alpha1.InterpreterOperationInterpretStatus,
			response: &configv1alpha1.ResourceInterpreterResponse{
				UID:        types.UID(testUID),
				Successful: true,
				RawStatus: &runtime.RawExtension{
					Object: &unstructured.Unstructured{
						Object: map[string]interface{}{
							"available": true,
							"replicas": map[string]interface{}{
								"ready":     3,
								"total":     5,
								"updated":   2,
								"available": 3,
							},
						},
					},
				},
			},
			checkFunc: func(t *testing.T, attr *ResponseAttributes) {
				require.NotNil(t, attr.RawStatus)
				obj := attr.RawStatus.Object.(*unstructured.Unstructured)
				assert.True(t, obj.Object["available"].(bool))
				replicas := obj.Object["replicas"].(map[string]interface{})
				assert.Equal(t, 3, replicas["ready"])
			},
		},
		{
			name:      "unsuccessful response without status",
			operation: configv1alpha1.InterpreterOperationInterpretHealth,
			response: &configv1alpha1.ResourceInterpreterResponse{
				UID:        types.UID(testUID),
				Successful: false,
			},
			wantError:     true,
			errorContains: "require response.status when response.successful is false",
		},
		{
			name:      "interpret health with valid response",
			operation: configv1alpha1.InterpreterOperationInterpretHealth,
			response: &configv1alpha1.ResourceInterpreterResponse{
				UID:        types.UID(testUID),
				Successful: true,
				Healthy:    ptr.To(true),
			},
			checkFunc: func(t *testing.T, attr *ResponseAttributes) {
				require.NotNil(t, attr.Healthy)
				assert.True(t, attr.Healthy)
			},
		},
		{
			name:      "prune operation with valid patch",
			operation: configv1alpha1.InterpreterOperationPrune,
			response: &configv1alpha1.ResourceInterpreterResponse{
				UID:        types.UID(testUID),
				Successful: true,
				Patch:      []byte(`[{"op": "remove", "path": "/spec/template/spec/containers/0/image"}]`),
				PatchType:  ptr.To(configv1alpha1.PatchTypeJSONPatch),
			},
			checkFunc: func(t *testing.T, attr *ResponseAttributes) {
				assert.NotEmpty(t, attr.Patch)
				assert.Equal(t, configv1alpha1.PatchTypeJSONPatch, attr.PatchType)
				// Verify patch is valid JSON
				var patchObj interface{}
				err := json.Unmarshal(attr.Patch, &patchObj)
				assert.NoError(t, err)
			},
		},
		{
			name:      "invalid operation type",
			operation: configv1alpha1.InterpreterOperation(invalidOpType),
			response: &configv1alpha1.ResourceInterpreterResponse{
				UID:        types.UID(testUID),
				Successful: true,
			},
			wantError:     true,
			errorContains: "wrong operation type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr, err := verifyResourceInterpreterContext(tt.operation, tt.response)

			if tt.wantError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, attr)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, attr)
			if tt.checkFunc != nil {
				tt.checkFunc(t, attr)
			}
		})
	}
}

func TestVerifyResourceInterpreterContextWithPatch(t *testing.T) {
	const (
		validPatch = `[{"op": "add", "path": "/spec/replicas", "value": 3}]`
	)

	tests := []struct {
		name          string
		response      *configv1alpha1.ResourceInterpreterResponse
		wantError     bool
		errorContains string
	}{
		{
			name: "valid JSON patch",
			response: &configv1alpha1.ResourceInterpreterResponse{
				Patch:     []byte(validPatch),
				PatchType: ptr.To(configv1alpha1.PatchTypeJSONPatch),
			},
			wantError: false,
		},
		{
			name: "empty patch with no patch type",
			response: &configv1alpha1.ResourceInterpreterResponse{
				Patch:     nil,
				PatchType: nil,
			},
			wantError: false,
		},
		{
			name: "patch without patch type",
			response: &configv1alpha1.ResourceInterpreterResponse{
				Patch:     []byte(validPatch),
				PatchType: nil,
			},
			wantError:     true,
			errorContains: "nil response.patchType",
		},
		{
			name: "empty patch with patch type",
			response: &configv1alpha1.ResourceInterpreterResponse{
				Patch:     []byte{},
				PatchType: ptr.To(configv1alpha1.PatchTypeJSONPatch),
			},
			wantError:     true,
			errorContains: "empty response.patch",
		},
		{
			name: "nil patch with patch type",
			response: &configv1alpha1.ResourceInterpreterResponse{
				Patch:     nil,
				PatchType: ptr.To(configv1alpha1.PatchTypeJSONPatch),
			},
			wantError:     true,
			errorContains: "empty response.patch",
		},
		{
			name: "invalid patch type",
			response: &configv1alpha1.ResourceInterpreterResponse{
				Patch:     []byte(validPatch),
				PatchType: ptr.To(configv1alpha1.PatchType("invalid")),
			},
			wantError:     true,
			errorContains: "invalid response.patchType",
		},
		{
			name: "long valid JSON patch",
			response: &configv1alpha1.ResourceInterpreterResponse{
				Patch: []byte(`[
                    {"op": "add", "path": "/spec/replicas", "value": 3},
                    {"op": "remove", "path": "/spec/template/spec/containers/0/image"},
                    {"op": "replace", "path": "/metadata/labels/version", "value": "v2"}
                ]`),
				PatchType: ptr.To(configv1alpha1.PatchTypeJSONPatch),
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := verifyResourceInterpreterContextWithPatch(tt.response)

			if tt.wantError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				return
			}

			assert.NoError(t, err)
		})
	}
}
