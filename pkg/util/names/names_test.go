/*
Copyright The Karmada Authors.

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

package names

import "testing"

func TestGenerateExecutionSpaceName(t *testing.T) {
	type args struct {
		clusterName string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{name: "normal cluster name",
			args:    args{clusterName: "member-cluster-normal"},
			want:    "karmada-es-member-cluster-normal",
			wantErr: false,
		},
		{name: "empty member cluster name",
			args:    args{clusterName: ""},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateExecutionSpaceName(tt.args.clusterName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateExecutionSpaceName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GenerateExecutionSpaceName() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetClusterName(t *testing.T) {
	type args struct {
		executionSpaceName string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{name: "normal execution space name",
			args:    args{executionSpaceName: "karmada-es-member-cluster-normal"},
			want:    "member-cluster-normal",
			wantErr: false,
		},
		{name: "invalid member cluster",
			args:    args{executionSpaceName: "invalid"},
			want:    "",
			wantErr: true,
		},
		{name: "empty execution space name",
			args:    args{executionSpaceName: ""},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetClusterName(tt.args.executionSpaceName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetClusterName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetClusterName() got = %v, want %v", got, tt.want)
			}
		})
	}
}
