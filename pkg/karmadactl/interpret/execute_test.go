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

func TestOptions_runExecute(t *testing.T) {
	tests := []struct {
		name    string
		options *Options
		want    string
		wantErr bool
	}{
		{
			name: "execute retain",
			options: &Options{
				FilenameOptions: resource.FilenameOptions{Filenames: []string{"./testdata/customization.yml"}},
				Operation:       "retain",
				DesiredFile:     "./testdata/desired.yml",
				ObservedFile:    "./testdata/observed.yml",
				Rules:           interpreter.AllResourceInterpreterCustomizationRules,
			},
			want: `---
# [1/1] retained:
apiVersion: apps/v1
kind: Deployment
metadata:
    annotations:
        cluster: cluster1
    name: nginx
spec:
    replicas: 3
    selector:
        matchLabels:
            app: nginx
    template:
        metadata:
            labels:
                app: nginx
        spec:
            containers:
                - image: nginx:alpine
                  name: nginx
status: {}
`,
		},
		{
			name: "execute interpretReplica",
			options: &Options{
				FilenameOptions: resource.FilenameOptions{Filenames: []string{"./testdata/customization.yml"}},
				Operation:       "interpretReplica",
				ObservedFile:    "./testdata/observed.yml",
				Rules:           interpreter.AllResourceInterpreterCustomizationRules,
			},
			want: `---
# [1/2] replica:
3
---
# [2/2] requires:
resourceRequest:
    cpu: 100m
`,
		},
		{
			name: "execute reviseReplica",
			options: &Options{
				FilenameOptions: resource.FilenameOptions{Filenames: []string{"./testdata/customization.yml"}},
				Operation:       "reviseReplica",
				ObservedFile:    "./testdata/observed.yml",
				DesiredReplica:  2,
				Rules:           interpreter.AllResourceInterpreterCustomizationRules,
			},
			want: `---
# [1/1] revised:
apiVersion: apps/v1
kind: Deployment
metadata:
    annotations:
        cluster: cluster1
    name: nginx
spec:
    replicas: 2
    selector:
        matchLabels:
            app: nginx
    template:
        metadata:
            labels:
                app: nginx
        spec:
            containers:
                - image: nginx:alpine
                  name: nginx
                  resources:
                    limits:
                        cpu: 100m
status:
    readyReplicas: 3
`,
		},
		{
			name: "execute interpretStatus",
			options: &Options{
				FilenameOptions: resource.FilenameOptions{Filenames: []string{"./testdata/customization.yml"}},
				Operation:       "interpretStatus",
				ObservedFile:    "./testdata/observed.yml",
				Rules:           interpreter.AllResourceInterpreterCustomizationRules,
			},
			want: `---
# [1/1] status:
readyReplicas: 3
`,
		},
		{
			name: "execute interpretHealth",
			options: &Options{
				FilenameOptions: resource.FilenameOptions{Filenames: []string{"./testdata/customization.yml"}},
				Operation:       "interpretHealth",
				ObservedFile:    "./testdata/observed.yml",
				Rules:           interpreter.AllResourceInterpreterCustomizationRules,
			},
			want: `---
# [1/1] healthy:
true
`,
		},
		{
			name: "execute interpretDependency",
			options: &Options{
				FilenameOptions: resource.FilenameOptions{Filenames: []string{"./testdata/customization.yml"}},
				Operation:       "interpretDependency",
				ObservedFile:    "./testdata/observed.yml",
				Rules:           interpreter.AllResourceInterpreterCustomizationRules,
			},
			want: `---
# [1/1] dependencies:
- apiVersion: v1
  kind: ServiceAccount
  name: nginx
`,
		},
		{
			name: "execute aggregateStatus",
			options: &Options{
				FilenameOptions: resource.FilenameOptions{Filenames: []string{"./testdata/customization.yml"}},
				Operation:       "aggregateStatus",
				ObservedFile:    "./testdata/observed.yml",
				StatusFile:      "./testdata/status.yml",
				Rules:           interpreter.AllResourceInterpreterCustomizationRules,
			},
			want: `---
# [1/1] aggregatedStatus:
apiVersion: apps/v1
kind: Deployment
metadata:
    annotations:
        cluster: cluster1
    name: nginx
spec:
    replicas: 3
    selector:
        matchLabels:
            app: nginx
    template:
        metadata:
            labels:
                app: nginx
        spec:
            containers:
                - image: nginx:alpine
                  name: nginx
                  resources:
                    limits:
                        cpu: 100m
status:
    readyReplicas: 5
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
				t.Errorf("runExecute() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("runExecute() = %q, want %q", got, tt.want)
			}
		})
	}
}
