package simulator

import (
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidXDR is returned when a transaction envelope cannot be decoded.
var ErrInvalidXDR = errors.New("invalid transaction xdr")

// ContractIDPayloadLen is the length of the raw hash behind a Soroban
// contract address (StrKey "C...").
const ContractIDPayloadLen = 32

// strKeyContractVersion is the StrKey version byte for a contract address.
// Stellar encodes the address type in the high bits; type CONTRACT is 1 and
// the version byte is 1 << 3 << 3 = 6 << 3 = 0x30.
const strKeyContractVersion = 6 << 3

var contractBase32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// DecodeTransactionXDR decodes a base64-encoded transaction envelope. It
// accepts both padded and unpadded base64 so operators can paste either form.
func DecodeTransactionXDR(xdr string) ([]byte, error) {
	trimmed := strings.TrimSpace(xdr)
	if trimmed == "" {
		return nil, fmt.Errorf("%w: empty transaction envelope", ErrInvalidXDR)
	}
	raw, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(trimmed)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidXDR, err)
	}
	if len(raw) < 8 {
		return nil, fmt.Errorf("%w: envelope is only %d bytes", ErrInvalidXDR, len(raw))
	}
	return raw, nil
}

// ExtractContractCandidates scans a decoded transaction envelope for SCAddress
// CONTRACT values and returns them as StrKey contract ids, in the order they
// appear and without duplicates.
//
// A contract address is encoded as a 4-byte SC_ADDRESS_TYPE_CONTRACT enum (1)
// followed by a 32-byte hash, so the scan looks for that shape. It is a
// best-effort extraction: the caller intersects the candidates with the set of
// contracts it has indexed, which removes the (astronomically unlikely)
// coincidental matches.
func ExtractContractCandidates(raw []byte) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)
	for i := 0; i+4+ContractIDPayloadLen <= len(raw); i++ {
		if binary.BigEndian.Uint32(raw[i:i+4]) != scAddressTypeContract {
			continue
		}
		id, err := EncodeContractID(raw[i+4 : i+4+ContractIDPayloadLen])
		if err != nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// scAddressTypeContract is the SCAddress union discriminant for a contract id.
const scAddressTypeContract = 1

// EncodeContractID encodes the raw 32-byte payload of a contract address as its
// StrKey representation (a 56-character "C..." string).
func EncodeContractID(payload []byte) (string, error) {
	if len(payload) != ContractIDPayloadLen {
		return "", fmt.Errorf("contract payload must be %d bytes, got %d", ContractIDPayloadLen, len(payload))
	}
	versioned := make([]byte, 0, 1+ContractIDPayloadLen+2)
	versioned = append(versioned, strKeyContractVersion)
	versioned = append(versioned, payload...)
	sum := crc16XModem(versioned)
	versioned = append(versioned, byte(sum&0xff), byte(sum>>8))
	return contractBase32.EncodeToString(versioned), nil
}

// crc16XModem computes the CRC16-XModem checksum used by StrKey.
func crc16XModem(data []byte) uint16 {
	var crc uint16
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}
