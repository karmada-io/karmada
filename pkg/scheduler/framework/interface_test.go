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

package framework

import (
	"errors"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPluginToResult_Merge(t *testing.T) {
	tests := []struct {
		name    string
		results PluginToResult
		want    *Result
	}{
		{
			name:    "empty results",
			results: PluginToResult{},
			want:    nil,
		},
		{
			name: "all success results",
			results: PluginToResult{
				"plugin1": NewResult(Success),
				"plugin2": NewResult(Success),
			},
			want: NewResult(Success),
		},
		{
			name: "mixed results with unschedulable",
			results: PluginToResult{
				"plugin1": NewResult(Success),
				"plugin2": NewResult(Unschedulable, "reason1"),
				"plugin3": NewResult(Success),
			},
			want: NewResult(Unschedulable, "reason1"),
		},
		{
			name: "mixed results with error",
			results: PluginToResult{
				"plugin1": NewResult(Success),
				"plugin2": NewResult(Unschedulable, "reason1"),
				"plugin3": NewResult(Error, "error occurred"),
			},
			want: NewResult(Error, "reason1", "error occurred"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.results.Merge()
			if tt.want == nil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.Equal(t, tt.want.code, got.code)

				// Sort the reasons before comparing
				sortedWantReasons := make([]string, len(tt.want.reasons))
				copy(sortedWantReasons, tt.want.reasons)
				sort.Strings(sortedWantReasons)

				sortedGotReasons := make([]string, len(got.reasons))
				copy(sortedGotReasons, got.reasons)
				sort.Strings(sortedGotReasons)

				assert.Equal(t, sortedWantReasons, sortedGotReasons)

				if tt.want.err != nil {
					assert.Error(t, got.err)
				} else {
					assert.NoError(t, got.err)
				}
			}
		})
	}
}

func TestResult_IsSuccess(t *testing.T) {
	tests := []struct {
		name   string
		result *Result
		want   bool
	}{
		{
			name:   "nil result",
			result: nil,
			want:   true,
		},
		{
			name:   "success result",
			result: NewResult(Success),
			want:   true,
		},
		{
			name:   "unschedulable result",
			result: NewResult(Unschedulable),
			want:   false,
		},
		{
			name:   "error result",
			result: NewResult(Error),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.result.IsSuccess())
		})
	}
}

func TestResult_AsError(t *testing.T) {
	tests := []struct {
		name     string
		result   *Result
		wantErr  bool
		errorMsg string
	}{
		{
			name:    "success result",
			result:  NewResult(Success),
			wantErr: false,
		},
		{
			name:     "unschedulable result",
			result:   NewResult(Unschedulable, "reason1", "reason2"),
			wantErr:  true,
			errorMsg: "reason1, reason2",
		},
		{
			name:     "error result",
			result:   NewResult(Error, "error occurred"),
			wantErr:  true,
			errorMsg: "error occurred",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.result.AsError()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tt.errorMsg, err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestResult_AsResult(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantCode    Code
		wantReasons []string
	}{
		{
			name:        "non-nil error",
			err:         errors.New("test error"),
			wantCode:    Error,
			wantReasons: []string{"test error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AsResult(tt.err)
			assert.Equal(t, tt.wantCode, got.code)
			assert.Equal(t, tt.wantReasons, got.reasons)
			assert.Equal(t, tt.err, got.err)
		})
	}
}

func TestResult_Code(t *testing.T) {
	tests := []struct {
		name   string
		result *Result
		want   Code
	}{
		{
			name:   "nil result",
			result: nil,
			want:   Success,
		},
		{
			name:   "success result",
			result: NewResult(Success),
			want:   Success,
		},
		{
			name:   "unschedulable result",
			result: NewResult(Unschedulable),
			want:   Unschedulable,
		},
		{
			name:   "error result",
			result: NewResult(Error),
			want:   Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result.Code()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCode_String(t *testing.T) {
	tests := []struct {
		name string
		code Code
		want string
	}{
		{
			name: "Success code",
			code: Success,
			want: "Success",
		},
		{
			name: "Unschedulable code",
			code: Unschedulable,
			want: "Unschedulable",
		},
		{
			name: "Error code",
			code: Error,
			want: "Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.code.String()
			assert.Equal(t, tt.want, got)
		})
	}
}
