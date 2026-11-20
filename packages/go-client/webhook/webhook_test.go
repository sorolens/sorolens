package webhook

import (
	"errors"
	"net/http"
	"testing"
	"time"
)

// Shared vector from docs/webhooks.md, reproduced by the indexer and the
// Python and Rust SDKs.
const (
	vectorSecret    = "whsec_example_0123456789abcdef0123456789abcdef"
	vectorTimestamp = int64(1700000000)
	vectorSignature = "97dc0f80c8e0c9be056f34e48e360411160395a1b3e3a7978092d869315c4615"
)

func vectorBody() []byte {
	return []byte(`{"event":"watchdog.alert","severity":"Critical"}`)
}

func TestSign_PublishedVector(t *testing.T) {
	if got := Sign(vectorSecret, vectorTimestamp, vectorBody()); got != vectorSignature {
		t.Fatalf("Sign = %q, want the published vector %q", got, vectorSignature)
	}
}

func TestVerify_PublishedVector(t *testing.T) {
	headers := http.Header{}
	headers.Set(TimestampHeader, "1700000000")
	headers.Set(SignatureHeader, SignatureHeaderValue(vectorTimestamp, vectorSignature))

	if err := Verify(vectorSecret, headers, vectorBody(), time.Unix(vectorTimestamp, 0), 0); err != nil {
		t.Fatalf("published vector failed to verify: %v", err)
	}
}

func TestVerify_RoundTrip(t *testing.T) {
	const secret = "whsec_round_trip"
	body := []byte(`{"contract_id":"CABC"}`)
	now := time.Unix(1700000000, 0)
	headers := http.Header{}
	headers.Set(TimestampHeader, "1700000000")
	headers.Set(SignatureHeader, SignatureHeaderValue(now.Unix(), Sign(secret, now.Unix(), body)))

	if err := Verify(secret, headers, body, now, 0); err != nil {
		t.Fatalf("Verify rejected a valid delivery: %v", err)
	}
}

func TestVerify_RejectsTamperedBody(t *testing.T) {
	const secret = "whsec_tamper"
	now := time.Unix(1700000000, 0)
	body := []byte(`{"severity":"Critical"}`)
	headers := http.Header{}
	headers.Set(TimestampHeader, "1700000000")
	headers.Set(SignatureHeader, SignatureHeaderValue(now.Unix(), Sign(secret, now.Unix(), body)))

	err := Verify(secret, headers, []byte(`{"severity":"Info"}`), now, 0)
	if !errors.Is(err, ErrSignatureMismatch) {
		t.Fatalf("want ErrSignatureMismatch, got %v", err)
	}
}

func TestVerify_RejectsWrongSecret(t *testing.T) {
	now := time.Unix(1700000000, 0)
	body := []byte("payload")
	headers := http.Header{}
	headers.Set(TimestampHeader, "1700000000")
	headers.Set(SignatureHeader, SignatureHeaderValue(now.Unix(), Sign("whsec_right", now.Unix(), body)))

	err := Verify("whsec_wrong", headers, body, now, 0)
	if !errors.Is(err, ErrSignatureMismatch) {
		t.Fatalf("want ErrSignatureMismatch, got %v", err)
	}
}

func TestVerify_ReplayWindow(t *testing.T) {
	const secret = "whsec_replay"
	body := []byte("payload")
	signedAt := time.Unix(1700000000, 0)
	headers := http.Header{}
	headers.Set(TimestampHeader, "1700000000")
	headers.Set(SignatureHeader, SignatureHeaderValue(signedAt.Unix(), Sign(secret, signedAt.Unix(), body)))

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
			if err := Verify(secret, headers, body, tc.now, 0); !errors.Is(err, tc.wantErr) {
				t.Fatalf("want %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestVerify_RejectsMalformedHeaders(t *testing.T) {
	const secret = "whsec_malformed"
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
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := VerifyHeaders(secret, tc.signature, tc.signedTS, body, now, 0); !errors.Is(err, tc.wantErr) {
				t.Fatalf("want %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestVerify_AcceptsAnyMatchingV1Signature(t *testing.T) {
	const (
		oldSecret = "whsec_old"
		newSecret = "whsec_new"
	)
	now := time.Unix(1700000000, 0)
	body := []byte("payload")
	header := "t=1700000000,v1=" + Sign(oldSecret, now.Unix(), body) +
		",v1=" + Sign(newSecret, now.Unix(), body)

	if err := VerifyHeaders(newSecret, header, "1700000000", body, now, 0); err != nil {
		t.Fatalf("Verify with the new secret: %v", err)
	}
	if err := VerifyHeaders(oldSecret, header, "1700000000", body, now, 0); err != nil {
		t.Fatalf("Verify with the old secret: %v", err)
	}
}
