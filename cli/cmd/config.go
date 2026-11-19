package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type fileConfig struct {
	APIURL       string `yaml:"api_url"`
	Network      string `yaml:"network"`
	Output       string `yaml:"output"`
	OutputFormat string `yaml:"output_format"`
}

func applyConfig(cmd *cobra.Command, _ []string) error {
	config, err := loadUserConfig()
	if err != nil {
		return err
	}

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
	if globalConfig.Output != "table" && globalConfig.Output != "json" {
		return fmt.Errorf("invalid output format %q: must be table or json", globalConfig.Output)
	}
	globalConfig.JSON = globalConfig.Output == "json"
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
