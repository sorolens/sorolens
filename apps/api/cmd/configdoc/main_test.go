package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot is the repository root relative to this package directory.
const repoRoot = "../../../.."

// TestConfigurationReferenceIsFresh fails when docs/configuration.md does not
// match what the generator produces from the current code, i.e. when an
// option was added, removed or re-described without regenerating the file.
// It runs in the existing Go CI job as part of `go test ./...` (issue #846).
func TestConfigurationReferenceIsFresh(t *testing.T) {
	comps, err := Collect(repoRoot)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	want := Render(comps)
	got, err := os.ReadFile(filepath.Join(repoRoot, "docs", "configuration.md"))
	if err != nil {
		t.Fatalf("read docs/configuration.md: %v", err)
	}
	if string(got) != want {
		t.Fatalf("docs/configuration.md is stale: configuration options changed without regenerating it.\nRun `make config-docs` and commit the result.")
	}
}

// TestEveryOptionIsDescribed keeps descriptions next to the code: an option
// without a comment at its read site (or in its .env.example) fails here.
func TestEveryOptionIsDescribed(t *testing.T) {
	comps, err := Collect(repoRoot)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	for _, c := range comps {
		if len(c.Options) == 0 {
			t.Errorf("%s: no options found in %s", c.Title, c.Path)
		}
		for _, o := range c.Options {
			if strings.TrimSpace(o.Description) == "" {
				t.Errorf("%s: %s (read in %s) has no description; add a comment on the line above the read", c.Title, o.Name, o.Source)
			}
		}
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanGo(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "svc/config.go", `package svc

import "os"

type Config struct {
	// Port is the HTTP port.
	Port string
}

func getEnvDefault(k, d string) string { return d }

func load() (*Config, error) {
	var missing []string
	// DB_URL is the database connection string.
	if os.Getenv("DB_URL") == "" {
		missing = append(missing, "DB_URL")
	}
	cfg := &Config{Port: getEnvDefault("SVC_PORT", "8080")}
	_ = os.Getenv("PATH")
	os.Setenv("IGNORED_SET", "x")
	return cfg, nil
}
`)
	writeFile(t, dir, "svc/config_test.go", `package svc

import "os"

var _ = os.Getenv("TEST_ONLY_VAR")
`)

	opts, err := scanGo(dir, "svc")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Option{}
	for _, o := range opts {
		got[o.Name] = o
	}
	if len(got) != 2 {
		t.Fatalf("options = %+v, want DB_URL and SVC_PORT only", opts)
	}
	if o := got["DB_URL"]; !o.Required || o.Description != "DB_URL is the database connection string." {
		t.Errorf("DB_URL = %+v", o)
	}
	if o := got["SVC_PORT"]; o.Default != "8080" || o.Description != "Port is the HTTP port." || o.Required {
		t.Errorf("SVC_PORT = %+v", o)
	}
}

func TestScanDotenv(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".env.example", `# ==== Header ====

# Public API URL.
# Required/Optional: Required.
# Default: http://localhost:8080
NEXT_PUBLIC_API_URL=http://example

# Shared credentials for the bucket.
KEY_ID=abc
SECRET=

# Optional metrics port.
# METRICS_PORT=9090
`)
	opts, err := scanDotenv(dir, ".env.example")
	if err != nil {
		t.Fatal(err)
	}
	if len(opts) != 4 {
		t.Fatalf("options = %+v, want 4", opts)
	}
	if o := opts[0]; o.Name != "NEXT_PUBLIC_API_URL" || !o.Required || o.Default != "http://localhost:8080" || o.Description != "Public API URL." {
		t.Errorf("first = %+v", o)
	}
	if opts[1].Description != "Shared credentials for the bucket." || opts[2].Description != opts[1].Description {
		t.Errorf("grouped lines should share the comment: %+v %+v", opts[1], opts[2])
	}
	if o := opts[3]; o.Name != "METRICS_PORT" || o.Default != "9090" || o.Description != "Optional metrics port." {
		t.Errorf("commented-out option = %+v", o)
	}
}
