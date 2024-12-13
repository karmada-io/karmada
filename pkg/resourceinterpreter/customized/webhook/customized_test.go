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

package webhook

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	webhookutil "k8s.io/apiserver/pkg/util/webhook"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/customized/webhook/configmanager"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/customized/webhook/request"
)

func TestHookEnabled(t *testing.T) {
	tests := []struct {
		name      string
		hasSynced bool
		hooks     []configmanager.WebhookAccessor
		gvk       schema.GroupVersionKind
		operation configv1alpha1.InterpreterOperation
		want      bool
	}{
		{
			name:      "not synced",
			hasSynced: false,
			want:      false,
		},
		{
			name:      "no hooks",
			hasSynced: true,
			hooks:     []configmanager.WebhookAccessor{},
			gvk: schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
			operation: configv1alpha1.InterpreterOperationInterpretReplica,
			want:      false,
		},
		{
			name:      "matching hook exists",
			hasSynced: true,
			hooks: []configmanager.WebhookAccessor{
				&mockWebhookAccessor{
					uid: "test-hook",
					rules: []configv1alpha1.RuleWithOperations{
						{
							Operations: []configv1alpha1.InterpreterOperation{
								configv1alpha1.InterpreterOperationInterpretReplica,
							},
							Rule: configv1alpha1.Rule{
								APIGroups:   []string{"apps"},
								APIVersions: []string{"v1"},
								Kinds:       []string{"Deployment"},
							},
						},
					},
				},
			},
			gvk: schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
			operation: configv1alpha1.InterpreterOperationInterpretReplica,
			want:      true,
		},
		{
			name:      "no matching hook",
			hasSynced: true,
			hooks: []configmanager.WebhookAccessor{
				&mockWebhookAccessor{
					uid: "test-hook",
					rules: []configv1alpha1.RuleWithOperations{
						{
							Operations: []configv1alpha1.InterpreterOperation{
								configv1alpha1.InterpreterOperationInterpretReplica,
							},
							Rule: configv1alpha1.Rule{
								APIGroups:   []string{"apps"},
								APIVersions: []string{"v1"},
								Kinds:       []string{"Deployment"},
							},
						},
					},
				},
			},
			gvk: schema.GroupVersionKind{
				Group:   "batch",
				Version: "v1",
				Kind:    "Job",
			},
			operation: configv1alpha1.InterpreterOperationInterpretReplica,
			want:      false,
		},
		{
			name:      "hook with wildcard matches",
			hasSynced: true,
			hooks: []configmanager.WebhookAccessor{
				&mockWebhookAccessor{
					uid: "test-hook",
					rules: []configv1alpha1.RuleWithOperations{
						{
							Operations: []configv1alpha1.InterpreterOperation{
								configv1alpha1.InterpreterOperationInterpretReplica,
							},
							Rule: configv1alpha1.Rule{
								APIGroups:   []string{"*"},
								APIVersions: []string{"*"},
								Kinds:       []string{"*"},
							},
						},
					},
				},
			},
			gvk: schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
			operation: configv1alpha1.InterpreterOperationInterpretReplica,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interpreter := &CustomizedInterpreter{
				hookManager: &mockConfigManager{
					hasSynced: tt.hasSynced,
					hooks:     tt.hooks,
				},
			}

			got := interpreter.HookEnabled(tt.gvk, tt.operation)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetReplicas(t *testing.T) {
	tests := []struct {
		name          string
		hasSynced     bool
		hooks         []configmanager.WebhookAccessor
		attributes    *request.Attributes
		wantReplicas  int32
		wantRequires  *workv1alpha2.ReplicaRequirements
		wantMatched   bool
		wantErr       bool
		errorContains string
	}{
		{
			name:      "not synced",
			hasSynced: false,
			attributes: &request.Attributes{
				Object: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name":      "test-deployment",
							"namespace": "test-ns",
						},
					},
				},
				Operation: configv1alpha1.InterpreterOperationInterpretReplica,
			},
			wantErr:       true,
			errorContains: "not yet ready to handle request",
		},
		{
			name:      "no matching hooks",
			hasSynced: true,
			hooks:     []configmanager.WebhookAccessor{},
			attributes: &request.Attributes{
				Object: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name":      "test-deployment",
							"namespace": "test-ns",
						},
					},
				},
				Operation: configv1alpha1.InterpreterOperationInterpretReplica,
			},
			wantMatched: false,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interpreter := &CustomizedInterpreter{
				hookManager: &mockConfigManager{
					hasSynced: tt.hasSynced,
					hooks:     tt.hooks,
				},
			}

			replicas, requires, matched, err := interpreter.GetReplicas(context.Background(), tt.attributes)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantMatched, matched)
			if matched {
				assert.Equal(t, tt.wantReplicas, replicas)
				assert.Equal(t, tt.wantRequires, requires)
			}
		})
	}
}

func TestPatch(t *testing.T) {
	tests := []struct {
		name          string
		hasSynced     bool
		hooks         []configmanager.WebhookAccessor
		attributes    *request.Attributes
		wantObject    *unstructured.Unstructured
		wantMatched   bool
		wantErr       bool
		errorContains string
	}{
		{
			name:      "not synced",
			hasSynced: false,
			attributes: &request.Attributes{
				Object: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name":      "test-deployment",
							"namespace": "test-ns",
						},
					},
				},
				Operation: configv1alpha1.InterpreterOperationRetain,
			},
			wantErr:       true,
			errorContains: "not yet ready to handle request",
		},
		{
			name:      "no matching hooks",
			hasSynced: true,
			hooks:     []configmanager.WebhookAccessor{},
			attributes: &request.Attributes{
				Object: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name":      "test-deployment",
							"namespace": "test-ns",
						},
					},
				},
				Operation: configv1alpha1.InterpreterOperationRetain,
			},
			wantMatched: false,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interpreter := &CustomizedInterpreter{
				hookManager: &mockConfigManager{
					hasSynced: tt.hasSynced,
					hooks:     tt.hooks,
				},
			}

			object, matched, err := interpreter.Patch(context.Background(), tt.attributes)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantMatched, matched)
			if matched {
				assert.Equal(t, tt.wantObject, object)
			}
		})
	}
}

func TestGetFirstRelevantHook(t *testing.T) {
	tests := []struct {
		name      string
		hooks     []configmanager.WebhookAccessor
		gvk       schema.GroupVersionKind
		operation configv1alpha1.InterpreterOperation
		wantHook  string
	}{
		{
			name:  "no hooks",
			hooks: []configmanager.WebhookAccessor{},
			gvk: schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
			operation: configv1alpha1.InterpreterOperationInterpretReplica,
			wantHook:  "",
		},
		{
			name: "single matching hook",
			hooks: []configmanager.WebhookAccessor{
				&mockWebhookAccessor{
					uid: "hook-1",
					rules: []configv1alpha1.RuleWithOperations{
						{
							Operations: []configv1alpha1.InterpreterOperation{
								configv1alpha1.InterpreterOperationInterpretReplica,
							},
							Rule: configv1alpha1.Rule{
								APIGroups:   []string{"apps"},
								APIVersions: []string{"v1"},
								Kinds:       []string{"Deployment"},
							},
						},
					},
				},
			},
			gvk: schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
			operation: configv1alpha1.InterpreterOperationInterpretReplica,
			wantHook:  "hook-1",
		},
		{
			name: "multiple matching hooks - should select alphabetically first",
			hooks: []configmanager.WebhookAccessor{
				&mockWebhookAccessor{
					uid: "hook-2",
					rules: []configv1alpha1.RuleWithOperations{
						{
							Operations: []configv1alpha1.InterpreterOperation{
								configv1alpha1.InterpreterOperationInterpretReplica,
							},
							Rule: configv1alpha1.Rule{
								APIGroups:   []string{"apps"},
								APIVersions: []string{"v1"},
								Kinds:       []string{"Deployment"},
							},
						},
					},
				},
				&mockWebhookAccessor{
					uid: "hook-1",
					rules: []configv1alpha1.RuleWithOperations{
						{
							Operations: []configv1alpha1.InterpreterOperation{
								configv1alpha1.InterpreterOperationInterpretReplica,
							},
							Rule: configv1alpha1.Rule{
								APIGroups:   []string{"apps"},
								APIVersions: []string{"v1"},
								Kinds:       []string{"Deployment"},
							},
						},
					},
				},
			},
			gvk: schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
			operation: configv1alpha1.InterpreterOperationInterpretReplica,
			wantHook:  "hook-1",
		},
		{
			name: "wildcard matches",
			hooks: []configmanager.WebhookAccessor{
				&mockWebhookAccessor{
					uid: "hook-1",
					rules: []configv1alpha1.RuleWithOperations{
						{
							Operations: []configv1alpha1.InterpreterOperation{
								configv1alpha1.InterpreterOperationInterpretReplica,
							},
							Rule: configv1alpha1.Rule{
								APIGroups:   []string{"*"},
								APIVersions: []string{"*"},
								Kinds:       []string{"*"},
							},
						},
					},
				},
			},
			gvk: schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
			operation: configv1alpha1.InterpreterOperationInterpretReplica,
			wantHook:  "hook-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interpreter := &CustomizedInterpreter{
				hookManager: &mockConfigManager{
					hasSynced: true,
					hooks:     tt.hooks,
				},
			}

			got := interpreter.getFirstRelevantHook(tt.gvk, tt.operation)
			if tt.wantHook == "" {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.Equal(t, tt.wantHook, got.GetUID())
			}
		})
	}
}

func TestInterpret(t *testing.T) {
	defaultDeployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name": "test",
			},
		},
	}

	tests := []struct {
		name          string
		hasSynced     bool
		hooks         []configmanager.WebhookAccessor
		attributes    *request.Attributes
		wantResponse  *request.ResponseAttributes
		wantMatched   bool
		wantErr       bool
		errorContains string
		setupContext  func() (context.Context, context.CancelFunc)
	}{
		{
			name:      "not synced",
			hasSynced: false,
			attributes: &request.Attributes{
				Object:    defaultDeployment,
				Operation: configv1alpha1.InterpreterOperationInterpretReplica,
			},
			wantErr:       true,
			errorContains: "not yet ready to handle request",
		},
		{
			name:      "no matching hooks",
			hasSynced: true,
			hooks:     []configmanager.WebhookAccessor{},
			attributes: &request.Attributes{
				Object:    defaultDeployment,
				Operation: configv1alpha1.InterpreterOperationInterpretReplica,
			},
			wantMatched: false,
			wantErr:     false,
		},
		{
			name:      "context timeout",
			hasSynced: true,
			hooks: []configmanager.WebhookAccessor{
				&mockWebhookAccessor{
					uid: "test-hook",
					rules: []configv1alpha1.RuleWithOperations{
						{
							Operations: []configv1alpha1.InterpreterOperation{
								configv1alpha1.InterpreterOperationInterpretReplica,
							},
							Rule: configv1alpha1.Rule{
								APIGroups:   []string{"apps"},
								APIVersions: []string{"v1"},
								Kinds:       []string{"Deployment"},
							},
						},
					},
				},
			},
			attributes: &request.Attributes{
				Object:    defaultDeployment,
				Operation: configv1alpha1.InterpreterOperationInterpretReplica,
			},
			wantMatched: true,
			wantErr:     true,
			setupContext: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), time.Nanosecond)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interpreter := &CustomizedInterpreter{
				hookManager: &mockConfigManager{
					hasSynced: tt.hasSynced,
					hooks:     tt.hooks,
				},
			}

			ctx := context.Background()
			var cancel context.CancelFunc
			if tt.setupContext != nil {
				ctx, cancel = tt.setupContext()
				defer cancel()
			}

			response, matched, err := interpreter.interpret(ctx, tt.attributes)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantMatched, matched)
			if matched {
				assert.Equal(t, tt.wantResponse, response)
			}
		})
	}
}

func TestShouldCallHook(t *testing.T) {
	tests := []struct {
		name      string
		hook      *mockWebhookAccessor
		gvk       schema.GroupVersionKind
		operation configv1alpha1.InterpreterOperation
		want      bool
	}{
		{
			name: "exact match",
			hook: &mockWebhookAccessor{
				rules: []configv1alpha1.RuleWithOperations{
					{
						Operations: []configv1alpha1.InterpreterOperation{
							configv1alpha1.InterpreterOperationInterpretReplica,
						},
						Rule: configv1alpha1.Rule{
							APIGroups:   []string{"apps"},
							APIVersions: []string{"v1"},
							Kinds:       []string{"Deployment"},
						},
					},
				},
			},
			gvk: schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
			operation: configv1alpha1.InterpreterOperationInterpretReplica,
			want:      true,
		},
		{
			name: "wildcard match",
			hook: &mockWebhookAccessor{
				rules: []configv1alpha1.RuleWithOperations{
					{
						Operations: []configv1alpha1.InterpreterOperation{
							configv1alpha1.InterpreterOperationInterpretReplica,
						},
						Rule: configv1alpha1.Rule{
							APIGroups:   []string{"*"},
							APIVersions: []string{"*"},
							Kinds:       []string{"*"},
						},
					},
				},
			},
			gvk: schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
			operation: configv1alpha1.InterpreterOperationInterpretReplica,
			want:      true,
		},
		{
			name: "no operation match",
			hook: &mockWebhookAccessor{
				rules: []configv1alpha1.RuleWithOperations{
					{
						Operations: []configv1alpha1.InterpreterOperation{
							configv1alpha1.InterpreterOperationInterpretHealth,
						},
						Rule: configv1alpha1.Rule{
							APIGroups:   []string{"apps"},
							APIVersions: []string{"v1"},
							Kinds:       []string{"Deployment"},
						},
					},
				},
			},
			gvk: schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
			operation: configv1alpha1.InterpreterOperationInterpretReplica,
			want:      false,
		},
		{
			name: "no kind match",
			hook: &mockWebhookAccessor{
				rules: []configv1alpha1.RuleWithOperations{
					{
						Operations: []configv1alpha1.InterpreterOperation{
							configv1alpha1.InterpreterOperationInterpretReplica,
						},
						Rule: configv1alpha1.Rule{
							APIGroups:   []string{"apps"},
							APIVersions: []string{"v1"},
							Kinds:       []string{"StatefulSet"},
						},
					},
				},
			},
			gvk: schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
			operation: configv1alpha1.InterpreterOperationInterpretReplica,
			want:      false,
		},
		{
			name: "multiple rules - one matches",
			hook: &mockWebhookAccessor{
				rules: []configv1alpha1.RuleWithOperations{
					{
						Operations: []configv1alpha1.InterpreterOperation{
							configv1alpha1.InterpreterOperationInterpretHealth,
						},
						Rule: configv1alpha1.Rule{
							APIGroups:   []string{"apps"},
							APIVersions: []string{"v1"},
							Kinds:       []string{"StatefulSet"},
						},
					},
					{
						Operations: []configv1alpha1.InterpreterOperation{
							configv1alpha1.InterpreterOperationInterpretReplica,
						},
						Rule: configv1alpha1.Rule{
							APIGroups:   []string{"apps"},
							APIVersions: []string{"v1"},
							Kinds:       []string{"Deployment"},
						},
					},
				},
			},
			gvk: schema.GroupVersionKind{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			},
			operation: configv1alpha1.InterpreterOperationInterpretReplica,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldCallHook(tt.hook, tt.gvk, tt.operation)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestApplyPatch(t *testing.T) {
	defaultObject := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name": "test",
			},
		},
	}

	tests := []struct {
		name      string
		object    *unstructured.Unstructured
		patch     []byte
		patchType configv1alpha1.PatchType
		wantErr   bool
		validate  func(*testing.T, *unstructured.Unstructured)
	}{
		{
			name:      "empty patch and empty patch type",
			object:    defaultObject.DeepCopy(),
			patch:     []byte{},
			patchType: "",
			wantErr:   false,
			validate: func(t *testing.T, got *unstructured.Unstructured) {
				assert.Equal(t, "test", got.GetName())
			},
		},
		{
			name:      "empty patch with JSONPatch type",
			object:    defaultObject.DeepCopy(),
			patch:     []byte{},
			patchType: configv1alpha1.PatchTypeJSONPatch,
			wantErr:   false,
			validate: func(t *testing.T, got *unstructured.Unstructured) {
				assert.Equal(t, "test", got.GetName())
			},
		},
		{
			name:      "empty JSON patch array",
			object:    defaultObject.DeepCopy(),
			patch:     []byte(`[]`),
			patchType: configv1alpha1.PatchTypeJSONPatch,
			wantErr:   false,
			validate: func(t *testing.T, got *unstructured.Unstructured) {
				assert.Equal(t, "test", got.GetName())
			},
		},
		{
			name:      "valid JSON patch - replace",
			object:    defaultObject.DeepCopy(),
			patch:     []byte(`[{"op": "replace", "path": "/metadata/name", "value": "test-patched"}]`),
			patchType: configv1alpha1.PatchTypeJSONPatch,
			wantErr:   false,
			validate: func(t *testing.T, got *unstructured.Unstructured) {
				assert.Equal(t, "test-patched", got.GetName())
			},
		},
		{
			name:      "valid JSON patch - add",
			object:    defaultObject.DeepCopy(),
			patch:     []byte(`[{"op": "add", "path": "/metadata/annotations", "value": {"key": "value"}}]`),
			patchType: configv1alpha1.PatchTypeJSONPatch,
			wantErr:   false,
			validate: func(t *testing.T, got *unstructured.Unstructured) {
				annotations := got.GetAnnotations()
				assert.Equal(t, "value", annotations["key"])
			},
		},
		{
			name:      "invalid JSON patch format",
			object:    defaultObject.DeepCopy(),
			patch:     []byte(`invalid json patch`),
			patchType: configv1alpha1.PatchTypeJSONPatch,
			wantErr:   true,
		},
		{
			name:      "invalid JSON patch operation",
			object:    defaultObject.DeepCopy(),
			patch:     []byte(`[{"op": "invalid", "path": "/metadata/name", "value": "test"}]`),
			patchType: configv1alpha1.PatchTypeJSONPatch,
			wantErr:   true,
		},
		{
			name:      "unsupported patch type",
			object:    defaultObject.DeepCopy(),
			patch:     []byte(`{}`),
			patchType: "UnsupportedType",
			wantErr:   true,
		},
		{
			name:      "invalid object for marshaling",
			object:    &unstructured.Unstructured{Object: map[string]interface{}{"invalid": make(chan int)}},
			patch:     []byte(`[{"op": "replace", "path": "/metadata/name", "value": "test"}]`),
			patchType: configv1alpha1.PatchTypeJSONPatch,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := applyPatch(tt.object, tt.patch, tt.patchType)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tt.validate != nil {
				tt.validate(t, got)
			}
		})
	}
}

func TestGetDependencies(t *testing.T) {
	tests := []struct {
		name          string
		hasSynced     bool
		hooks         []configmanager.WebhookAccessor
		attributes    *request.Attributes
		wantDeps      []configv1alpha1.DependentObjectReference
		wantMatched   bool
		wantErr       bool
		errorContains string
	}{
		{
			name:      "not synced",
			hasSynced: false,
			attributes: &request.Attributes{
				Object: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name":      "test-deployment",
							"namespace": "test-ns",
						},
					},
				},
				Operation: configv1alpha1.InterpreterOperationInterpretDependency,
			},
			wantErr:       true,
			errorContains: "not yet ready to handle request",
		},
		{
			name:      "no matching hooks",
			hasSynced: true,
			hooks:     []configmanager.WebhookAccessor{},
			attributes: &request.Attributes{
				Object: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name":      "test-deployment",
							"namespace": "test-ns",
						},
					},
				},
				Operation: configv1alpha1.InterpreterOperationInterpretDependency,
			},
			wantMatched: false,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interpreter := &CustomizedInterpreter{
				hookManager: &mockConfigManager{
					hasSynced: tt.hasSynced,
					hooks:     tt.hooks,
				},
			}

			deps, matched, err := interpreter.GetDependencies(context.Background(), tt.attributes)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantMatched, matched)
			if matched {
				assert.Equal(t, tt.wantDeps, deps)
			}
		})
	}
}

func TestReflectStatus(t *testing.T) {
	tests := []struct {
		name          string
		hasSynced     bool
		hooks         []configmanager.WebhookAccessor
		attributes    *request.Attributes
		wantStatus    *runtime.RawExtension
		wantMatched   bool
		wantErr       bool
		errorContains string
	}{
		{
			name:      "not synced",
			hasSynced: false,
			attributes: &request.Attributes{
				Object: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name":      "test-deployment",
							"namespace": "test-ns",
						},
					},
				},
				Operation: configv1alpha1.InterpreterOperationAggregateStatus,
			},
			wantErr:       true,
			errorContains: "not yet ready to handle request",
		},
		{
			name:      "no matching hooks",
			hasSynced: true,
			hooks:     []configmanager.WebhookAccessor{},
			attributes: &request.Attributes{
				Object: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name":      "test-deployment",
							"namespace": "test-ns",
						},
					},
				},
				Operation: configv1alpha1.InterpreterOperationAggregateStatus,
			},
			wantMatched: false,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interpreter := &CustomizedInterpreter{
				hookManager: &mockConfigManager{
					hasSynced: tt.hasSynced,
					hooks:     tt.hooks,
				},
			}

			status, matched, err := interpreter.ReflectStatus(context.Background(), tt.attributes)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantMatched, matched)
			if matched {
				assert.Equal(t, tt.wantStatus, status)
			}
		})
	}
}

func TestInterpretHealth(t *testing.T) {
	tests := []struct {
		name          string
		hasSynced     bool
		hooks         []configmanager.WebhookAccessor
		attributes    *request.Attributes
		wantHealthy   bool
		wantMatched   bool
		wantErr       bool
		errorContains string
	}{
		{
			name:      "not synced",
			hasSynced: false,
			attributes: &request.Attributes{
				Object: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name":      "test-deployment",
							"namespace": "test-ns",
						},
					},
				},
				Operation: configv1alpha1.InterpreterOperationInterpretHealth,
			},
			wantErr:       true,
			errorContains: "not yet ready to handle request",
		},
		{
			name:      "no matching hooks",
			hasSynced: true,
			hooks:     []configmanager.WebhookAccessor{},
			attributes: &request.Attributes{
				Object: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name":      "test-deployment",
							"namespace": "test-ns",
						},
					},
				},
				Operation: configv1alpha1.InterpreterOperationInterpretHealth,
			},
			wantMatched: false,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interpreter := &CustomizedInterpreter{
				hookManager: &mockConfigManager{
					hasSynced: tt.hasSynced,
					hooks:     tt.hooks,
				},
			}

			healthy, matched, err := interpreter.InterpretHealth(context.Background(), tt.attributes)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantMatched, matched)
			if matched {
				assert.Equal(t, tt.wantHealthy, healthy)
			}
		})
	}
}

// Mock Implementations

// mockConfigManager implements configmanager.ConfigManager interface for testing
type mockConfigManager struct {
	hasSynced bool
	hooks     []configmanager.WebhookAccessor
}

func (m *mockConfigManager) HasSynced() bool {
	return m.hasSynced
}

func (m *mockConfigManager) HookAccessors() []configmanager.WebhookAccessor {
	return m.hooks
}

// mockWebhookAccessor implements configmanager.WebhookAccessor interface for testing
type mockWebhookAccessor struct {
	uid             string
	name            string
	configName      string
	rules           []configv1alpha1.RuleWithOperations
	timeoutSeconds  *int32
	contextVersions []string
}

func (m *mockWebhookAccessor) GetUID() string                                { return m.uid }
func (m *mockWebhookAccessor) GetName() string                               { return m.name }
func (m *mockWebhookAccessor) GetConfigurationName() string                  { return m.configName }
func (m *mockWebhookAccessor) GetRules() []configv1alpha1.RuleWithOperations { return m.rules }
func (m *mockWebhookAccessor) GetTimeoutSeconds() *int32                     { return m.timeoutSeconds }
func (m *mockWebhookAccessor) GetInterpreterContextVersions() []string       { return m.contextVersions }
func (m *mockWebhookAccessor) GetClientConfig() admissionregistrationv1.WebhookClientConfig {
	return admissionregistrationv1.WebhookClientConfig{
		URL: ptr.To("https://test-webhook"),
	}
}
func (m *mockWebhookAccessor) GetRESTClient(_ *webhookutil.ClientManager) (*rest.RESTClient, error) {
	return nil, nil
}
