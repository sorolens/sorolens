package verify

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestVerifyCounterFixtureEndToEnd rebuilds the checked-in `contracts/counter`
// fixture with the real Stellar CLI and asserts the rebuild is reproducible by
// verifying it twice: the second run must match the hash produced by the first.
//
// It requires the `stellar` CLI and a Rust toolchain, so it skips on machines
// (and in CI images) that do not ship them rather than failing spuriously. This
// is the fixture-contract end-to-end test requested by issue #263.
func TestVerifyCounterFixtureEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("stellar"); err != nil {
		t.Skip("stellar CLI is not installed; skipping the fixture end-to-end verification")
	}
	fixture, err := counterFixtureDir()
	if err != nil {
		t.Skipf("counter fixture is not available: %v", err)
	}
	archive, err := tarGzDir(fixture)
	if err != nil {
		t.Fatalf("archive the counter fixture: %v", err)
	}

	svc := NewService(Options{WorkspaceDir: t.TempDir(), Timeout: 15 * time.Minute})
	ctx := context.Background()

	// Probe build: the on-chain hash is deliberately wrong so the build runs and
	// we learn the hash this toolchain produces for the fixture.
	probe, err := svc.Verify(ctx, Request{
		ContractID:  "CCOUNTERFIXTURE",
		OnChainHash: strings.Repeat("00", 32),
		Source:      Source{Kind: SourceArchive, Filename: "counter.tar.gz", Content: archive},
	})
	if err != nil {
		t.Fatalf("probe verification: %v", err)
	}
	if probe.CompiledHash == "" {
		t.Skipf("the counter fixture did not build in this environment (diagnostics: %v)", probe.Diagnostics)
	}

	second, err := svc.Verify(ctx, Request{
		ContractID:  "CCOUNTERFIXTURE",
		OnChainHash: probe.CompiledHash,
		Source:      Source{Kind: SourceArchive, Filename: "counter.tar.gz", Content: archive},
	})
	if err != nil {
		t.Fatalf("second verification: %v", err)
	}
	if second.Status != StatusVerified || !second.Matched {
		t.Fatalf("fixture rebuild is not reproducible: first=%s second=%s diagnostics=%v",
			probe.CompiledHash, second.CompiledHash, second.Diagnostics)
	}
}

// counterFixtureDir resolves the repository's checked-in counter contract from
// this test file's location: <root>/apps/api/internal/verify/fixture_test.go.
func counterFixtureDir() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("cannot locate the test source file")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
	dir := filepath.Join(root, "contracts", "counter")
	if _, err := os.Stat(filepath.Join(dir, "Cargo.toml")); err != nil {
		return "", err
	}
	return dir, nil
}

// tarGzDir packages a directory into an in-memory .tar.gz archive, naming
// entries relative to the directory's parent so the crate keeps its folder name.
func tarGzDir(dir string) ([]byte, error) {
	parent := filepath.Dir(dir)
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	walkErr := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "target", ".git":
				return filepath.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(parent, path)
		if relErr != nil {
			return relErr
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		hdr, hdrErr := tar.FileInfoHeader(info, "")
		if hdrErr != nil {
			return hdrErr
		}
		hdr.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		f, openErr := os.Open(path)
		if openErr != nil {
			return openErr
		}
		_, copyErr := io.Copy(tw, f)
		f.Close()
		return copyErr
	})
	if walkErr != nil {
		return nil, walkErr
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
