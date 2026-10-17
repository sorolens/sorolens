// Command webhooks verifies a Sorolens webhook delivery with the published
// HMAC-SHA256 scheme (docs/webhooks.md). It runs entirely offline: it proves the
// pinned test vector, then shows a tampered body and a stale timestamp being
// rejected. Pin your own receiver against the same vector.
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sorolens/go-client/examples/internal/harness"
)

const tolerance = 5 * time.Minute

var (
	errMalformedHeader = errors.New("malformed X-Sorolens-Signature header")
	errNoSignature     = errors.New("no v1 signature present")
	errTimestampDrift  = errors.New("timestamp outside the replay window")
	errSignature       = errors.New("signature mismatch")
)

// verify checks a delivery against the subscription's signing secret. It hashes
// the raw body, compares with hmac.Equal (constant time), and enforces the
// replay window. Verify before parsing the JSON.
func verify(secret, signatureHeader, timestampHeader string, body []byte, now time.Time) error {
	timestamp, signatures, err := parseSignatureHeader(signatureHeader)
	if err != nil {
		return err
	}
	if len(signatures) == 0 {
		return errNoSignature
	}

	headerTimestamp, err := strconv.ParseInt(strings.TrimSpace(timestampHeader), 10, 64)
	if err != nil || headerTimestamp != timestamp {
		return errMalformedHeader
	}
	if drift := now.Unix() - timestamp; drift > int64(tolerance.Seconds()) || drift < -int64(tolerance.Seconds()) {
		return errTimestampDrift
	}

	expected := sign(secret, timestamp, body)
	for _, signature := range signatures {
		if hmac.Equal([]byte(signature), []byte(expected)) {
			return nil
		}
	}
	return errSignature
}

// sign returns hex(HMAC-SHA256(secret, "<timestamp>.<body>")).
func sign(secret string, timestamp int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(timestamp, 10)))
	mac.Write([]byte("."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// parseSignatureHeader reads "t=<unix>[,v1=<hex>...]" in any order.
func parseSignatureHeader(header string) (int64, []string, error) {
	var (
		timestamp  int64
		seen       bool
		signatures []string
	)
	for _, part := range strings.Split(header, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			return 0, nil, errMalformedHeader
		}
		switch strings.TrimSpace(key) {
		case "t":
			ts, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
			if err != nil {
				return 0, nil, errMalformedHeader
			}
			timestamp, seen = ts, true
		case "v1":
			if v := strings.TrimSpace(value); v != "" {
				signatures = append(signatures, v)
			}
		}
	}
	if !seen {
		return 0, nil, errMalformedHeader
	}
	return timestamp, signatures, nil
}

func run(out io.Writer) error {
	signedAt := time.Unix(1700000000, 0)
	header := "t=" + harness.WebhookTimestamp + ",v1=" + harness.WebhookSignature
	body := []byte(harness.WebhookBody)

	if err := verify(harness.WebhookSecret, header, harness.WebhookTimestamp, body, signedAt); err != nil {
		return fmt.Errorf("pinned vector did not verify: %w", err)
	}
	fmt.Fprintln(out, "pinned test vector verified")

	tampered := []byte(`{"event":"watchdog.alert","severity":"Critical","extra":true}`)
	if err := verify(harness.WebhookSecret, header, harness.WebhookTimestamp, tampered, signedAt); !errors.Is(err, errSignature) {
		return fmt.Errorf("tampered body was not rejected (err=%v)", err)
	}
	fmt.Fprintln(out, "tampered body rejected")

	if err := verify(harness.WebhookSecret, header, harness.WebhookTimestamp, body, signedAt.Add(6*time.Minute)); !errors.Is(err, errTimestampDrift) {
		return fmt.Errorf("stale delivery was not rejected (err=%v)", err)
	}
	fmt.Fprintln(out, "stale delivery rejected")

	return nil
}

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "webhooks:", err)
		os.Exit(1)
	}
}
