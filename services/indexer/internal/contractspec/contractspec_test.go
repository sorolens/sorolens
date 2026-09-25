package contractspec

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// ---- fixture builders ------------------------------------------------------

// wasmWithSection wraps payload in a minimal Wasm module containing a
// "contractspecv0" custom section. A dummy type section is emitted first so the
// walker has to skip a non-custom section to reach the spec.
func wasmWithSection(t *testing.T, name string, payload []byte) []byte {
	t.Helper()

	var out []byte
	out = append(out, wasmMagic...)
	out = append(out, 0x01, 0x00, 0x00, 0x00) // version 1

	// Section id 1 (type) with an empty body: not a custom section.
	out = append(out, 0x01, 0x00)

	body := binary.AppendUvarint(nil, uint64(len(name)))
	body = append(body, name...)
	body = append(body, payload...)

	out = append(out, sectionIDCustom)
	out = binary.AppendUvarint(out, uint64(len(body)))
	out = append(out, body...)

	return out
}

// mustEntry marshals one SCSpecEntry of the given kind.
func mustEntry(t *testing.T, kind xdr.ScSpecEntryKind, value any) []byte {
	t.Helper()

	entry, err := xdr.NewScSpecEntry(kind, value)
	if err != nil {
		t.Fatalf("NewScSpecEntry(%d): %v", kind, err)
	}
	var buf bytes.Buffer
	if _, err := xdr.Marshal(&buf, entry); err != nil {
		t.Fatalf("marshal entry: %v", err)
	}
	return buf.Bytes()
}

// specWithFunctions builds the "contractspecv0" payload for a simple transfer
// function, preceded by a UDT struct and followed by an event so the parser has
// to decode and skip both.
func specWithFunctions(t *testing.T) []byte {
	t.Helper()

	var payload []byte
	payload = append(payload, mustEntry(t, xdr.ScSpecEntryKindScSpecEntryUdtStructV0, xdr.ScSpecUdtStructV0{
		Doc:  "A transfer record",
		Lib:  "sorolens",
		Name: "TransferRecord",
		Fields: []xdr.ScSpecUdtStructFieldV0{{
			Name: "amount",
			Type: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeI128},
		}},
	})...)

	payload = append(payload, mustEntry(t, xdr.ScSpecEntryKindScSpecEntryFunctionV0, xdr.ScSpecFunctionV0{
		Doc:  "Transfer tokens between two accounts",
		Name: "transfer",
		Inputs: []xdr.ScSpecFunctionInputV0{
			{Name: "from", Type: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeAddress}},
			{Name: "to", Type: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeAddress}},
			{Name: "amount", Type: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeI128}},
		},
		Outputs: []xdr.ScSpecTypeDef{{Type: xdr.ScSpecTypeScSpecTypeVoid}},
	})...)

	payload = append(payload, mustEntry(t, xdr.ScSpecEntryKindScSpecEntryEventV0, xdr.ScSpecEventV0{
		Name:       "Transfer",
		DataFormat: xdr.ScSpecEventDataFormatScSpecEventDataFormatSingleValue,
	})...)

	return payload
}

// ---- tests -----------------------------------------------------------------

func TestParseWasmExtractsFunctions(t *testing.T) {
	wasm := wasmWithSection(t, SpecCustomSection, specWithFunctions(t))

	spec, err := ParseWasm(wasm)
	if err != nil {
		t.Fatalf("ParseWasm: %v", err)
	}

	if len(spec.Functions) != 1 {
		t.Fatalf("got %d functions, want 1 (non-function entries must be skipped)", len(spec.Functions))
	}

	fn := spec.Functions[0]
	if fn.Name != "transfer" {
		t.Errorf("Name = %q, want %q", fn.Name, "transfer")
	}
	if fn.Doc != "Transfer tokens between two accounts" {
		t.Errorf("Doc = %q", fn.Doc)
	}
	if len(fn.Inputs) != 3 {
		t.Fatalf("got %d inputs, want 3", len(fn.Inputs))
	}
	for i, want := range []struct{ name, kind string }{
		{"from", "address"},
		{"to", "address"},
		{"amount", "i128"},
	} {
		if fn.Inputs[i].Name != want.name {
			t.Errorf("input %d name = %q, want %q", i, fn.Inputs[i].Name, want.name)
		}
		if fn.Inputs[i].Type.Kind != want.kind {
			t.Errorf("input %d kind = %q, want %q", i, fn.Inputs[i].Type.Kind, want.kind)
		}
	}
	if len(fn.Outputs) != 1 || fn.Outputs[0].Kind != "void" {
		t.Errorf("Outputs = %+v, want a single void", fn.Outputs)
	}
}

func TestParseWasmRendersNestedTypes(t *testing.T) {
	udt := xdr.ScSpecTypeDef{
		Type: xdr.ScSpecTypeScSpecTypeUdt,
		Udt:  &xdr.ScSpecTypeUdt{Name: "TransferRecord"},
	}
	vecOfUdt := xdr.ScSpecTypeDef{
		Type: xdr.ScSpecTypeScSpecTypeVec,
		Vec:  &xdr.ScSpecTypeVec{ElementType: udt},
	}
	optionU32 := xdr.ScSpecTypeDef{
		Type:   xdr.ScSpecTypeScSpecTypeOption,
		Option: &xdr.ScSpecTypeOption{ValueType: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeU32}},
	}
	mapType := xdr.ScSpecTypeDef{
		Type: xdr.ScSpecTypeScSpecTypeMap,
		Map: &xdr.ScSpecTypeMap{
			KeyType:   xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeAddress},
			ValueType: xdr.ScSpecTypeDef{Type: xdr.ScSpecTypeScSpecTypeI128},
		},
	}
	tuple := xdr.ScSpecTypeDef{
		Type: xdr.ScSpecTypeScSpecTypeTuple,
		Tuple: &xdr.ScSpecTypeTuple{ValueTypes: []xdr.ScSpecTypeDef{
			{Type: xdr.ScSpecTypeScSpecTypeU32},
			{Type: xdr.ScSpecTypeScSpecTypeAddress},
		}},
	}
	bytesN := xdr.ScSpecTypeDef{
		Type:   xdr.ScSpecTypeScSpecTypeBytesN,
		BytesN: &xdr.ScSpecTypeBytesN{N: 32},
	}

	payload := mustEntry(t, xdr.ScSpecEntryKindScSpecEntryFunctionV0, xdr.ScSpecFunctionV0{
		Name: "complex",
		Inputs: []xdr.ScSpecFunctionInputV0{
			{Name: "records", Type: vecOfUdt},
			{Name: "maybe", Type: optionU32},
			{Name: "balances", Type: mapType},
			{Name: "pair", Type: tuple},
			{Name: "digest", Type: bytesN},
		},
		Outputs: []xdr.ScSpecTypeDef{{Type: xdr.ScSpecTypeScSpecTypeVoid}},
	})

	spec, err := ParseWasm(wasmWithSection(t, SpecCustomSection, payload))
	if err != nil {
		t.Fatalf("ParseWasm: %v", err)
	}

	byName := map[string]Type{}
	for _, in := range spec.Functions[0].Inputs {
		byName[in.Name] = in.Type
	}

	if got := byName["records"]; got.Kind != "vec" || got.Elem == nil || got.Elem.Kind != "udt" || got.Elem.Name != "TransferRecord" {
		t.Errorf("records = %+v, want vec<udt TransferRecord>", got)
	}
	if got := byName["maybe"]; got.Kind != "option" || got.Elem == nil || got.Elem.Kind != "u32" {
		t.Errorf("maybe = %+v, want option<u32>", got)
	}
	if got := byName["balances"]; got.Kind != "map" || got.Key == nil || got.Key.Kind != "address" || got.Value == nil || got.Value.Kind != "i128" {
		t.Errorf("balances = %+v, want map<address, i128>", got)
	}
	if got := byName["pair"]; got.Kind != "tuple" || len(got.Tuple) != 2 || got.Tuple[1].Kind != "address" {
		t.Errorf("pair = %+v, want tuple<u32, address>", got)
	}
	if got := byName["digest"]; got.Kind != "bytes_n" || got.N != 32 {
		t.Errorf("digest = %+v, want bytes_n 32", got)
	}
}

// TestSpecMarshalsToJSONTree pins the wire shape the /spec endpoint returns.
func TestSpecMarshalsToJSONTree(t *testing.T) {
	spec, err := ParseWasm(wasmWithSection(t, SpecCustomSection, specWithFunctions(t)))
	if err != nil {
		t.Fatalf("ParseWasm: %v", err)
	}

	raw, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}

	for _, want := range []string{
		`"functions"`,
		`"name":"transfer"`,
		`"name":"from"`,
		`"kind":"address"`,
		`"kind":"i128"`,
		`"kind":"void"`,
	} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("JSON output missing %s; got %s", want, raw)
		}
	}
}

func TestParseWasmErrorsWhenSpecSectionMissing(t *testing.T) {
	// A valid Wasm module with a custom section under a different name.
	wasm := wasmWithSection(t, "notthespec", []byte{0x01})

	if _, err := ParseWasm(wasm); err == nil {
		t.Fatal("ParseWasm: expected an error for a module without a spec section")
	}
}

func TestParseWasmRejectsNonWasm(t *testing.T) {
	if _, err := ParseWasm([]byte("this is not wasm")); err == nil {
		t.Fatal("ParseWasm: expected an error for non-Wasm input")
	}
}

func TestParseWasmRejectsTruncatedSection(t *testing.T) {
	payload := specWithFunctions(t)
	truncated := payload[:len(payload)-1]

	if _, err := ParseWasm(wasmWithSection(t, SpecCustomSection, truncated)); err == nil {
		t.Fatal("ParseWasm: expected an error for a truncated spec section")
	}
}

func TestParseSectionEmptyYieldsEmptyFunctionList(t *testing.T) {
	spec, err := ParseSection(nil)
	if err != nil {
		t.Fatalf("ParseSection(nil): %v", err)
	}
	if spec.Functions == nil {
		t.Error("Functions should be an empty slice, not nil, so it marshals as []")
	}
	if len(spec.Functions) != 0 {
		t.Errorf("got %d functions, want 0", len(spec.Functions))
	}
}
