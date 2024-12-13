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

package exec

import (
	"bytes"
	"strings"
	"testing"

	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/cli-runtime/pkg/resource"
	kubectlexec "k8s.io/kubectl/pkg/cmd/exec"

	"github.com/karmada-io/karmada/pkg/karmadactl/options"
)

func TestValidateExecute(t *testing.T) {
	tests := []struct {
		name     string
		execOpts *CommandExecOptions
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "Validate_WithMemberOperationScopeAndWithoutMemberCluster_MemberClusterOptMustBeSpecified",
			execOpts: &CommandExecOptions{OperationScope: options.Members},
			wantErr:  true,
			errMsg:   "must specify a member cluster",
		},
		{
			name: "Validate_WithMemberOperationScopeAndWithMemberCluster_Validated",
			execOpts: &CommandExecOptions{
				OperationScope: options.Members,
				Cluster:        "test-cluster",
				KubectlExecOptions: &kubectlexec.ExecOptions{
					FilenameOptions: resource.FilenameOptions{},
					ResourceName:    "test-pod",
					Command:         []string{"date"},
					StreamOptions: kubectlexec.StreamOptions{
						IOStreams: genericiooptions.IOStreams{
							Out:    &bytes.Buffer{},
							ErrOut: &bytes.Buffer{},
						},
					},
				},
			},

			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.execOpts.Validate()
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
		})
	}
}
