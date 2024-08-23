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

package version

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
)

func TestInfo_String(t *testing.T) {
	tests := []struct {
		name string
		info Info
		want string
	}{
		{
			name: "test1",
			info: Info{
				GitVersion:   "1.3.0",
				GitCommit:    "da070e68f3318410c8c70ed8186a2bc4736dacbd",
				GitTreeState: "clean",
				BuildDate:    "2022-08-31T13:09:22Z",
				GoVersion:    "go1.18.3",
				Compiler:     "gc",
				Platform:     "linux/amd64",
			},
			want: `version.Info{GitVersion:"1.3.0", GitCommit:"da070e68f3318410c8c70ed8186a2bc4736dacbd", GitTreeState:"clean", BuildDate:"2022-08-31T13:09:22Z", GoVersion:"go1.18.3", Compiler:"gc", Platform:"linux/amd64"}`,
		},
		{
			name: "empty info",
			info: Info{},
			want: `version.Info{GitVersion:"", GitCommit:"", GitTreeState:"", BuildDate:"", GoVersion:"", Compiler:"", Platform:""}`,
		},
	}

	fields := []string{"GitVersion", "GitCommit", "GitTreeState", "BuildDate", "GoVersion", "Compiler", "Platform"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.info.String()
			if got != tt.want {
				t.Errorf("Info.String() = %v, want %v", got, tt.want)
			}
			for _, field := range fields {
				if !strings.Contains(got, field) {
					t.Errorf("Info.String() does not contain %s", field)
				}
			}
		})
	}
}

func TestGet(t *testing.T) {
	info := Get()

	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"GitVersion", info.GitVersion, gitVersion},
		{"GitCommit", info.GitCommit, gitCommit},
		{"GitTreeState", info.GitTreeState, gitTreeState},
		{"BuildDate", info.BuildDate, buildDate},
		{"GoVersion", info.GoVersion, runtime.Version()},
		{"Compiler", info.Compiler, runtime.Compiler},
		{"Platform", info.Platform, fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("Get().%s = %v, want %v", tt.name, tt.got, tt.expected)
			}
		})
	}
}

func TestInfoFields(t *testing.T) {
	tests := []struct {
		name     string
		info     Info
		field    string
		expected string
	}{
		{"GitVersion", Info{GitVersion: "v1.0.0"}, "GitVersion", "v1.0.0"},
		{"GitCommit", Info{GitCommit: "abcdef123456"}, "GitCommit", "abcdef123456"},
		{"GitTreeState", Info{GitTreeState: "clean"}, "GitTreeState", "clean"},
		{"BuildDate", Info{BuildDate: "2023-04-01T00:00:00Z"}, "BuildDate", "2023-04-01T00:00:00Z"},
		{"GoVersion", Info{GoVersion: "go1.16"}, "GoVersion", "go1.16"},
		{"Compiler", Info{Compiler: "gc"}, "Compiler", "gc"},
		{"Platform", Info{Platform: "linux/amd64"}, "Platform", "linux/amd64"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := ""
			switch tt.field {
			case "GitVersion":
				value = tt.info.GitVersion
			case "GitCommit":
				value = tt.info.GitCommit
			case "GitTreeState":
				value = tt.info.GitTreeState
			case "BuildDate":
				value = tt.info.BuildDate
			case "GoVersion":
				value = tt.info.GoVersion
			case "Compiler":
				value = tt.info.Compiler
			case "Platform":
				value = tt.info.Platform
			}

			if value != tt.expected {
				t.Errorf("%s = %v, want %v", tt.field, value, tt.expected)
			}
		})
	}
}
