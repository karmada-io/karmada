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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.info.String(); got != tt.want {
				t.Errorf("Info.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
