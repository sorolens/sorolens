package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// writePlugin creates a fixture plugin that prints its arguments and the
// SOROLENS_* variables it received. It works on Linux, macOS and Windows.
func writePlugin(t *testing.T, dir, name string, exit int) string {
	t.Helper()
	var file, body string
	if runtime.GOOS == "windows" {
		file = pluginPrefix + name + ".bat"
		body = "@echo off\r\necho args=%*\r\necho url=%SOROLENS_API_URL%\r\necho net=%SOROLENS_NETWORK%\r\necho profile=%SOROLENS_PROFILE%\r\necho key=%SOROLENS_API_KEY%\r\necho ver=%SOROLENS_PLUGIN_API_VERSION%\r\nexit /b " + string(rune('0'+exit)) + "\r\n"
	} else {
		file = pluginPrefix + name
		body = "#!/bin/sh\necho \"args=$*\"\necho \"url=$SOROLENS_API_URL\"\necho \"net=$SOROLENS_NETWORK\"\necho \"profile=$SOROLENS_PROFILE\"\necho \"key=$SOROLENS_API_KEY\"\necho \"ver=$SOROLENS_PLUGIN_API_VERSION\"\nexit " + string(rune('0'+exit)) + "\n"
	}
	path := filepath.Join(dir, file)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func pluginTestEnv(t *testing.T) string {
	t.Helper()
	useTempHome(t)
	useFakeStore(t)
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("SOROLENS_API_KEY", "inherited-secret")
	globalConfig = Config{}
	passAPIKeyToPlugin = false
	resetCommandFlags(rootCmd)
	return dir
}

func TestPluginDispatchPassesContextNotKeys(t *testing.T) {
	dir := pluginTestEnv(t)
	writePlugin(t, dir, "hello", 0)

	var out, errb bytes.Buffer
	handled, code, err := tryPlugin([]string{"--api-url", "https://x.example", "--network", "mainnet", "hello", "a", "--flag", "b"}, &out, &errb)
	if err != nil || !handled || code != 0 {
		t.Fatalf("handled=%v code=%d err=%v stderr=%s", handled, code, err, errb.String())
	}
	got := out.String()
	for _, want := range []string{"args=a --flag b", "url=https://x.example", "net=mainnet", "ver=1"} {
		if !strings.Contains(got, want) {
			t.Errorf("plugin output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "inherited-secret") {
		t.Errorf("API key leaked to plugin by default:\n%s", got)
	}
}

func TestPluginReceivesKeyOnlyWhenRequested(t *testing.T) {
	dir := pluginTestEnv(t)
	writePlugin(t, dir, "hello", 0)
	store := useFakeStore(t)
	store.keys["prod"] = "profile-secret"
	cfgHome, _ := os.UserHomeDir()
	writeCfg(t, cfgHome, "profiles:\n  prod:\n    api_url: https://prod.example\n    network: mainnet\n")

	var out, errb bytes.Buffer
	if _, _, err := tryPlugin([]string{"--profile", "prod", "--plugin-api-key", "hello"}, &out, &errb); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "key=profile-secret") || !strings.Contains(got, "profile=prod") || !strings.Contains(got, "url=https://prod.example") {
		t.Errorf("explicit key request not honoured:\n%s", got)
	}
}

func TestPluginExitCodePropagates(t *testing.T) {
	dir := pluginTestEnv(t)
	writePlugin(t, dir, "failing", 3)
	var out, errb bytes.Buffer
	handled, code, err := tryPlugin([]string{"failing"}, &out, &errb)
	if !handled || code != 3 || err != nil {
		t.Fatalf("handled=%v code=%d err=%v", handled, code, err)
	}
}

func TestMissingPluginIsClearError(t *testing.T) {
	pluginTestEnv(t)
	var out, errb bytes.Buffer
	handled, code, err := tryPlugin([]string{"nope"}, &out, &errb)
	if !handled || code == 0 || err == nil || !strings.Contains(err.Error(), "sorolens-nope") {
		t.Fatalf("handled=%v code=%d err=%v", handled, code, err)
	}
}

func TestBuiltinsCannotBeShadowed(t *testing.T) {
	dir := pluginTestEnv(t)
	writePlugin(t, dir, "contracts", 0)
	var out, errb bytes.Buffer
	handled, _, _ := tryPlugin([]string{"contracts", "list"}, &out, &errb)
	if handled {
		t.Fatal("built-in command was dispatched to a plugin")
	}
	list := discoverPlugins(dir)
	if len(list) != 1 || !list[0].Shadowed {
		t.Fatalf("shadowing plugin should be flagged: %+v", list)
	}
}

func TestInvalidPluginNamesAndNonExecutables(t *testing.T) {
	dir := pluginTestEnv(t)
	var out, errb bytes.Buffer
	for _, name := range []string{"../evil", "a;b", "A$(x)"} {
		if handled, _, _ := tryPlugin([]string{name}, &out, &errb); handled {
			t.Errorf("%q should not be dispatched", name)
		}
	}
	// A directory and a non-executable file named sorolens-* are skipped.
	if err := os.Mkdir(filepath.Join(dir, "sorolens-dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sorolens-data.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := discoverPlugins(dir); len(got) != 0 {
		t.Fatalf("expected no plugins, got %+v", got)
	}
}

func TestDiscoveryDoesNotExecute(t *testing.T) {
	dir := pluginTestEnv(t)
	marker := filepath.Join(dir, "ran")
	var file, body string
	if runtime.GOOS == "windows" {
		file, body = "sorolens-touch.bat", "@echo off\r\ntype nul > \""+marker+"\"\r\n"
	} else {
		file, body = "sorolens-touch", "#!/bin/sh\n: > '"+marker+"'\n"
	}
	if err := os.WriteFile(filepath.Join(dir, file), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	rootCmd.SetArgs([]string{"plugin", "list"})
	out, err := captureOutput(t, rootCmd.Execute)
	if err != nil || !strings.Contains(out, "touch") {
		t.Fatalf("list failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("plugin was executed during discovery")
	}
}
