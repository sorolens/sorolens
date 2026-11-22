package scval

import (
	"encoding/json"
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// scErrorB64 encodes an scvError ScVal with go-stellar-sdk, the reference
// encoder, so the hand-written decoder is checked against the real wire format.
func scErrorB64(t *testing.T, e xdr.ScError) string {
	t.Helper()
	b64, err := xdr.MarshalBase64(xdr.ScVal{Type: xdr.ScValTypeScvError, Error: &e})
	if err != nil {
		t.Fatalf("marshal ScVal: %v", err)
	}
	return b64
}

func hostErr(typ xdr.ScErrorType, code xdr.ScErrorCode) xdr.ScError {
	return xdr.ScError{Type: typ, Code: &code}
}

func TestDecode_ScError(t *testing.T) {
	contractCode := xdr.Uint32(42)
	tests := []struct {
		name  string
		in    xdr.ScError
		want  ScError
		human string
	}{
		{
			name:  "contract error keeps the contract code",
			in:    xdr.ScError{Type: xdr.ScErrorTypeSceContract, ContractCode: &contractCode},
			want:  ScError{Type: ScErrorTypeContract, Code: 42},
			human: "Error(Contract, #42)",
		},
		{
			name:  "budget exceeded",
			in:    hostErr(xdr.ScErrorTypeSceBudget, xdr.ScErrorCodeScecExceededLimit),
			want:  ScError{Type: ScErrorTypeBudget, Code: 5},
			human: "Error(Budget, ExceededLimit)",
		},
		{
			name:  "auth invalid action",
			in:    hostErr(xdr.ScErrorTypeSceAuth, xdr.ScErrorCodeScecInvalidAction),
			want:  ScError{Type: ScErrorTypeAuth, Code: 6},
			human: "Error(Auth, InvalidAction)",
		},
		{
			name:  "storage missing value",
			in:    hostErr(xdr.ScErrorTypeSceStorage, xdr.ScErrorCodeScecMissingValue),
			want:  ScError{Type: ScErrorTypeStorage, Code: 3},
			human: "Error(Storage, MissingValue)",
		},
		{
			name:  "wasm vm unexpected size",
			in:    hostErr(xdr.ScErrorTypeSceWasmVm, xdr.ScErrorCodeScecUnexpectedSize),
			want:  ScError{Type: ScErrorTypeWasmVm, Code: 9},
			human: "Error(WasmVm, UnexpectedSize)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Decode(scErrorB64(t, tt.in))
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if got.Type != "scvError" {
				t.Errorf("Type = %q, want scvError", got.Type)
			}
			if got.Value != tt.want {
				t.Errorf("Value = %#v, want %#v", got.Value, tt.want)
			}
			if got.Human != tt.human {
				t.Errorf("Human = %q, want %q", got.Human, tt.human)
			}
		})
	}
}

func TestDecode_ScErrorUnknownTypeAndCodeKeepRawNumbers(t *testing.T) {
	// Type 12 and code 77 do not exist in the current XDR. The value must
	// still decode, with the raw numbers in its printed form.
	got, err := Decode(encodeVal(scvError, 0, 0, 0, 12, 0, 0, 0, 77))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got.Value != (ScError{Type: 12, Code: 77}) {
		t.Fatalf("Value = %#v", got.Value)
	}
	if got.Human != "Error(#12, #77)" {
		t.Fatalf("Human = %q", got.Human)
	}

	known, _ := Decode(encodeVal(scvError, 0, 0, 0, 3, 0, 0, 0, 99))
	if known.Human != "Error(Storage, #99)" {
		t.Fatalf("unknown code on a known type: Human = %q", known.Human)
	}
}

func TestDecode_ScErrorTruncatedFallsBackToRaw(t *testing.T) {
	b := encodeVal(scvError, 0, 0, 0, 7)
	got, err := Decode(b)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got.Value != b || got.Type != "scvError" {
		t.Fatalf("truncated scvError must fall back to raw base64, got %#v", got)
	}
}

func TestScError_MarshalJSON(t *testing.T) {
	out, err := json.Marshal([]any{ScError{Type: ScErrorTypeAuth, Code: 6}})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `["Error(Auth, InvalidAction)"]` {
		t.Fatalf("got %s", out)
	}
}
