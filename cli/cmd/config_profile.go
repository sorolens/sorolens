package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/sorolens/sorolens/cli/internal/client"
	"github.com/sorolens/sorolens/cli/internal/format"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const legacyProfileName = "default"

var profileNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show and manage CLI configuration and profiles",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the resolved configuration (API keys are masked)",
	Args:  cobra.NoArgs,
	RunE:  runConfigShow,
}

var configProfileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage named profiles (API URL, network and API key)",
}

var (
	profileAddAPIURL   string
	profileAddNetwork  string
	profileAddKeyStdin bool
	profileAddUse      bool
)

var profileAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Create or update a profile",
	Long: `Create or update a named profile. The API key is read from standard input
with --api-key-stdin so it never appears in shell history or the process list,
and is stored in the OS keyring (a 0600 file with a warning when no keyring
is available).`,
	Example: `  printf '%s' "$KEY" | sorolens config profile add prod --api-url https://api.example.com --network mainnet --api-key-stdin --use`,
	Args:    cobra.ExactArgs(1),
	RunE:    runProfileAdd,
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List profiles",
	Args:  cobra.NoArgs,
	RunE:  runProfileList,
}

var profileUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Set the default profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfileUse,
}

var profileRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a profile and its stored API key",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfileRemove,
}

func init() {
	profileAddCmd.Flags().StringVar(&profileAddAPIURL, "api-url", "", "Sorolens API base URL for this profile")
	profileAddCmd.Flags().StringVar(&profileAddNetwork, "network", "", "Stellar network for this profile")
	profileAddCmd.Flags().BoolVar(&profileAddKeyStdin, "api-key-stdin", false, "Read the API key from standard input")
	profileAddCmd.Flags().BoolVar(&profileAddUse, "use", false, "Make this the default profile")
	configProfileCmd.AddCommand(profileAddCmd, profileListCmd, profileUseCmd, profileRemoveCmd)
	configCmd.AddCommand(configShowCmd, configProfileCmd)
	rootCmd.AddCommand(configCmd)
}

// newClient builds an API client for the active configuration and attaches
// the resolved API key, when one is set.
func newClient(timeout time.Duration) *client.Client {
	c := client.New(globalConfig.APIURL, timeout)
	if key := resolvedAPIKey(); key != "" {
		c = c.WithAPIKey(key)
	}
	return c
}

// resolvedAPIKey returns the API key for the current invocation, in
// precedence order: the global --api-key flag, the SOROLENS_API_KEY env var,
// or the api_key config-file entry (all resolved into globalConfig.APIKey by
// applyConfig), then the active profile's stored key.
func resolvedAPIKey() string {
	if globalConfig.APIKey != "" {
		return globalConfig.APIKey
	}
	if globalConfig.Profile != "" {
		if key, err := newSecretStore().Get(globalConfig.Profile); err == nil && key != "" {
			return key
		}
	}
	return ""
}

func userConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	return filepath.Join(home, ".sorolens.yml"), nil
}

func saveConfig(cfg fileConfig) error {
	path, err := userConfigPath()
	if err != nil {
		return err
	}
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	return writeFileAtomic(path, b)
}

// migrateLegacy moves the single-profile top-level api_url/network of an
// existing config file into a "default" profile the first time profiles are
// written. Configs that never use profiles are left untouched.
func migrateLegacy(cfg *fileConfig) {
	if len(cfg.Profiles) > 0 || (cfg.APIURL == "" && cfg.Network == "") {
		return
	}
	cfg.Profiles = map[string]profileConfig{legacyProfileName: {APIURL: cfg.APIURL, Network: cfg.Network}}
	if cfg.DefaultProfile == "" {
		cfg.DefaultProfile = legacyProfileName
	}
	cfg.APIURL, cfg.Network = "", ""
}

func runProfileAdd(cmd *cobra.Command, args []string) error {
	name := args[0]
	if !profileNamePattern.MatchString(name) {
		return fmt.Errorf("invalid profile name %q: use 1-64 letters, digits, '.', '_' or '-'", name)
	}
	cfg, err := loadUserConfig()
	if err != nil {
		return err
	}
	migrateLegacy(&cfg)
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]profileConfig{}
	}
	prof := cfg.Profiles[name]
	if profileAddAPIURL != "" {
		prof.APIURL = profileAddAPIURL
	}
	if profileAddNetwork != "" {
		prof.Network = profileAddNetwork
	}
	if profileAddKeyStdin {
		raw, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return fmt.Errorf("read API key: %w", err)
		}
		key := strings.TrimSpace(string(raw))
		if key == "" {
			return errors.New("no API key received on standard input")
		}
		loc, err := newSecretStore().Set(name, key)
		if err != nil {
			return fmt.Errorf("store API key: %w", err)
		}
		prof.KeyRef = loc
	}
	cfg.Profiles[name] = prof
	if profileAddUse || cfg.DefaultProfile == "" {
		cfg.DefaultProfile = name
	}
	if err := saveConfig(cfg); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "Saved profile %q\n", name)
	return nil
}

type profileRow struct {
	Name    string `json:"name"`
	APIURL  string `json:"api_url"`
	Network string `json:"network"`
	HasKey  bool   `json:"has_key"`
	Default bool   `json:"default"`
}

func runProfileList(_ *cobra.Command, _ []string) error {
	cfg, err := loadUserConfig()
	if err != nil {
		return err
	}
	migrateLegacy(&cfg) // in memory only: shows a legacy config as "default"
	names := make([]string, 0, len(cfg.Profiles))
	for n := range cfg.Profiles {
		names = append(names, n)
	}
	sort.Strings(names)
	rows := make([]profileRow, 0, len(names))
	for _, n := range names {
		p := cfg.Profiles[n]
		rows = append(rows, profileRow{Name: n, APIURL: p.APIURL, Network: p.Network, HasKey: p.KeyRef != "", Default: n == cfg.DefaultProfile})
	}
	if globalConfig.JSON {
		return format.PrintJSON(rows)
	}
	if len(rows) == 0 {
		fmt.Println("No profiles configured. Add one with 'sorolens config profile add <name>'.")
		return nil
	}
	table := make([][]string, len(rows))
	for i, r := range rows {
		mark, key := "", "no"
		if r.Default {
			mark = "*"
		}
		if r.HasKey {
			key = "yes"
		}
		table[i] = []string{mark, r.Name, r.APIURL, r.Network, key}
	}
	fmt.Println(format.RenderTable([]string{"", "Name", "API URL", "Network", "API Key"}, table))
	return nil
}

func runProfileUse(_ *cobra.Command, args []string) error {
	cfg, err := loadUserConfig()
	if err != nil {
		return err
	}
	migrateLegacy(&cfg)
	if _, ok := cfg.Profiles[args[0]]; !ok {
		return fmt.Errorf("unknown profile %q", args[0])
	}
	cfg.DefaultProfile = args[0]
	if err := saveConfig(cfg); err != nil {
		return err
	}
	fmt.Printf("Default profile is now %q\n", args[0])
	return nil
}

func runProfileRemove(_ *cobra.Command, args []string) error {
	cfg, err := loadUserConfig()
	if err != nil {
		return err
	}
	migrateLegacy(&cfg)
	name := args[0]
	if _, ok := cfg.Profiles[name]; !ok {
		return fmt.Errorf("unknown profile %q", name)
	}
	if err := newSecretStore().Delete(name); err != nil {
		return fmt.Errorf("remove API key: %w", err)
	}
	delete(cfg.Profiles, name)
	if cfg.DefaultProfile == name {
		cfg.DefaultProfile = ""
	}
	if err := saveConfig(cfg); err != nil {
		return err
	}
	fmt.Printf("Removed profile %q\n", name)
	return nil
}

func runConfigShow(_ *cobra.Command, _ []string) error {
	key := "(none)"
	if resolvedAPIKey() != "" {
		key = "********"
	}
	profile := globalConfig.Profile
	if profile == "" {
		profile = "(none)"
	}
	out := map[string]string{
		"profile": profile,
		"api_url": globalConfig.APIURL,
		"network": globalConfig.Network,
		"output":  globalConfig.Output,
		"api_key": key,
	}
	if globalConfig.JSON {
		return format.PrintJSON(out)
	}
	fmt.Println(format.RenderKeyValue([][]string{
		{"Profile", profile}, {"API URL", out["api_url"]}, {"Network", out["network"]},
		{"Output", out["output"]}, {"API Key", key},
	}))
	return nil
}
