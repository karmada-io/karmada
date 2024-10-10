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

package util

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		args     interface{}
		want     []byte
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "ParseTemplate_WithMissingVariable_ParsingFailed",
			template: "Hello, {{.Name}}!",
			args:     struct{ Missing string }{Missing: "World"},
			want:     nil,
			wantErr:  true,
			errMsg:   "error when executing template",
		},
		{
			name:     "ParseTemplate_WithInvalidTemplateSyntax_ParsingFailed",
			template: "Hello, {{.Name!",
			args:     struct{ Name string }{Name: "World"},
			want:     nil,
			wantErr:  true,
			errMsg:   "error when parsing template",
		},
		{
			name:     "ParseTemplate_ValidTemplateWithVariable_ParsingSucceeded",
			template: "Hello, {{.Name}}!",
			args:     struct{ Name string }{Name: "World"},
			want:     []byte("Hello, World!"),
			wantErr:  false,
		},
		{
			name:     "ParseTemplate_EmptyTemplate_ParsingSucceeded",
			template: "",
			args:     nil,
			want:     []byte(""),
			wantErr:  false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseTemplate(test.template, test.args)
			if err == nil && test.wantErr {
				t.Errorf("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if !bytes.Equal(got, test.want) {
				t.Errorf("expected parsed template bytes to be %v, but got %v", test.want, got)
			}
		})
	}
}
