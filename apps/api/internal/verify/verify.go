// Package verify implements Soroban contract source verification: it
// materializes submitted source (an uploaded source archive or a pinned Git
// commit), runs a build in a throwaway workspace, and compares the SHA-256 of
// the resulting Wasm against the hash recorded for the contract on-chain.
//
// # Isolation
//
// The build runs as a child process in a per-request temporary directory with
// a minimal, deterministic environment, a hard timeout, and capped output, and
// the command is executed directly (never through a shell) so request data can
// never be interpreted as shell syntax. This is process-level isolation only:
// it is NOT a Firecracker microVM and NOT a container sandbox. A production
// deployment should run this service on a dedicated, network-restricted worker
// with a pinned toolchain image; the package deliberately has no dependency on
// such infrastructure so the endpoint can ship before the sandbox does.
//
// # Determinism
//
// The environment pins CARGO_INCREMENTAL=0, SOURCE_DATE_EPOCH=0, TZ=UTC and
// LC_ALL=C so two builds of the same source tend to be byte-identical. True
// reproducibility additionally requires a pinned Rust/Stellar toolchain and a
// pinned dependency set (Cargo.lock); the verifier reports toolchain versions
// and warns when they differ from the versions declared in the request.
package verify

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Source kinds accepted by Verify.
const (
	SourceArchive = "archive"
	SourceGit     = "git"
)

// Verification statuses.
const (
	StatusVerified = "verified"
	StatusFailed   = "failed"
)

// Diagnostic severities.
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
	SeverityInfo    = "info"
)

// SourceKind enumerates the supported source kinds.
type SourceKind string

// Source describes where the contract source comes from.
type Source struct {
	Kind SourceKind
	// Archive source fields.
	Filename string
	Content  []byte
	// Git source fields.
	GitURL    string
	GitCommit string
	// Subdir optionally points at the crate inside the materialized source
	// when the archive or repository contains more than one crate.
	Subdir string
}

// ExpectedToolchain lets a submitter declare the compiler versions the
// contract was originally built with. Mismatches are reported as warnings
// because they are the most common cause of a hash mismatch.
type ExpectedToolchain struct {
	StellarVersion string
	RustcVersion   string
}

// Request is one verification attempt.
type Request struct {
	ContractID  string
	OnChainHash string
	Source      Source
	// Artifact optionally names the built .wasm relative to the crate
	// directory, bypassing artifact discovery.
	Artifact    string
	Expected    ExpectedToolchain
	SubmittedAt time.Time
}

// Diagnostic is one actionable verification finding.
type Diagnostic struct {
	Code     string
	Severity string
	Message  string
	Hint     string
}

// Result is the outcome of a verification attempt. A non-nil error from Verify
// means the attempt could not be carried out at all (an internal failure);
// an actionable verdict — verified or failed — is always returned with a nil
// error so the caller can persist and return it.
type Result struct {
	Status         string
	Matched        bool
	OnChainHash    string
	CompiledHash   string
	SourceKind     string
	SourceRef      string
	SourceDigest   string
	StellarVersion string
	RustcVersion   string
	CargoVersion   string
	Diagnostics    []Diagnostic
	BuildLog       string
	SubmittedAt    time.Time
	VerifiedAt     time.Time
	Duration       time.Duration
}

// Options configures a Service. The zero value is usable: it builds with
// `stellar contract build`, a 10 minute timeout, an 8 MiB upload cap, an
// os.TempDir() workspace, and https-only Git sources.
type Options struct {
	// BuildCommand is the deterministic build invocation. It is executed
	// directly, never through a shell, so arguments cannot be injected.
	BuildCommand []string
	// Timeout bounds a single build.
	Timeout time.Duration
	// WorkspaceDir is the parent directory for per-request workspaces.
	// Empty means os.TempDir().
	WorkspaceDir string
	// MaxSourceBytes caps an uploaded archive.
	MaxSourceBytes int64
	// MaxLogBytes caps captured build output.
	MaxLogBytes int
	// GitSchemes lists the URL schemes allowed for Git sources.
	GitSchemes []string
	// Now is injectable for tests. Defaults to time.Now.
	Now func() time.Time
}

func (o Options) withDefaults() Options {
	if len(o.BuildCommand) == 0 {
		o.BuildCommand = []string{"stellar", "contract", "build"}
	}
	if o.Timeout <= 0 {
		o.Timeout = 10 * time.Minute
	}
	if o.MaxSourceBytes <= 0 {
		o.MaxSourceBytes = 8 << 20
	}
	if o.MaxLogBytes <= 0 {
		o.MaxLogBytes = 64 << 10
	}
	if len(o.GitSchemes) == 0 {
		o.GitSchemes = []string{"https"}
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return o
}

// Service runs contract source verification.
type Service struct {
	opts Options
}

// NewService returns a Service with the supplied options and defaults applied.
func NewService(opts Options) *Service {
	return &Service{opts: opts.withDefaults()}
}

// BuildCommand returns the configured build command after defaults are applied.
func (s *Service) BuildCommand() []string {
	return append([]string(nil), s.opts.BuildCommand...)
}

var (
	commitRe = regexp.MustCompile(`^[0-9a-fA-F]{7,64}$`)
	hashRe   = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// Verify runs one verification attempt. See the package documentation for the
// isolation and determinism caveats.
func (s *Service) Verify(ctx context.Context, req Request) (res Result, err error) {
	start := s.opts.Now()
	res = Result{
		Status:      StatusFailed,
		OnChainHash: normalizeHash(req.OnChainHash),
		SourceKind:  string(req.Source.Kind),
		Diagnostics: []Diagnostic{},
		SubmittedAt: req.SubmittedAt,
	}
	if res.SubmittedAt.IsZero() {
		res.SubmittedAt = start
	}
	defer func() {
		res.Duration = s.opts.Now().Sub(start)
		if res.VerifiedAt.IsZero() {
			res.VerifiedAt = s.opts.Now()
		}
	}()

	if res.OnChainHash == "" {
		res.Diagnostics = append(res.Diagnostics, Diagnostic{
			Code:     "ONCHAIN_HASH_UNKNOWN",
			Severity: SeverityError,
			Message:  "the contract's on-chain Wasm hash is unknown, so the build cannot be compared",
			Hint:     "wait for the indexer to record the deployed Wasm hash for this contract, then re-submit",
		})
		return res, nil
	}

	kind := strings.TrimSpace(string(req.Source.Kind))
	switch kind {
	case SourceArchive:
		if len(req.Source.Content) == 0 {
			return res, errors.New("source archive is empty")
		}
		if int64(len(req.Source.Content)) > s.opts.MaxSourceBytes {
			return res, fmt.Errorf("source archive is %d bytes, exceeding the %d byte limit",
				len(req.Source.Content), s.opts.MaxSourceBytes)
		}
	case SourceGit:
		if err := validateGitURL(strings.TrimSpace(req.Source.GitURL), s.opts.GitSchemes); err != nil {
			return res, err
		}
		if !commitRe.MatchString(strings.TrimSpace(req.Source.GitCommit)) {
			return res, fmt.Errorf("git commit %q must be a 7-64 character hex SHA", req.Source.GitCommit)
		}
	default:
		return res, fmt.Errorf("unsupported source kind %q (want %q or %q)",
			kind, SourceArchive, SourceGit)
	}

	workDir, err := os.MkdirTemp(s.opts.WorkspaceDir, "sorolens-verify-")
	if err != nil {
		return res, fmt.Errorf("create verification workspace: %w", err)
	}
	defer os.RemoveAll(workDir)

	srcRoot := filepath.Join(workDir, "src")
	if err := os.MkdirAll(srcRoot, 0o700); err != nil {
		return res, fmt.Errorf("create source root: %w", err)
	}

	switch kind {
	case SourceArchive:
		digest, extractErr := extractArchive(srcRoot, req.Source.Content, s.opts.MaxSourceBytes*20)
		if extractErr != nil {
			res.Diagnostics = append(res.Diagnostics, Diagnostic{
				Code:     "SOURCE_ARCHIVE_INVALID",
				Severity: SeverityError,
				Message:  extractErr.Error(),
				Hint:     "upload a .zip, .tar.gz or .tgz archive whose root (or a single wrapper directory) contains the contract's Cargo.toml",
			})
			return res, nil
		}
		res.SourceRef = strings.TrimSpace(req.Source.Filename)
		res.SourceDigest = digest
	case SourceGit:
		gitURL := strings.TrimSpace(req.Source.GitURL)
		gitCommit := strings.TrimSpace(req.Source.GitCommit)
		if gitErr := materializeGit(ctx, srcRoot, gitURL, gitCommit); gitErr != nil {
			res.Diagnostics = append(res.Diagnostics, Diagnostic{
				Code:     "SOURCE_GIT_FETCH_FAILED",
				Severity: SeverityError,
				Message:  gitErr.Error(),
				Hint:     "check that the repository is public, the pinned commit exists, and the URL uses an allowed scheme",
			})
			return res, nil
		}
		ref := gitURL + "@" + gitCommit
		res.SourceRef = ref
		res.SourceDigest = sha256Hex([]byte(ref))
	}

	crateDir, err := locateCrate(srcRoot, req.Source.Subdir)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, Diagnostic{
			Code:     "SOURCE_MANIFEST_NOT_FOUND",
			Severity: SeverityError,
			Message:  err.Error(),
			Hint:     "set source.subdir to the directory containing the contract's Cargo.toml",
		})
		return res, nil
	}

	res.StellarVersion = detectVersion(ctx, "stellar", "--version")
	res.RustcVersion = detectVersion(ctx, "rustc", "--version")
	res.CargoVersion = detectVersion(ctx, "cargo", "--version")
	res.Diagnostics = append(res.Diagnostics,
		expectedVersionDiagnostics(req.Expected, res.StellarVersion, res.RustcVersion)...)

	targetDir := filepath.Join(workDir, "target")
	log, buildErr := s.runBuild(ctx, crateDir, targetDir)
	res.BuildLog = log
	if buildErr != nil {
		res.Diagnostics = append(res.Diagnostics, classifyBuildFailure(log, buildErr)...)
		return res, nil
	}

	artifact, err := findArtifact(targetDir, crateDir, req.Artifact)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, Diagnostic{
			Code:     "BUILD_ARTIFACT_NOT_FOUND",
			Severity: SeverityError,
			Message:  err.Error(),
			Hint:     "set artifact to the path of the built .wasm, relative to the crate directory",
		})
		return res, nil
	}
	data, err := os.ReadFile(artifact)
	if err != nil {
		return res, fmt.Errorf("read built wasm: %w", err)
	}

	res.CompiledHash = sha256Hex(data)
	res.Matched = strings.EqualFold(res.CompiledHash, res.OnChainHash)
	if res.Matched {
		res.Status = StatusVerified
	} else {
		res.Diagnostics = append(res.Diagnostics, hashMismatchDiagnostic(res.OnChainHash, res.CompiledHash))
	}
	return res, nil
}

// ---- source materialization ------------------------------------------------

func extractArchive(dest string, content []byte, maxExtracted int64) (string, error) {
	digest := sha256Hex(content)
	switch {
	case len(content) >= 2 && content[0] == 'P' && content[1] == 'K':
		return digest, extractZip(dest, content, maxExtracted)
	case len(content) >= 2 && content[0] == 0x1f && content[1] == 0x8b:
		return digest, extractTarGz(dest, content, maxExtracted)
	default:
		return "", errors.New("unrecognized archive format: expected zip, tar.gz or tgz")
	}
}

func extractZip(dest string, content []byte, maxExtracted int64) error {
	zr, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return fmt.Errorf("read zip archive: %w", err)
	}
	var total int64
	for _, f := range zr.File {
		target, err := safeJoin(dest, f.Name)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
			continue
		}
		// Never materialize symlinks: they can point outside the workspace.
		if f.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("open %s: %w", f.Name, err)
		}
		n, copyErr := copyCapped(target, rc, maxExtracted-total)
		rc.Close()
		if copyErr != nil {
			return copyErr
		}
		total += n
		if total >= maxExtracted {
			return errors.New("archive expands beyond the allowed size")
		}
	}
	return nil
}

func extractTarGz(dest string, content []byte, maxExtracted int64) error {
	gz, err := gzip.NewReader(bytes.NewReader(content))
	if err != nil {
		return fmt.Errorf("read gzip stream: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	var total int64
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar stream: %w", err)
		}
		target, err := safeJoin(dest, hdr.Name)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return err
			}
			n, copyErr := copyCapped(target, tr, maxExtracted-total)
			if copyErr != nil {
				return copyErr
			}
			total += n
			if total >= maxExtracted {
				return errors.New("archive expands beyond the allowed size")
			}
		default:
			// Skip symlinks, hardlinks, devices and anything else that could
			// escape the workspace.
		}
	}
	return nil
}

func copyCapped(path string, r io.Reader, remaining int64) (int64, error) {
	if remaining <= 0 {
		return 0, errors.New("archive expands beyond the allowed size")
	}
	out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return 0, err
	}
	defer out.Close()
	return io.Copy(out, io.LimitReader(r, remaining))
}

// safeJoin resolves an archive entry against dest, rejecting absolute paths
// and any entry containing a ".." segment.
func safeJoin(dest, name string) (string, error) {
	normalized := strings.ReplaceAll(name, "\\", "/")
	if normalized == "" {
		return "", errors.New("archive contains an entry with an empty name")
	}
	if strings.HasPrefix(normalized, "/") {
		return "", fmt.Errorf("archive entry %q is an absolute path", name)
	}
	for _, seg := range strings.Split(normalized, "/") {
		if seg == ".." {
			return "", fmt.Errorf("archive entry %q contains a parent-directory segment", name)
		}
	}
	target := filepath.Join(dest, filepath.Clean(normalized))
	if !within(dest, target) {
		return "", fmt.Errorf("archive entry %q escapes the destination directory", name)
	}
	return target, nil
}

func validateGitURL(raw string, schemes []string) error {
	if raw == "" {
		return errors.New("git URL is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid git URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return errors.New("git URL must be absolute and include a host")
	}
	for _, allowed := range schemes {
		if u.Scheme != allowed {
			continue
		}
		if u.User != nil {
			return errors.New("git URL must not embed credentials")
		}
		return nil
	}
	return fmt.Errorf("git URL scheme %q is not allowed (allowed: %s)",
		u.Scheme, strings.Join(schemes, ", "))
}

func materializeGit(ctx context.Context, dir, gitURL, commit string) error {
	steps := [][]string{
		{"git", "init", "--quiet"},
		{"git", "remote", "add", "origin", gitURL},
		{"git", "fetch", "--quiet", "--depth", "1", "origin", commit},
		{"git", "checkout", "--quiet", "--detach", "FETCH_HEAD"},
	}
	for _, step := range steps {
		cmd := exec.CommandContext(ctx, step[0], step[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_TERMINAL_PROMPT=0",
			"GCM_INTERACTIVE=never",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("git %s failed: %v: %s", step[1], err, truncate(strings.TrimSpace(string(out)), 2000))
		}
	}
	return nil
}

// ---- crate + artifact discovery -------------------------------------------

// locateCrate finds the directory holding the contract's Cargo.toml. When the
// source wraps the repository in a single top-level directory (as GitHub
// archive tarballs do) it descends automatically; if several crates are found
// at the same shallowest depth it asks the caller to disambiguate via subdir.
func locateCrate(root, subdir string) (string, error) {
	if sub := strings.TrimSpace(subdir); sub != "" {
		clean := filepath.Join(root, filepath.Clean("/"+strings.ReplaceAll(sub, "\\", "/")))
		if !within(root, clean) {
			return "", fmt.Errorf("subdir %q escapes the source root", sub)
		}
		if fileExists(filepath.Join(clean, "Cargo.toml")) {
			return clean, nil
		}
		return "", fmt.Errorf("no Cargo.toml in subdir %q", sub)
	}
	if fileExists(filepath.Join(root, "Cargo.toml")) {
		return root, nil
	}
	candidates := shallowManifests(root, 4, 8)
	switch len(candidates) {
	case 0:
		return "", errors.New("no Cargo.toml found in the submitted source")
	case 1:
		return candidates[0], nil
	default:
		rel := make([]string, 0, len(candidates))
		for _, c := range candidates {
			r, relErr := filepath.Rel(root, c)
			if relErr != nil {
				r = c
			}
			rel = append(rel, r)
		}
		return "", fmt.Errorf("multiple candidate crates found (%s); set source.subdir",
			strings.Join(rel, ", "))
	}
}

// shallowManifests returns the directories containing a Cargo.toml at the
// shallowest depth found, up to maxDepth, ignoring build output.
func shallowManifests(root string, maxDepth, limit int) []string {
	var all []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			if d.Name() == "Cargo.toml" {
				all = append(all, filepath.Dir(path))
			}
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil || rel == "." {
			return nil
		}
		if len(strings.Split(rel, string(os.PathSeparator))) > maxDepth {
			return filepath.SkipDir
		}
		switch d.Name() {
		case "target", ".git", ".cargo", "node_modules":
			return filepath.SkipDir
		}
		return nil
	})
	if len(all) == 0 {
		return nil
	}
	sort.Slice(all, func(i, j int) bool {
		return pathDepth(root, all[i]) < pathDepth(root, all[j])
	})
	if len(all) > limit {
		all = all[:limit]
	}
	min := pathDepth(root, all[0])
	out := make([]string, 0, len(all))
	for _, p := range all {
		if pathDepth(root, p) != min {
			break
		}
		out = append(out, p)
	}
	return out
}

func pathDepth(root, p string) int {
	rel, err := filepath.Rel(root, p)
	if err != nil || rel == "." {
		return 0
	}
	return len(strings.Split(rel, string(os.PathSeparator)))
}

// findArtifact locates the built .wasm. It prefers the artifact named after
// the crate in the standard Soroban release directories before falling back to
// a search of the target tree.
func findArtifact(targetDir, crateDir, explicit string) (string, error) {
	if e := strings.TrimSpace(explicit); e != "" {
		candidates := []string{e}
		if !filepath.IsAbs(e) {
			candidates = []string{filepath.Join(crateDir, e), filepath.Join(targetDir, e)}
		}
		for _, c := range candidates {
			if fileExists(c) {
				return c, nil
			}
		}
		return "", fmt.Errorf("artifact %q was not produced by the build", e)
	}

	crateName := packageName(filepath.Join(crateDir, "Cargo.toml"))
	releaseDirs := []string{
		filepath.Join(targetDir, "wasm32v1-none", "release"),
		filepath.Join(targetDir, "wasm32-unknown-unknown", "release"),
	}
	if crateName != "" {
		for _, dir := range releaseDirs {
			for _, stem := range []string{crateName, strings.ReplaceAll(crateName, "-", "_")} {
				p := filepath.Join(dir, stem+".wasm")
				if fileExists(p) {
					return p, nil
				}
			}
		}
	}

	var found []string
	_ = filepath.WalkDir(targetDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path == targetDir {
				return nil
			}
			switch d.Name() {
			case "deps", "build", ".fingerprint", "incremental":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".wasm") {
			found = append(found, path)
		}
		return nil
	})
	sort.Strings(found)

	switch len(found) {
	case 0:
		return "", errors.New("the build produced no .wasm artifact")
	case 1:
		return found[0], nil
	}
	if crateName != "" {
		for _, p := range found {
			stem := strings.TrimSuffix(filepath.Base(p), ".wasm")
			if stem == crateName || stem == strings.ReplaceAll(crateName, "-", "_") {
				return p, nil
			}
		}
	}
	names := make([]string, 0, len(found))
	for _, p := range found {
		names = append(names, filepath.Base(p))
	}
	if len(names) > 5 {
		names = names[:5]
	}
	return "", fmt.Errorf("ambiguous build output (%s); set artifact", strings.Join(names, ", "))
}

// packageName extracts `[package] name` from a Cargo.toml without pulling in a
// TOML dependency.
func packageName(tomlPath string) string {
	data, err := os.ReadFile(tomlPath)
	if err != nil {
		return ""
	}
	inPackage := false
	for _, line := range strings.Split(string(data), "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "[") {
			inPackage = strings.HasPrefix(t, "[package]")
			continue
		}
		if !inPackage || strings.HasPrefix(t, "#") {
			continue
		}
		eq := strings.IndexByte(t, '=')
		if eq < 0 || strings.TrimSpace(t[:eq]) != "name" {
			continue
		}
		v := strings.TrimSpace(t[eq+1:])
		if i := strings.IndexByte(v, '#'); i >= 0 {
			v = strings.TrimSpace(v[:i])
		}
		return strings.Trim(strings.TrimSpace(v), `"'`)
	}
	return ""
}

// ---- build -----------------------------------------------------------------

func (s *Service) runBuild(parent context.Context, crateDir, targetDir string) (string, error) {
	ctx, cancel := context.WithTimeout(parent, s.opts.Timeout)
	defer cancel()

	env, err := s.buildEnv(targetDir)
	if err != nil {
		return "", err
	}

	out := &cappedBuffer{max: s.opts.MaxLogBytes}
	cmd := exec.CommandContext(ctx, s.opts.BuildCommand[0], s.opts.BuildCommand[1:]...)
	cmd.Dir = crateDir
	cmd.Env = env
	cmd.Stdout = out
	cmd.Stderr = out

	runErr := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return out.String(), fmt.Errorf("build timed out after %s", s.opts.Timeout)
	}
	return out.String(), runErr
}

// buildEnv builds the deterministic, minimal environment for the build child
// process. It intentionally replaces the process environment rather than
// extending it.
func (s *Service) buildEnv(targetDir string) ([]string, error) {
	home := filepath.Join(filepath.Dir(targetDir), ".home")
	if err := os.MkdirAll(home, 0o700); err != nil {
		return nil, fmt.Errorf("create build home: %w", err)
	}
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + home,
		"CARGO_TARGET_DIR=" + targetDir,
		"CARGO_INCREMENTAL=0",
		"RUSTFLAGS=",
		"SOURCE_DATE_EPOCH=0",
		"TZ=UTC",
		"LC_ALL=C",
		"LANG=C",
		"CI=true",
		"GIT_TERMINAL_PROMPT=0",
	}
	// Reuse the host toolchain caches when present so the sandbox can build
	// offline; a pinned, pre-populated image is the reproducible alternative.
	if v := os.Getenv("CARGO_HOME"); v != "" {
		env = append(env, "CARGO_HOME="+v)
	}
	if v := os.Getenv("RUSTUP_HOME"); v != "" {
		env = append(env, "RUSTUP_HOME="+v)
	}
	return env, nil
}

func detectVersion(ctx context.Context, name string, args ...string) string {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, name, args...).Output()
	if err != nil {
		return ""
	}
	return firstLine(string(out))
}

// ---- diagnostics -----------------------------------------------------------

var buildFailureRules = []struct {
	code     string
	contains []string
	message  string
	hint     string
}{
	{
		code:     "TOOLCHAIN_MISSING",
		contains: []string{"no such command", "executable file not found", "command not found", "not found in $path"},
		message:  "the build toolchain is not available in the verification environment",
		hint:     "install the Stellar CLI and the wasm target on the verifier host (cargo install --locked stellar-cli, rustup target add wasm32v1-none)",
	},
	{
		code:     "COMPILER_VERSION_MISMATCH",
		contains: []string{"package requires rustc", "requires rustc", "rust-version", "minimum supported rust version"},
		message:  "the crate requires a different Rust compiler version than the one installed",
		hint:     "declare the expected rustc version in the request and install it on the verifier with rustup toolchain install <version>",
	},
	{
		code:     "NIGHTLY_FEATURE_REQUIRED",
		contains: []string{"only allowed on nightly", "e0554", "nightly-only", "feature(restrictions"},
		message:  "the crate uses a feature that is only available on a nightly compiler",
		hint:     "verify the build does not rely on nightly-only features, or pin the nightly toolchain the on-chain Wasm was built with",
	},
	{
		code:     "WASM_TARGET_MISSING",
		contains: []string{"target may not be installed", "can't find crate for `std`", "wasm32-unknown-unknown target", "wasm32v1-none target", "e0463"},
		message:  "the WebAssembly compilation target is not installed",
		hint:     "rustup target add wasm32v1-none (or wasm32-unknown-unknown for older SDKs)",
	},
	{
		code:     "DEPENDENCY_MISSING",
		contains: []string{"no matching package named", "failed to select a version", "failed to get `", "network failure", "spurious network error"},
		message:  "a crate dependency could not be resolved",
		hint:     "commit Cargo.lock, ensure the verifier can reach the crate registry, or vendor the dependencies",
	},
	{
		code:     "MANIFEST_INVALID",
		contains: []string{"failed to parse manifest", "failed to load manifest", "invalid type:"},
		message:  "the crate manifest could not be parsed",
		hint:     "check Cargo.toml for syntax errors and confirm it matches the crate that produced the deployed Wasm",
	},
	{
		code:     "LINKER_MISSING",
		contains: []string{"linker `", "linker not found", "wasm-ld"},
		message:  "the WebAssembly linker is not available",
		hint:     "install lld / wasm-ld in the verifier image",
	},
	{
		code:     "BUILD_TIMEOUT",
		contains: []string{"build timed out"},
		message:  "the build exceeded the verification timeout",
		hint:     "raise VERIFY_TIMEOUT or reduce the crate's dependency graph",
	},
	{
		code:     "BUILD_LOCK_TIMEOUT",
		contains: []string{"blocking waiting for file lock", "waiting for file lock"},
		message:  "the build waited on a stale Cargo lock",
		hint:     "ensure CARGO_TARGET_DIR and CARGO_HOME are private to the verification sandbox",
	},
}

func classifyBuildFailure(log string, runErr error) []Diagnostic {
	// The most actionable signal is often on the exec error itself (a missing
	// binary, a timeout), so match against both the captured output and it.
	haystack := strings.ToLower(log)
	if runErr != nil {
		haystack += "\n" + strings.ToLower(runErr.Error())
	}
	for _, rule := range buildFailureRules {
		for _, needle := range rule.contains {
			if !strings.Contains(haystack, strings.ToLower(needle)) {
				continue
			}
			msg := rule.message
			if line := firstErrorLine(log); line != "" {
				msg += ": " + line
			}
			return []Diagnostic{{
				Code:     rule.code,
				Severity: SeverityError,
				Message:  msg,
				Hint:     rule.hint,
			}}
		}
	}
	msg := "the build failed"
	if runErr != nil {
		msg = "the build failed: " + runErr.Error()
	}
	if line := firstErrorLine(log); line != "" {
		msg += " (" + line + ")"
	}
	return []Diagnostic{{
		Code:     "BUILD_FAILED",
		Severity: SeverityError,
		Message:  msg,
		Hint:     "inspect build_log for the compiler output; make sure the submitted commit is the one that produced the deployed Wasm",
	}}
}

func firstErrorLine(log string) string {
	for _, line := range strings.Split(log, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || !strings.Contains(strings.ToLower(t), "error") {
			continue
		}
		return truncate(t, 300)
	}
	return ""
}

func hashMismatchDiagnostic(onChain, compiled string) Diagnostic {
	return Diagnostic{
		Code:     "HASH_MISMATCH",
		Severity: SeverityError,
		Message:  fmt.Sprintf("compiled Wasm hash %s does not match the on-chain hash %s", compiled, onChain),
		Hint:     "the usual causes are a different compiler/toolchain version, feature flags that were not enabled, or a commit that differs from the deployed build",
	}
}

func expectedVersionDiagnostics(want ExpectedToolchain, stellar, rustc string) []Diagnostic {
	var out []Diagnostic
	if v := strings.TrimSpace(want.StellarVersion); v != "" && stellar != "" && !strings.Contains(stellar, v) {
		out = append(out, Diagnostic{
			Code:     "COMPILER_VERSION_MISMATCH",
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("expected stellar %s but the verifier is running %s", v, stellar),
			Hint:     "install the matching Stellar CLI version on the verifier for a byte-identical rebuild",
		})
	}
	if v := strings.TrimSpace(want.RustcVersion); v != "" && rustc != "" && !strings.Contains(rustc, v) {
		out = append(out, Diagnostic{
			Code:     "COMPILER_VERSION_MISMATCH",
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("expected rustc %s but the verifier is running %s", v, rustc),
			Hint:     "install the matching Rust toolchain on the verifier for a byte-identical rebuild",
		})
	}
	return out
}

// ---- small helpers ---------------------------------------------------------

func normalizeHash(h string) string {
	h = strings.ToLower(strings.TrimSpace(h))
	h = strings.TrimPrefix(h, "0x")
	if !hashRe.MatchString(h) {
		return ""
	}
	return h
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func within(root, p string) bool {
	return p == root || strings.HasPrefix(p, root+string(os.PathSeparator))
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// cappedBuffer captures at most max bytes of child output while always
// reporting the full write length so the child never blocks on a full pipe.
type cappedBuffer struct {
	buf bytes.Buffer
	max int
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	if remaining := c.max - c.buf.Len(); remaining > 0 {
		if len(p) <= remaining {
			c.buf.Write(p)
		} else {
			c.buf.Write(p[:remaining])
		}
	}
	return len(p), nil
}

func (c *cappedBuffer) String() string { return c.buf.String() }
