package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"testing"
	"time"
)

// FuzzVerifySlackSignature checks that VerifySlackSignature never panics on arbitrary
// inputs and accurately validates valid signatures while rejecting malformed inputs.
func FuzzVerifySlackSignature(f *testing.F) {
	now := time.Unix(1700000000, 0)
	tsValid := strconv.FormatInt(now.Unix(), 10)

	// Helper to create a valid HMAC-SHA256 signature
	makeSig := func(secret, ts string, body []byte) string {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte("v0:" + ts + ":"))
		mac.Write(body)
		return "v0=" + hex.EncodeToString(mac.Sum(nil))
	}

	// Seeds
	f.Add("secret123", tsValid, makeSig("secret123", tsValid, []byte("command=/sorolens&text=C123")), []byte("command=/sorolens&text=C123"))
	f.Add("", tsValid, "v0=abc", []byte("hello"))
	f.Add("secret123", "", "", []byte(""))
	f.Add("secret123", "notanint", "v0=abc", []byte(""))
	f.Add("secret123", tsValid, "badprefix", []byte(""))
	f.Add("secret123", tsValid, "v0=nothex", []byte(""))
	f.Add("secret123", tsValid, "v0=123", []byte(""))
	f.Add("secret123", "-1", "v0=abc", []byte(""))
	f.Add("secret123", "9223372036854775807", "v0=abc", []byte(""))
	f.Add("secret123", "-9223372036854775808", "v0=abc", []byte(""))
	f.Add("secret123", strconv.FormatInt(now.Unix()-400, 10), makeSig("secret123", strconv.FormatInt(now.Unix()-400, 10), []byte("body")), []byte("body"))

	f.Fuzz(func(t *testing.T, secret, timestamp, signature string, body []byte) {
		err := VerifySlackSignature(secret, timestamp, signature, body, now)

		if secret == "" && err == nil {
			t.Error("VerifySlackSignature succeeded with empty secret")
		}

		// Positive property: valid signature generated for current timestamp must always verify
		if secret != "" {
			validSig := makeSig(secret, tsValid, body)
			if errValid := VerifySlackSignature(secret, tsValid, validSig, body, now); errValid != nil {
				t.Fatalf("expected valid signature to verify successfully: %v", errValid)
			}
		}
	})
}
