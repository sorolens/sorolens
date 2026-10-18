// Package contractspec extracts the SEP-48 contract interface description
// embedded in a Soroban contract's Wasm and renders it as a JSON-friendly
// tree of callable functions with their argument and return types.
//
// A Soroban contract that derives its spec (the default for `#[contractimpl]`)
// carries a Wasm *custom section* named "contractspecv0". Its payload is a
// back-to-back sequence of `SCSpecEntry` XDR values: one per exported
// function, plus entries describing user-defined types and events.
//
// This package decodes that sequence and projects only the function entries,
// because "what functions can I call, and with what arguments?" is the
// question the dashboard needs to answer. Type references to user-defined
// types are preserved by name, so the tree is self-describing without having
// to cross-reference the UDT entries.
//
// XDR decoding is delegated to github.com/stellar/go-stellar-sdk/xdr rather
// than hand-rolled here: SCSpecEntry is a six-way union over deeply nested
// types, and transcription errors in a parser like that are silent and costly.
// (The other decoders in this repo — internal/wasm and apps/api's scval.go —
// are hand-rolled because they read a handful of fixed-layout structs.)
//
// Parsing is defensive: a contract with no spec section, a truncated section,
// or an entry this package does not understand yields an error from ParseWasm
// and no partial result, so callers can log a warning and carry on indexing.
package contractspec

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// SpecCustomSection is the name of the Wasm custom section holding the
// SEP-48 spec entries.
const SpecCustomSection = "contractspecv0"

// wasmMagic is the 4-byte header every Wasm module starts with ("\0asm").
var wasmMagic = []byte{0x00, 0x61, 0x73, 0x6d}

// sectionIDCustom is the Wasm section id for a custom section.
const sectionIDCustom = 0

// Spec is the parsed interface of a contract.
type Spec struct {
	// Functions is every exported function, in spec order. It is never nil,
	// so it marshals as [] rather than null.
	Functions []Function `json:"functions"`
}

// Function is one callable contract function.
type Function struct {
	// Name is the exported function name (e.g. "transfer").
	Name string `json:"name"`
	// Doc is the doc comment attached to the function, if any.
	Doc string `json:"doc,omitempty"`
	// Inputs are the positional arguments, in call order.
	Inputs []Input `json:"inputs"`
	// Outputs are the return values. Most functions return exactly one.
	Outputs []Type `json:"outputs"`
}

// Input is a single function argument.
type Input struct {
	Name string `json:"name"`
	Doc  string `json:"doc,omitempty"`
	Type Type   `json:"type"`
}

// Type is a node in the argument/return type tree.
//
// Only the fields relevant to Kind are populated: Elem for vec/option, Key and
// Value for map, Ok and Err for result, Tuple for tuple, N for bytes_n, and
// Name for udt.
type Type struct {
	Kind  string `json:"kind"`
	Name  string `json:"name,omitempty"`
	Elem  *Type  `json:"elem,omitempty"`
	Key   *Type  `json:"key,omitempty"`
	Value *Type  `json:"value,omitempty"`
	Ok    *Type  `json:"ok,omitempty"`
	Err   *Type  `json:"err,omitempty"`
	Tuple []Type `json:"tuple,omitempty"`
	N     uint32 `json:"n,omitempty"`
}

// ParseWasm extracts and parses the spec from a Wasm module. It returns an
// error when the module is not valid Wasm, carries no "contractspecv0"
// section, or contains an entry that cannot be decoded.
func ParseWasm(wasm []byte) (*Spec, error) {
	payload, ok := customSection(wasm, SpecCustomSection)
	if !ok {
		return nil, fmt.Errorf("contractspec: no %q custom section", SpecCustomSection)
	}
	return ParseSection(payload)
}

// ParseSection decodes a raw "contractspecv0" section payload: a concatenated
// sequence of SCSpecEntry values. Entries other than functions are decoded and
// skipped so that the reader stays aligned.
func ParseSection(payload []byte) (*Spec, error) {
	spec := &Spec{Functions: []Function{}}
	r := bytes.NewReader(payload)

	for r.Len() > 0 {
		var entry xdr.ScSpecEntry
		if _, err := xdr.Unmarshal(r, &entry); err != nil {
			return nil, fmt.Errorf("contractspec: decode entry: %w", err)
		}
		if entry.Kind != xdr.ScSpecEntryKindScSpecEntryFunctionV0 {
			continue
		}
		if entry.FunctionV0 == nil {
			return nil, fmt.Errorf("contractspec: function entry %d has no body", entry.Kind)
		}
		spec.Functions = append(spec.Functions, functionFromXDR(entry.FunctionV0))
	}

	return spec, nil
}

// functionFromXDR projects an SCSpecFunctionV0 into the JSON model.
func functionFromXDR(f *xdr.ScSpecFunctionV0) Function {
	fn := Function{
		Name:    string(f.Name),
		Doc:     f.Doc,
		Inputs:  make([]Input, 0, len(f.Inputs)),
		Outputs: make([]Type, 0, len(f.Outputs)),
	}
	for _, in := range f.Inputs {
		fn.Inputs = append(fn.Inputs, Input{
			Name: in.Name,
			Doc:  in.Doc,
			Type: typeFromXDR(in.Type),
		})
	}
	for _, out := range f.Outputs {
		fn.Outputs = append(fn.Outputs, typeFromXDR(out))
	}
	return fn
}

// typeFromXDR converts an SCSpecTypeDef union into the recursive Type tree.
func typeFromXDR(t xdr.ScSpecTypeDef) Type {
	switch t.Type {
	case xdr.ScSpecTypeScSpecTypeVal:
		return Type{Kind: "val"}
	case xdr.ScSpecTypeScSpecTypeBool:
		return Type{Kind: "bool"}
	case xdr.ScSpecTypeScSpecTypeVoid:
		return Type{Kind: "void"}
	case xdr.ScSpecTypeScSpecTypeError:
		return Type{Kind: "error"}
	case xdr.ScSpecTypeScSpecTypeU32:
		return Type{Kind: "u32"}
	case xdr.ScSpecTypeScSpecTypeI32:
		return Type{Kind: "i32"}
	case xdr.ScSpecTypeScSpecTypeU64:
		return Type{Kind: "u64"}
	case xdr.ScSpecTypeScSpecTypeI64:
		return Type{Kind: "i64"}
	case xdr.ScSpecTypeScSpecTypeTimepoint:
		return Type{Kind: "timepoint"}
	case xdr.ScSpecTypeScSpecTypeDuration:
		return Type{Kind: "duration"}
	case xdr.ScSpecTypeScSpecTypeU128:
		return Type{Kind: "u128"}
	case xdr.ScSpecTypeScSpecTypeI128:
		return Type{Kind: "i128"}
	case xdr.ScSpecTypeScSpecTypeU256:
		return Type{Kind: "u256"}
	case xdr.ScSpecTypeScSpecTypeI256:
		return Type{Kind: "i256"}
	case xdr.ScSpecTypeScSpecTypeBytes:
		return Type{Kind: "bytes"}
	case xdr.ScSpecTypeScSpecTypeString:
		return Type{Kind: "string"}
	case xdr.ScSpecTypeScSpecTypeSymbol:
		return Type{Kind: "symbol"}
	case xdr.ScSpecTypeScSpecTypeAddress:
		return Type{Kind: "address"}
	case xdr.ScSpecTypeScSpecTypeMuxedAddress:
		return Type{Kind: "muxed_address"}
	case xdr.ScSpecTypeScSpecTypeOption:
		if t.Option == nil {
			return Type{Kind: "option"}
		}
		elem := typeFromXDR(t.Option.ValueType)
		return Type{Kind: "option", Elem: &elem}
	case xdr.ScSpecTypeScSpecTypeResult:
		if t.Result == nil {
			return Type{Kind: "result"}
		}
		ok := typeFromXDR(t.Result.OkType)
		errType := typeFromXDR(t.Result.ErrorType)
		return Type{Kind: "result", Ok: &ok, Err: &errType}
	case xdr.ScSpecTypeScSpecTypeVec:
		if t.Vec == nil {
			return Type{Kind: "vec"}
		}
		elem := typeFromXDR(t.Vec.ElementType)
		return Type{Kind: "vec", Elem: &elem}
	case xdr.ScSpecTypeScSpecTypeMap:
		if t.Map == nil {
			return Type{Kind: "map"}
		}
		key := typeFromXDR(t.Map.KeyType)
		value := typeFromXDR(t.Map.ValueType)
		return Type{Kind: "map", Key: &key, Value: &value}
	case xdr.ScSpecTypeScSpecTypeTuple:
		if t.Tuple == nil {
			return Type{Kind: "tuple"}
		}
		members := make([]Type, 0, len(t.Tuple.ValueTypes))
		for _, vt := range t.Tuple.ValueTypes {
			members = append(members, typeFromXDR(vt))
		}
		return Type{Kind: "tuple", Tuple: members}
	case xdr.ScSpecTypeScSpecTypeBytesN:
		if t.BytesN == nil {
			return Type{Kind: "bytes_n"}
		}
		return Type{Kind: "bytes_n", N: uint32(t.BytesN.N)}
	case xdr.ScSpecTypeScSpecTypeUdt:
		if t.Udt == nil {
			return Type{Kind: "udt"}
		}
		return Type{Kind: "udt", Name: t.Udt.Name}
	default:
		// A type added by a newer protocol version. Preserve the raw
		// discriminant instead of failing the whole parse.
		return Type{Kind: fmt.Sprintf("unknown(%d)", int32(t.Type))}
	}
}

// customSection returns the payload of the named Wasm custom section, with the
// section's internal name field already stripped.
func customSection(wasm []byte, name string) ([]byte, bool) {
	if len(wasm) < 8 || !bytes.Equal(wasm[0:4], wasmMagic) {
		return nil, false
	}

	// Skip the magic and the 4-byte version.
	pos := 8
	for pos < len(wasm) {
		sectionID := wasm[pos]
		pos++

		size, n := binary.Uvarint(wasm[pos:])
		if n <= 0 {
			return nil, false
		}
		pos += n

		end := pos + int(size)
		if size > uint64(len(wasm)) || end > len(wasm) {
			return nil, false
		}
		body := wasm[pos:end]
		pos = end

		if sectionID != sectionIDCustom {
			continue
		}

		// A custom section body starts with a length-prefixed name.
		nameLen, n := binary.Uvarint(body)
		if n <= 0 {
			continue
		}
		nameEnd := n + int(nameLen)
		if nameLen > uint64(len(body)) || nameEnd > len(body) {
			continue
		}
		if string(body[n:nameEnd]) == name {
			return body[nameEnd:], true
		}
	}

	return nil, false
}
