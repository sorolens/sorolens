// Package wasm implements the minimal XDR encode/decode needed to track
// Soroban contract Wasm-hash (code) upgrades on-chain.
//
// It deliberately avoids importing the stellar/go SDK (consistent with the
// rest of the monorepo, see apps/api/internal/soroban/scval.go). It encodes
// the LedgerKey for a contract-instance ContractData entry and extracts the
// wasm hash from the getLedgerEntries entry XDR.
//
// Wire layout handled here (Protocol 20-22):
//
//	LedgerKey:
//	  union switch (LedgerEntryType): CONTRACT_DATA = 6        (u32)
//	  struct LedgerKeyContractData:
//	    union SCAddress: SC_ADDRESS_TYPE_CONTRACT = 1           (u32)
//	    opaque uint256[32] contractId
//	    union SCVal: scvLedgerKeyContractInstance = 20          (u32, no payload)
//
//	LedgerEntry.xdr (from getLedgerEntries):
//	  u32 lastModifiedLedgerSeq
//	  u32 LedgerEntryType := CONTRACT_DATA = 6
//	  u32 ContractDataEntryExt := 0 (void)
//	  union SCAddress                                         (u32 + 32 bytes)
//	  union SCVal key: scvLedgerKeyContractInstance = 20       (u32, no payload)
//	  union SCVal val: scvContractInstance = 19                (u32)
//	    struct ScContractInstance:
//	      hash wasm    (opaque[32])   <- the wasm hash lives FIRST
//	      ScVec storage …
//
// Parsing is defensive: every discriminant is validated and trailing fields
// (expirationLedgerSeq etc.) are never read, so minor protocol additions do
// not break extraction. On any mismatch the decoder returns ok=false and the
// caller skips silently instead of failing the whole poll pass.
package wasm

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

// XDR discriminants used by this package (Protocol 20+).
const (
	ledgerEntryTypeContractData  = 6  // LedgerEntryType.CONTRACT_DATA
	scAddressTypeContract        = 1  // SCAddressType.SCADDRESS_TYPE_CONTRACT
	scvLedgerKeyContractInstance = 20 // SCValType.scvLedgerKeyContractInstance
	scvContractInstance          = 19 // SCValType.scvContractInstance
)

// ContractInstanceKey builds the base64-encoded XDR LedgerKey that selects the
// contract-instance ContractData entry for the given 64-hex-char contract ID.
func ContractInstanceKey(contractIDHex string) (string, error) {
	id, err := hex.DecodeString(contractIDHex)
	if err != nil {
		return "", fmt.Errorf("wasm: invalid contract id %q: %w", contractIDHex, err)
	}
	if len(id) != 32 {
		return "", fmt.Errorf("wasm: contract id %q is %d bytes, want 32", contractIDHex, len(id))
	}

	buf := make([]byte, 0, 4+4+32+4)
	putU32 := func(v uint32) { buf = binary.BigEndian.AppendUint32(buf, v) }
	putU32(ledgerEntryTypeContractData)
	putU32(scAddressTypeContract)
	buf = append(buf, id...)
	putU32(scvLedgerKeyContractInstance)

	return base64.StdEncoding.EncodeToString(buf), nil
}

// WasmHashFromInstanceEntry extracts the lowercase hex wasm hash from the
// base64-encoded XDR of a contract-instance ContractData ledger entry returned
// by getLedgerEntries. It returns ok=false when the entry is not a decodable
// contract-instance entry (e.g. a ContractCode entry or a future reordering).
func WasmHashFromInstanceEntry(entryXDR string) (hash string, ok bool) {
	raw, err := base64.StdEncoding.DecodeString(entryXDR)
	if err != nil {
		return "", false
	}

	c := &cursor{buf: raw}
	if _, ok := c.u32(); !ok { // lastModifiedLedgerSeq
		return "", false
	}
	if v, ok := c.u32(); !ok || v != ledgerEntryTypeContractData {
		return "", false
	}
	if v, ok := c.u32(); !ok || v != 0 { // ContractDataEntryExt (V0 = void)
		return "", false
	}
	if v, ok := c.u32(); !ok || v > 1 { // SCAddress type (0 account, 1 contract)
		return "", false
	}
	if _, ok := c.bytes(32); !ok { // address bytes
		return "", false
	}
	if v, ok := c.u32(); !ok || v != scvLedgerKeyContractInstance {
		return "", false
	}
	if v, ok := c.u32(); !ok || v != scvContractInstance {
		return "", false
	}
	w, ok := c.bytes(32)
	if !ok {
		return "", false
	}
	return hex.EncodeToString(w), true
}

// ---- tiny XDR cursor -------------------------------------------------------

type cursor struct {
	buf []byte
	off int
}

func (c *cursor) u32() (uint32, bool) {
	if len(c.buf)-c.off < 4 {
		return 0, false
	}
	v := binary.BigEndian.Uint32(c.buf[c.off : c.off+4])
	c.off += 4
	return v, true
}

func (c *cursor) bytes(n int) ([]byte, bool) {
	if len(c.buf)-c.off < n {
		return nil, false
	}
	out := c.buf[c.off : c.off+n]
	c.off += n
	return out, true
}
