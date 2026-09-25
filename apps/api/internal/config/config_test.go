package config

import (
	"strings"
	"testing"
	"time"
)

func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://localhost/sorolens")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
}

func TestLoad_TimeoutDefaults(t *testing.T) {
	setRequired(t)
	t.Setenv("API_REQUEST_TIMEOUT", "")
	t.Setenv("API_STREAM_TIMEOUT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RequestTimeout != 30*time.Second {
		t.Errorf("RequestTimeout = %v, want 30s", cfg.RequestTimeout)
	}
	if cfg.StreamTimeout != 5*time.Minute {
		t.Errorf("StreamTimeout = %v, want 5m", cfg.StreamTimeout)
	}
}

func TestLoad_TimeoutOverrides(t *testing.T) {
	setRequired(t)
	t.Setenv("API_REQUEST_TIMEOUT", "10s")
	t.Setenv("API_STREAM_TIMEOUT", "90s")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RequestTimeout != 10*time.Second || cfg.StreamTimeout != 90*time.Second {
		t.Errorf("got request=%v stream=%v, want 10s and 90s", cfg.RequestTimeout, cfg.StreamTimeout)
	}
}

// A zero or negative timeout would silently remove the cap issue #154 adds,
// so Load rejects it instead of accepting it.
func TestLoad_RejectsInvalidTimeouts(t *testing.T) {
	for _, tc := range []struct{ key, val string }{
		{"API_REQUEST_TIMEOUT", "0s"},
		{"API_REQUEST_TIMEOUT", "-5s"},
		{"API_REQUEST_TIMEOUT", "thirty"},
		{"API_STREAM_TIMEOUT", "0"},
		{"API_STREAM_TIMEOUT", "5 minutes"},
	} {
		t.Run(tc.key+"="+tc.val, func(t *testing.T) {
			setRequired(t)
			t.Setenv(tc.key, tc.val)
			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("err = %v, want an error naming %s", err, tc.key)
			}
		})
	}
}
