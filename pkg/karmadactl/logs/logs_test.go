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

package logs

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	kubectllogs "k8s.io/kubectl/pkg/cmd/logs"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/karmada-io/karmada/pkg/karmadactl/util"
)

type testFactory struct {
	util.Factory
}

func (t *testFactory) FactoryForMemberCluster(string) (cmdutil.Factory, error) {
	return nil, errors.New("failed to create factory for member cluster")
}

func TestCompleteLogOptions(t *testing.T) {
	tests := []struct {
		name     string
		cobraCmd *cobra.Command
		args     []string
		f        util.Factory
		logsOpts *CommandLogsOptions
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "CompleteLogOptions_WithoutCluster_ClusterMustBeSpecified",
			logsOpts: &CommandLogsOptions{Cluster: ""},
			wantErr:  true,
			errMsg:   "must specify a cluster",
		},
		{
			name:     "CompleteLogOptions_WithoutArgumentsAndSelector_GotLogsUsage",
			cobraCmd: &cobra.Command{},
			logsOpts: &CommandLogsOptions{
				Cluster: "test-cluster",
				KubectlLogsOptions: &kubectllogs.LogsOptions{
					Selector: "",
				},
			},
			args:    make([]string, 0),
			wantErr: true,
			errMsg:  logsUsageErrStr,
		},
		{
			name:     "CompleteLogOptions_WithArgumentsAndSelector_GotLogsUsage",
			cobraCmd: &cobra.Command{},
			logsOpts: &CommandLogsOptions{
				Cluster: "test-cluster",
				KubectlLogsOptions: &kubectllogs.LogsOptions{
					Selector: "env=test",
				},
			},
			args:    []string{"nginx"},
			wantErr: true,
			errMsg:  "only a selector (-l) or a POD name is allowed",
		},
		{
			name:     "CompleteLogOptions_WithTwoOrMoreArgs_GotLogsUsage",
			cobraCmd: &cobra.Command{},
			logsOpts: &CommandLogsOptions{
				Cluster: "test-cluster",
				KubectlLogsOptions: &kubectllogs.LogsOptions{
					Selector: "env=test",
				},
			},
			args:    []string{"nginx-container", "nginx-container-2", "nginx-pod"},
			wantErr: true,
			errMsg:  logsUsageErrStr,
		},
		{
			name:     "CompleteLogOptions_ReturnMemberClusterFactory_FailedToReturnMemberClusterFactory",
			cobraCmd: &cobra.Command{},
			f:        &testFactory{},
			logsOpts: &CommandLogsOptions{
				Cluster: "test-cluster",
				KubectlLogsOptions: &kubectllogs.LogsOptions{
					Selector: "env=test",
				},
			},
			args:    []string{"nginx-container", "nginx-pod"},
			wantErr: true,
			errMsg:  "failed to create factory for member cluster",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.logsOpts.Complete(test.cobraCmd, test.args, test.f)
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
