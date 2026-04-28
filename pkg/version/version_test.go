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
				GitVersion:     "1.3.0",
				GitCommit:      "da070e68f3318410c8c70ed8186a2bc4736dacbd",
				GitTreeState:   "clean",
				GitShortCommit: "851c78564",
				BuildDate:      "2022-08-31T13:09:22Z",
				GoVersion:      "go1.18.3",
				Compiler:       "gc",
				Platform:       "linux/amd64",
			},
			want: `version.Info{GitVersion:"1.3.0", GitCommit:"da070e68f3318410c8c70ed8186a2bc4736dacbd", GitShortCommit:"851c78564", GitTreeState:"clean", BuildDate:"2022-08-31T13:09:22Z", GoVersion:"go1.18.3", Compiler:"gc", Platform:"linux/amd64"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.info.String(); got != tt.want {
				t.Errorf("Info.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
