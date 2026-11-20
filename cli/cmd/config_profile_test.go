package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeStore is an in-memory secretStore.
type fakeStore struct{ keys map[string]string }

func (f *fakeStore) Get(p string) (string, error) {
	if v, ok := f.keys[p]; ok {
		return v, nil
	}
	return "", errSecretNotFound
}
func (f *fakeStore) Set(p, k string) (string, error) { f.keys[p] = k; return "keyring", nil }
func (f *fakeStore) Delete(p string) error           { delete(f.keys, p); return nil }

func useFakeStore(t *testing.T) *fakeStore {
	t.Helper()
	fs := &fakeStore{keys: map[string]string{}}
	old := newSecretStore
	newSecretStore = func() secretStore { return fs }
	t.Cleanup(func() { newSecretStore = old })
	return fs
}

func useTempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("SOROLENS_PROFILE", "")
	t.Setenv("SOROLENS_API_URL", "")
	t.Setenv("SOROLENS_NETWORK", "")
	return home
}

func writeCfg(t *testing.T, home, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(home, ".sorolens.yml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyConfigStillWorks(t *testing.T) {
	home := useTempHome(t)
	writeCfg(t, home, "api_url: https://legacy.example\nnetwork: mainnet\n")
	rootCmd.SetArgs([]string{"config", "show"})
	out, err := captureOutput(t, rootCmd.Execute)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "https://legacy.example") || !strings.Contains(out, "mainnet") {
		t.Fatalf("legacy config ignored:\n%s", out)
	}
}

func TestProfileAddMigratesLegacyAndPrecedence(t *testing.T) {
	home := useTempHome(t)
	store := useFakeStore(t)
	writeCfg(t, home, "api_url: https://legacy.example\nnetwork: mainnet\n")

	rootCmd.SetArgs([]string{"config", "profile", "add", "prod", "--api-url", "https://prod.example", "--network", "mainnet", "--api-key-stdin"})
	rootCmd.SetIn(strings.NewReader("secret-key-123\n"))
	if _, err := captureOutput(t, rootCmd.Execute); err != nil {
		t.Fatal(err)
	}
	rootCmd.SetIn(nil)
	if store.keys["prod"] != "secret-key-123" {
		t.Fatal("key not stored in keyring store")
	}
	raw, _ := os.ReadFile(filepath.Join(home, ".sorolens.yml"))
	if strings.Contains(string(raw), "secret-key-123") {
		t.Fatal("API key leaked into config file")
	}
	cfg, _ := loadUserConfig()
	if _, ok := cfg.Profiles["default"]; !ok || cfg.APIURL != "" {
		t.Fatalf("legacy config not migrated: %+v", cfg)
	}
	if cfg.DefaultProfile != "default" {
		t.Fatalf("default profile = %q", cfg.DefaultProfile)
	}

	// Default profile (migrated legacy values).
	rootCmd.SetArgs([]string{"config", "show"})
	out, _ := captureOutput(t, rootCmd.Execute)
	if !strings.Contains(out, "https://legacy.example") {
		t.Fatalf("default profile not applied:\n%s", out)
	}
	// SOROLENS_PROFILE beats the default; the key is masked.
	t.Setenv("SOROLENS_PROFILE", "prod")
	rootCmd.SetArgs([]string{"config", "show"})
	out, _ = captureOutput(t, rootCmd.Execute)
	if !strings.Contains(out, "https://prod.example") || !strings.Contains(out, "********") || strings.Contains(out, "secret-key-123") {
		t.Fatalf("env profile/mask wrong:\n%s", out)
	}
	// --profile beats the env var.
	rootCmd.SetArgs([]string{"--profile", "default", "config", "show"})
	out, _ = captureOutput(t, rootCmd.Execute)
	if !strings.Contains(out, "https://legacy.example") {
		t.Fatalf("--profile should override env:\n%s", out)
	}
	// An explicit --api-url beats the profile.
	rootCmd.SetArgs([]string{"--profile", "prod", "--api-url", "https://flag.example", "config", "show"})
	out, _ = captureOutput(t, rootCmd.Execute)
	if !strings.Contains(out, "https://flag.example") {
		t.Fatalf("flag should beat profile:\n%s", out)
	}
}

func TestProfileListUseRemove(t *testing.T) {
	home := useTempHome(t)
	store := useFakeStore(t)
	_ = home
	for _, a := range [][]string{
		{"config", "profile", "add", "a", "--api-url", "https://a.example"},
		{"config", "profile", "add", "b", "--api-url", "https://b.example", "--api-key-stdin"},
	} {
		rootCmd.SetArgs(a)
		rootCmd.SetIn(strings.NewReader("k"))
		if _, err := captureOutput(t, rootCmd.Execute); err != nil {
			t.Fatal(err)
		}
	}
	rootCmd.SetIn(nil)
	rootCmd.SetArgs([]string{"config", "profile", "use", "b"})
	if _, err := captureOutput(t, rootCmd.Execute); err != nil {
		t.Fatal(err)
	}
	rootCmd.SetArgs([]string{"config", "profile", "list"})
	out, _ := captureOutput(t, rootCmd.Execute)
	if !strings.Contains(out, "a.example") || !strings.Contains(out, "b.example") || !strings.Contains(out, "*") {
		t.Fatalf("list output:\n%s", out)
	}
	rootCmd.SetArgs([]string{"config", "profile", "remove", "b"})
	if _, err := captureOutput(t, rootCmd.Execute); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.keys["b"]; ok {
		t.Fatal("key not deleted with profile")
	}
	rootCmd.SetArgs([]string{"--profile", "b", "config", "show"})
	if _, err := captureOutput(t, rootCmd.Execute); err == nil {
		t.Fatal("removed profile should be unknown")
	}
}

type failingKeyring struct{}

func (failingKeyring) Get(_, _ string) (string, error) { return "", errors.New("no keyring") }
func (failingKeyring) Set(_, _, _ string) error        { return errors.New("no keyring") }
func (failingKeyring) Delete(_, _ string) error        { return errors.New("no keyring") }

func TestFileFallbackWhenKeyringUnavailable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.yml")
	var warned string
	s := &layeredStore{keyring: failingKeyring{}, filePath: func() (string, error) { return path, nil }, warn: func(m string) { warned = m }}

	loc, err := s.Set("prod", "k-1")
	if err != nil || loc != "file" {
		t.Fatalf("Set = %q, %v", loc, err)
	}
	if !strings.Contains(warned, "no OS keyring") {
		t.Fatalf("expected warning, got %q", warned)
	}
	if st, err := os.Stat(path); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("credentials file mode: %v %v", st, err)
	}
	if got, err := s.Get("prod"); err != nil || got != "k-1" {
		t.Fatalf("Get = %q, %v", got, err)
	}
	if err := s.Delete("prod"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("prod"); !errors.Is(err, errSecretNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestInvalidProfileName(t *testing.T) {
	useTempHome(t)
	useFakeStore(t)
	rootCmd.SetArgs([]string{"config", "profile", "add", "bad name!"})
	if _, err := captureOutput(t, rootCmd.Execute); err == nil {
		t.Fatal("expected invalid name error")
	}
}
