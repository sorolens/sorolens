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
