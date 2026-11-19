// Package cursor holds the opaque pagination cursor encoding shared by the
// list endpoints. It is deliberately small, pure and dependency-free so it
// can be mutation-tested on a schedule (issue #849): cursor parsing is
// security-adjacent because every list endpoint trusts its output as a
// keyset component, so a silent mis-parse means wrong or leaked rows.
package cursor

import (
	"encoding/base64"
	"strconv"
	"strings"
)

// Encode wraps a raw keyset value in standard base64 so it can travel in a
// query string without delimiter ambiguity. An empty value stays empty.
func Encode(raw string) string {
	if raw == "" {
		return ""
	}
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

// Decode reverses Encode. The boolean reports whether the value is usable:
// an empty cursor decodes to "" (start of the list) and a malformed one is
// rejected rather than silently reinterpreted.
func Decode(encoded string) (string, bool) {
	if encoded == "" {
		return "", true
	}
	b, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", false
	}
	return string(b), true
}

// ParseInvocation decodes the raw (already base64-decoded) global
// invocation cursor "<ledger>:<tx_hash>" into its keyset components. An
// empty cursor means "start at the newest invocation".
func ParseInvocation(raw string) (uint32, string, bool) {
	if raw == "" {
		return 0, "", true
	}
	ledgerStr, txHash, found := strings.Cut(raw, ":")
	if !found || txHash == "" {
		return 0, "", false
	}
	n, err := strconv.ParseUint(ledgerStr, 10, 32)
	if err != nil || n == 0 {
		return 0, "", false
	}
	return uint32(n), txHash, true
}
