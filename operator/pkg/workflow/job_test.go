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

package workflow

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

type TestRunData struct {
	name string
}

func TestInitData(t *testing.T) {
	tests := []struct {
		name        string
		job         *Job
		wantRunData RunData
		wantErr     bool
		errMsg      string
	}{
		{
			name: "InitData_NoRunDataIsInitialized_InitializeRunDataWithError",
			job: &Job{
				runDataInitializer: func() (RunData, error) {
					return nil, fmt.Errorf("failed to initialize run data")
				},
				runData: nil,
			},
			wantRunData: nil,
			wantErr:     true,
			errMsg:      "failed to initialize run data",
		},
		{
			name: "InitData_NoRunDataIsInitialized_InitializeRunData",
			job: &Job{
				runDataInitializer: func() (RunData, error) {
					return &TestRunData{name: "test"}, nil
				},
				runData: nil,
			},
			wantRunData: &TestRunData{name: "test"},
			wantErr:     false,
		},
		{
			name: "InitData_RunDataIsAlreadyInitialized_InitializeRunData",
			job: &Job{
				runDataInitializer: func() (RunData, error) {
					return &TestRunData{name: "test"}, nil
				},
				runData: &TestRunData{name: "already-exist"},
			},
			wantRunData: &TestRunData{name: "already-exist"},
			wantErr:     false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runData, err := test.job.initData()
			if err == nil && test.wantErr {
				t.Errorf("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error msg %s to be part of %s", test.errMsg, err.Error())
			}
			if !reflect.DeepEqual(runData, test.wantRunData) {
				t.Errorf("unmatched run data equality, expected %v but got %v", test.wantRunData, runData)
			}
		})
	}
}

func TestRun(t *testing.T) {
	tests := []struct {
		name        string
		job         *Job
		wantRunData RunData
		wantErr     bool
		errMsg      string
	}{
		{
			name: "Run_NilRunData_RunDataIsInitialized",
			job: &Job{
				runDataInitializer: func() (RunData, error) {
					return nil, fmt.Errorf("failed to initialize run data")
				},
				runData: nil,
			},
			wantErr: true,
			errMsg:  "failed to initialize run data",
		},
		{
			name: "Run_Task_TaskRunSuccessfully",
			job: &Job{
				runDataInitializer: func() (RunData, error) {
					return &TestRunData{name: "test"}, nil
				},
				runData: nil,
				Tasks: []Task{
					{
						Name: "SkipRunningTask",
						Skip: func(RunData) (bool, error) {
							return true, nil
						},
					},
					{
						Name: "RunSubTask",
						Run: func(RunData) error {
							return nil
						},
						Tasks: []Task{
							{
								Name: "RunSubTask_2",
								Run: func(RunData) error {
									return nil
								},
							},
						},
						RunSubTasks: true,
					},
				},
			},
			wantRunData: &TestRunData{name: "test"},
			wantErr:     false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.job.Run()
			if err == nil && test.wantErr {
				t.Errorf("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error msg %s to be part of %s", test.errMsg, err.Error())
			}
			if !reflect.DeepEqual(test.job.runData, test.wantRunData) {
				t.Errorf("unmatched run data equality, expected %v but got %v", test.wantRunData, test.job.runData)
			}
		})
	}
}
