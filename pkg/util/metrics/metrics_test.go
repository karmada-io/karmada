package metrics

import (
	"errors"
	"testing"
)

func TestGetResultByError(t *testing.T) {
	tests := []struct {
		name  string
		error error
		want  string
	}{
		{
			name:  "error is nil",
			error: nil,
			want:  "success",
		},
		{
			name:  "error is not nil",
			error: errors.New("hello,error"),
			want:  "error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetResultByError(tt.error); got != tt.want {
				t.Errorf("GetResultByError() = %v, want %v", got, tt.want)
			}
		})
	}
}
