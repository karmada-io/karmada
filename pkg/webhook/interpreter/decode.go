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

package interpreter

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/json"
)

// Decoder knows how to decode the contents of an resource interpreter
// request into a concrete object.
type Decoder struct {
	codecs serializer.CodecFactory
}

// NewDecoder creates a Decoder given the runtime.Scheme.
func NewDecoder(scheme *runtime.Scheme) *Decoder {
	return &Decoder{
		codecs: serializer.NewCodecFactory(scheme),
	}
}

// Decode decodes the inlined object in the ResourceInterpreterRequest into the passed-in runtime.Object.
// If you want to decode the ObservedObject in the ResourceInterpreterRequest, use DecodeRaw.
// It errors out if req.Object.Raw is empty i.e. containing 0 raw bytes.
func (d *Decoder) Decode(req Request, into runtime.Object) error {
	if len(req.Object.Raw) == 0 {
		return fmt.Errorf("there is no content to decode")
	}
	return d.DecodeRaw(req.Object, into)
}

// DecodeRaw decodes a RawExtension object into the passed-in runtime.Object.
// It errors out if rawObj is empty i.e. containing 0 raw bytes.
func (d *Decoder) DecodeRaw(rawObj runtime.RawExtension, into runtime.Object) error {
	if len(rawObj.Raw) == 0 {
		return fmt.Errorf("there is no content to decode")
	}
	if unstructuredInto, isUnstructured := into.(*unstructured.Unstructured); isUnstructured {
		// unmarshal into unstructured's underlying object to avoid calling the decoder
		return json.Unmarshal(rawObj.Raw, &unstructuredInto.Object)
	}

	deserializer := d.codecs.UniversalDeserializer()
	return runtime.DecodeInto(deserializer, rawObj.Raw, into)
}
