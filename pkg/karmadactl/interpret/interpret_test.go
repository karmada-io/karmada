package interpret

import (
	"testing"
)

func TestOptions_Validate(t *testing.T) {
	tests := []struct {
		name    string
		options *Options
		wantErr bool
	}{
		{
			name: "operation is unknown",
			options: &Options{
				Operation: "unknown-operation",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.options.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
