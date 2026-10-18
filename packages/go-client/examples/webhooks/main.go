// Verify the HMAC-SHA256 signature on incoming Sorolens webhook deliveries.
//
// Wire format (see docs/webhooks.md):
//
//	X-Sorolens-Timestamp: <unix seconds>
//	X-Sorolens-Signature: t=<unix seconds>,v1=<hex HMAC-SHA256>
//
//	signed_payload = "<timestamp>." + raw_body
//	signature      = hex(HMAC_SHA256(signing_secret, signed_payload))
//
// The signing secret is the raw "whsec_..." string. Timestamps outside a
// ±5 minute window are rejected to stop replayed deliveries.
//
// This example signs a delivery and verifies it end to end, so it runs
// offline:
//
//	go run ./examples/webhooks
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const toleranceSeconds = 300

func sign(secret string, rawBody []byte, timestamp int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = fmt.Fprintf(mac, "%d.", timestamp)
	_, _ = mac.Write(rawBody)
	return fmt.Sprintf("t=%d,v1=%s", timestamp, hex.EncodeToString(mac.Sum(nil)))
}

func verify(secret string, rawBody []byte, timestamp, signatureHeader string, now int64) error {
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid X-Sorolens-Timestamp: %w", err)
	}
	if delta := now - ts; delta > toleranceSeconds || -delta > toleranceSeconds {
		return fmt.Errorf("signature timestamp outside the tolerance window (%ds)", delta)
	}

	var received string
	for _, piece := range strings.Split(signatureHeader, ",") {
		if value, ok := strings.CutPrefix(piece, "v1="); ok {
			received = value
		}
	}
	if received == "" {
		return fmt.Errorf("signature header is missing the v1 value")
	}

	expected := sign(secret, rawBody, ts)
	_, expectedHex, ok := strings.Cut(expected, "v1=")
	if !ok {
		return fmt.Errorf("failed to extract expected signature")
	}
	if !hmac.Equal([]byte(strings.ToLower(received)), []byte(expectedHex)) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}

func expectRejection(label string, err error) {
	if err == nil {
		fmt.Fprintf(os.Stderr, "rejected (%s): signature should have been rejected\n", label)
		os.Exit(1)
	}
	fmt.Printf("  rejected (%s): %v\n", label, err)
}

func main() {
	secret := os.Getenv("SOROLENS_WEBHOOK_SECRET")
	if secret == "" {
		secret = "whsec_example_secret"
	}
	body := []byte(`{"event":"contract.alert","contract_id":"CDEMO"}`)
	now := time.Now().Unix()
	header := sign(secret, body, now)

	if err := verify(secret, body, strconv.FormatInt(now, 10), header, now); err != nil {
		fmt.Fprintf(os.Stderr, "authentic delivery rejected: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  accepted: %.24s...\n", header)

	expectRejection("tampered body", verify(secret, []byte(`{"event":"evil"}`), strconv.FormatInt(now, 10), header, now))
	expectRejection("wrong secret", verify("whsec_other", body, strconv.FormatInt(now, 10), header, now))

	stale := now - 600
	expectRejection("replayed delivery", verify(secret, body, strconv.FormatInt(stale, 10), sign(secret, body, stale), now))

	fmt.Println("webhook signature verification passed")
}
