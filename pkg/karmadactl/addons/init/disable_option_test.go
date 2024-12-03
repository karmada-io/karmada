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

package init

import (
	"fmt"
	"strings"
	"testing"
)

func TestValidateDisableAddon(t *testing.T) {
	clusterName, namespace := "test-cluster", "test"
	tests := []struct {
		name         string
		disableOpts  *CommandAddonsDisableOption
		validateArgs []string
		prep         func() error
		wantErr      bool
		errMsg       string
	}{
		{
			name:         "Validate_EmptyAddonNames_AddonNamesMustBeNotNull",
			disableOpts:  &CommandAddonsDisableOption{},
			validateArgs: nil,
			prep:         func() error { return nil },
			wantErr:      true,
			errMsg:       "addonNames must be not be null",
		},
		{
			name: "Validate_WithoutMemberClusterForKarmadaSchedulerEstimatorAddon_MemberClusterIsNeeded",
			disableOpts: &CommandAddonsDisableOption{
				GlobalCommandOptions: GlobalCommandOptions{
					Namespace: namespace,
				},
			},
			validateArgs: []string{EstimatorResourceName},
			prep: func() error {
				Addons["karmada-scheduler-estimator"] = &Addon{Name: EstimatorResourceName}
				return nil
			},
			wantErr: true,
			errMsg:  "member cluster and config is needed when disable karmada-scheduler-estimator",
		},
		{
			name: "Validate_WithMemberClusterForKarmadaSchedulerEstimatorAddon_Validated",
			disableOpts: &CommandAddonsDisableOption{
				GlobalCommandOptions: GlobalCommandOptions{
					Namespace: namespace,
					Cluster:   clusterName,
				},
			},
			validateArgs: []string{EstimatorResourceName},
			prep: func() error {
				Addons["karmada-scheduler-estimator"] = &Addon{Name: EstimatorResourceName}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(); err != nil {
				t.Fatalf("failed to prep for validating disable addon, got an error: %v", err)
			}
			err := test.disableOpts.Validate(test.validateArgs)
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

func TestRunDisableAddon(t *testing.T) {
	tests := []struct {
		name         string
		disableOpts  *CommandAddonsDisableOption
		validateArgs []string
		prep         func() error
		wantErr      bool
		errMsg       string
	}{
		{
			name: "Run_DisableKarmadaDeschedulerAddon_FailedToDisableWithNetworkIssue",
			disableOpts: &CommandAddonsDisableOption{
				Force: true,
			},
			validateArgs: []string{DeschedulerResourceName},
			prep: func() error {
				Addons["karmada-descheduler"] = &Addon{
					Name: string(DeschedulerResourceName),
					Disable: func(*CommandAddonsDisableOption) error {
						return fmt.Errorf("got network issue while disabling %s addon", DeschedulerResourceName)
					},
				}
				return nil
			},
			wantErr: true,
			errMsg:  fmt.Sprintf("got network issue while disabling %s addon", DeschedulerResourceName),
		},
		{
			name: "Run_DisableKarmadaDeschedulerAddon_Disabled",
			disableOpts: &CommandAddonsDisableOption{
				Force: true,
			},
			validateArgs: []string{DeschedulerResourceName},
			prep: func() error {
				Addons["karmada-descheduler"] = &Addon{
					Name:    string(DeschedulerResourceName),
					Disable: func(*CommandAddonsDisableOption) error { return nil },
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(); err != nil {
				t.Fatalf("failed to prep before disabling addon, got an error: %v", err)
			}
			err := test.disableOpts.Run(test.validateArgs)
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
