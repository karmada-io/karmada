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

// DecoderInjector is used by the ControllerManager to inject decoder into webhook handlers.
type DecoderInjector interface {
	InjectDecoder(*Decoder)
}

// InjectDecoderInto will set decoder on i and return the result if it implements Decoder.
// Returns false if i does not implement Decoder.
func InjectDecoderInto(decoder *Decoder, i interface{}) bool {
	if s, ok := i.(DecoderInjector); ok {
		s.InjectDecoder(decoder)
		return true
	}
	return false
}
