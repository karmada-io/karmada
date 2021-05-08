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

func TestValidateClusterProxyURL(t *testing.T) {
	var tests = []struct {
		name         string
		proxy        string
		expectError  bool
		expectErrMsg string
	}{
		{
			name:        "valid http",
			proxy:       "http://example.com",
			expectError: false,
		},
		{
			name:        "valid https",
			proxy:       "https://example.com",
			expectError: false,
		},
		{
			name:        "valid socks5",
			proxy:       "socks5://example.com",
			expectError: false,
		},
		{
			name:         "no schema is not allowed",
			proxy:        "example",
			expectError:  true,
			expectErrMsg: `unsupported scheme "", must be http, https, or socks5`,
		},
		{
			name:         "schema out of range is not allowed",
			proxy:        "socks4://example.com",
			expectError:  true,
			expectErrMsg: `unsupported scheme "socks4", must be http, https, or socks5`,
		},
	}

	for _, test := range tests {
		tc := test
		t.Run(tc.name, func(t *testing.T) {
			errs := ValidateClusterProxyURL(tc.proxy)
			if !tc.expectError && len(errs) != 0 {
				t.Errorf("not expect errors but got: %v", errs)
			} else if tc.expectError && tc.expectErrMsg != strings.Join(errs, ",") {
				t.Errorf("expected error: %v, but got: %v", tc.expectErrMsg, strings.Join(errs, ","))
			}
		})
	}
}
