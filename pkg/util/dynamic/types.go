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
	stdjson "encoding/json"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
)

// RawObject is a Kubernetes object that keeps the original JSON object bytes in raw.
//
// The ObjectMeta copy is intentionally kept beside raw so client-go cache key and
// index functions can use the normal metav1.Object accessors without decoding the
// full JSON object into map[string]interface{}.
type RawObject struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// raw stores the original JSON object bytes.
	// It preserves the full payload while RawObject decodes only type and object metadata.
	raw []byte
}

var _ metav1.Type = &RawObject{}
var _ metav1.Object = &RawObject{}
var _ runtime.Object = &RawObject{}

// NewRawObject decodes object metadata from raw JSON and stores a copy of raw.
func NewRawObject(raw []byte) (*RawObject, error) {
	obj := &RawObject{}
	if err := json.Unmarshal(raw, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

type partialObjectMetadata struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

// UnmarshalJSON keeps a copy of the original object JSON and decodes only type
// and object metadata into structured fields.
func (o *RawObject) UnmarshalJSON(raw []byte) error {
	var decoded partialObjectMetadata
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return err
	}

	o.TypeMeta = decoded.TypeMeta
	o.ObjectMeta = decoded.ObjectMeta
	o.raw = append(o.raw[:0], raw...)
	return nil
}

// MarshalJSON writes raw overlaid with the current type and object metadata.
func (o *RawObject) MarshalJSON() ([]byte, error) {
	if len(o.raw) == 0 {
		return json.Marshal(partialObjectMetadata{TypeMeta: o.TypeMeta, ObjectMeta: o.ObjectMeta})
	}

	fields := map[string]stdjson.RawMessage{}
	if err := stdjson.Unmarshal(o.raw, &fields); err != nil {
		return nil, err
	}

	if o.APIVersion != "" {
		apiVersionRaw, err := json.Marshal(o.APIVersion)
		if err != nil {
			return nil, err
		}
		fields["apiVersion"] = apiVersionRaw
	}
	if o.Kind != "" {
		kindRaw, err := json.Marshal(o.Kind)
		if err != nil {
			return nil, err
		}
		fields["kind"] = kindRaw
	}

	metadataRaw, err := json.Marshal(o.ObjectMeta)
	if err != nil {
		return nil, err
	}
	fields["metadata"] = metadataRaw

	return json.Marshal(fields)
}

// GetAPIVersion returns the API version of the raw object.
func (o *RawObject) GetAPIVersion() string {
	return o.APIVersion
}

// SetAPIVersion sets the API version of the raw object.
func (o *RawObject) SetAPIVersion(version string) {
	o.APIVersion = version
}

// GetKind returns the kind of the raw object.
func (o *RawObject) GetKind() string {
	return o.Kind
}

// SetKind sets the kind of the raw object.
func (o *RawObject) SetKind(kind string) {
	o.Kind = kind
}

// DeepCopy returns a deep copy of RawObject.
func (o *RawObject) DeepCopy() *RawObject {
	if o == nil {
		return nil
	}

	out := new(RawObject)
	out.TypeMeta = o.TypeMeta
	out.ObjectMeta = *o.ObjectMeta.DeepCopy()
	out.raw = append([]byte(nil), o.raw...)
	return out
}

// DeepCopyObject implements runtime.Object.
func (o *RawObject) DeepCopyObject() runtime.Object {
	return o.DeepCopy()
}

// ToUnstructured converts RawObject to an unstructured object.
func (o *RawObject) ToUnstructured() (*unstructured.Unstructured, error) {
	u := &unstructured.Unstructured{}
	if len(o.raw) > 0 {
		if err := json.Unmarshal(o.raw, &u.Object); err != nil {
			return nil, err
		}
	}
	if o.APIVersion != "" && o.Kind != "" {
		u.SetGroupVersionKind(schema.FromAPIVersionAndKind(o.APIVersion, o.Kind))
	}
	applyObjectMeta(u, o)
	return u, nil
}

// ConvertTo converts RawObject to the provided object.
func (o *RawObject) ConvertTo(out any) error {
	if out == nil {
		return fmt.Errorf("cannot convert RawObject to nil")
	}
	if len(o.raw) > 0 {
		if err := json.Unmarshal(o.raw, &out); err != nil {
			return err
		}
	}

	if r, ok := out.(runtime.Object); ok && o.APIVersion != "" && o.Kind != "" {
		r.GetObjectKind().SetGroupVersionKind(schema.FromAPIVersionAndKind(o.APIVersion, o.Kind))
	}
	if obj, ok := out.(metav1.Object); ok {
		applyObjectMeta(obj, o)
	}
	return nil
}

// nolint:gocyclo
func applyObjectMeta(out metav1.Object, o *RawObject) {
	if o.Namespace != "" {
		out.SetNamespace(o.Namespace)
	}
	if o.Name != "" {
		out.SetName(o.Name)
	}
	if o.GenerateName != "" {
		out.SetGenerateName(o.GenerateName)
	}
	if o.UID != "" {
		out.SetUID(o.UID)
	}
	if o.ResourceVersion != "" {
		out.SetResourceVersion(o.ResourceVersion)
	}
	if o.Generation != 0 {
		out.SetGeneration(o.Generation)
	}
	if o.Labels != nil {
		out.SetLabels(o.Labels)
	}
	if o.Annotations != nil {
		out.SetAnnotations(o.Annotations)
	}
	if o.OwnerReferences != nil {
		out.SetOwnerReferences(o.OwnerReferences)
	}
	if o.Finalizers != nil {
		out.SetFinalizers(o.Finalizers)
	}
	if o.ManagedFields != nil {
		out.SetManagedFields(o.ManagedFields)
	}
	if !o.CreationTimestamp.IsZero() {
		out.SetCreationTimestamp(o.CreationTimestamp)
	}
	if o.DeletionTimestamp != nil {
		out.SetDeletionTimestamp(o.DeletionTimestamp)
	}
	if o.DeletionGracePeriodSeconds != nil {
		seconds := *o.DeletionGracePeriodSeconds
		out.SetDeletionGracePeriodSeconds(&seconds)
	}
}

// RawObjectList is the list counterpart used by Reflector list extraction.
type RawObjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []RawObject `json:"items"`
}

var _ runtime.Object = &RawObjectList{}

// NewRawObjectList decodes a JSON list and preserves each item as raw JSON.
func NewRawObjectList(raw []byte) (*RawObjectList, error) {
	list := &RawObjectList{}
	if err := json.Unmarshal(raw, list); err != nil {
		return nil, err
	}
	return list, nil
}

// UnmarshalJSON decodes list metadata and items, preserving the original JSON
// for each item. When list items omit apiVersion/kind, their TypeMeta fields are
// inferred from the list TypeMeta, matching unstructured list decoding behavior.
func (l *RawObjectList) UnmarshalJSON(raw []byte) error {
	type rawObjectList RawObjectList

	var decoded rawObjectList
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return err
	}

	*l = RawObjectList(decoded)
	itemKind := strings.TrimSuffix(l.Kind, "List")
	for i := range l.Items {
		if l.Items[i].APIVersion == "" && l.Items[i].Kind == "" {
			l.Items[i].APIVersion = l.APIVersion
			l.Items[i].Kind = itemKind
		}
	}
	return nil
}

// DeepCopy returns a deep copy of RawObjectList.
func (l *RawObjectList) DeepCopy() *RawObjectList {
	if l == nil {
		return nil
	}

	out := new(RawObjectList)
	out.TypeMeta = l.TypeMeta
	out.ListMeta = l.ListMeta
	if l.RemainingItemCount != nil {
		out.RemainingItemCount = new(int64)
		*out.RemainingItemCount = *l.RemainingItemCount
	}
	if l.ShardInfo != nil {
		out.ShardInfo = l.ShardInfo.DeepCopy()
	}
	if l.Items != nil {
		out.Items = make([]RawObject, len(l.Items))
		for i := range l.Items {
			out.Items[i] = *l.Items[i].DeepCopy()
		}
	}
	return out
}

// DeepCopyObject implements runtime.Object.
func (l *RawObjectList) DeepCopyObject() runtime.Object {
	return l.DeepCopy()
}
