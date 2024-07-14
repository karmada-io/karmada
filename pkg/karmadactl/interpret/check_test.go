/*
Copyright 2022 The Karmada Authors.

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

package interpret

import (
	"testing"

	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/cli-runtime/pkg/resource"

	cmdtesting "github.com/karmada-io/karmada/pkg/karmadactl/util/testing"
	"github.com/karmada-io/karmada/pkg/util/interpreter"
)

func TestOptions_runCheck(t *testing.T) {
	tests := []struct {
		name    string
		options *Options
		want    string
		wantErr bool
	}{
		{
			name: "Has errors in file",
			options: &Options{
				Rules:           interpreter.AllResourceInterpreterCustomizationRules,
				FilenameOptions: resource.FilenameOptions{Filenames: []string{"./testdata/customization_error.yml"}},
				Check:           true,
			},
			wantErr: true,
			want: `-----------------------------------
SOURCE: not-customization
not a ResourceInterpreterCustomization, got /v1, Kind=Pod
-----------------------------------
SOURCE: api-version-unset
target.apiVersion no set
-----------------------------------
SOURCE: kind-unset
target.kind no set
`,
		},
		{
			name: "customization has error",
			options: &Options{
				Rules:           interpreter.AllResourceInterpreterCustomizationRules,
				FilenameOptions: resource.FilenameOptions{Filenames: []string{"./testdata/customization_check.yml"}},
				Check:           true,
			},
			wantErr: true,
			want: `-----------------------------------
SOURCE: customization-check
TARGET: apps/v1 Deployment   
RULERS:
    Retain:                  PASS
    InterpretReplica:        ERROR: <string> line:1(column:10) near 'format':   parse error   
    ReviseReplica:           UNSET
    InterpretStatus:         UNSET
    AggregateStatus:         UNSET
    InterpretHealth:         UNSET
    InterpretDependency:     UNSET
`,
		},
		{
			name: "customization has no error",
			options: &Options{
				Rules:           interpreter.AllResourceInterpreterCustomizationRules,
				FilenameOptions: resource.FilenameOptions{Filenames: []string{"./testdata/customization.yml"}},
				Check:           true,
			},
			wantErr: false,
			want: `-----------------------------------
SOURCE: customization
TARGET: apps/v1 Deployment   
RULERS:
    Retain:                  PASS
    InterpretReplica:        PASS
    ReviseReplica:           PASS
    InterpretStatus:         PASS
    AggregateStatus:         PASS
    InterpretHealth:         PASS
    InterpretDependency:     PASS
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tf := cmdtesting.NewTestFactory()
			defer tf.Cleanup()

			streams, _, buf, _ := genericiooptions.NewTestIOStreams()
			tt.options.IOStreams = streams
			if err := tt.options.Complete(tf, nil, nil); err != nil {
				t.Fatal(err)
			}
			if err := tt.options.Validate(); err != nil {
				t.Fatal(err)
			}

			err := tt.options.Run()
			if (err != nil) != tt.wantErr {
				t.Errorf("runCheck() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("runCheck() = %q, want %q", got, tt.want)
			}
		})
	}
}
