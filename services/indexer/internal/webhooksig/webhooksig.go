// Package webhooksig implements the HMAC-SHA256 scheme Sorolens uses to sign
// outgoing webhook deliveries, so a receiver can prove a payload came from
// Sorolens and was not modified in transit.
//
// # Wire format
//
// Every delivery carries two headers:
//
//	X-Sorolens-Timestamp: <unix seconds>
//	X-Sorolens-Signature: t=<unix seconds>,v1=<hex HMAC-SHA256>
//
// The signed payload is the timestamp, a "." separator and the exact request
// body:
//
//	signed_payload = "<timestamp>." + raw_body
//	signature      = hex(HMAC_SHA256(signing_secret, signed_payload))
//
// The signing secret is the full "whsec_..." string the subscriber was given,
// used as raw key bytes. The same construction is published in docs/webhooks.md
// so receivers can reimplement it in any language; this package is the
// executable reference.
//
// # Replay protection
//
// Because the timestamp is inside the MAC it cannot be altered, and Verify (and
// the published algorithm) rejects anything outside a +/- five minute window.
// See DefaultTolerance.
package webhooksig

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	// SecretPrefix marks a Sorolens webhook signing secret.
	SecretPrefix = "whsec_"
	// SignatureScheme is the current signature version advertised in the
	// signature header and covered by the MAC scheme.
	SignatureScheme = "v1"
	// SignatureHeader and TimestampHeader are the headers a delivery sets.
	SignatureHeader = "X-Sorolens-Signature"
	TimestampHeader = "X-Sorolens-Timestamp"
	// DefaultTolerance is the replay window: a signature whose timestamp is
	// further than this from the receiver's clock is rejected.
	DefaultTolerance = 5 * time.Minute

	// secretBytes is the entropy of a signing secret (256 bits).
	secretBytes = 32
)

// Verification errors. Callers can use errors.Is to distinguish a stale
// delivery (retryable) from a forged one (do not process).
var (
	// ErrMalformedHeader means the signature or timestamp header was missing
	// or not parseable.
	ErrMalformedHeader = errors.New("webhooksig: malformed signature header")
	// ErrTimestampOutsideWindow means the timestamp is outside the replay
	// window (too old, or too far in the future).
	ErrTimestampOutsideWindow = errors.New("webhooksig: timestamp outside the replay window")
	// ErrNoSignature means the header carried no v1 signature.
	ErrNoSignature = errors.New("webhooksig: no v1 signature present")
	// ErrSignatureMismatch means no v1 signature matched the expected MAC.
	ErrSignatureMismatch = errors.New("webhooksig: signature mismatch")
)

// GenerateSecret returns a new random signing secret: SecretPrefix followed by
// the base64 (unpadded, URL-safe) encoding of 32 random bytes.
func GenerateSecret() (string, error) {
	var b [secretBytes]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("webhooksig: generate secret: %w", err)
	}
	return SecretPrefix + base64.RawURLEncoding.EncodeToString(b[:]), nil
}

// HashSecret returns the SHA-256 hex digest of a signing secret. It is what the
// API persists alongside the secret so the stored value can be audited without
// exposing the key material.
func HashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// SignedPayload returns the exact byte string covered by a signature for the
// given timestamp and body: "<timestamp>.<body>".
func SignedPayload(timestamp int64, payload []byte) []byte {
	out := make([]byte, 0, 20+len(payload))
	out = strconv.AppendInt(out, timestamp, 10)
	out = append(out, '.')
	out = append(out, payload...)
	return out
}

// Sign returns the hex HMAC-SHA256 of the signed payload for a secret.
func Sign(secret string, timestamp int64, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(SignedPayload(timestamp, payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// SignatureHeaderValue renders the X-Sorolens-Signature value for a timestamp
// and signature: "t=<timestamp>,v1=<signature>".
func SignatureHeaderValue(timestamp int64, signature string) string {
	return fmt.Sprintf("t=%d,%s=%s", timestamp, SignatureScheme, signature)
}

// ParseSignatureHeader parses an X-Sorolens-Signature value of the form
// "t=<unix seconds>,v1=<hex>[,v1=<hex>...]". Order is not significant and
// multiple v1 signatures are collected so a receiver can accept either of two
// secrets during rotation.
func ParseSignatureHeader(header string) (int64, []string, error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return 0, nil, ErrMalformedHeader
	}

	var (
		timestamp int64
		haveT     bool
		sigs      []string
	)
	for _, part := range strings.Split(header, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			return 0, nil, ErrMalformedHeader
		}
		key, value := strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1])
		switch key {
		case "t":
			ts, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return 0, nil, ErrMalformedHeader
			}
			timestamp = ts
			haveT = true
		case SignatureScheme:
			if value != "" {
				sigs = append(sigs, value)
			}
		}
	}
	if !haveT {
		return 0, nil, ErrMalformedHeader
	}
	return timestamp, sigs, nil
}

// Verify checks a delivery against a signing secret.
//
// It requires both the signature and the standalone timestamp header, checks
// that the two agree, enforces the replay window, and compares the expected MAC
// against every v1 signature with a constant-time comparison.
//
// A tolerance <= 0 falls back to DefaultTolerance.
func Verify(secret, signatureHeader, timestampHeader string, payload []byte, now time.Time, tolerance time.Duration) error {
	if tolerance <= 0 {
		tolerance = DefaultTolerance
	}

	timestamp, sigs, err := ParseSignatureHeader(signatureHeader)
	if err != nil {
		return err
	}
	if len(sigs) == 0 {
		return ErrNoSignature
	}

	// The standalone timestamp header must be present and must agree with the
	// timestamp that was actually signed.
	headerTS, err := strconv.ParseInt(strings.TrimSpace(timestampHeader), 10, 64)
	if err != nil || headerTS != timestamp {
		return ErrMalformedHeader
	}

	if now.Unix()-timestamp > int64(tolerance.Seconds()) ||
		timestamp-now.Unix() > int64(tolerance.Seconds()) {
		return ErrTimestampOutsideWindow
	}

	expected := Sign(secret, timestamp, payload)
	for _, sig := range sigs {
		if hmac.Equal([]byte(sig), []byte(expected)) {
			return nil
		}
	}
	return ErrSignatureMismatch
}
