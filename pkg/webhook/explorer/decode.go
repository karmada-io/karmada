package explorer

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/json"
)

// Decoder knows how to decode the contents of an explorer
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

// Decode decodes the inlined object in the ExploreRequest into the passed-in runtime.Object.
// If you want to decode the ObservedObject in the ExploreRequest, use DecodeRaw.
// It errors out if req.Object.Raw is empty i.e. containing 0 raw bytes.
func (d *Decoder) Decode(req Request, into runtime.Object) error {
	if len(req.Object.Raw) == 0 {
		return fmt.Errorf("there is no context to decode")
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
