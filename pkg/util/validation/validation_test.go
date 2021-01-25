package validation

import (
	"strings"
	"testing"
)

func TestValidateClusterName(t *testing.T) {
	var tests = []struct {
		name        string
		cluster     string
		expectError bool
	}{
		{
			name:        "valid cluster",
			cluster:     "valid-cluster",
			expectError: false,
		},
		{
			name:        "contains invalid character is not allowed",
			cluster:     "invalid.cluster",
			expectError: true,
		},
		{
			name:        "empty name is not allowed",
			cluster:     "",
			expectError: true,
		},
		{
			name:        "too long name is not allowed",
			cluster:     "abcdefghijklmnopqrstuvwxyz01234567890123456789012", // 49 characters
			expectError: true,
		},
	}

	for _, test := range tests {
		tc := test
		errs := ValidateClusterName(tc.cluster)
		if len(errs) > 0 && tc.expectError != true {
			t.Fatalf("expect no error but got: %s", strings.Join(errs, ";"))
		}
		if len(errs) == 0 && tc.expectError == true {
			t.Fatalf("expect an error but got none")
		}
	}
}
