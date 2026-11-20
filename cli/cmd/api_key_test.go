package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for the global --api-key flag (issue #565). The key resolves in
// precedence order — --api-key flag, SOROLENS_API_KEY env var, the api_key
// config-file entry, then the active profile's stored key — and reaches
// every command that talks to the API. track and label are exercised here;
// both build their client through newClient.

const testContractID565 = "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

// apiKeyServer returns a fake API that records the X-API-Key header of every
// request and answers the track and label endpoints with minimal payloads.
func apiKeyServer(t *testing.T) (*httptest.Server, *string) {
	t.Helper()
	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/contracts":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":      testContractID565,
				"network": "testnet",
				"status":  "active",
			})
		case "/api/v1/labels":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"label": "treasury",
				"value": "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
				"scope": "workspace",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &gotKey
}

func TestTrackAPIKeyFromFlag(t *testing.T) {
	useTempHome(t)
	t.Setenv("SOROLENS_API_KEY", "")
	srv, gotKey := apiKeyServer(t)

	// The flag works before the subcommand, as a global flag should.
	rootCmd.SetArgs([]string{"--api-key", "flag-key-123", "--api-url", srv.URL, "track", testContractID565, "--no-color"})
	out, err := captureOutput(t, rootCmd.Execute)
	require.NoError(t, err)
	assert.Equal(t, "flag-key-123", *gotKey, "track must send the --api-key flag value")
	assert.Contains(t, out, "Tracking")
}

func TestTrackAPIKeyFromEnv(t *testing.T) {
	useTempHome(t)
	t.Setenv("SOROLENS_API_KEY", "env-key-456")
	srv, gotKey := apiKeyServer(t)

	rootCmd.SetArgs([]string{"track", testContractID565, "--api-url", srv.URL, "--no-color"})
	out, err := captureOutput(t, rootCmd.Execute)
	require.NoError(t, err)
	assert.Equal(t, "env-key-456", *gotKey, "track must send the SOROLENS_API_KEY env value")
	assert.Contains(t, out, "Tracking")
}

func TestTrackAPIKeyFromConfigFile(t *testing.T) {
	home := useTempHome(t)
	t.Setenv("SOROLENS_API_KEY", "")
	writeCfg(t, home, "api_key: file-key-789\n")
	srv, gotKey := apiKeyServer(t)

	rootCmd.SetArgs([]string{"track", testContractID565, "--api-url", srv.URL, "--no-color"})
	out, err := captureOutput(t, rootCmd.Execute)
	require.NoError(t, err)
	assert.Equal(t, "file-key-789", *gotKey, "track must send the api_key config-file entry")
	assert.Contains(t, out, "Tracking")
}

func TestAPIKeyPrecedence(t *testing.T) {
	t.Run("flag beats env var", func(t *testing.T) {
		useTempHome(t)
		t.Setenv("SOROLENS_API_KEY", "env-key-456")
		srv, gotKey := apiKeyServer(t)

		rootCmd.SetArgs([]string{"track", testContractID565, "--api-url", srv.URL, "--api-key", "flag-key-123", "--no-color"})
		_, err := captureOutput(t, rootCmd.Execute)
		require.NoError(t, err)
		assert.Equal(t, "flag-key-123", *gotKey)
	})

	t.Run("env var beats config file", func(t *testing.T) {
		home := useTempHome(t)
		t.Setenv("SOROLENS_API_KEY", "env-key-456")
		writeCfg(t, home, "api_key: file-key-789\n")
		srv, gotKey := apiKeyServer(t)

		rootCmd.SetArgs([]string{"track", testContractID565, "--api-url", srv.URL, "--no-color"})
		_, err := captureOutput(t, rootCmd.Execute)
		require.NoError(t, err)
		assert.Equal(t, "env-key-456", *gotKey)
	})

	t.Run("config file beats profile key", func(t *testing.T) {
		home := useTempHome(t)
		t.Setenv("SOROLENS_API_KEY", "")
		store := useFakeStore(t)
		store.keys["prod"] = "profile-key-999"
		writeCfg(t, home, "api_key: file-key-789\ndefault_profile: prod\nprofiles:\n  prod:\n    api_url: https://prod.example\n    network: mainnet\n")
		srv, gotKey := apiKeyServer(t)

		rootCmd.SetArgs([]string{"track", testContractID565, "--api-url", srv.URL, "--no-color"})
		_, err := captureOutput(t, rootCmd.Execute)
		require.NoError(t, err)
		assert.Equal(t, "file-key-789", *gotKey)
	})

	t.Run("profile key is the fallback", func(t *testing.T) {
		home := useTempHome(t)
		t.Setenv("SOROLENS_API_KEY", "")
		store := useFakeStore(t)
		store.keys["prod"] = "profile-key-999"
		writeCfg(t, home, "default_profile: prod\nprofiles:\n  prod:\n    api_url: https://prod.example\n    network: mainnet\n")
		srv, gotKey := apiKeyServer(t)

		rootCmd.SetArgs([]string{"track", testContractID565, "--api-url", srv.URL, "--no-color"})
		_, err := captureOutput(t, rootCmd.Execute)
		require.NoError(t, err)
		assert.Equal(t, "profile-key-999", *gotKey)
	})
}

func TestLabelAPIKeyFromFlag(t *testing.T) {
	useTempHome(t)
	t.Setenv("SOROLENS_API_KEY", "")
	srv, gotKey := apiKeyServer(t)

	rootCmd.SetArgs([]string{"label", "treasury", "GAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "--api-url", srv.URL, "--api-key", "label-key-1", "--no-color"})
	out, err := captureOutput(t, rootCmd.Execute)
	require.NoError(t, err)
	assert.Equal(t, "label-key-1", *gotKey, "label must send the --api-key flag value")
	assert.Contains(t, out, "Registered treasury")
}

func TestGlobalAPIKeyFlagRegistered(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("api-key")
	require.NotNil(t, flag, "root command should have a persistent --api-key flag")
	assert.Equal(t, "string", flag.Value.Type(), "--api-key should be a string flag")
	// The tui command must not define its own local --api-key: a local flag
	// with the same name would shadow (or clash with) the persistent one.
	assert.Nil(t, tuiCmd.Flags().Lookup("api-key"), "tui must not define a local --api-key flag")
}

func TestConfigShowMasksGlobalAPIKey(t *testing.T) {
	useTempHome(t)
	t.Setenv("SOROLENS_API_KEY", "show-secret-key")

	rootCmd.SetArgs([]string{"config", "show"})
	out, err := captureOutput(t, rootCmd.Execute)
	require.NoError(t, err)
	assert.Contains(t, out, "********")
	assert.NotContains(t, out, "show-secret-key")
	assert.True(t, strings.Contains(out, "API Key"), "config show should report the key presence:\n%s", out)
}
