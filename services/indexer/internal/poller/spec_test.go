package poller

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/sorolens/sorolens/services/indexer/internal/contractspec"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// ---- fixture helpers -------------------------------------------------------

func u32le(v uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	return b
}

func joinBytes(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// wasmWithSpec builds a minimal Wasm module carrying a contractspecv0 section
// with a single `ping(who: address) -> u32` function.
func wasmWithSpec(t *testing.T) []byte {
	t.Helper()

	entry, err := xdr.NewScSpecEntry(xdr.ScSpecEntryKindScSpecEntryFunctionV0, xdr.ScSpecFunctionV0{
		Name: "ping",
		Inputs: []xdr.ScSpecFunctionInputV0{{
			Name: "who",
			Type: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeAddress},
		}},
		Outputs: []xdr.ScSpecTypeDef{{Type: xdr.ScSpecTypeScSpecTypeU32}},
	})
	if err != nil {
		t.Fatalf("NewScSpecEntry: %v", err)
	}

	var buf bytes.Buffer
	if _, err := xdr.Marshal(&buf, entry); err != nil {
		t.Fatalf("marshal spec entry: %v", err)
	}

	name := []byte(contractspec.SpecCustomSection)
	body := append(binary.AppendUvarint(nil, uint64(len(name))), name...)
	body = append(body, buf.Bytes()...)

	out := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00} // magic + version
	out = append(out, 0x00)                                       // custom section
	out = binary.AppendUvarint(out, uint64(len(body)))
	out = append(out, body...)
	return out
}

// instanceEntryXDR builds a ContractData instance entry carrying wasmHash.
func instanceEntryXDR(wasmHash []byte) string {
	raw := joinBytes(
		u32le(100), // lastModifiedLedgerSeq
		u32le(6),   // CONTRACT_DATA
		u32le(0),   // ext
		u32le(1),   // SCAddress type contract
		make([]byte, 32),
		u32le(20), // scvLedgerKeyContractInstance
		u32le(19), // scvContractInstance
		wasmHash,
		u32le(0), // trailing field the parser ignores
	)
	return base64.StdEncoding.EncodeToString(raw)
}

// codeEntryXDR builds a CONTRACT_CODE entry carrying the Wasm bytes.
func codeEntryXDR(code, wasmHash []byte) string {
	raw := joinBytes(
		u32le(100),
		u32le(7), // CONTRACT_CODE
		u32le(0),
		wasmHash,
		u32le(uint32(len(code))),
		code,
	)
	return base64.StdEncoding.EncodeToString(raw)
}

// specRPC returns both fixture entries for any requested key. The extractors
// reject the entry whose type does not match, so serving both is unambiguous.
type specRPC struct {
	instanceXDR string
	codeXDR     string
	calls       int
}

func (r *specRPC) GetLatestLedger(context.Context) (*LatestLedger, error) {
	return &LatestLedger{Sequence: 1}, nil
}

func (r *specRPC) GetEvents(context.Context, uint32, uint32, []EventFilter) (*GetEventsResult, error) {
	return &GetEventsResult{}, nil
}

func (r *specRPC) GetTransaction(context.Context, string) (*TransactionResult, error) {
	return &TransactionResult{}, nil
}

func (r *specRPC) GetLedgerEntries(_ context.Context, keys []string) (*GetLedgerEntriesResult, error) {
	r.calls++
	out := &GetLedgerEntriesResult{}
	for _, k := range keys {
		out.Entries = append(out.Entries,
			LedgerEntry{Key: k, XDR: r.instanceXDR},
			LedgerEntry{Key: k, XDR: r.codeXDR},
		)
	}
	return out, nil
}

// specStore is a Store that additionally implements ContractSpecStore. It
// embeds the package's fakeStore so it satisfies the full Store interface.
type specStore struct {
	*fakeStore
	saved   map[string]ContractSpec
	upserts int
}

func newSpecStore() *specStore {
	return &specStore{fakeStore: newFakeStore(nil), saved: map[string]ContractSpec{}}
}

func (s *specStore) GetContractSpec(_ context.Context, contractID string) (ContractSpec, bool, error) {
	cs, ok := s.saved[contractID]
	return cs, ok, nil
}

func (s *specStore) UpsertContractSpec(_ context.Context, spec ContractSpec) error {
	s.saved[spec.ContractID] = spec
	s.upserts++
	return nil
}

func specHash() []byte {
	hash := make([]byte, 32)
	for i := range hash {
		hash[i] = byte(i)
	}
	return hash
}

// contractID is a syntactically valid 64-hex contract id: ContractInstanceKey
// rejects anything that does not decode to exactly 32 bytes.
func contractID() string {
	id := make([]byte, 32)
	for i := range id {
		id[i] = 0xAB
	}
	return hex.EncodeToString(id)
}

// ---- tests -----------------------------------------------------------------

func TestCacheContractSpecParsesAndStores(t *testing.T) {
	code := wasmWithSpec(t)
	hash := specHash()

	store := newSpecStore()
	rpc := &specRPC{
		instanceXDR: instanceEntryXDR(hash),
		codeXDR:     codeEntryXDR(code, hash),
	}
	p := &Poller{store: store, log: testLogger()}

	p.cacheContractSpec(context.Background(), rpc, Contract{ID: contractID(), Network: "testnet"})

	if store.upserts != 1 {
		t.Fatalf("got %d upserts, want 1", store.upserts)
	}

	saved := store.saved[contractID()]
	if saved.WasmHash != hex.EncodeToString(hash) {
		t.Errorf("WasmHash = %q, want %q", saved.WasmHash, hex.EncodeToString(hash))
	}

	var parsed contractspec.Spec
	if err := json.Unmarshal(saved.Spec, &parsed); err != nil {
		t.Fatalf("stored spec is not valid JSON: %v", err)
	}
	if len(parsed.Functions) != 1 {
		t.Fatalf("got %d functions, want 1", len(parsed.Functions))
	}
	if parsed.Functions[0].Name != "ping" {
		t.Errorf("function name = %q, want %q", parsed.Functions[0].Name, "ping")
	}
	if len(parsed.Functions[0].Inputs) != 1 || parsed.Functions[0].Inputs[0].Type.Kind != "address" {
		t.Errorf("inputs = %+v, want one address argument", parsed.Functions[0].Inputs)
	}
	if len(parsed.Functions[0].Outputs) != 1 || parsed.Functions[0].Outputs[0].Kind != "u32" {
		t.Errorf("outputs = %+v, want one u32", parsed.Functions[0].Outputs)
	}

	// A second pass must not re-fetch: the cache is checked first.
	before := rpc.calls
	p.cacheContractSpec(context.Background(), rpc, Contract{ID: contractID()})
	if rpc.calls != before {
		t.Errorf("expected no RPC traffic when a spec is already cached (calls %d -> %d)", before, rpc.calls)
	}
}

// TestCacheContractSpecSkipsUnsupportedStore covers a Store that does not
// implement ContractSpecStore: spec caching is silently skipped.
func TestCacheContractSpecSkipsUnsupportedStore(t *testing.T) {
	rpc := &specRPC{}
	p := &Poller{store: newFakeStore(nil), log: testLogger()}

	// Must neither panic nor touch the network.
	p.cacheContractSpec(context.Background(), rpc, Contract{ID: contractID()})

	if rpc.calls != 0 {
		t.Errorf("expected no RPC calls for an unsupported store, got %d", rpc.calls)
	}
}

// TestCacheContractSpecIgnoresUnparseableWasm covers the acceptance criterion
// that a parse failure logs a warning and does not break indexing: a valid
// Wasm module with no spec section yields no row and no error.
func TestCacheContractSpecIgnoresUnparseableWasm(t *testing.T) {
	hash := specHash()
	// Valid Wasm, but no contractspecv0 custom section.
	code := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}

	store := newSpecStore()
	rpc := &specRPC{
		instanceXDR: instanceEntryXDR(hash),
		codeXDR:     codeEntryXDR(code, hash),
	}
	p := &Poller{store: store, log: testLogger()}

	p.cacheContractSpec(context.Background(), rpc, Contract{ID: contractID()})

	if store.upserts != 0 {
		t.Errorf("got %d upserts, want 0 for a contract without a spec section", store.upserts)
	}
}

// TestCacheContractSpecToleratesMissingEntries covers an RPC that returns no
// usable entries at all.
func TestCacheContractSpecToleratesMissingEntries(t *testing.T) {
	store := newSpecStore()
	rpc := &specRPC{} // both fixtures empty
	p := &Poller{store: store, log: testLogger()}

	p.cacheContractSpec(context.Background(), rpc, Contract{ID: contractID()})

	if store.upserts != 0 {
		t.Errorf("got %d upserts, want 0 when no ledger entries resolve", store.upserts)
	}
}
