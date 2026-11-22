package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
