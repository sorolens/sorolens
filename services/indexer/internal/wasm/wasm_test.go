package wasm

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func be32(v uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	return b
}

// buildInstanceEntry builds a valid contract-instance ContractData
// LedgerEntry.xdr fixture from the same wire layout ContractInstanceKey uses.
func buildInstanceEntry(contractID, wasmHash []byte, lastModified uint32) []byte {
	var out []byte
	out = append(out, be32(lastModified)...)
	out = append(out, be32(ledgerEntryTypeContractData)...)
	out = append(out, be32(0)...) // ContractDataEntryExt V0
	out = append(out, be32(scAddressTypeContract)...)
	out = append(out, contractID...)
	out = append(out, be32(scvLedgerKeyContractInstance)...)
	out = append(out, be32(scvContractInstance)...)
	out = append(out, wasmHash...)
	// trailing fields that the parser must ignore (e.g. expirationLedgerSeq)
	out = append(out, be32(0)...)
	return out
}

func TestContractInstanceKey(t *testing.T) {
	contractID := make([]byte, 32)
	for i := range contractID {
		contractID[i] = byte(i)
	}
	idHex := hex.EncodeToString(contractID)

	key, err := ContractInstanceKey(idHex)
	if err != nil {
		t.Fatalf("ContractInstanceKey: %v", err)
	}

	raw, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	want := append([]byte{}, be32(ledgerEntryTypeContractData)...)
	want = append(want, be32(scAddressTypeContract)...)
	want = append(want, contractID...)
	want = append(want, be32(scvLedgerKeyContractInstance)...)
	if !bytes.Equal(raw, want) {
		t.Fatalf("key mismatch:\n got %x\nwant %x", raw, want)
	}
}

func TestContractInstanceKey_rejectsBadInput(t *testing.T) {
	for _, id := range []string{"", "zz", "abcd", "nothex", "00"} {
		if _, err := ContractInstanceKey(id); err == nil {
			t.Errorf("expected error for %q", id)
		}
	}
}

func TestWasmHashFromInstanceEntry(t *testing.T) {
	contractID := make([]byte, 32)
	for i := range contractID {
		contractID[i] = byte(0xA0 + i)
	}
	wasmHash := make([]byte, 32)
	for i := range wasmHash {
		wasmHash[i] = byte(i)
	}
	entryXDR := base64.StdEncoding.EncodeToString(buildInstanceEntry(contractID, wasmHash, 123456))

	got, ok := WasmHashFromInstanceEntry(entryXDR)
	if !ok {
		t.Fatal("expected ok=true for valid instance entry")
	}
	want := hex.EncodeToString(wasmHash)
	if got != want {
		t.Fatalf("hash mismatch:\n got %s\nwant %s", got, want)
	}
}

func TestWasmHashFromInstanceEntry_rejectsNonInstance(t *testing.T) {
	cases := map[string]struct {
		build func() []byte
	}{
		"contract code entry": {
			build: func() []byte {
				// lastModified + LedgerEntryType CONTRACT_CODE=7 ...
				return append(be32(10), be32(7)...)
			},
		},
		"wrong key scval": {
			build: func() []byte {
				out := append([]byte{}, be32(10)...)
				out = append(out, be32(ledgerEntryTypeContractData)...)
				out = append(out, be32(0)...)
				out = append(out, be32(scAddressTypeContract)...)
				out = append(out, make([]byte, 32)...)
				out = append(out, be32(21)...) // scvLedgerKeyNonce instead of instance
				return out
			},
		},
		"wrong value scval": {
			build: func() []byte {
				out := append([]byte{}, be32(10)...)
				out = append(out, be32(ledgerEntryTypeContractData)...)
				out = append(out, be32(0)...)
				out = append(out, be32(scAddressTypeContract)...)
				out = append(out, make([]byte, 32)...)
				out = append(out, be32(scvLedgerKeyContractInstance)...)
				out = append(out, be32(15)...) // scvSymbol
				return out
			},
		},
		"truncated hash": {
			build: func() []byte {
				out := append([]byte{}, be32(10)...)
				out = append(out, be32(ledgerEntryTypeContractData)...)
				out = append(out, be32(0)...)
				out = append(out, be32(scAddressTypeContract)...)
				out = append(out, make([]byte, 32)...)
				out = append(out, be32(scvLedgerKeyContractInstance)...)
				out = append(out, be32(scvContractInstance)...)
				out = append(out, make([]byte, 31)...)
				return out
			},
		},
		"empty input": {
			build: func() []byte { return []byte{} },
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			xdr := base64.StdEncoding.EncodeToString(tc.build())
			if hash, ok := WasmHashFromInstanceEntry(xdr); ok {
				t.Errorf("expected ok=false, got hash=%s", hash)
			}
		})
	}
}

// buildCodeEntry builds a valid CONTRACT_CODE LedgerEntry.xdr fixture holding
// the given Wasm bytecode.
func buildCodeEntry(code, hash []byte, lastModified uint32) []byte {
	var out []byte
	out = append(out, be32(lastModified)...)
	out = append(out, be32(ledgerEntryTypeContractCode)...)
	out = append(out, be32(0)...) // ContractCodeEntryExt V0
	out = append(out, hash...)
	out = append(out, be32(uint32(len(code)))...)
	out = append(out, code...)
	return out
}

func TestContractCodeKey(t *testing.T) {
	hash := make([]byte, 32)
	for i := range hash {
		hash[i] = byte(i)
	}

	key, err := ContractCodeKey(hex.EncodeToString(hash))
	if err != nil {
		t.Fatalf("ContractCodeKey: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		t.Fatalf("decode key: %v", err)
	}

	want := append(be32(ledgerEntryTypeContractCode), hash...)
	if !bytes.Equal(raw, want) {
		t.Errorf("ContractCodeKey = %x, want %x", raw, want)
	}
}

func TestContractCodeKeyRejectsBadHash(t *testing.T) {
	if _, err := ContractCodeKey("not-hex"); err == nil {
		t.Error("expected an error for non-hex input")
	}
	if _, err := ContractCodeKey(hex.EncodeToString(make([]byte, 31))); err == nil {
		t.Error("expected an error for a 31-byte hash")
	}
}

func TestWasmCodeFromCodeEntry(t *testing.T) {
	code := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	entry := buildCodeEntry(code, make([]byte, 32), 42)

	got, ok := WasmCodeFromCodeEntry(base64.StdEncoding.EncodeToString(entry))
	if !ok {
		t.Fatal("expected ok=true for a well-formed code entry")
	}
	if !bytes.Equal(got, code) {
		t.Errorf("code = %x, want %x", got, code)
	}
}

func TestWasmCodeFromCodeEntryRejectsOtherEntryTypes(t *testing.T) {
	// A contract-instance entry must not be read as code.
	instance := buildInstanceEntry(make([]byte, 32), make([]byte, 32), 1)
	if _, ok := WasmCodeFromCodeEntry(base64.StdEncoding.EncodeToString(instance)); ok {
		t.Error("expected ok=false for a contract-data entry")
	}

	if _, ok := WasmCodeFromCodeEntry("!!!not base64!!!"); ok {
		t.Error("expected ok=false for invalid base64")
	}
}

func TestWasmCodeFromCodeEntryRejectsTruncatedCode(t *testing.T) {
	code := []byte{0x00, 0x61, 0x73, 0x6d}
	entry := buildCodeEntry(code, make([]byte, 32), 1)
	// Lop off the last byte so the declared code length overruns the buffer.
	truncated := entry[:len(entry)-1]

	if _, ok := WasmCodeFromCodeEntry(base64.StdEncoding.EncodeToString(truncated)); ok {
		t.Error("expected ok=false for a truncated code entry")
	}
}
