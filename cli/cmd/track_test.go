package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureErrOutput mirrors captureOutput but reads stderr, so tests can assert
// a command did not print anything itself.
func captureErrOutput(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	globalConfig = Config{Timeout: 10 * time.Second}
	resetCommandFlags(rootCmd)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStderr := os.Stderr
	os.Stderr = w

	outChan := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		outChan <- buf.String()
	}()

	fnErr := fn()

	_ = w.Close()
	os.Stderr = oldStderr
	out := <-outChan
	_ = r.Close()

	return out, fnErr
}

func TestTrackCommand_Flags(t *testing.T) {
	// Verify that the track command has the expected flags.
	trackCmd := trackCmd

	// Check that --alias flag exists
	aliasFlag := trackCmd.Flags().Lookup("alias")
	require.NotNil(t, aliasFlag, "track command should have --alias flag")

	// Check that --preset flag exists
	presetFlag := trackCmd.Flags().Lookup("preset")
	require.NotNil(t, presetFlag, "track command should have --preset flag")
	assert.Equal(t, "", presetFlag.DefValue, "--preset flag should have empty default")
}

func TestTrackCommand_UsageIncludesPresets(t *testing.T) {
	// Verify that the usage template includes the preset list.
	usageTemplate := usageTemplate()
	assert.Contains(t, usageTemplate, "Available presets:")
	assert.Contains(t, usageTemplate, "dex")
	assert.Contains(t, usageTemplate, "lending")
	assert.Contains(t, usageTemplate, "stablecoin")
}

func TestTrackCommand_PresetFlagRegistered(t *testing.T) {
	// Verify that --preset is a string flag.
	presetFlag := trackCmd.Flags().Lookup("preset")
	require.NotNil(t, presetFlag)
	assert.Equal(t, "string", presetFlag.Value.Type(), "--preset should be a string flag")
}

// Test that the track command requires exactly one argument.
func TestTrackCommand_Args(t *testing.T) {
	trackCmd := trackCmd

	// RunE should not be called with wrong number of args because Args: cobra.ExactArgs(1)
	// We just verify the command structure.
	assert.Equal(t, "track <contract-id>", trackCmd.Use)
	assert.Equal(t, "Register a contract for tracking", trackCmd.Short)
}

// Ensure the root command can be initialized without errors.
func TestRootCommand_WithTrack(t *testing.T) {
	// This test ensures that trackCmd can be added to rootCmd without issues.
	// We create a fresh root command and add track to it.
	root := &cobra.Command{Use: "test"}
	root.AddCommand(trackCmd)

	assert.Len(t, root.Commands(), 1)
	assert.Equal(t, trackCmd, root.Commands()[0])
}

func TestTrackCommandErrorNon2xx(t *testing.T) {
	contractID := "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{
				"code":    "NOT_FOUND",
				"message": "contract not found",
			},
		})
	}))
	defer srv.Close()

	rootCmd.SetArgs([]string{"track", contractID, "--api-url", srv.URL, "--no-color"})
	_, err := captureOutput(t, func() error {
		return rootCmd.Execute()
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// Failures must surface only through the returned error, which Execute prints
// once — printing inside runTrack would emit the same error twice.
func TestTrackCommandErrorPrintedOnce(t *testing.T) {
	contractID := "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{
				"code":    "NOT_FOUND",
				"message": "contract not found",
			},
		})
	}))
	defer srv.Close()

	rootCmd.SetArgs([]string{"track", contractID, "--api-url", srv.URL})
	stderr, err := captureErrOutput(t, rootCmd.Execute)
	require.Error(t, err)
	assert.Empty(t, stderr, "runTrack must not print the error itself")
	assert.Contains(t, err.Error(), "NOT_FOUND")
	assert.Contains(t, err.Error(), "contract not found")
}

// The unknown-preset path must also report the failure only via the returned
// error, keeping the --help hint.
func TestTrackCommandPresetErrorPrintedOnce(t *testing.T) {
	contractID := "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	rootCmd.SetArgs([]string{"track", contractID, "--preset", "bogus"})
	stderr, err := captureErrOutput(t, rootCmd.Execute)
	require.Error(t, err)
	assert.Empty(t, stderr, "runTrack must not print the error itself")
	assert.Contains(t, err.Error(), `unknown preset type "bogus"`)
	assert.Contains(t, err.Error(), "track --help")
}
