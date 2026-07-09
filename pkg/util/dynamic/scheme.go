/*
Copyright 2026 The Karmada Authors.

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

package dynamic

import (
	"encoding/json"
	"io"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	jsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
)

var basicScheme = runtime.NewScheme()
var parameterScheme = runtime.NewScheme()
var dynamicParameterCodec = runtime.NewParameterCodec(parameterScheme)

var versionV1 = schema.GroupVersion{Version: "v1"}

func init() {
	metav1.AddToGroupVersion(basicScheme, versionV1)
	metav1.AddToGroupVersion(parameterScheme, versionV1)
}

func newBasicNegotiatedSerializer() basicNegotiatedSerializer {
	fallback := jsonserializer.NewSerializerWithOptions(
		jsonserializer.DefaultMetaFactory,
		basicScheme,
		basicScheme,
		jsonserializer.SerializerOptions{},
	)
	supportedMediaTypes := []runtime.SerializerInfo{
		{
			MediaType:        "application/json",
			MediaTypeType:    "application",
			MediaTypeSubType: "json",
			EncodesAsText:    true,
			Serializer:       &rawJSONSerializer{fallback: fallback},
			StreamSerializer: &runtime.StreamSerializerInfo{
				EncodesAsText: true,
				Serializer:    &rawJSONSerializer{fallback: fallback},
				Framer:        jsonserializer.Framer,
			},
		},
	}
	return basicNegotiatedSerializer{supportedMediaTypes: supportedMediaTypes}
}

type basicNegotiatedSerializer struct {
	supportedMediaTypes []runtime.SerializerInfo
}

func (s basicNegotiatedSerializer) SupportedMediaTypes() []runtime.SerializerInfo {
	return s.supportedMediaTypes
}

func (s basicNegotiatedSerializer) EncoderForVersion(encoder runtime.Encoder, gv runtime.GroupVersioner) runtime.Encoder {
	return runtime.WithVersionEncoder{
		Version:     gv,
		Encoder:     encoder,
		ObjectTyper: permissiveTyper{basicScheme},
	}
}

func (s basicNegotiatedSerializer) DecoderToVersion(decoder runtime.Decoder, _ runtime.GroupVersioner) runtime.Decoder {
	return decoder
}

// The dynamic client has historically accepted objects with missing or empty
// apiVersion and kind as arguments to write request methods.
type permissiveTyper struct {
	nested runtime.ObjectTyper
}

func (t permissiveTyper) ObjectKinds(obj runtime.Object) ([]schema.GroupVersionKind, bool, error) {
	kinds, unversioned, err := t.nested.ObjectKinds(obj)
	if err == nil {
		return kinds, unversioned, nil
	}
	switch typed := obj.(type) {
	case *RawObject:
		return []schema.GroupVersionKind{typed.GroupVersionKind()}, false, nil
	case *RawObjectList:
		return []schema.GroupVersionKind{typed.GroupVersionKind()}, false, nil
	default:
		return nil, false, err
	}
}

func (t permissiveTyper) Recognizes(schema.GroupVersionKind) bool {
	return true
}

type rawJSONSerializer struct {
	fallback runtime.Serializer
}

var _ runtime.Serializer = &rawJSONSerializer{}

func (s *rawJSONSerializer) Encode(obj runtime.Object, w io.Writer) error {
	return s.fallback.Encode(obj, w)
}

func (s *rawJSONSerializer) Decode(data []byte, defaults *schema.GroupVersionKind, into runtime.Object) (runtime.Object, *schema.GroupVersionKind, error) {
	switch target := into.(type) {
	case *RawObject:
		if err := json.Unmarshal(data, target); err != nil {
			return nil, nil, err
		}
		gvk := target.GroupVersionKind()
		return target, &gvk, nil
	case *RawObjectList:
		if err := json.Unmarshal(data, target); err != nil {
			return nil, nil, err
		}
		gvk := target.GroupVersionKind()
		return target, &gvk, nil
	}

	if into != nil {
		return s.fallback.Decode(data, defaults, into)
	}

	var tm metav1.TypeMeta
	if err := json.Unmarshal(data, &tm); err != nil {
		return nil, nil, err
	}
	if tm.Kind == "Status" && tm.APIVersion == versionV1.String() {
		status := &metav1.Status{}
		out, gvk, err := s.fallback.Decode(data, defaults, status)
		if err != nil {
			return nil, nil, err
		}
		return out, gvk, nil
	}

	if isJSONList(data) {
		list := &RawObjectList{}
		if err := json.Unmarshal(data, list); err != nil {
			return nil, nil, err
		}
		gvk := list.GroupVersionKind()
		return list, &gvk, nil
	}

	obj := &RawObject{}
	if err := json.Unmarshal(data, obj); err != nil {
		return nil, nil, err
	}
	gvk := obj.GroupVersionKind()
	return obj, &gvk, nil
}

// Identifier implements runtime.EncoderWithAllocator compatibility for callers
// that inspect serializer identity.
func (s *rawJSONSerializer) Identifier() runtime.Identifier {
	return runtime.Identifier("raw-json")
}

func isJSONList(data []byte) bool {
	type detector struct {
		metav1.TypeMeta `json:",inline"`
		Metadata        struct {
			Name         string `json:"name,omitempty"`
			Namespace    string `json:"namespace,omitempty"`
			GenerateName string `json:"generateName,omitempty"`
			UID          string `json:"uid,omitempty"`
		} `json:"metadata,omitempty"`
		Items json.RawMessage `json:"items"`
	}

	var det detector
	if err := json.Unmarshal(data, &det); err != nil {
		return false
	}
	if len(det.Items) == 0 {
		return false
	}
	var items []json.RawMessage
	if err := json.Unmarshal(det.Items, &items); err != nil {
		return false
	}
	if det.Metadata.Name != "" || det.Metadata.Namespace != "" || det.Metadata.GenerateName != "" || det.Metadata.UID != "" {
		return false
	}
	return det.Kind != "" && (det.Kind == "List" || strings.HasSuffix(det.Kind, "List"))
}
