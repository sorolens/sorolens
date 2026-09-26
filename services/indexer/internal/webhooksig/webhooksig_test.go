package webhooksig

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestGenerateSecret(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret: %v", err)
	}
	if !strings.HasPrefix(secret, SecretPrefix) {
		t.Errorf("secret %q does not start with %q", secret, SecretPrefix)
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(secret, SecretPrefix))
	if err != nil {
		t.Fatalf("secret body is not base64url: %v", err)
	}
	if len(raw) != secretBytes {
		t.Errorf("secret entropy = %d bytes, want %d", len(raw), secretBytes)
	}

	other, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret: %v", err)
	}
	if secret == other {
		t.Error("two generated secrets must differ")
	}
}

func TestHashSecret_MatchesSHA256(t *testing.T) {
	// Well-known SHA-256 of the empty string.
	const emptySHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if got := HashSecret(""); got != emptySHA256 {
		t.Errorf("HashSecret(\"\") = %q, want %q", got, emptySHA256)
	}
}

// TestSign_UsesTimestampDotBody pins the signed payload format independently of
// SignedPayload, so a change to that helper cannot silently change the wire
// contract documented in docs/webhooks.md.
func TestSign_UsesTimestampDotBody(t *testing.T) {
	const (
		secret    = "whsec_test"
		timestamp = int64(1700000000)
	)
	body := []byte(`{"severity":"Critical"}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("1700000000." + string(body)))
	want := hex.EncodeToString(mac.Sum(nil))

	if got := Sign(secret, timestamp, body); got != want {
		t.Errorf("Sign = %q, want %q", got, want)
	}
}

func TestVerify_RoundTrip(t *testing.T) {
	secret, _ := GenerateSecret()
	body := []byte(`{"contract_id":"CABC","severity":"Critical"}`)
	now := time.Unix(1700000000, 0)

	sig := Sign(secret, now.Unix(), body)
	err := Verify(secret, SignatureHeaderValue(now.Unix(), sig), "1700000000", body, now, 0)
	if err != nil {
		t.Fatalf("Verify rejected a valid signature: %v", err)
	}
}

func TestVerify_RejectsTamperedBody(t *testing.T) {
	secret, _ := GenerateSecret()
	now := time.Unix(1700000000, 0)
	body := []byte(`{"severity":"Critical"}`)
	sig := Sign(secret, now.Unix(), body)

	tampered := []byte(`{"severity":"Info"}`)
	err := Verify(secret, SignatureHeaderValue(now.Unix(), sig), "1700000000", tampered, now, 0)
	if !errors.Is(err, ErrSignatureMismatch) {
		t.Fatalf("want ErrSignatureMismatch, got %v", err)
	}
}

func TestVerify_RejectsWrongSecret(t *testing.T) {
	secret, _ := GenerateSecret()
	other, _ := GenerateSecret()
	now := time.Unix(1700000000, 0)
	body := []byte("payload")

	sig := Sign(secret, now.Unix(), body)
	err := Verify(other, SignatureHeaderValue(now.Unix(), sig), "1700000000", body, now, 0)
	if !errors.Is(err, ErrSignatureMismatch) {
		t.Fatalf("want ErrSignatureMismatch, got %v", err)
	}
}

func TestVerify_ReplayWindow(t *testing.T) {
	secret, _ := GenerateSecret()
	body := []byte("payload")
	signedAt := time.Unix(1700000000, 0)
	sig := Sign(secret, signedAt.Unix(), body)
	header := SignatureHeaderValue(signedAt.Unix(), sig)

	cases := []struct {
		name    string
		now     time.Time
		wantErr error
	}{
		{"fresh", signedAt.Add(time.Second), nil},
		{"at the boundary", signedAt.Add(DefaultTolerance), nil},
		{"just outside", signedAt.Add(DefaultTolerance + time.Second), ErrTimestampOutsideWindow},
		{"far in the past", signedAt.Add(-time.Hour), ErrTimestampOutsideWindow},
		{"clock skew forward", signedAt.Add(-DefaultTolerance), nil},
		{"far in the future", signedAt.Add(time.Hour), ErrTimestampOutsideWindow},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Verify(secret, header, "1700000000", body, tc.now, 0)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("want %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestVerify_RejectsMalformedHeaders(t *testing.T) {
	secret, _ := GenerateSecret()
	now := time.Unix(1700000000, 0)
	body := []byte("payload")
	valid := Sign(secret, now.Unix(), body)

	cases := []struct {
		name      string
		signature string
		signedTS  string
		wantErr   error
	}{
		{"empty signature header", "", "1700000000", ErrMalformedHeader},
		{"missing t", "v1=" + valid, "1700000000", ErrMalformedHeader},
		{"non-numeric t", "t=abc,v1=" + valid, "1700000000", ErrMalformedHeader},
		{"no equals", "t1700000000", "1700000000", ErrMalformedHeader},
		{"only a timestamp", "t=1700000000", "1700000000", ErrNoSignature},
		{"timestamp header missing", "t=1700000000,v1=" + valid, "", ErrMalformedHeader},
		{"timestamp header disagrees", "t=1700000000,v1=" + valid, "1700000001", ErrMalformedHeader},
		{"timestamp header not numeric", "t=1700000000,v1=" + valid, "soon", ErrMalformedHeader},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Verify(secret, tc.signature, tc.signedTS, body, now, 0)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("want %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestVerify_AcceptsAnyMatchingV1Signature(t *testing.T) {
	// During rotation a receiver may hold two secrets; the sender should be
	// able to send both signatures in one header.
	oldSecret, _ := GenerateSecret()
	newSecret, _ := GenerateSecret()
	now := time.Unix(1700000000, 0)
	body := []byte("payload")

	oldSig := Sign(oldSecret, now.Unix(), body)
	newSig := Sign(newSecret, now.Unix(), body)
	header := "t=1700000000,v1=" + oldSig + ",v1=" + newSig

	if err := Verify(newSecret, header, "1700000000", body, now, 0); err != nil {
		t.Fatalf("Verify with the new secret: %v", err)
	}
	if err := Verify(oldSecret, header, "1700000000", body, now, 0); err != nil {
		t.Fatalf("Verify with the old secret: %v", err)
	}
}

func TestParseSignatureHeader_OrderIndependent(t *testing.T) {
	ts, sigs, err := ParseSignatureHeader("v1=deadbeef,t=42")
	if err != nil {
		t.Fatalf("ParseSignatureHeader: %v", err)
	}
	if ts != 42 {
		t.Errorf("timestamp = %d, want 42", ts)
	}
	if len(sigs) != 1 || sigs[0] != "deadbeef" {
		t.Errorf("signatures = %v, want [deadbeef]", sigs)
	}
}

func TestSignatureHeaderValue(t *testing.T) {
	if got := SignatureHeaderValue(1700000000, "abc"); got != "t=1700000000,v1=abc" {
		t.Errorf("SignatureHeaderValue = %q", got)
	}
}

func TestVerify_CustomTolerance(t *testing.T) {
	secret, _ := GenerateSecret()
	body := []byte("payload")
	signedAt := time.Unix(1700000000, 0)
	header := SignatureHeaderValue(signedAt.Unix(), Sign(secret, signedAt.Unix(), body))

	err := Verify(secret, header, "1700000000", body, signedAt.Add(time.Minute), time.Second)
	if !errors.Is(err, ErrTimestampOutsideWindow) {
		t.Fatalf("want ErrTimestampOutsideWindow, got %v", err)
	}
}
