// Package watchdog implements the `sorolens watchdog` command tree, which
// talks to the on-chain sorolens-watchdog Soroban contract
// (contracts/watchdog). See ARCHITECTURE.md, "The watchdog vertical".
package watchdog

import (
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

// SEP-23 strkey version bytes. Each version byte is chosen so that the first
// base32 character of the encoded string identifies the key type.
const (
	versionByteContract = 2 << 3  // base32-encodes to 'C...'
	versionByteAccount  = 6 << 3  // base32-encodes to 'G...'
	versionByteSeed     = 18 << 3 // base32-encodes to 'S...'

	strkeyChecksumLen = 2  // trailing CRC16-XModem bytes
	addressPayloadLen = 32 // Ed25519 pubkey / contract hash size
	rawStrkeyLen      = 1 + addressPayloadLen + strkeyChecksumLen
	encodedStrkeyLen  = (rawStrkeyLen*8 + 4) / 5 // base32 no-padding encoded length
)

// encoding is unpadded RFC 4648 base32, as used by SEP-23 strkeys.
var encoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// errChecksum is returned when a strkey's trailing CRC16 does not match the
// checksum computed over its version byte and payload.
var errChecksum = errors.New("invalid checksum")

// decodeStrkey validates a SEP-23 strkey (correct length, base32 alphabet,
// trailing checksum) and returns its version byte and payload.
//
// It mirrors the reference implementation in stellar/go's strkey package
// without pulling in the SDK as a dependency: the watchdog commands shell out
// to the soroban CLI, so only this small amount of validation logic is needed
// locally.
func decodeStrkey(s string) (byte, []byte, error) {
	raw, err := decodeString(s)
	if err != nil {
		return 0, nil, err
	}
	if len(raw) < 1+strkeyChecksumLen {
		return 0, nil, errors.New("decoded string is too short")
	}
	version := raw[0]
	payload := raw[1 : len(raw)-strkeyChecksumLen]
	checksum := binary.LittleEndian.Uint16(raw[len(raw)-strkeyChecksumLen:])
	if crc16XModem(raw[:1+len(payload)]) != checksum {
		return 0, nil, errChecksum
	}
	return version, payload, nil
}

// decodeString base32-decodes a strkey, rejecting the empty string, the wrong
// length, and any character outside the RFC 4648 alphabet (which includes
// SEP-23's lowercase aliases).
func decodeString(s string) ([]byte, error) {
	if s == "" {
		return nil, errors.New("empty string")
	}
	if len(s) != encodedStrkeyLen {
		return nil, fmt.Errorf("invalid length: got %d characters, want %d", len(s), encodedStrkeyLen)
	}
	// Go's base32 decoder is case-sensitive; strkeys are not, so
	// canonicalise to upper case before decoding.
	raw, err := encoding.DecodeString(strings.ToUpper(s))
	if err != nil {
		return nil, fmt.Errorf("invalid base32: %w", err)
	}
	return raw, nil
}

// validateContractID checks that s is a well-formed contract address
// (56-character 'C...' strkey with a valid checksum).
func validateContractID(s string) error {
	version, payload, err := decodeStrkey(s)
	if err != nil {
		return fmt.Errorf("invalid contract id %q: %w", s, err)
	}
	if version != versionByteContract {
		return fmt.Errorf("invalid contract id %q: must be a 'C...' contract address", s)
	}
	if len(payload) != addressPayloadLen {
		return fmt.Errorf("invalid contract id %q: payload must be %d bytes, got %d", s, addressPayloadLen, len(payload))
	}
	return nil
}

// validateAccountID checks that s is a well-formed account address
// (56-character 'G...' strkey with a valid checksum).
func validateAccountID(s string) error {
	version, payload, err := decodeStrkey(s)
	if err != nil {
		return fmt.Errorf("invalid account id %q: %w", s, err)
	}
	if version != versionByteAccount {
		return fmt.Errorf("invalid account id %q: must be a 'G...' account address", s)
	}
	if len(payload) != addressPayloadLen {
		return fmt.Errorf("invalid account id %q: payload must be %d bytes, got %d", s, addressPayloadLen, len(payload))
	}
	return nil
}

// validateSeed checks that s is a well-formed secret seed ('S...' strkey).
func validateSeed(s string) error {
	version, payload, err := decodeStrkey(s)
	if err != nil {
		return fmt.Errorf("invalid secret key %q: %w", s, err)
	}
	if version != versionByteSeed {
		return fmt.Errorf("invalid secret key %q: must be an 'S...' secret seed", s)
	}
	if len(payload) != addressPayloadLen {
		return fmt.Errorf("invalid secret key %q: payload must be %d bytes, got %d", s, addressPayloadLen, len(payload))
	}
	return nil
}

// deriveAccountAddress returns the 'G...' account address that a given
// secret seed ('S...' strkey) signs for. It re-encodes the seed's payload
// with the account version byte, which is valid because both 'S' and 'G'
// strkeys carry the same 32-byte Ed25519 key material.
func deriveAccountAddress(seed string) (string, error) {
	if err := validateSeed(seed); err != nil {
		return "", err
	}
	_, payload, err := decodeStrkey(seed)
	if err != nil {
		return "", err
	}
	return encodeStrkey(versionByteAccount, payload)
}

// encodeStrkey encodes a version byte and payload into a SEP-23 strkey.
func encodeStrkey(version byte, payload []byte) (string, error) {
	if len(payload) > addressPayloadLen {
		return "", errors.New("data exceeds maximum payload size for strkey")
	}
	raw := make([]byte, 0, 1+len(payload)+strkeyChecksumLen)
	raw = append(raw, version)
	raw = append(raw, payload...)
	raw = binary.LittleEndian.AppendUint16(raw, crc16XModem(raw))
	return encoding.EncodeToString(raw), nil
}

// crc16XModem computes the CRC16-XModem (CCITT, poly 0x1021, init 0x0000)
// checksum used by SEP-23 strkeys.
func crc16XModem(data []byte) uint16 {
	crc := uint16(0)
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
