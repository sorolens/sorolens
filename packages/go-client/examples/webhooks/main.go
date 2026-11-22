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
// ±5 minute window are rejected to stop replayed deliveries. The
// github.com/sorolens/go-client/webhook package implements both the signing
// and verification sides.
//
// This example signs a delivery and verifies it end to end, so it runs
// offline:
//
//	go run ./examples/webhooks
package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/sorolens/go-client/webhook"
)

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
	now := time.Now()
	timestamp := now.Unix()
	timestampHeader := strconv.FormatInt(timestamp, 10)
	signatureHeader := webhook.SignatureHeaderValue(timestamp, webhook.Sign(secret, timestamp, body))

	if err := webhook.VerifyHeaders(secret, signatureHeader, timestampHeader, body, now, 0); err != nil {
		fmt.Fprintf(os.Stderr, "authentic delivery rejected: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  accepted: %.24s...\n", signatureHeader)

	expectRejection("tampered body",
		webhook.VerifyHeaders(secret, signatureHeader, timestampHeader, []byte(`{"event":"evil"}`), now, 0))
	expectRejection("wrong secret",
		webhook.VerifyHeaders("whsec_other", signatureHeader, timestampHeader, body, now, 0))

	stale := now.Add(-10 * time.Minute)
	staleHeader := webhook.SignatureHeaderValue(stale.Unix(), webhook.Sign(secret, stale.Unix(), body))
	expectRejection("replayed delivery",
		webhook.VerifyHeaders(secret, staleHeader, strconv.FormatInt(stale.Unix(), 10), body, now, 0))

	fmt.Println("webhook signature verification passed")
}
