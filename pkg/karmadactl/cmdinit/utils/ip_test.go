package utils

import (
	"testing"
)

func TestParseIP(t *testing.T) {
	tests := []struct {
		name     string
		IP       string
		expected int
	}{
		{"v4", "127.0.0.1", 4},
		{"v6", "1030::C9B4:FF12:48AA:1A2B", 6},
		{"v4-error", "333.0.0.0", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, IPType, _ := ParseIP(tt.IP)
			if IPType != tt.expected {
				t.Errorf("parse ip %d, want %d", IPType, tt.expected)
			}
		})
	}
}
