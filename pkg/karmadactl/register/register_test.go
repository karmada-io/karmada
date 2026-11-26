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

package register

import (
	"slices"
	"testing"
)

func TestMakeKarmadaAgentDeployment_ExtraArgs(t *testing.T) {
	o := &CommandRegisterOption{KarmadaAgentExtraArgs: []string{"--report-secrets=None", "--v=4"}}
	command := o.makeKarmadaAgentDeployment().Spec.Template.Spec.Containers[0].Command

	if command[0] != "/bin/karmada-agent" {
		t.Errorf("command[0] = %q, want /bin/karmada-agent", command[0])
	}
	for _, want := range []string{"--report-secrets=None", "--v=4"} {
		if !slices.Contains(command, want) {
			t.Errorf("karmada-agent command %v does not contain %q", command, want)
		}
	}
	if slices.Contains(command, "--v=2") {
		t.Errorf("karmada-agent command %v still contains the overridden default --v=2", command)
	}
}

func TestValidate_KarmadaAgentExtraArgs(t *testing.T) {
	newOpts := func(extraArgs []string) *CommandRegisterOption {
		return &CommandRegisterOption{
			ClusterName:           "member1",
			BootstrapToken:        &BootstrapTokenDiscovery{Token: "abcdef.0123456789abcdef", UnsafeSkipCAVerification: true},
			KarmadaAgentExtraArgs: extraArgs,
		}
	}
	if err := newOpts([]string{"--report-secrets=None"}).Validate(); err != nil {
		t.Errorf("Validate() returned an error for valid extra args: %v", err)
	}
	if err := newOpts([]string{"report-secrets=None"}).Validate(); err == nil {
		t.Error("Validate() returned no error for an extra arg without the -- prefix")
	}
}
