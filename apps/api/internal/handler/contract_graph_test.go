package handler

import (
	"testing"
)

func TestIsValidContractID(t *testing.T) {
	tests := []struct {
		id       string
		expected bool
	}{
		{"CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", true},
		{"CBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB", true},
		{"GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", false}, // Wrong prefix
		{"CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", false},  // Too short (55)
		{"caaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", false}, // Lowercase
		{"CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA1", false}, // '1' is not valid base32 in strkey
	}

	for _, tt := range tests {
		if got := isValidContractID(tt.id); got != tt.expected {
			t.Errorf("isValidContractID(%q) = %v, want %v", tt.id, got, tt.expected)
		}
	}
}
