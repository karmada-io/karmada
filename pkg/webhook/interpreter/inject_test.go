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
	"testing"
)

// MockDecoderInjector is a mock struct implementing the DecoderInjector interface for testing purposes.
type MockDecoderInjector struct {
	decoder *Decoder
}

// InjectDecoder implements the DecoderInjector interface by setting the decoder.
func (m *MockDecoderInjector) InjectDecoder(decoder *Decoder) {
	m.decoder = decoder
}

func TestInjectDecoder(t *testing.T) {
	tests := []struct {
		name             string
		mockInjector     interface{}
		decoder          *Decoder
		wantToBeInjected bool
	}{
		{
			name:             "InjectDecoder_ObjectImplementsDecoderInjector_Injected",
			mockInjector:     &MockDecoderInjector{},
			decoder:          &Decoder{},
			wantToBeInjected: true,
		},
		{
			name:             "InjectDecoder_ObjectNotImplementDecoderInjector_NotInjected",
			mockInjector:     struct{}{},
			decoder:          &Decoder{},
			wantToBeInjected: false,
		},
		{
			name:             "InjectDecoder_ObjectImplementsDecoderInjector_Injected",
			mockInjector:     &MockDecoderInjector{},
			wantToBeInjected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := InjectDecoderInto(test.decoder, test.mockInjector); got != test.wantToBeInjected {
				t.Errorf("expected status injection to be %t, but got %t", test.wantToBeInjected, got)
			}
			if test.wantToBeInjected && test.mockInjector.(*MockDecoderInjector).decoder != test.decoder {
				t.Errorf("failed to inject the correct decoder, expected %v but got %v", test.decoder, test.mockInjector.(*MockDecoderInjector).decoder)
			}
		})
	}
}
