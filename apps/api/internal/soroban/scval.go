package soroban

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math/big"
)

// ScVal is a decoded Soroban XDR ScVal.
//
// v0.1 implementation note: full XDR decoding of nested types (vec, map,
// address, contract instance) requires either the stellar/go SDK or a
// hand-written XDR codec, both of which are out of scope for this session.
// For those types, Value holds the raw base64 string and Human is
// "<type>:<base64>". Primitive types (bool, void, u32, i32, u64, i64,
// u128, i128, string, symbol, bytes) are fully decoded. Upgrade this
// decoder in the xdr package session when @stellar/stellar-sdk XDR types
// are available.
type ScVal struct {
	// Type is the XDR ScValType name, e.g. "scvU32", "scvSymbol".
	Type string
	// Value holds the decoded Go value for primitive types, or the raw
	// base64 string for complex types.
	Value any
	// Human is a display-friendly string representation.
	Human string
}

// XDR ScValType discriminants (Protocol 20+).
const (
	scvBool                      = 0
	scvVoid                      = 1
	scvError                     = 2
	scvU32                       = 3
	scvI32                       = 4
	scvU64                       = 5
	scvI64                       = 6
	scvTimePoint                 = 7
	scvDuration                  = 8
	scvU128                      = 9
	scvI128                      = 10
	scvU256                      = 11
	scvI256                      = 12
	scvBytes                     = 13
	scvString                    = 14
	scvSymbol                    = 15
	scvVec                       = 16
	scvMap                       = 17
	scvAddress                   = 18
	scvContractInstance          = 19
	scvLedgerKeyContractInstance = 20
	scvLedgerKeyNonce            = 21
)

var scvTypeNames = map[uint32]string{
	scvBool:                      "scvBool",
	scvVoid:                      "scvVoid",
	scvError:                     "scvError",
	scvU32:                       "scvU32",
	scvI32:                       "scvI32",
	scvU64:                       "scvU64",
	scvI64:                       "scvI64",
	scvTimePoint:                 "scvTimePoint",
	scvDuration:                  "scvDuration",
	scvU128:                      "scvU128",
	scvI128:                      "scvI128",
	scvU256:                      "scvU256",
	scvI256:                      "scvI256",
	scvBytes:                     "scvBytes",
	scvString:                    "scvString",
	scvSymbol:                    "scvSymbol",
	scvVec:                       "scvVec",
	scvMap:                       "scvMap",
	scvAddress:                   "scvAddress",
	scvContractInstance:          "scvContractInstance",
	scvLedgerKeyContractInstance: "scvLedgerKeyContractInstance",
	scvLedgerKeyNonce:            "scvLedgerKeyNonce",
}

// DecodeScVal decodes a base64-encoded XDR ScVal into a ScVal struct.
// Returns an error only for undecodable input (not base64, too short).
// Unknown types are returned with their raw base64 value rather than an error.
func DecodeScVal(b64 string) (ScVal, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return ScVal{}, fmt.Errorf("scval: base64 decode: %w", err)
	}
	if len(raw) < 4 {
		return ScVal{}, fmt.Errorf("scval: too short (%d bytes)", len(raw))
	}

	disc := binary.BigEndian.Uint32(raw[:4])
	typeName, known := scvTypeNames[disc]
	if !known {
		typeName = fmt.Sprintf("scvUnknown(%d)", disc)
	}

	body := raw[4:]

	switch disc {
	case scvVoid:
		return ScVal{Type: typeName, Value: nil, Human: "void"}, nil

	case scvBool:
		if len(body) < 4 {
			break
		}
		v := binary.BigEndian.Uint32(body[:4]) != 0
		return ScVal{Type: typeName, Value: v, Human: fmt.Sprintf("%v", v)}, nil

	case scvU32:
		if len(body) < 4 {
			break
		}
		v := binary.BigEndian.Uint32(body[:4])
		return ScVal{Type: typeName, Value: v, Human: fmt.Sprintf("%d", v)}, nil

	case scvI32:
		if len(body) < 4 {
			break
		}
		v := int32(binary.BigEndian.Uint32(body[:4]))
		return ScVal{Type: typeName, Value: v, Human: fmt.Sprintf("%d", v)}, nil

	case scvU64, scvTimePoint, scvDuration:
		if len(body) < 8 {
			break
		}
		v := binary.BigEndian.Uint64(body[:8])
		return ScVal{Type: typeName, Value: v, Human: fmt.Sprintf("%d", v)}, nil

	case scvI64:
		if len(body) < 8 {
			break
		}
		v := int64(binary.BigEndian.Uint64(body[:8]))
		return ScVal{Type: typeName, Value: v, Human: fmt.Sprintf("%d", v)}, nil

	case scvU128:
		// hi uint64 + lo uint64
		if len(body) < 16 {
			break
		}
		hi := new(big.Int).SetUint64(binary.BigEndian.Uint64(body[:8]))
		lo := new(big.Int).SetUint64(binary.BigEndian.Uint64(body[8:16]))
		v := new(big.Int).Or(new(big.Int).Lsh(hi, 64), lo)
		return ScVal{Type: typeName, Value: v, Human: v.String()}, nil

	case scvI128:
		// hi int64 + lo uint64 (two's complement 128-bit)
		if len(body) < 16 {
			break
		}
		hi := int64(binary.BigEndian.Uint64(body[:8]))
		lo := binary.BigEndian.Uint64(body[8:16])
		v := new(big.Int).SetInt64(hi)
		v.Lsh(v, 64)
		v.Or(v, new(big.Int).SetUint64(lo))
		return ScVal{Type: typeName, Value: v, Human: v.String()}, nil

	case scvString, scvSymbol:
		// XDR variable-length opaque: 4-byte length, then bytes, padded to 4.
		s, err := xdrDecodeString(body)
		if err != nil {
			break
		}
		return ScVal{Type: typeName, Value: s, Human: s}, nil

	case scvBytes:
		if len(body) < 4 {
			break
		}
		length := binary.BigEndian.Uint32(body[:4])
		if uint32(len(body)) < 4+length {
			break
		}
		v := body[4 : 4+length]
		h := fmt.Sprintf("%x", v)
		return ScVal{Type: typeName, Value: v, Human: h}, nil
	}

	// For complex or undecodable types, fall back to raw base64.
	return ScVal{
		Type:  typeName,
		Value: b64,
		Human: typeName + ":" + b64,
	}, nil
}

// xdrDecodeString decodes an XDR variable-length string from body.
func xdrDecodeString(body []byte) (string, error) {
	if len(body) < 4 {
		return "", fmt.Errorf("too short for string length")
	}
	length := binary.BigEndian.Uint32(body[:4])
	if uint32(len(body)) < 4+length {
		return "", fmt.Errorf("body too short: need %d bytes, have %d", 4+length, len(body))
	}
	return string(body[4 : 4+length]), nil
}
