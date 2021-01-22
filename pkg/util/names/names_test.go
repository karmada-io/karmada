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
