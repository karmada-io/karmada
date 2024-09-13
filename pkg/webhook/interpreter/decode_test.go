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
	"fmt"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	schema "k8s.io/apimachinery/pkg/runtime/schema"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

type Interface interface {
	GetAPIVersion() string
	GetKind() string
	GetName() string
}

// MyTestPod represents a simplified version of a Kubernetes Pod for testing purposes.
// It includes basic fields such as API version, kind, and metadata.
type MyTestPod struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name string `json:"name"`
	} `json:"metadata"`
}

// DeepCopyObject creates a deep copy of the MyTestPod instance.
// This method is part of the runtime.Object interface and ensures that modifications
// to the copy do not affect the original object.
func (p *MyTestPod) DeepCopyObject() runtime.Object {
	return &MyTestPod{
		APIVersion: p.APIVersion,
		Kind:       p.Kind,
		Metadata:   p.Metadata,
	}
}

// GetObjectKind returns the schema.ObjectKind for the MyTestPod instance.
// This method is part of the runtime.Object interface and provides the API version
// and kind of the object, which is used for object identification in Kubernetes.
func (p *MyTestPod) GetObjectKind() schema.ObjectKind {
	return &metav1.TypeMeta{
		APIVersion: p.APIVersion,
		Kind:       p.Kind,
	}
}

// GetAPIVersion returns the API version of the MyTestPod.
func (p *MyTestPod) GetAPIVersion() string {
	return p.APIVersion
}

// GetKind returns the kind of the MyTestPod.
func (p *MyTestPod) GetKind() string {
	return p.Kind
}

// GetName returns the name of the MyTestPod.
func (p *MyTestPod) GetName() string {
	return p.Metadata.Name
}

func TestNewDecoder(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "NewDecoder_ValidDecoder_DecoderIsValid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			decoder := NewDecoder(scheme)
			if decoder == nil {
				t.Errorf("expected decoder to not be nil")
			}
		})
	}
}

func TestDecodeRaw(t *testing.T) {
	tests := []struct {
		name       string
		apiVersion string
		kind       string
		objName    string
		rawObj     *runtime.RawExtension
		into       Interface
		prep       func(re *runtime.RawExtension, apiVersion, kind, name string) error
		verify     func(into Interface, apiVersion, kind, name string) error
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "DecodeRaw_ValidRaw_DecodeRawIsSuccessful",
			objName:    "test-pod",
			kind:       "Pod",
			apiVersion: "v1",
			rawObj: &runtime.RawExtension{
				Raw: []byte{},
			},
			into: &unstructured.Unstructured{},
			prep: func(re *runtime.RawExtension, apiVersion, kind, name string) error {
				re.Raw = []byte(fmt.Sprintf(`{"apiVersion": "%s", "kind": "%s", "metadata": {"name": "%s"}}`, apiVersion, kind, name))
				return nil
			},
			verify:  verifyRuntimeObject,
			wantErr: false,
		},
		{
			name:       "DecodeRaw_IntoNonUnstructuredType_RawDecoded",
			objName:    "test-pod",
			kind:       "Pod",
			apiVersion: "v1",
			rawObj: &runtime.RawExtension{
				Raw: []byte{},
			},
			into: &MyTestPod{},
			prep: func(re *runtime.RawExtension, apiVersion, kind, name string) error {
				re.Raw = []byte(fmt.Sprintf(`{"apiVersion": "%s", "kind": "%s", "metadata": {"name": "%s"}}`, apiVersion, kind, name))
				return nil
			},
			verify:  verifyRuntimeObject,
			wantErr: false,
		},
		{
			name: "DecodeRaw_EmptyRaw_NoContentToDecode",
			rawObj: &runtime.RawExtension{
				Raw: []byte{},
			},
			into:    &unstructured.Unstructured{},
			prep:    func(*runtime.RawExtension, string, string, string) error { return nil },
			verify:  func(Interface, string, string, string) error { return nil },
			wantErr: true,
			errMsg:  "there is no content to decode",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.rawObj, test.apiVersion, test.kind, test.objName); err != nil {
				t.Errorf("failed to prep the runtime raw extension object: %v", err)
			}
			scheme := runtime.NewScheme()
			decoder := NewDecoder(scheme)
			intoObj, ok := test.into.(runtime.Object)
			if !ok {
				t.Errorf("failed to type assert into object into runtime rawextension")
			}
			err := decoder.DecodeRaw(*test.rawObj, intoObj)
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error while decoding the raw: %v", err)
			}
			if err == nil && test.wantErr {
				t.Errorf("expected an error, but got none")
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected %s error msg to contain %s", err.Error(), test.errMsg)
			}
			if err := test.verify(test.into, test.apiVersion, test.kind, test.objName); err != nil {
				t.Errorf("failed to verify decoding the raw: %v", err)
			}
		})
	}
}

func TestDecode(t *testing.T) {
	tests := []struct {
		name       string
		apiVersion string
		kind       string
		objName    string
		req        *Request
		into       Interface
		prep       func(re *Request, apiVersion, kind, name string) error
		verify     func(into Interface, apiVersion, kind, name string) error
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "Decode_ValidRequest_DecodeRequestIsSuccessful",
			objName:    "test-pod",
			kind:       "Pod",
			apiVersion: "v1",
			req: &Request{
				ResourceInterpreterRequest: configv1alpha1.ResourceInterpreterRequest{
					Object: runtime.RawExtension{},
				},
			},
			into: &unstructured.Unstructured{},
			prep: func(re *Request, apiVersion, kind, name string) error {
				re.ResourceInterpreterRequest.Object.Raw = []byte(fmt.Sprintf(`{"apiVersion": "%s", "kind": "%s", "metadata": {"name": "%s"}}`, apiVersion, kind, name))
				return nil
			},
			verify:  verifyRuntimeObject,
			wantErr: false,
		},
		{
			name: "Decode_EmptyRaw_NoContentToDecode",
			req: &Request{
				ResourceInterpreterRequest: configv1alpha1.ResourceInterpreterRequest{
					Object: runtime.RawExtension{},
				},
			},
			into:    &unstructured.Unstructured{},
			prep:    func(*Request, string, string, string) error { return nil },
			verify:  func(Interface, string, string, string) error { return nil },
			wantErr: true,
			errMsg:  "there is no content to decode",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.req, test.apiVersion, test.kind, test.objName); err != nil {
				t.Errorf("failed to prep the runtime raw extension object: %v", err)
			}
			scheme := runtime.NewScheme()
			decoder := NewDecoder(scheme)
			if decoder == nil {
				t.Errorf("expected decoder to not be nil")
			}
			intoObj, ok := test.into.(runtime.Object)
			if !ok {
				t.Errorf("failed to type assert into object into runtime rawextension")
			}
			err := decoder.Decode(*test.req, intoObj)
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error while decoding the raw: %v", err)
			}
			if err == nil && test.wantErr {
				t.Errorf("expected an error, but got none")
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected %s error msg to contain %s", err.Error(), test.errMsg)
			}
			if err := test.verify(test.into, test.apiVersion, test.kind, test.objName); err != nil {
				t.Errorf("failed to verify decoding the raw: %v", err)
			}
		})
	}
}

// verifyRuntimeObject checks if the runtime object (`into`) matches the given
// `apiVersion`, `kind`, and `name`. It returns an error if any field doesn't match.
func verifyRuntimeObject(into Interface, apiVersion, kind, name string) error {
	if got := into.GetAPIVersion(); got != apiVersion {
		return fmt.Errorf("expected API version '%s', got '%s'", apiVersion, got)
	}
	if got := into.GetKind(); got != kind {
		return fmt.Errorf("expected kind '%s', got '%s'", kind, got)
	}
	if got := into.GetName(); got != name {
		return fmt.Errorf("expected name '%s', got '%s'", name, got)
	}
	return nil
}
