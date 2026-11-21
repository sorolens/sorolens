package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sorolens/sorolens/cli/cmd"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func main() {
	if err := generate(); err != nil {
		fmt.Fprintln(os.Stderr, "generate CLI documentation:", err)
		os.Exit(1)
	}
}

func generate() error {
	root := cmd.RootCommand()
	root.DisableAutoGenTag = true
	if err := generateMarkdown(root, filepath.Join("..", "docs", "cli", "reference")); err != nil {
		return err
	}

	// Pin the man-page date so freshness checks don't vary by generation date.
	manDate := time.Date(2026, time.November, 20, 0, 0, 0, 0, time.UTC)
	manHeader := &doc.GenManHeader{
		Title:   "SOROLENS",
		Section: "1",
		Date:    &manDate,
	}
	return generateMan(root, manHeader, filepath.Join("..", "docs", "cli", "man"))
}

func generateMarkdown(root *cobra.Command, dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("clear Markdown output: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create Markdown output: %w", err)
	}
	if err := doc.GenMarkdownTree(root, dir); err != nil {
		return fmt.Errorf("generate Markdown: %w", err)
	}
	if err := normalizeMarkdown(dir); err != nil {
		return err
	}
	return nil
}

func normalizeMarkdown(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read Markdown output: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read generated Markdown %s: %w", entry.Name(), err)
		}
		content = append(bytes.TrimRight(content, "\n"), '\n')
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return fmt.Errorf("normalize generated Markdown %s: %w", entry.Name(), err)
		}
	}
	return nil
}

func generateMan(root *cobra.Command, header *doc.GenManHeader, dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("clear man page output: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create man page output: %w", err)
	}
	if err := doc.GenManTree(root, header, dir); err != nil {
		return fmt.Errorf("generate man pages: %w", err)
	}
	return nil
}
