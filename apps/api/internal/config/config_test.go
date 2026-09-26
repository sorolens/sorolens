package config

import "testing"

func TestMaxBodyBytesFromEnvDefault(t *testing.T) {
	t.Setenv("REQUEST_MAX_BODY_BYTES", "")

	got, err := MaxBodyBytesFromEnv()
	if err != nil {
		t.Fatalf("MaxBodyBytesFromEnv() error = %v, want nil", err)
	}
	if got != DefaultRequestMaxBodyBytes {
		t.Errorf("MaxBodyBytesFromEnv() = %d, want %d", got, DefaultRequestMaxBodyBytes)
	}
}

func TestMaxBodyBytesFromEnvOverride(t *testing.T) {
	t.Setenv("REQUEST_MAX_BODY_BYTES", "2097152")

	got, err := MaxBodyBytesFromEnv()
	if err != nil {
		t.Fatalf("MaxBodyBytesFromEnv() error = %v, want nil", err)
	}
	if got != 2<<20 {
		t.Errorf("MaxBodyBytesFromEnv() = %d, want %d", got, 2<<20)
	}
}

func TestMaxBodyBytesFromEnvRejectsNonPositiveAndNonNumeric(t *testing.T) {
	for _, raw := range []string{"abc", "0", "-1", "1.5"} {
		raw := raw
		t.Run(raw, func(t *testing.T) {
			t.Setenv("REQUEST_MAX_BODY_BYTES", raw)

			if _, err := MaxBodyBytesFromEnv(); err == nil {
				t.Errorf("MaxBodyBytesFromEnv() with %q error = nil, want error", raw)
			}
		})
	}
}
