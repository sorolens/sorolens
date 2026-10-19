package scval

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
	scvVec    = 16
	scvMap    = 17
	scvAddress = 18
)

var scvTypeNames = map[uint32]string{
	scvBool: "scvBool", scvVoid: "scvVoid",
	2: "scvError", scvU32: "scvU32", scvI32: "scvI32",
	scvU64: "scvU64", scvI64: "scvI64",
	7: "scvTimePoint", 8: "scvDuration",
	scvU128: "scvU128", scvI128: "scvI128",
	11: "scvU256", 12: "scvI256",
	scvBytes: "scvBytes", scvString: "scvString", scvSymbol: "scvSymbol",
	scvVec: "scvVec", scvMap: "scvMap", scvAddress: "scvAddress",
	19: "scvContractInstance", 20: "scvLedgerKeyContractInstance", 21: "scvLedgerKeyNonce",
}

// Decode decodes a base64-encoded XDR ScVal.
func Decode(b64 string) (scVal, error) {
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
		return scVal{Type: typeName, Value: v.String(), Human: v.String()}, nil
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
	case scvVec:
		if len(body) < 4 {
			break
		}
		vecLength := binary.BigEndian.Uint32(body[:4])
		var vec []any
		offset := uint32(4)
		for i := uint32(0); i < vecLength; i++ {
			if offset+4 > uint32(len(body)) {
				break
			}
			elemLength := binary.BigEndian.Uint32(body[offset : offset+4])
			offset += 4
			if offset+elemLength > uint32(len(body)) {
				break
			}
			elemB64 := base64.StdEncoding.EncodeToString(body[offset : offset+elemLength])
			elem, err := Decode(elemB64)
			if err != nil {
				vec = append(vec, elemB64)
			} else {
				vec = append(vec, elem.Value)
			}
			offset += elemLength
		}
		return scVal{Type: typeName, Value: vec, Human: fmt.Sprintf("vec[%d]", len(vec))}, nil
	case scvMap:
		if len(body) < 4 {
			break
		}
		mapLength := binary.BigEndian.Uint32(body[:4])
		result := make(map[string]any)
		offset := uint32(4)
		for i := uint32(0); i < mapLength; i++ {
			if offset+4 > uint32(len(body)) {
				break
			}
			keyLength := binary.BigEndian.Uint32(body[offset : offset+4])
			offset += 4
			if offset+keyLength > uint32(len(body)) {
				break
			}
			keyB64 := base64.StdEncoding.EncodeToString(body[offset : offset+keyLength])
			offset += keyLength
			if offset+4 > uint32(len(body)) {
				break
			}
			valLength := binary.BigEndian.Uint32(body[offset : offset+4])
			offset += 4
			if offset+valLength > uint32(len(body)) {
				break
			}
			valB64 := base64.StdEncoding.EncodeToString(body[offset : offset+valLength])
			offset += valLength
			key, err := Decode(keyB64)
			if err != nil {
				continue
			}
			val, err := Decode(valB64)
			if err != nil {
				continue
			}
			keyStr := fmt.Sprintf("%v", key.Value)
			result[keyStr] = val.Value
		}
		return scVal{Type: typeName, Value: result, Human: fmt.Sprintf("map[%d]", len(result))}, nil
	case scvAddress:
		if len(body) < 32 {
			break
		}
		addr := body[:32]
		return scVal{Type: typeName, Value: addr, Human: fmt.Sprintf("%x", addr)}, nil
	}
	return scVal{Type: typeName, Value: b64, Human: typeName + ":" + b64}, nil
}
