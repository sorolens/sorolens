// Package webhook verifies incoming Sorolens webhook deliveries.
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
// used as raw key bytes. The algorithm and a worked test vector are published
// in docs/webhooks.md.
//
// # Usage
//
//	body, _ := io.ReadAll(r.Body)
//	if err := webhook.Verify(secret, r.Header, body, time.Now(), 0); err != nil {
//		http.Error(w, "invalid signature", http.StatusUnauthorized)
//		return
//	}
//
// A tolerance <= 0 selects DefaultTolerance, the documented five-minute replay
// window. Verification errors can be told apart with errors.Is: a stale
// delivery (ErrTimestampOutsideWindow) may be retried, a forged one
// (ErrSignatureMismatch) must not be processed.
package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	// SignatureHeader is the header carrying the timestamp and v1 signatures.
	SignatureHeader = "X-Sorolens-Signature"
	// TimestampHeader is the standalone header carrying the signed timestamp.
	TimestampHeader = "X-Sorolens-Timestamp"
	// SignatureScheme is the current signature version.
	SignatureScheme = "v1"
	// DefaultTolerance is the replay window applied when none is supplied.
	DefaultTolerance = 5 * time.Minute
)

// Verification errors.
var (
	// ErrMalformedHeader means the signature or timestamp header was missing
	// or not parseable.
	ErrMalformedHeader = errors.New("webhook: malformed signature header")
	// ErrTimestampOutsideWindow means the timestamp is outside the replay
	// window (too old, or too far in the future).
	ErrTimestampOutsideWindow = errors.New("webhook: timestamp outside the replay window")
	// ErrNoSignature means the header carried no v1 signature.
	ErrNoSignature = errors.New("webhook: no v1 signature present")
	// ErrSignatureMismatch means no v1 signature matched the expected MAC.
	ErrSignatureMismatch = errors.New("webhook: signature mismatch")
)

// SignedPayload returns the exact byte string covered by a signature for the
// given timestamp and body: "<timestamp>.<body>".
func SignedPayload(timestamp int64, body []byte) []byte {
	out := make([]byte, 0, 20+len(body))
	out = strconv.AppendInt(out, timestamp, 10)
	out = append(out, '.')
	out = append(out, body...)
	return out
}

// Sign returns the hex HMAC-SHA256 of the signed payload for a secret.
func Sign(secret string, timestamp int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(SignedPayload(timestamp, body))
	return hex.EncodeToString(mac.Sum(nil))
}

// SignatureHeaderValue renders the X-Sorolens-Signature value for a timestamp
// and signature: "t=<timestamp>,v1=<signature>".
func SignatureHeaderValue(timestamp int64, signature string) string {
	return fmt.Sprintf("t=%d,%s=%s", timestamp, SignatureScheme, signature)
}

// Verify checks a delivery against a signing secret using the request headers.
// It is equivalent to VerifyHeaders with the two Sorolens headers.
func Verify(secret string, headers http.Header, body []byte, now time.Time, tolerance time.Duration) error {
	return VerifyHeaders(secret, headers.Get(SignatureHeader), headers.Get(TimestampHeader), body, now, tolerance)
}

// VerifyHeaders checks a delivery against a signing secret.
//
// It requires both the signature and the standalone timestamp header, checks
// that they agree, enforces the replay window, and compares the expected MAC
// against every v1 signature in constant time.
//
// A tolerance <= 0 falls back to DefaultTolerance.
func VerifyHeaders(secret, signatureHeader, timestampHeader string, body []byte, now time.Time, tolerance time.Duration) error {
	if tolerance <= 0 {
		tolerance = DefaultTolerance
	}

	timestamp, signatures, err := ParseSignatureHeader(signatureHeader)
	if err != nil {
		return err
	}
	if len(signatures) == 0 {
		return ErrNoSignature
	}

	headerTimestamp, err := strconv.ParseInt(strings.TrimSpace(timestampHeader), 10, 64)
	if err != nil || headerTimestamp != timestamp {
		return ErrMalformedHeader
	}

	if now.Unix()-timestamp > int64(tolerance.Seconds()) ||
		timestamp-now.Unix() > int64(tolerance.Seconds()) {
		return ErrTimestampOutsideWindow
	}

	expected := Sign(secret, timestamp, body)
	for _, signature := range signatures {
		if hmac.Equal([]byte(signature), []byte(expected)) {
			return nil
		}
	}
	return ErrSignatureMismatch
}

// ParseSignatureHeader parses an X-Sorolens-Signature value of the form
// "t=<unix seconds>,v1=<hex>[,v1=<hex>...]". Order is not significant and
// multiple v1 signatures are collected, so a receiver can accept either of two
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
