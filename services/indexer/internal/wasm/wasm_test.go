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

func TestContractCodeKey(t *testing.T) {
	hash := make([]byte, 32)
	for i := range hash {
		hash[i] = byte(i + 1)
	}
	hashHex := hex.EncodeToString(hash)

	key, err := ContractCodeKey(hashHex)
	if err != nil {
		t.Fatalf("ContractCodeKey: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	want := append([]byte{}, be32(ledgerEntryTypeContractCode)...)
	want = append(want, hash...)
	if !bytes.Equal(raw, want) {
		t.Fatalf("key mismatch:\n got %x\nwant %x", raw, want)
	}
}

func TestContractCodeKey_rejectsBadInput(t *testing.T) {
	for _, h := range []string{"", "zz", "abcd", "00"} {
		if _, err := ContractCodeKey(h); err == nil {
			t.Errorf("expected error for %q", h)
		}
	}
}

func buildCodeEntry(wasmHash, code []byte, lastModified uint32, extV1 bool) []byte {
	var out []byte
	out = append(out, be32(lastModified)...)
	out = append(out, be32(ledgerEntryTypeContractCode)...)
	if extV1 {
		out = append(out, be32(1)...)
		out = append(out, make([]byte, 4*12)...)
	} else {
		out = append(out, be32(0)...)
	}
	out = append(out, wasmHash...)
	out = append(out, be32(uint32(len(code)))...)
	out = append(out, code...)
	pad := (4 - (len(code) % 4)) % 4
	out = append(out, make([]byte, pad)...)
	return out
}

func TestWasmCodeFromEntry(t *testing.T) {
	wasmHash := make([]byte, 32)
	for i := range wasmHash {
		wasmHash[i] = byte(i)
	}
	code := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00, 0xde, 0xad}
	entryXDR := base64.StdEncoding.EncodeToString(buildCodeEntry(wasmHash, code, 42, false))

	got, ok := WasmCodeFromEntry(entryXDR)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if !bytes.Equal(got, code) {
		t.Fatalf("code mismatch:\n got %x\nwant %x", got, code)
	}
}

func TestWasmCodeFromEntry_extV1(t *testing.T) {
	wasmHash := make([]byte, 32)
	code := []byte{0x00, 0x61, 0x73, 0x6d}
	entryXDR := base64.StdEncoding.EncodeToString(buildCodeEntry(wasmHash, code, 7, true))
	got, ok := WasmCodeFromEntry(entryXDR)
	if !ok {
		t.Fatal("expected ok=true for ext v1")
	}
	if !bytes.Equal(got, code) {
		t.Fatalf("code mismatch:\n got %x\nwant %x", got, code)
	}
}

func TestWasmCodeFromEntry_rejectsBad(t *testing.T) {
	if _, ok := WasmCodeFromEntry(base64.StdEncoding.EncodeToString([]byte{})); ok {
		t.Error("expected ok=false for empty")
	}
	bad := append(be32(1), be32(6)...)
	if _, ok := WasmCodeFromEntry(base64.StdEncoding.EncodeToString(bad)); ok {
		t.Error("expected ok=false for non-code entry")
	}
}
