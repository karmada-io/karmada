/*
Copyright 2021 The Karmada Authors.

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
package clusterrestriction

import (
	"fmt"
	"reflect"
	"testing"

	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func TestMutatingAdmitCreate(t *testing.T) {
	testAgentName := "system:node:cluster-1"
	testCases := []struct {
		name           string
		object         metav1.Object
		agentOperating string
		expected       admission.Response
	}{
		{
			name:           `no owner annotation should add the owner annotation with itself`,
			object:         &metav1.ObjectMeta{},
			agentOperating: testAgentName,
			expected: admission.Patched(
				fmt.Sprintf(`Patched annotation key/value: %s=%s`, OwnerAnnotationKey, testAgentName),
				jsonpatch.NewOperation("add", "/metadata/annotations", map[string]string{}),
				jsonpatch.NewOperation("add", ownerAnnotationPath, testAgentName),
			),
		},
		{
			name: `with different owner annotation should deny`,
			object: &metav1.ObjectMeta{
				Annotations: map[string]string{
					OwnerAnnotationKey: "different-agentName",
				},
			},
			agentOperating: testAgentName,
			expected:       denyToOperateOwnerAnnotation,
		},
		{
			name: `with empty owner annotation should deny`,
			object: &metav1.ObjectMeta{
				Annotations: map[string]string{
					OwnerAnnotationKey: "",
				},
			},
			agentOperating: testAgentName,
			expected:       denyToOperateOwnerAnnotation,
		},
		{
			name: `with self owner annotation should deny`,
			object: &metav1.ObjectMeta{
				Annotations: map[string]string{
					OwnerAnnotationKey: testAgentName,
				},
			},
			agentOperating: testAgentName,
			expected:       denyToOperateOwnerAnnotation,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			a := MutatingAdmission{}
			got := a.admitCreate(
				testCase.object,
				testCase.agentOperating,
			)
			if !reflect.DeepEqual(got, testCase.expected) {
				t.Errorf("want %+v, but got %+v", testCase.expected, got)
			}
		})
	}
}

func TestMutatingAdmitConnect(t *testing.T) {
	testAgentName := "system:node:cluster-1"
	connectDenied := cantOperateOwnedByAnother(admissionv1.Connect)

	testCases := []struct {
		name           string
		object         metav1.Object
		agentOperating string
		expected       admission.Response
	}{
		{
			name: `with self owner annotation should allow`,
			object: &metav1.ObjectMeta{
				Annotations: map[string]string{
					OwnerAnnotationKey: testAgentName,
				},
			},
			agentOperating: testAgentName,
			expected:       admission.Allowed(""),
		},
		{
			name: `with different owner annotation should deny`,
			object: &metav1.ObjectMeta{
				Annotations: map[string]string{
					OwnerAnnotationKey: "different-agentName",
				},
			},
			agentOperating: testAgentName,
			expected:       connectDenied,
		},
		{
			name: `with empty owner annotation should deny`,
			object: &metav1.ObjectMeta{
				Annotations: map[string]string{
					OwnerAnnotationKey: "",
				},
			},
			agentOperating: testAgentName,
			expected:       connectDenied,
		},
		{
			name:           `no owner annotation should allow for backward compatibility`,
			object:         &metav1.ObjectMeta{},
			agentOperating: testAgentName,
			expected:       allowForBackwardCompatibility,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			a := MutatingAdmission{}
			got := a.admitConnect(
				testCase.object,
				testCase.agentOperating,
			)
			if !reflect.DeepEqual(got, testCase.expected) {
				t.Errorf("want %+v, but got %+v", testCase.expected, got)
			}
		})
	}
}

func TestMutatingAdmitUpdate(t *testing.T) {
	testMutatingAdmitUpdateOrDelete(t, admissionv1.Update)
}
func TestMutatingAdmitDelete(t *testing.T) {
	testMutatingAdmitUpdateOrDelete(t, admissionv1.Delete)
}

func testMutatingAdmitUpdateOrDelete(t *testing.T, operation admissionv1.Operation) {
	testAgentName := "system:node:cluster-1"
	cantOperateOthers := cantOperateOwnedByAnother(operation)

	testCases := []struct {
		name           string
		object         metav1.Object
		oldObject      metav1.Object
		agentOperating string
		expected       admission.Response
	}{
		{
			name: `owner annotation is not changed and owned by itself should allow`,
			object: &metav1.ObjectMeta{
				Annotations: map[string]string{
					OwnerAnnotationKey: testAgentName,
				},
			},
			oldObject: &metav1.ObjectMeta{
				Annotations: map[string]string{
					OwnerAnnotationKey: testAgentName,
				},
			},
			agentOperating: testAgentName,
			expected:       admission.Allowed(""),
		},
		{
			name: `owner annotation is not changed but owned by another agent should deny`,
			object: &metav1.ObjectMeta{
				Annotations: map[string]string{
					OwnerAnnotationKey: "different-agent",
				},
			},
			oldObject: &metav1.ObjectMeta{
				Annotations: map[string]string{
					OwnerAnnotationKey: "different-agent",
				},
			},
			agentOperating: testAgentName,
			expected:       cantOperateOthers,
		},
		{
			name: `updating owner annotation to itself (stealing) should deny`,
			object: &metav1.ObjectMeta{
				Annotations: map[string]string{
					OwnerAnnotationKey: testAgentName,
				},
			},
			oldObject: &metav1.ObjectMeta{
				Annotations: map[string]string{
					OwnerAnnotationKey: "different-agent",
				},
			},
			agentOperating: testAgentName,
			expected:       cantOperateOthers,
		},
		{
			name: `updating owner annotation to other (giving away) should deny`,
			object: &metav1.ObjectMeta{
				Annotations: map[string]string{
					OwnerAnnotationKey: "another-agent",
				},
			},
			oldObject: &metav1.ObjectMeta{
				Annotations: map[string]string{
					OwnerAnnotationKey: testAgentName,
				},
			},
			agentOperating: testAgentName,
			expected:       denyToOperateOwnerAnnotation,
		},
		{
			name: `adding owner annotation should deny`,
			object: &metav1.ObjectMeta{
				Annotations: map[string]string{
					OwnerAnnotationKey: "any",
				},
			},
			oldObject:      &metav1.ObjectMeta{},
			agentOperating: testAgentName,
			expected:       denyToOperateOwnerAnnotation,
		},
		{
			name:   `deleting owner annotation should deny`,
			object: &metav1.ObjectMeta{},
			oldObject: &metav1.ObjectMeta{
				Annotations: map[string]string{
					OwnerAnnotationKey: "any",
				},
			},
			agentOperating: testAgentName,
			expected:       denyToOperateOwnerAnnotation,
		},
		{
			name: `no owner annotation should allow for backward compatibility`,
			object: &metav1.ObjectMeta{
				Labels: map[string]string{"key": "after"},
			},
			oldObject: &metav1.ObjectMeta{
				Labels: map[string]string{"key": "after"},
			},
			agentOperating: testAgentName,
			expected:       allowForBackwardCompatibility,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			a := MutatingAdmission{}
			got := a.admitUpdateOrDelete(
				testCase.object,
				testCase.oldObject,
				testCase.agentOperating,
				operation,
			)
			if !reflect.DeepEqual(got, testCase.expected) {
				t.Errorf("want %+v, but got %+v", testCase.expected, got)
			}
		})
	}
}

func TestCreateOwnerAnnotationPatch(t *testing.T) {
	testOwnerName := "owner"

	testCases := []struct {
		name     string
		object   metav1.Object
		expected []jsonpatch.JsonPatchOperation
	}{
		{
			name:   `no annotations`,
			object: &metav1.ObjectMeta{},
			expected: []jsonpatch.JsonPatchOperation{
				jsonpatch.NewOperation(
					"add",
					"/metadata/annotations",
					map[string]string{},
				),
				jsonpatch.NewOperation(
					"add",
					"/metadata/annotations/clusterrestriction.karmada.io~1owner",
					testOwnerName,
				),
			},
		},
		{
			name: `with annotations, without owner key`,
			object: &metav1.ObjectMeta{
				Annotations: map[string]string{"key": "value"},
			},
			expected: []jsonpatch.JsonPatchOperation{
				jsonpatch.NewOperation(
					"add",
					ownerAnnotationPath,
					testOwnerName,
				),
			},
		},
		{
			name: `with annotations, with owner key`,
			object: &metav1.ObjectMeta{
				Annotations: map[string]string{OwnerAnnotationKey: "system:node:cluster-0"},
			},
			expected: []jsonpatch.JsonPatchOperation{
				jsonpatch.NewOperation(
					"replace",
					ownerAnnotationPath,
					testOwnerName,
				),
			},
		},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			got := createOwnerAnnotationPatch(
				testOwnerName,
				testCase.object,
			)
			if !reflect.DeepEqual(got, testCase.expected) {
				t.Errorf("want %+v, but got %+v", testCase.expected, got)
			}
		})
	}
}
