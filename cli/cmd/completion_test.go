package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestCompletionCommandRegistered(t *testing.T) {
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Name() == "completion" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("completion command is not registered on the root command")
	}
	if completionCmd.Use != "completion [bash|zsh|fish|powershell]" {
		t.Fatalf("unexpected usage: %q", completionCmd.Use)
	}
	want := []string{"bash", "zsh", "fish", "powershell"}
	if len(completionCmd.ValidArgs) != len(want) {
		t.Fatalf("unexpected valid args: %v", completionCmd.ValidArgs)
	}
	for i, shell := range want {
		if completionCmd.ValidArgs[i] != shell {
			t.Fatalf("unexpected valid args: %v", completionCmd.ValidArgs)
		}
	}
}

func TestCompletionCommandGeneratesScripts(t *testing.T) {
	markers := map[string]string{
		"bash":       "# bash completion V2 for sorolens",
		"zsh":        "#compdef sorolens",
		"fish":       "# fish completion for sorolens",
		"powershell": "# powershell completion for sorolens",
	}
	for shell, marker := range markers {
		var buf bytes.Buffer
		completionCmd.SetOut(&buf)
		if err := completionCmd.RunE(completionCmd, []string{shell}); err != nil {
			t.Fatalf("completion %s failed: %v", shell, err)
		}
		out := buf.String()
		if len(out) == 0 {
			t.Fatalf("completion %s produced empty output", shell)
		}
		if !strings.Contains(out, marker) {
			t.Fatalf("completion %s output missing marker %q", shell, marker)
		}
	}
}

func TestCompletionCommandRejectsBadArgs(t *testing.T) {
	if err := completionCmd.Args(completionCmd, []string{}); err == nil {
		t.Fatal("completion command accepted zero arguments")
	}
	if err := completionCmd.Args(completionCmd, []string{"bash", "zsh"}); err == nil {
		t.Fatal("completion command accepted two arguments")
	}
	if err := completionCmd.Args(completionCmd, []string{"tcsh"}); err == nil {
		t.Fatal("completion command accepted an unsupported shell")
	}
	if err := completionCmd.RunE(completionCmd, []string{"tcsh"}); err == nil {
		t.Fatal("completion command generated a script for an unsupported shell")
	}
}
