package main

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math/big"
)

// scVal is a decoded Soroban XDR ScVal.
type scVal struct {
	Type  string
	Value any
	Human string
}

const (
	scvBool   = 0
	scvVoid   = 1
	scvU32    = 3
	scvI32    = 4
	scvU64    = 5
	scvI64    = 6
	scvU128   = 9
	scvI128   = 10
	scvBytes  = 13
	scvString = 14
	scvSymbol = 15
)

var scvTypeNames = map[uint32]string{
	scvBool: "scvBool", scvVoid: "scvVoid",
	2: "scvError", scvU32: "scvU32", scvI32: "scvI32",
	scvU64: "scvU64", scvI64: "scvI64",
	7: "scvTimePoint", 8: "scvDuration",
	scvU128: "scvU128", scvI128: "scvI128",
	11: "scvU256", 12: "scvI256",
	scvBytes: "scvBytes", scvString: "scvString", scvSymbol: "scvSymbol",
	16: "scvVec", 17: "scvMap", 18: "scvAddress",
	19: "scvContractInstance", 20: "scvLedgerKeyContractInstance", 21: "scvLedgerKeyNonce",
}

// decodeScVal decodes a base64-encoded XDR ScVal.
func decodeScVal(b64 string) (scVal, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return scVal{}, fmt.Errorf("scval: base64 decode: %w", err)
	}
	if len(raw) < 4 {
		return scVal{}, fmt.Errorf("scval: too short (%d bytes)", len(raw))
	}
	disc := binary.BigEndian.Uint32(raw[:4])
	typeName, known := scvTypeNames[disc]
	if !known {
		typeName = fmt.Sprintf("scvUnknown(%d)", disc)
	}
	body := raw[4:]
	switch disc {
	case scvVoid:
		return scVal{Type: typeName, Value: nil, Human: "void"}, nil
	case scvBool:
		if len(body) < 4 {
			break
		}
		v := binary.BigEndian.Uint32(body[:4]) != 0
		return scVal{Type: typeName, Value: v, Human: fmt.Sprintf("%v", v)}, nil
	case scvU32:
		if len(body) < 4 {
			break
		}
		v := binary.BigEndian.Uint32(body[:4])
		return scVal{Type: typeName, Value: v, Human: fmt.Sprintf("%d", v)}, nil
	case scvI32:
		if len(body) < 4 {
			break
		}
		v := int32(binary.BigEndian.Uint32(body[:4]))
		return scVal{Type: typeName, Value: v, Human: fmt.Sprintf("%d", v)}, nil
	case scvU64, 7, 8: // scvU64, scvTimePoint, scvDuration
		if len(body) < 8 {
			break
		}
		v := binary.BigEndian.Uint64(body[:8])
		return scVal{Type: typeName, Value: v, Human: fmt.Sprintf("%d", v)}, nil
	case scvI64:
		if len(body) < 8 {
			break
		}
		v := int64(binary.BigEndian.Uint64(body[:8]))
		return scVal{Type: typeName, Value: v, Human: fmt.Sprintf("%d", v)}, nil
	case scvU128:
		if len(body) < 16 {
			break
		}
		hi := new(big.Int).SetUint64(binary.BigEndian.Uint64(body[:8]))
		lo := new(big.Int).SetUint64(binary.BigEndian.Uint64(body[8:16]))
		v := new(big.Int).Or(new(big.Int).Lsh(hi, 64), lo)
		return scVal{Type: typeName, Value: v, Human: v.String()}, nil
	case scvI128:
		if len(body) < 16 {
			break
		}
		hi := int64(binary.BigEndian.Uint64(body[:8]))
		lo := binary.BigEndian.Uint64(body[8:16])
		v := new(big.Int).SetInt64(hi)
		v.Lsh(v, 64)
		v.Or(v, new(big.Int).SetUint64(lo))
		return scVal{Type: typeName, Value: v, Human: v.String()}, nil
	case scvString, scvSymbol:
		if len(body) < 4 {
			break
		}
		length := binary.BigEndian.Uint32(body[:4])
		if uint32(len(body)) < 4+length {
			break
		}
		s := string(body[4 : 4+length])
		return scVal{Type: typeName, Value: s, Human: s}, nil
	case scvBytes:
		if len(body) < 4 {
			break
		}
		length := binary.BigEndian.Uint32(body[:4])
		if uint32(len(body)) < 4+length {
			break
		}
		v := body[4 : 4+length]
		return scVal{Type: typeName, Value: v, Human: fmt.Sprintf("%x", v)}, nil
	}
	return scVal{Type: typeName, Value: b64, Human: typeName + ":" + b64}, nil
}
