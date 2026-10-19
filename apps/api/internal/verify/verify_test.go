package verify

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

const counterBuildScript = `mkdir -p "$CARGO_TARGET_DIR/wasm32v1-none/release" && printf 'wasm-bytes' > "$CARGO_TARGET_DIR/wasm32v1-none/release/counter.wasm"`

// tarGz builds an in-memory .tar.gz fixture from a name -> content map.
func tarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, name := range names {
		body := files[name]
		hdr := &tar.Header{
			Name:     name,
			Mode:     0o644,
			Size:     int64(len(body)),
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func counterSource(t *testing.T) []byte {
	t.Helper()
	return tarGz(t, map[string]string{
		"counter/Cargo.toml": "[package]\nname = \"counter\"\nversion = \"0.1.0\"\nedition = \"2021\"\n",
		"counter/src/lib.rs": "pub fn hello() -> u32 { 1 }\n",
	})
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	return NewService(Options{
		BuildCommand: []string{"sh", "-c", counterBuildScript},
		Timeout:      30 * time.Second,
		WorkspaceDir: t.TempDir(),
	})
}

func TestVerifyArchiveMatchesOnChainHash(t *testing.T) {
	want := sha256Hex([]byte("wasm-bytes"))

	res, err := newTestService(t).Verify(context.Background(), Request{
		ContractID:  "CCOUNTER",
		OnChainHash: strings.ToUpper(want), // comparison must be case-insensitive
		Source: Source{
			Kind:     SourceArchive,
			Filename: "counter.tar.gz",
			Content:  counterSource(t),
		},
	})
	if err != nil {
		t.Fatalf("Verify returned internal error: %v", err)
	}
	if res.Status != StatusVerified || !res.Matched {
		t.Fatalf("status=%q matched=%v diagnostics=%v", res.Status, res.Matched, res.Diagnostics)
	}
	if res.CompiledHash != want {
		t.Errorf("compiled hash = %s, want %s", res.CompiledHash, want)
	}
	if res.SourceDigest == "" {
		t.Error("source digest was not recorded")
	}
	if res.SourceKind != SourceArchive {
		t.Errorf("source kind = %q, want %q", res.SourceKind, SourceArchive)
	}
	if len(res.Diagnostics) != 0 {
		t.Errorf("expected no diagnostics, got %v", res.Diagnostics)
	}
}

func TestVerifyArchiveHashMismatchIsActionable(t *testing.T) {
	res, err := newTestService(t).Verify(context.Background(), Request{
		ContractID:  "CCOUNTER",
		OnChainHash: strings.Repeat("ab", 32),
		Source: Source{
			Kind:     SourceArchive,
			Filename: "counter.tar.gz",
			Content:  counterSource(t),
		},
	})
	if err != nil {
		t.Fatalf("Verify returned internal error: %v", err)
	}
	if res.Status != StatusFailed || res.Matched {
		t.Fatalf("status=%q matched=%v, want failed", res.Status, res.Matched)
	}
	if res.CompiledHash == "" {
		t.Fatal("compiled hash should still be reported on a mismatch")
	}
	if len(res.Diagnostics) != 1 || res.Diagnostics[0].Code != "HASH_MISMATCH" {
		t.Fatalf("diagnostics = %v, want a single HASH_MISMATCH", res.Diagnostics)
	}
	if res.Diagnostics[0].Hint == "" {
		t.Error("HASH_MISMATCH diagnostic should carry a hint")
	}
}

func TestVerifyBuildFailureReportsCompilerRule(t *testing.T) {
	svc := NewService(Options{
		BuildCommand: []string{"sh", "-c", `echo "error: package requires rustc 1.90.0 or newer" >&2; exit 1`},
		Timeout:      30 * time.Second,
		WorkspaceDir: t.TempDir(),
	})
	res, err := svc.Verify(context.Background(), Request{
		ContractID:  "CCOUNTER",
		OnChainHash: strings.Repeat("ab", 32),
		Source: Source{
			Kind:     SourceArchive,
			Filename: "counter.tar.gz",
			Content:  counterSource(t),
		},
	})
	if err != nil {
		t.Fatalf("Verify returned internal error: %v", err)
	}
	if res.Status != StatusFailed {
		t.Fatalf("status = %q, want failed", res.Status)
	}
	if len(res.Diagnostics) != 1 || res.Diagnostics[0].Code != "COMPILER_VERSION_MISMATCH" {
		t.Fatalf("diagnostics = %v, want COMPILER_VERSION_MISMATCH", res.Diagnostics)
	}
	if !strings.Contains(res.BuildLog, "requires rustc") {
		t.Errorf("build log was not captured: %q", res.BuildLog)
	}
}

func TestVerifyMissingToolchainIsReported(t *testing.T) {
	svc := NewService(Options{
		BuildCommand: []string{"definitely-not-a-real-stellar-binary-263"},
		Timeout:      30 * time.Second,
		WorkspaceDir: t.TempDir(),
	})
	res, err := svc.Verify(context.Background(), Request{
		ContractID:  "CCOUNTER",
		OnChainHash: strings.Repeat("ab", 32),
		Source: Source{
			Kind:     SourceArchive,
			Filename: "counter.tar.gz",
			Content:  counterSource(t),
		},
	})
	if err != nil {
		t.Fatalf("Verify returned internal error: %v", err)
	}
	if len(res.Diagnostics) != 1 || res.Diagnostics[0].Code != "TOOLCHAIN_MISSING" {
		t.Fatalf("diagnostics = %v, want TOOLCHAIN_MISSING", res.Diagnostics)
	}
}

func TestVerifyUnknownOnChainHashShortCircuits(t *testing.T) {
	res, err := NewService(Options{WorkspaceDir: t.TempDir()}).Verify(context.Background(), Request{
		ContractID: "CCOUNTER",
		Source:     Source{Kind: SourceArchive, Content: []byte("PK")},
	})
	if err != nil {
		t.Fatalf("Verify returned internal error: %v", err)
	}
	if res.Status != StatusFailed {
		t.Fatalf("status = %q, want failed", res.Status)
	}
	if len(res.Diagnostics) != 1 || res.Diagnostics[0].Code != "ONCHAIN_HASH_UNKNOWN" {
		t.Fatalf("diagnostics = %v, want ONCHAIN_HASH_UNKNOWN", res.Diagnostics)
	}
}

func TestVerifyAmbiguousCratesAskForSubdir(t *testing.T) {
	archive := tarGz(t, map[string]string{
		"a/Cargo.toml": "[package]\nname = \"a\"\n",
		"b/Cargo.toml": "[package]\nname = \"b\"\n",
	})
	res, err := newTestService(t).Verify(context.Background(), Request{
		ContractID:  "CCOUNTER",
		OnChainHash: strings.Repeat("ab", 32),
		Source:      Source{Kind: SourceArchive, Filename: "multi.tar.gz", Content: archive},
	})
	if err != nil {
		t.Fatalf("Verify returned internal error: %v", err)
	}
	if len(res.Diagnostics) != 1 || res.Diagnostics[0].Code != "SOURCE_MANIFEST_NOT_FOUND" {
		t.Fatalf("diagnostics = %v, want SOURCE_MANIFEST_NOT_FOUND", res.Diagnostics)
	}
}

func TestExtractArchiveRejectsTraversal(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("../../evil.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("pwned")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	dest := t.TempDir()
	if _, err := extractArchive(dest, buf.Bytes(), 1<<20); err == nil {
		t.Fatal("expected the escaping archive entry to be rejected")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dest), "evil.txt")); err == nil {
		t.Fatal("archive entry escaped the destination directory")
	}
}

func TestLocateCrateDisambiguatesWithSubdir(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"alpha", "beta"} {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname = \""+name+"\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := locateCrate(root, ""); err == nil {
		t.Fatal("expected an ambiguity error for two sibling crates")
	}
	got, err := locateCrate(root, "beta")
	if err != nil {
		t.Fatalf("locateCrate with subdir: %v", err)
	}
	if filepath.Base(got) != "beta" {
		t.Errorf("located %q, want .../beta", got)
	}
}

func TestPackageNameParsesCargoManifest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Cargo.toml")
	body := "[package]\nname = \"counter-contract\" # the deployed crate\nversion = \"0.1.0\"\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := packageName(path); got != "counter-contract" {
		t.Errorf("packageName = %q, want counter-contract", got)
	}
}

func TestNormalizeHash(t *testing.T) {
	valid := strings.Repeat("AB", 32)
	if got := normalizeHash(" 0x" + valid + " "); got != strings.ToLower(valid) {
		t.Errorf("normalizeHash = %q, want %q", got, strings.ToLower(valid))
	}
	if got := normalizeHash("not-a-hash"); got != "" {
		t.Errorf("normalizeHash(invalid) = %q, want empty", got)
	}
}

func TestValidateGitURL(t *testing.T) {
	allowed := []string{"https"}
	if err := validateGitURL("https://github.com/sorolens/sorolens", allowed); err != nil {
		t.Errorf("https URL should be allowed: %v", err)
	}
	for _, bad := range []string{
		"http://github.com/sorolens/sorolens",
		"file:///etc/passwd",
		"https://user:token@github.com/sorolens/sorolens",
		"not a url",
	} {
		if err := validateGitURL(bad, allowed); err == nil {
			t.Errorf("validateGitURL(%q) should fail", bad)
		}
	}
}

func TestFindArtifactPrefersCrateName(t *testing.T) {
	target := t.TempDir()
	release := filepath.Join(target, "wasm32v1-none", "release")
	if err := os.MkdirAll(release, 0o755); err != nil {
		t.Fatal(err)
	}
	crateDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(crateDir, "Cargo.toml"), []byte("[package]\nname = \"counter\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"counter.wasm", "helper.wasm"} {
		if err := os.WriteFile(filepath.Join(release, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(target, "stray.wasm"), []byte("stray"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := findArtifact(target, crateDir, "")
	if err != nil {
		t.Fatalf("findArtifact: %v", err)
	}
	if filepath.Base(got) != "counter.wasm" {
		t.Errorf("findArtifact = %q, want counter.wasm", got)
	}
}
