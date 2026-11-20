package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sorolens/sorolens/cli/internal/format"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type fileConfig struct {
	APIURL       string `yaml:"api_url"`
	Network      string `yaml:"network"`
	Output       string `yaml:"output"`
	OutputFormat string `yaml:"output_format"`

	// DefaultProfile names the profile used when --profile and
	// SOROLENS_PROFILE are not set. Profiles are optional: a config file with
	// only the top-level fields above keeps working unchanged.
	DefaultProfile string                   `yaml:"default_profile,omitempty"`
	Profiles       map[string]profileConfig `yaml:"profiles,omitempty"`
}

// profileConfig holds one named profile. API keys are never stored here, only
// where they live (KeyRef: "keyring" or "file").
type profileConfig struct {
	APIURL  string `yaml:"api_url,omitempty"`
	Network string `yaml:"network,omitempty"`
	KeyRef  string `yaml:"key_ref,omitempty"`
}

func applyConfig(cmd *cobra.Command, _ []string) error {
	config, err := loadUserConfig()
	if err != nil {
		return err
	}

	// SOROLENS_PROFILE is the profile used when --profile is not given; it
	// overrides the config file's default profile. A profile overrides the
	// legacy top-level api_url and network values.
	profileName := firstNonEmpty(globalConfig.Profile, os.Getenv("SOROLENS_PROFILE"), config.DefaultProfile)
	prof, found := config.Profiles[profileName]
	if profileName != "" && !found {
		return fmt.Errorf("unknown profile %q (see 'sorolens config profile list')", profileName)
	}
	globalConfig.Profile = profileName
	config.APIURL = firstNonEmpty(prof.APIURL, config.APIURL)
	config.Network = firstNonEmpty(prof.Network, config.Network)

	if !cmd.Flags().Changed("api-url") {
		globalConfig.APIURL = firstNonEmpty(os.Getenv("SOROLENS_API_URL"), config.APIURL, "http://localhost:8080")
	}
	if !cmd.Flags().Changed("network") {
		// SOROLENS_NETWORK is the default network for commands when --network is not
		// given and the config file sets none; defaults to testnet.
		globalConfig.Network = firstNonEmpty(os.Getenv("SOROLENS_NETWORK"), config.Network, "testnet")
	}

	if cmd.Flags().Changed("json") {
		globalConfig.Output = "table"
		if globalConfig.JSON {
			globalConfig.Output = "json"
		}
	} else if cmd.Flags().Changed("output") {
		globalConfig.Output = strings.ToLower(strings.TrimSpace(globalConfig.Output))
	} else {
		globalConfig.Output = strings.ToLower(firstNonEmpty(
			// SOROLENS_OUTPUT is the default output format when --output is not given.
			os.Getenv("SOROLENS_OUTPUT"),
			// SOROLENS_OUTPUT_FORMAT is a legacy alias of SOROLENS_OUTPUT.
			os.Getenv("SOROLENS_OUTPUT_FORMAT"),
			config.Output,
			config.OutputFormat,
			"table",
		))
	}
	if !format.ValidFormat(globalConfig.Output) {
		return fmt.Errorf("invalid output format %q: must be table, json, yaml or jsonl", globalConfig.Output)
	}
	var query *format.Query
	if strings.TrimSpace(globalConfig.Query) != "" {
		q, err := format.ParseQuery(globalConfig.Query)
		if err != nil {
			return fmt.Errorf("invalid --query: %w", err)
		}
		query = q
		if globalConfig.Output == "table" {
			if cmd.Flags().Changed("output") {
				return fmt.Errorf("--query needs a structured output format: use --output json, yaml or jsonl")
			}
			// A query only makes sense on structured data, so it implies JSON
			// unless a structured format was chosen.
			globalConfig.Output = "json"
		}
	}
	globalConfig.JSON = globalConfig.Output != "table"
	format.SetOptions(format.Options{Format: globalConfig.Output, Query: query})
	return nil
}

func loadUserConfig() (fileConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return fileConfig{}, fmt.Errorf("find home directory: %w", err)
	}
	return loadConfigFile(filepath.Join(home, ".sorolens.yml"))
}

func loadConfigFile(path string) (fileConfig, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return fileConfig{}, nil
	}
	if err != nil {
		return fileConfig{}, fmt.Errorf("read config file: %w", err)
	}
	var config fileConfig
	if err := yaml.Unmarshal(b, &config); err != nil {
		return fileConfig{}, fmt.Errorf("parse config file: %w", err)
	}
	return config, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
