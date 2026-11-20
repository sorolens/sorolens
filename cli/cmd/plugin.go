package cmd

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/sorolens/sorolens/cli/internal/format"
	"github.com/spf13/cobra"
)

// pluginPrefix is the executable name prefix for external subcommands:
// `sorolens foo` runs `sorolens-foo` found on PATH (git/cargo style).
const pluginPrefix = "sorolens-"

// pluginAPIVersion is the version of the environment contract documented in
// docs/cli-plugins.md. It is bumped only on incompatible changes.
const pluginAPIVersion = "1"

// pluginNamePattern restricts plugin names so a subcommand can never contain a
// path separator or shell metacharacter.
var pluginNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

var passAPIKeyToPlugin bool

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Inspect external subcommand plugins",
	Long: `External subcommands are executables named sorolens-<name> on PATH. Running
'sorolens <name> [args...]' dispatches to them when <name> is not a built-in
command. See docs/cli-plugins.md for the environment contract.`,
}

var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "List plugins found on PATH (nothing is executed)",
	Args:  cobra.NoArgs,
	RunE:  runPluginList,
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&passAPIKeyToPlugin, "plugin-api-key", false,
		"When dispatching to a plugin, pass the active profile's API key as SOROLENS_API_KEY (off by default)")
	pluginCmd.AddCommand(pluginListCmd)
	rootCmd.AddCommand(pluginCmd)
}

// pluginInfo describes one discovered plugin executable.
type pluginInfo struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Shadowed bool   `json:"shadows_builtin"`
}

// builtinNames returns every built-in command name and alias.
func builtinNames() map[string]bool {
	names := map[string]bool{"help": true, "completion": true}
	for _, c := range rootCmd.Commands() {
		names[c.Name()] = true
		for _, a := range c.Aliases {
			names[a] = true
		}
	}
	return names
}

// isExecutable reports whether the directory entry is a runnable file. It
// only inspects metadata; it never runs the file.
func isExecutable(path string, info fs.FileInfo) bool {
	if info.IsDir() || !info.Mode().IsRegular() {
		return false
	}
	if runtime.GOOS == "windows" {
		switch strings.ToLower(filepath.Ext(path)) {
		case ".exe", ".bat", ".cmd", ".com":
			return true
		}
		return false
	}
	return info.Mode().Perm()&0o111 != 0
}

// discoverPlugins scans pathEnv for sorolens-* executables. The first match
// on PATH wins. Nothing is executed.
func discoverPlugins(pathEnv string) []pluginInfo {
	builtins := builtinNames()
	seen := map[string]bool{}
	var out []pluginInfo
	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			file := e.Name()
			if !strings.HasPrefix(strings.ToLower(file), pluginPrefix) {
				continue
			}
			full := filepath.Join(dir, file)
			info, err := os.Stat(full) // follows symlinks
			if err != nil || !isExecutable(full, info) {
				continue
			}
			name := file[len(pluginPrefix):]
			if runtime.GOOS == "windows" {
				name = strings.TrimSuffix(name, filepath.Ext(name))
			}
			if !pluginNamePattern.MatchString(name) || seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, pluginInfo{Name: name, Path: full, Shadowed: builtins[name]})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func runPluginList(_ *cobra.Command, _ []string) error {
	plugins := discoverPlugins(os.Getenv("PATH"))
	for _, p := range plugins {
		if p.Shadowed {
			fmt.Fprintf(os.Stderr, "warning: %s shadows the built-in command %q and will never be run\n", p.Path, p.Name)
		}
	}
	if globalConfig.JSON {
		if plugins == nil {
			plugins = []pluginInfo{}
		}
		return format.PrintJSON(plugins)
	}
	if len(plugins) == 0 {
		fmt.Println("No plugins found. Install an executable named sorolens-<name> on PATH.")
		return nil
	}
	rows := make([][]string, len(plugins))
	for i, p := range plugins {
		note := ""
		if p.Shadowed {
			note = "ignored (shadows built-in)"
		}
		rows[i] = []string{p.Name, p.Path, note}
	}
	fmt.Println(format.RenderTable([]string{"Name", "Path", "Note"}, rows))
	return nil
}

// pluginEnv builds the environment for a plugin: the parent environment
// without any inherited API key, plus the documented SOROLENS_* contract.
// The API key is only included when explicitly requested.
func pluginEnv(parent []string, apiKey string) []string {
	env := make([]string, 0, len(parent)+8)
	for _, kv := range parent {
		k := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			k = kv[:i]
		}
		switch strings.ToUpper(k) {
		case "SOROLENS_API_KEY", "SOROLENS_API_URL", "SOROLENS_NETWORK", "SOROLENS_PROFILE",
			"SOROLENS_OUTPUT", "SOROLENS_OUTPUT_FORMAT", "SOROLENS_PLUGIN_API_VERSION":
			continue
		}
		env = append(env, kv)
	}
	env = append(env,
		"SOROLENS_PLUGIN_API_VERSION="+pluginAPIVersion,
		"SOROLENS_API_URL="+globalConfig.APIURL,
		"SOROLENS_NETWORK="+globalConfig.Network,
		"SOROLENS_OUTPUT="+globalConfig.Output,
	)
	if globalConfig.Profile != "" {
		env = append(env, "SOROLENS_PROFILE="+globalConfig.Profile)
	}
	if apiKey != "" {
		env = append(env, "SOROLENS_API_KEY="+apiKey)
	}
	return env
}

// unknownCommandName returns the first positional argument that cobra did not
// recognise as a built-in command, or "" when args do not name a command.
// Global flags may precede it (`sorolens --profile prod foo`).
func unknownCommandName(args []string) (name string, index int) {
	flags := rootCmd.PersistentFlags()
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			return "", -1
		}
		if strings.HasPrefix(a, "-") {
			if strings.HasPrefix(a, "--") && !strings.Contains(a, "=") {
				if f := flags.Lookup(strings.TrimPrefix(a, "--")); f != nil && f.NoOptDefVal == "" {
					i++ // skip the flag's value
				}
			}
			continue
		}
		return a, i
	}
	return "", -1
}

// tryPlugin dispatches `sorolens <name> ...` to sorolens-<name> when <name> is
// not a built-in command. handled is false when args do not name an unknown
// command, so normal cobra handling proceeds.
func tryPlugin(args []string, stdout, stderr io.Writer) (handled bool, exitCode int, err error) {
	name, idx := unknownCommandName(args)
	if name == "" || builtinNames()[name] {
		return false, 0, nil
	}
	if sub, _, ferr := rootCmd.Find(args); ferr == nil && sub != rootCmd {
		return false, 0, nil
	}
	if !pluginNamePattern.MatchString(name) {
		return false, 0, nil // let cobra report an unknown command
	}
	path, lerr := exec.LookPath(pluginPrefix + name)
	if lerr != nil {
		return true, 1, fmt.Errorf("unknown command %q: no built-in command or plugin %q found on PATH (try 'sorolens plugin list' or 'sorolens --help')", name, pluginPrefix+name)
	}

	// Resolve global flags and config that precede the plugin name so the
	// plugin receives the effective API URL, network, profile and format.
	if perr := rootCmd.ParseFlags(args[:idx]); perr != nil {
		return true, 1, perr
	}
	if aerr := applyConfig(rootCmd, nil); aerr != nil {
		return true, 1, aerr
	}
	apiKey := ""
	if passAPIKeyToPlugin && globalConfig.Profile != "" {
		if k, kerr := newSecretStore().Get(globalConfig.Profile); kerr == nil {
			apiKey = k
		}
	}

	// Arguments are passed as a vector: no shell is involved.
	c := exec.Command(path, args[idx+1:]...)
	c.Stdin = os.Stdin
	c.Stdout = stdout
	c.Stderr = stderr
	c.Env = pluginEnv(os.Environ(), apiKey)
	if rerr := c.Run(); rerr != nil {
		var ee *exec.ExitError
		if errors.As(rerr, &ee) {
			return true, ee.ExitCode(), nil
		}
		return true, 1, fmt.Errorf("run plugin %s: %w", pluginPrefix+name, rerr)
	}
	return true, 0, nil
}
