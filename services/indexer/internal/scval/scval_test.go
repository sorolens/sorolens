package scval

import (
	"testing"
)

// These XDR values are from the integration test (tests/integration/mock_rpc/main.go)
// and were generated with stellar/go-stellar-sdk
const (
	symbolTransfer = "AAAADwAAAAh0cmFuc2Zlcg==" // Symbol("transfer")
	i128_1000      = "AAAACgAAAAAAAAAAAAAAAAAAA+g=" // I128(1000)
)

func TestDecodeScVal_Symbol(t *testing.T) {
	result, err := Decode(symbolTransfer)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if result.Type != "scvSymbol" {
		t.Errorf("Type = %v, want scvSymbol", result.Type)
	}
	if result.Value != "transfer" {
		t.Errorf("Value = %v, want transfer", result.Value)
	}
}

func TestDecodeScVal_I128(t *testing.T) {
	result, err := Decode(i128_1000)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if result.Type != "scvI128" {
		t.Errorf("Type = %v, want scvI128", result.Type)
	}
	if result.Human != "1000" {
		t.Errorf("Human = %v, want 1000", result.Human)
	}
	// Value should be string for JSON compatibility
	if result.Value != "1000" {
		t.Errorf("Value = %v, want 1000", result.Value)
	}
}

func TestDecodeScVal_Malformed(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"invalid base64", "invalid!!"},
		{"too short", "AQ=="},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decode(tt.input)
			if err == nil {
				t.Error("Decode() expected error, got nil")
			}
		})
	}
}
