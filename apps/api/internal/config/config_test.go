package config

import (
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

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

func TestPoolSettings_DefaultsAndApply(t *testing.T) {
	for _, key := range []string{
		"DB_MAX_CONNS",
		"DB_MIN_CONNS",
		"DB_MAX_CONN_LIFETIME",
		"DB_MAX_CONN_IDLE_TIME",
	} {
		t.Setenv(key, "")
	}

	settings, err := PoolSettingsFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if settings != (PoolSettings{
		MaxConns:        DefaultDBMaxConns,
		MinConns:        DefaultDBMinConns,
		MaxConnLifetime: DefaultDBMaxConnLifetime,
		MaxConnIdleTime: DefaultDBMaxConnIdleTime,
	}) {
		t.Fatalf("PoolSettingsFromEnv() = %+v, want defaults", settings)
	}

	poolCfg, err := pgxpool.ParseConfig("postgres://localhost/sorolens")
	if err != nil {
		t.Fatal(err)
	}
	settings.Apply(poolCfg)
	if poolCfg.MaxConns != DefaultDBMaxConns ||
		poolCfg.MinConns != DefaultDBMinConns ||
		poolCfg.MaxConnLifetime != DefaultDBMaxConnLifetime ||
		poolCfg.MaxConnIdleTime != DefaultDBMaxConnIdleTime {
		t.Fatalf("applied pool config = %+v, want defaults", poolCfg)
	}
}

func TestPoolSettings_Overrides(t *testing.T) {
	t.Setenv("DB_MAX_CONNS", "40")
	t.Setenv("DB_MIN_CONNS", "5")
	t.Setenv("DB_MAX_CONN_LIFETIME", "2h")
	t.Setenv("DB_MAX_CONN_IDLE_TIME", "15m")

	settings, err := PoolSettingsFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	want := PoolSettings{
		MaxConns:        40,
		MinConns:        5,
		MaxConnLifetime: 2 * time.Hour,
		MaxConnIdleTime: 15 * time.Minute,
	}
	if settings != want {
		t.Fatalf("PoolSettingsFromEnv() = %+v, want %+v", settings, want)
	}
}

func TestPoolSettings_RejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		set  func(*testing.T)
	}{
		{
			name: "max conns is zero",
			set:  func(t *testing.T) { t.Setenv("DB_MAX_CONNS", "0") },
		},
		{
			name: "min conns exceeds max",
			set: func(t *testing.T) {
				t.Setenv("DB_MAX_CONNS", "2")
				t.Setenv("DB_MIN_CONNS", "3")
			},
		},
		{
			name: "invalid lifetime",
			set:  func(t *testing.T) { t.Setenv("DB_MAX_CONN_LIFETIME", "forever") },
		},
		{
			name: "negative idle time",
			set:  func(t *testing.T) { t.Setenv("DB_MAX_CONN_IDLE_TIME", "-1s") },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.set(t)
			if _, err := PoolSettingsFromEnv(); err == nil {
				t.Fatal("PoolSettingsFromEnv() error = nil, want error")
			}
		})
	}
}
