package rulelang

import "strings"

// strKeyAlphabet is the RFC 4648 base32 alphabet used by Stellar StrKey.
const strKeyAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"

// contractVersionByte is the StrKey version byte for a Soroban contract.
const contractVersionByte = 0x10

// ValidContractID reports whether s is a syntactically valid Soroban contract
// StrKey (a 56-character C-address with a correct CRC16-XModem checksum).
func ValidContractID(s string) bool {
	if len(s) != 56 || s[0] != 'C' {
		return false
	}
	// Decode base32 (no padding).
	var bits uint64
	var nbits uint
	decoded := make([]byte, 0, 35)
	for i := 0; i < len(s); i++ {
		idx := strings.IndexByte(strKeyAlphabet, s[i])
		if idx < 0 {
			return false
		}
		bits = bits<<5 | uint64(idx)
		nbits += 5
		for nbits >= 8 {
			nbits -= 8
			decoded = append(decoded, byte(bits>>nbits))
		}
		// Drop the bits already emitted so the accumulator cannot overflow.
		if nbits == 0 {
			bits = 0
		} else {
			bits &= 1<<nbits - 1
		}
	}
	if len(decoded) != 35 {
		return false
	}
	if decoded[0] != contractVersionByte {
		return false
	}
	payload := decoded[:33]
	want := uint16(decoded[33]) | uint16(decoded[34])<<8
	return crc16XModem(payload) == want
}

// crc16XModem computes the CRC16-XModem checksum used by StrKey.
func crc16XModem(data []byte) uint16 {
	var crc uint16
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}
