package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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

// Ensure the root command is initialized with the track subcommand.
func TestRootCommand_WithTrack(t *testing.T) {
	// trackCmd is registered on rootCmd in init(); assert membership without
	// re-parenting the shared command, which would strip its inherited flags.
	assert.Contains(t, rootCmd.Commands(), trackCmd)
}

// An API error response must surface as a command error so the process exits
// non-zero and scripts/CI can detect the failure.
func TestTrackCommand_APIErrorReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{
				"code":    "INVALID_INPUT",
				"message": "id must be a 56-character string starting with 'C'",
			},
		})
	}))
	defer srv.Close()

	rootCmd.SetArgs([]string{"track", "short-id", "--api-url", srv.URL})
	_, err := captureOutput(t, rootCmd.Execute)
	require.Error(t, err, "track must return a non-nil error when the API fails")
	assert.Contains(t, err.Error(), "INVALID_INPUT")
	assert.Contains(t, err.Error(), "56-character")
}

// An unknown --preset must surface as a command error rather than printing a
// message and exiting zero.
func TestTrackCommand_UnknownPresetReturnsError(t *testing.T) {
	rootCmd.SetArgs([]string{"track", contractsTestID, "--preset", "bogus"})
	_, err := captureOutput(t, rootCmd.Execute)
	require.Error(t, err, "track must return a non-nil error for an unknown preset")
	assert.Contains(t, err.Error(), `unknown preset type "bogus"`)
}
