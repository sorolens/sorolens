package scval

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"math/big"
	"reflect"
	"testing"
)

// These XDR values are from the integration test (tests/integration/mock_rpc/main.go)
// and were generated with stellar/go-stellar-sdk
const (
	symbolTransfer = "AAAADwAAAAh0cmFuc2Zlcg=="     // Symbol("transfer")
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

func b64(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

func encodeVal(disc uint32, body ...byte) string {
	buf := make([]byte, 4+len(body))
	binary.BigEndian.PutUint32(buf[:4], disc)
	copy(buf[4:], body)
	return b64(buf)
}

func TestDecodeScVal_AllTypes(t *testing.T) {
	tests := []struct {
		name      string
		b64       string
		wantType  string
		wantValue any
		wantHuman string
	}{
		{
			name:      "scvVoid",
			b64:       encodeVal(scvVoid),
			wantType:  "scvVoid",
			wantValue: nil,
			wantHuman: "void",
		},
		{
			name:      "scvBool true",
			b64:       encodeVal(scvBool, 0, 0, 0, 1),
			wantType:  "scvBool",
			wantValue: true,
			wantHuman: "true",
		},
		{
			name:      "scvBool false",
			b64:       encodeVal(scvBool, 0, 0, 0, 0),
			wantType:  "scvBool",
			wantValue: false,
			wantHuman: "false",
		},
		{
			name:      "scvU32",
			b64:       encodeVal(scvU32, 0, 0, 1, 0),
			wantType:  "scvU32",
			wantValue: uint32(256),
			wantHuman: "256",
		},
		{
			name: "scvI32 negative",
			b64: func() string {
				b := make([]byte, 4)
				val := int32(-42)
				binary.BigEndian.PutUint32(b, uint32(val))
				return encodeVal(scvI32, b...)
			}(),
			wantType:  "scvI32",
			wantValue: int32(-42),
			wantHuman: "-42",
		},
		{
			name: "scvU64",
			b64: func() string {
				b := make([]byte, 8)
				binary.BigEndian.PutUint64(b, 10000000000)
				return encodeVal(scvU64, b...)
			}(),
			wantType:  "scvU64",
			wantValue: uint64(10000000000),
			wantHuman: "10000000000",
		},
		{
			name: "scvTimePoint (disc 7)",
			b64: func() string {
				b := make([]byte, 8)
				binary.BigEndian.PutUint64(b, 1700000000)
				return encodeVal(7, b...)
			}(),
			wantType:  "scvTimePoint",
			wantValue: uint64(1700000000),
			wantHuman: "1700000000",
		},
		{
			name: "scvDuration (disc 8)",
			b64: func() string {
				b := make([]byte, 8)
				binary.BigEndian.PutUint64(b, 3600)
				return encodeVal(8, b...)
			}(),
			wantType:  "scvDuration",
			wantValue: uint64(3600),
			wantHuman: "3600",
		},
		{
			name: "scvI64 negative",
			b64: func() string {
				b := make([]byte, 8)
				val := int64(-999999)
				binary.BigEndian.PutUint64(b, uint64(val))
				return encodeVal(scvI64, b...)
			}(),
			wantType:  "scvI64",
			wantValue: int64(-999999),
			wantHuman: "-999999",
		},
		{
			name: "scvU128",
			b64: func() string {
				b := make([]byte, 16)
				binary.BigEndian.PutUint64(b[:8], 1)
				binary.BigEndian.PutUint64(b[8:], 2)
				return encodeVal(scvU128, b...)
			}(),
			wantType: "scvU128",
			wantValue: func() *big.Int {
				v := new(big.Int).SetUint64(1)
				v.Lsh(v, 64)
				v.Or(v, new(big.Int).SetUint64(2))
				return v
			}(),
			wantHuman: func() string {
				v := new(big.Int).SetUint64(1)
				v.Lsh(v, 64)
				v.Or(v, new(big.Int).SetUint64(2))
				return v.String()
			}(),
		},
		{
			name: "scvI128 negative",
			b64: func() string {
				b := make([]byte, 16)
				hiVal := int64(-1)
				binary.BigEndian.PutUint64(b[:8], uint64(hiVal))
				binary.BigEndian.PutUint64(b[8:], uint64(5))
				return encodeVal(scvI128, b...)
			}(),
			wantType: "scvI128",
			wantValue: func() string {
				hi := int64(-1)
				lo := uint64(5)
				v := new(big.Int).SetInt64(hi)
				v.Lsh(v, 64)
				v.Or(v, new(big.Int).SetUint64(lo))
				return v.String()
			}(),
			wantHuman: func() string {
				hi := int64(-1)
				lo := uint64(5)
				v := new(big.Int).SetInt64(hi)
				v.Lsh(v, 64)
				v.Or(v, new(big.Int).SetUint64(lo))
				return v.String()
			}(),
		},
		{
			name: "scvString",
			b64: func() string {
				str := "hello soroban"
				b := make([]byte, 4+len(str))
				binary.BigEndian.PutUint32(b[:4], uint32(len(str)))
				copy(b[4:], str)
				return encodeVal(scvString, b...)
			}(),
			wantType:  "scvString",
			wantValue: "hello soroban",
			wantHuman: "hello soroban",
		},
		{
			name: "scvBytes",
			b64: func() string {
				data := []byte{0xde, 0xad, 0xbe, 0xef}
				b := make([]byte, 4+len(data))
				binary.BigEndian.PutUint32(b[:4], uint32(len(data)))
				copy(b[4:], data)
				return encodeVal(scvBytes, b...)
			}(),
			wantType:  "scvBytes",
			wantValue: []byte{0xde, 0xad, 0xbe, 0xef},
			wantHuman: "deadbeef",
		},
		{
			name: "scvAddress (32 bytes)",
			b64: func() string {
				addr := bytes.Repeat([]byte{0x42}, 32)
				return encodeVal(scvAddress, addr...)
			}(),
			wantType:  "scvAddress",
			wantValue: bytes.Repeat([]byte{0x42}, 32),
			wantHuman: func() string {
				return bytesToHex(bytes.Repeat([]byte{0x42}, 32))
			}(),
		},
		{
			name: "scvVec with elements",
			b64: func() string {
				// vec containing 1 symbol ("ok")
				symBytes := make([]byte, 4+4+2)
				binary.BigEndian.PutUint32(symBytes[:4], scvSymbol)
				binary.BigEndian.PutUint32(symBytes[4:8], 2)
				copy(symBytes[8:], "ok")

				var buf bytes.Buffer
				// vec length: 1
				var vecLen [4]byte
				binary.BigEndian.PutUint32(vecLen[:], 1)
				buf.Write(vecLen[:])

				// element length
				var elemLen [4]byte
				binary.BigEndian.PutUint32(elemLen[:], uint32(len(symBytes)))
				buf.Write(elemLen[:])
				buf.Write(symBytes)

				return encodeVal(scvVec, buf.Bytes()...)
			}(),
			wantType:  "scvVec",
			wantValue: []any{"ok"},
			wantHuman: "vec[1]",
		},
		{
			name: "scvMap with string-to-i32 entry",
			b64: func() string {
				keyBytes := make([]byte, 4+4+4)
				binary.BigEndian.PutUint32(keyBytes[:4], scvString)
				binary.BigEndian.PutUint32(keyBytes[4:8], 4)
				copy(keyBytes[8:], "rate")

				valBytes := make([]byte, 4+4)
				binary.BigEndian.PutUint32(valBytes[:4], scvI32)
				binary.BigEndian.PutUint32(valBytes[4:], 100)

				var buf bytes.Buffer
				// map length: 1
				var mapLen [4]byte
				binary.BigEndian.PutUint32(mapLen[:], 1)
				buf.Write(mapLen[:])

				// key length + key
				var kLen [4]byte
				binary.BigEndian.PutUint32(kLen[:], uint32(len(keyBytes)))
				buf.Write(kLen[:])
				buf.Write(keyBytes)

				// val length + val
				var vLen [4]byte
				binary.BigEndian.PutUint32(vLen[:], uint32(len(valBytes)))
				buf.Write(vLen[:])
				buf.Write(valBytes)

				return encodeVal(scvMap, buf.Bytes()...)
			}(),
			wantType:  "scvMap",
			wantValue: map[string]any{"rate": int32(100)},
			wantHuman: "map[1]",
		},
		{
			name:      "scvUnknown fallback",
			b64:       encodeVal(999, 1, 2, 3),
			wantType:  "scvUnknown(999)",
			wantValue: encodeVal(999, 1, 2, 3),
			wantHuman: "scvUnknown(999):" + encodeVal(999, 1, 2, 3),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Decode(tt.b64)
			if err != nil {
				t.Fatalf("Decode() unexpected error: %v", err)
			}
			if got.Type != tt.wantType {
				t.Errorf("Type = %v, want %v", got.Type, tt.wantType)
			}
			if got.Human != tt.wantHuman {
				t.Errorf("Human = %v, want %v", got.Human, tt.wantHuman)
			}
			if !reflect.DeepEqual(got.Value, tt.wantValue) {
				t.Errorf("Value = %#v, want %#v", got.Value, tt.wantValue)
			}
		})
	}
}

func bytesToHex(b []byte) string {
	const hextable = "0123456789abcdef"
	dst := make([]byte, len(b)*2)
	for i, v := range b {
		dst[i*2] = hextable[v>>4]
		dst[i*2+1] = hextable[v&0x0f]
	}
	return string(dst)
}

func TestDecodeScVal_TruncatedBodies(t *testing.T) {
	// For each discriminator, when body is shorter than expected,
	// Decode falls back to raw string representation without erroring.
	tests := []struct {
		name     string
		disc     uint32
		body     []byte
		wantType string
	}{
		{"bool truncated", scvBool, []byte{1, 2}, "scvBool"},
		{"u32 truncated", scvU32, []byte{1, 2}, "scvU32"},
		{"i32 truncated", scvI32, []byte{1, 2}, "scvI32"},
		{"u64 truncated", scvU64, []byte{1, 2, 3, 4}, "scvU64"},
		{"i64 truncated", scvI64, []byte{1, 2, 3, 4}, "scvI64"},
		{"u128 truncated", scvU128, []byte{1, 2, 3, 4, 5, 6, 7, 8}, "scvU128"},
		{"i128 truncated", scvI128, []byte{1, 2, 3, 4, 5, 6, 7, 8}, "scvI128"},
		{"string truncated header", scvString, []byte{1, 2}, "scvString"},
		{"string truncated body", scvString, []byte{0, 0, 0, 10, 'a', 'b'}, "scvString"},
		{"bytes truncated header", scvBytes, []byte{1, 2}, "scvBytes"},
		{"bytes truncated body", scvBytes, []byte{0, 0, 0, 10, 0x01}, "scvBytes"},
		{"address truncated", scvAddress, []byte{1, 2, 3, 4}, "scvAddress"},
		{"vec truncated header", scvVec, []byte{1, 2}, "scvVec"},
		{"map truncated header", scvMap, []byte{1, 2}, "scvMap"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := encodeVal(tt.disc, tt.body...)
			got, err := Decode(raw)
			if err != nil {
				t.Fatalf("Decode() unexpected error on truncated body: %v", err)
			}
			if got.Type != tt.wantType {
				t.Errorf("Type = %v, want %v", got.Type, tt.wantType)
			}
			// Value should fallback to the raw base64 string
			if got.Value != raw {
				t.Errorf("Value = %v, want raw base64 fallback %v", got.Value, raw)
			}
		})
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
