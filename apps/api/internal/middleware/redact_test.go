package middleware

import (
	"testing"
)

func TestRedactUserAgent(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"Mozilla/5.0 (compatible)", "Mozilla/5.0 (compatible)"},
		{"sorolens-go-client/1.0", "sorolens-go-client/1.0"},
		{"sdk/1.0 Bearer sk-live-abcdef123456789012345678", "sdk/1.0 Bearer [redacted]"},
		{"sdk/1.0 api-key=sk-live-xxxxxxxxxxxxxxxxxxxxxxxxxxx", "sdk/1.0 api-key=[redacted]"},
		{"agent/2.0 Basic dXNlcjpwYXNzd29yZA==", "agent/2.0 Basic [redacted]"},
	}
	for _, tc := range tests {
		got := RedactUserAgent(tc.in)
		if got != tc.want {
			t.Errorf("RedactUserAgent(%q)\n  got  %q\n  want %q", tc.in, got, tc.want)
		}
	}
}

func TestRedactHeader(t *testing.T) {
	if got := RedactHeader("Authorization", "Bearer sk-xxx"); got != "[redacted]" {
		t.Errorf("Authorization: want [redacted], got %q", got)
	}
	if got := RedactHeader("X-API-Key", "supersecret"); got != "[redacted]" {
		t.Errorf("X-API-Key: want [redacted], got %q", got)
	}
	if got := RedactHeader("Content-Type", "application/json"); got != "application/json" {
		t.Errorf("Content-Type: want passthrough, got %q", got)
	}
	if got := RedactHeader("Accept", "text/html"); got != "text/html" {
		t.Errorf("Accept: want passthrough, got %q", got)
	}
}
