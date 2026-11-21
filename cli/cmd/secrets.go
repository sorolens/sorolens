package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
	"gopkg.in/yaml.v3"
)

// keyringService is the service name API keys are stored under in the OS
// keyring (macOS Keychain, Windows Credential Manager, Secret Service).
const keyringService = "sorolens"

// errSecretNotFound is returned by a secretStore when no key is stored.
var errSecretNotFound = errors.New("secret not found")

// secretStore holds API keys by profile name. Keys are never logged or
// printed by any implementation.
type secretStore interface {
	Get(profile string) (string, error)
	// Set stores the key and returns where it went: "keyring" or "file".
	Set(profile, key string) (location string, err error)
	Delete(profile string) error
}

// keyringBackend abstracts the OS keyring so tests can use a fake.
type keyringBackend interface {
	Get(service, user string) (string, error)
	Set(service, user, password string) error
	Delete(service, user string) error
}

type osKeyring struct{}

func (osKeyring) Get(s, u string) (string, error) { return keyring.Get(s, u) }
func (osKeyring) Set(s, u, p string) error        { return keyring.Set(s, u, p) }
func (osKeyring) Delete(s, u string) error        { return keyring.Delete(s, u) }

// layeredStore prefers the OS keyring and falls back to a 0600 credentials
// file (with a warning) when no keyring is available.
type layeredStore struct {
	keyring  keyringBackend
	filePath func() (string, error)
	warn     func(string)
}

// newSecretStore is a package variable so tests can inject a fake store.
var newSecretStore = func() secretStore {
	return &layeredStore{
		keyring:  osKeyring{},
		filePath: credentialsFilePath,
		warn:     func(m string) { fmt.Fprintln(os.Stderr, "warning:", m) },
	}
}

func credentialsFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	return filepath.Join(home, ".sorolens-credentials.yml"), nil
}

func (s *layeredStore) readFile() (map[string]string, error) {
	path, err := s.filePath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read credentials file: %w", err)
	}
	m := map[string]string{}
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("parse credentials file: %w", err)
	}
	return m, nil
}

func (s *layeredStore) writeFile(m map[string]string) error {
	path, err := s.filePath()
	if err != nil {
		return err
	}
	if len(m) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove credentials file: %w", err)
		}
		return nil
	}
	b, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, b)
}

func (s *layeredStore) Get(profile string) (string, error) {
	if v, err := s.keyring.Get(keyringService, profile); err == nil {
		return v, nil
	}
	m, err := s.readFile()
	if err != nil {
		return "", err
	}
	if v, ok := m[profile]; ok && v != "" {
		return v, nil
	}
	return "", errSecretNotFound
}

func (s *layeredStore) Set(profile, key string) (string, error) {
	if err := s.keyring.Set(keyringService, profile, key); err == nil {
		// Drop any stale plain-text copy left by an earlier fallback.
		_ = s.dropFile(profile)
		return "keyring", nil
	}
	m, err := s.readFile()
	if err != nil {
		return "", err
	}
	m[profile] = key
	if err := s.writeFile(m); err != nil {
		return "", err
	}
	s.warn("no OS keyring available; API key stored in a 0600 file instead of the keyring")
	return "file", nil
}

func (s *layeredStore) Delete(profile string) error {
	if err := s.keyring.Delete(keyringService, profile); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		// An unavailable keyring is not fatal: the file copy is removed below.
		_ = err
	}
	return s.dropFile(profile)
}

func (s *layeredStore) dropFile(profile string) error {
	m, err := s.readFile()
	if err != nil {
		return err
	}
	if _, ok := m[profile]; !ok {
		return nil
	}
	delete(m, profile)
	return s.writeFile(m)
}

// writeFileAtomic writes data to path with mode 0600 via a temp file + rename.
func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".sorolens-*.tmp")
	if err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
