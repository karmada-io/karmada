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

package scheme

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	flowcontrolv1beta3 "k8s.io/api/flowcontrol/v1beta3"
)

func TestFlowControlV1Beta3Registration(t *testing.T) {
	t.Run("FlowSchema registration", func(t *testing.T) {
		// Verify that FlowSchema is registered for v1beta3
		flowSchemaGVK := schema.GroupVersionKind{
			Group:   "flowcontrol.apiserver.k8s.io",
			Version: "v1beta3",
			Kind:    "FlowSchema",
		}
		if !Scheme.Recognizes(flowSchemaGVK) {
			t.Fatalf("FlowSchema v1beta3 is not registered in the scheme")
		}

		// Verify that we can create instances of FlowSchema
		flowSchema := &flowcontrolv1beta3.FlowSchema{}
		flowSchema.SetName("test-flowschema")
		flowSchema.SetNamespace("default")

		// Verify that the scheme can handle FlowSchema objects
		kinds, _, err := Scheme.ObjectKinds(flowSchema)
		if err != nil {
			t.Fatalf("Failed to get object kinds for FlowSchema: %v", err)
		}
		if len(kinds) == 0 {
			t.Fatalf("No kinds found for FlowSchema")
		}

		// Verify that FlowSchemaList is also registered
		flowSchemaListGVK := schema.GroupVersionKind{
			Group:   "flowcontrol.apiserver.k8s.io",
			Version: "v1beta3",
			Kind:    "FlowSchemaList",
		}
		if !Scheme.Recognizes(flowSchemaListGVK) {
			t.Errorf("FlowSchemaList v1beta3 is not registered in the scheme")
		}
	})

	t.Run("PriorityLevelConfiguration registration", func(t *testing.T) {
		// Verify that PriorityLevelConfiguration is registered for v1beta3
		priorityLevelGVK := schema.GroupVersionKind{
			Group:   "flowcontrol.apiserver.k8s.io",
			Version: "v1beta3",
			Kind:    "PriorityLevelConfiguration",
		}
		if !Scheme.Recognizes(priorityLevelGVK) {
			t.Fatalf("PriorityLevelConfiguration v1beta3 is not registered in the scheme")
		}

		// Verify that we can create instances of PriorityLevelConfiguration
		priorityLevel := &flowcontrolv1beta3.PriorityLevelConfiguration{}
		priorityLevel.SetName("test-prioritylevel")

		// Verify that the scheme can handle PriorityLevelConfiguration objects
		kinds, _, err := Scheme.ObjectKinds(priorityLevel)
		if err != nil {
			t.Fatalf("Failed to get object kinds for PriorityLevelConfiguration: %v", err)
		}
		if len(kinds) == 0 {
			t.Fatalf("No kinds found for PriorityLevelConfiguration")
		}

		// Verify that PriorityLevelConfigurationList is also registered
		priorityLevelListGVK := schema.GroupVersionKind{
			Group:   "flowcontrol.apiserver.k8s.io",
			Version: "v1beta3",
			Kind:    "PriorityLevelConfigurationList",
		}
		if !Scheme.Recognizes(priorityLevelListGVK) {
			t.Errorf("PriorityLevelConfigurationList v1beta3 is not registered in the scheme")
		}
	})

	t.Run("Encode/Decode FlowSchema", func(t *testing.T) {
		flowSchemaGVK := schema.GroupVersionKind{
			Group:   "flowcontrol.apiserver.k8s.io",
			Version: "v1beta3",
			Kind:    "FlowSchema",
		}

		// Create a FlowSchema with some data
		flowSchema := &flowcontrolv1beta3.FlowSchema{}
		flowSchema.SetName("test-flowschema")
		flowSchema.SetNamespace("default")

		// Verify that the scheme can encode FlowSchema
		encoder := Codecs.LegacyCodec(flowSchemaGVK.GroupVersion())
		flowSchemaData, err := runtime.Encode(encoder, flowSchema)
		if err != nil {
			t.Fatalf("Failed to encode FlowSchema: %v", err)
		}
		if len(flowSchemaData) == 0 {
			t.Fatalf("Encoded FlowSchema data is empty")
		}

		// Verify that the scheme can decode FlowSchema
		decoder := Codecs.UniversalDeserializer()
		decodedObj, gvk, err := decoder.Decode(flowSchemaData, &flowSchemaGVK, nil)
		if err != nil {
			t.Fatalf("Failed to decode FlowSchema: %v", err)
		}
		if decodedObj == nil {
			t.Fatalf("Decoded FlowSchema is nil")
		}
		if gvk.Group != flowSchemaGVK.Group || gvk.Version != flowSchemaGVK.Version || gvk.Kind != flowSchemaGVK.Kind {
			t.Errorf("Decoded GVK mismatch: got %v, expected %v", gvk, flowSchemaGVK)
		}

		// Verify the decoded object is of the correct type
		if _, ok := decodedObj.(*flowcontrolv1beta3.FlowSchema); !ok {
			t.Errorf("Decoded object is not *FlowSchema, got %T", decodedObj)
		}
	})

	t.Run("Encode/Decode PriorityLevelConfiguration", func(t *testing.T) {
		priorityLevelGVK := schema.GroupVersionKind{
			Group:   "flowcontrol.apiserver.k8s.io",
			Version: "v1beta3",
			Kind:    "PriorityLevelConfiguration",
		}

		// Create a PriorityLevelConfiguration with some data
		priorityLevel := &flowcontrolv1beta3.PriorityLevelConfiguration{}
		priorityLevel.SetName("test-prioritylevel")

		// Verify that the scheme can encode PriorityLevelConfiguration
		encoder := Codecs.LegacyCodec(priorityLevelGVK.GroupVersion())
		priorityLevelData, err := runtime.Encode(encoder, priorityLevel)
		if err != nil {
			t.Fatalf("Failed to encode PriorityLevelConfiguration: %v", err)
		}
		if len(priorityLevelData) == 0 {
			t.Fatalf("Encoded PriorityLevelConfiguration data is empty")
		}

		// Verify that the scheme can decode PriorityLevelConfiguration
		decoder := Codecs.UniversalDeserializer()
		decodedObj, gvk, err := decoder.Decode(priorityLevelData, &priorityLevelGVK, nil)
		if err != nil {
			t.Fatalf("Failed to decode PriorityLevelConfiguration: %v", err)
		}
		if decodedObj == nil {
			t.Fatalf("Decoded PriorityLevelConfiguration is nil")
		}
		if gvk.Group != priorityLevelGVK.Group || gvk.Version != priorityLevelGVK.Version || gvk.Kind != priorityLevelGVK.Kind {
			t.Errorf("Decoded GVK mismatch: got %v, expected %v", gvk, priorityLevelGVK)
		}

		// Verify the decoded object is of the correct type
		if _, ok := decodedObj.(*flowcontrolv1beta3.PriorityLevelConfiguration); !ok {
			t.Errorf("Decoded object is not *PriorityLevelConfiguration, got %T", decodedObj)
		}
	})

	t.Run("Scheme recognizes all flowcontrol v1beta3 types", func(t *testing.T) {
		expectedTypes := []schema.GroupVersionKind{
			{Group: "flowcontrol.apiserver.k8s.io", Version: "v1beta3", Kind: "FlowSchema"},
			{Group: "flowcontrol.apiserver.k8s.io", Version: "v1beta3", Kind: "FlowSchemaList"},
			{Group: "flowcontrol.apiserver.k8s.io", Version: "v1beta3", Kind: "PriorityLevelConfiguration"},
			{Group: "flowcontrol.apiserver.k8s.io", Version: "v1beta3", Kind: "PriorityLevelConfigurationList"},
		}

		for _, gvk := range expectedTypes {
			if !Scheme.Recognizes(gvk) {
				t.Errorf("Scheme does not recognize %v", gvk)
			}
		}
	})
}
