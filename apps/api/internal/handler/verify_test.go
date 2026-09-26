package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/config"
	"github.com/sorolens/sorolens/apps/api/internal/handler"
	"github.com/sorolens/sorolens/apps/api/internal/router"
	"github.com/sorolens/sorolens/apps/api/internal/store"
	"github.com/sorolens/sorolens/apps/api/internal/verify"
)

const verifyContractID = "CONTRACT_VERIFY_A"

// stubVerifier records the request it received and returns a canned result.
type stubVerifier struct {
	result verify.Result
	err    error
	got    verify.Request
}

func (s *stubVerifier) Verify(_ context.Context, req verify.Request) (verify.Result, error) {
	s.got = req
	return s.result, s.err
}

func newVerifyHandler(ms *store.MockStore, v handler.ContractVerifier) http.Handler {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	h := &handler.Handler{
		Store:       ms,
		DB:          &store.MockPinger{Healthy: true},
		Redis:       &store.MockPinger{Healthy: true},
		RedisClient: &mockRedisClient{},
		Logger:      logger,
		Verifier:    v,
	}
	return router.New(h, config.DefaultRequestMaxBodyBytes)
}

func seededVerifyStore(t *testing.T, wasmHash string) *store.MockStore {
	t.Helper()
	ms := seedRoleStore(t)
	if err := ms.UpsertContract(nil, store.Contract{
		ID:       verifyContractID,
		Network:  "testnet",
		Status:   "active",
		WasmHash: wasmHash,
	}); err != nil {
		t.Fatal(err)
	}
	return ms
}

func verifyRequestBody(t *testing.T) *bytes.Buffer {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"source": map[string]string{
			"kind":   "git",
			"url":    "https://github.com/sorolens/sorolens",
			"commit": "abcdef1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewBuffer(body)
}

func TestVerifyContractSuccessPersistsRecord(t *testing.T) {
	hash := strings.Repeat("ab", 32)
	ms := seededVerifyStore(t, hash)
	stub := &stubVerifier{result: verify.Result{
		Status:         verify.StatusVerified,
		Matched:        true,
		OnChainHash:    hash,
		CompiledHash:   hash,
		SourceKind:     verify.SourceGit,
		SourceRef:      "https://github.com/sorolens/sorolens@abcdef1",
		SourceDigest:   "digest",
		StellarVersion: "stellar 21.0.0",
		RustcVersion:   "rustc 1.84.0",
		CargoVersion:   "cargo 1.84.0",
		VerifiedAt:     time.Now().UTC(),
	}}
	srv := newVerifyHandler(ms, stub)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/"+verifyContractID+"/verify", verifyRequestBody(t))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", contributorUser)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}

	var resp struct {
		Status           string `json:"status"`
		Matched          bool   `json:"matched"`
		CompiledWasmHash string `json:"compiled_wasm_hash"`
		Diagnostics      []struct {
			Code string `json:"code"`
		} `json:"diagnostics"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Status != "verified" || !resp.Matched {
		t.Errorf("status=%q matched=%v, want verified/true", resp.Status, resp.Matched)
	}
	if resp.CompiledWasmHash != hash {
		t.Errorf("compiled_wasm_hash = %q, want %q", resp.CompiledWasmHash, hash)
	}
	if len(resp.Diagnostics) != 0 {
		t.Errorf("unexpected diagnostics: %v", resp.Diagnostics)
	}
	if stub.got.ContractID != verifyContractID {
		t.Errorf("verifier saw contract %q, want %q", stub.got.ContractID, verifyContractID)
	}
	if stub.got.OnChainHash != hash {
		t.Errorf("verifier saw on-chain hash %q, want %q", stub.got.OnChainHash, hash)
	}
	if stub.got.Source.Kind != verify.SourceGit || stub.got.Source.GitCommit != "abcdef1" {
		t.Errorf("verifier saw unexpected source: %+v", stub.got.Source)
	}

	record, err := ms.GetContractVerification(nil, verifyContractID)
	if err != nil {
		t.Fatalf("verification record was not persisted: %v", err)
	}
	if record.Status != store.VerificationVerified || !record.Matched {
		t.Errorf("persisted record = %+v, want verified", record)
	}
}

func TestVerifyContractFailureReturnsActionableDiagnostics(t *testing.T) {
	ms := seededVerifyStore(t, strings.Repeat("ab", 32))
	stub := &stubVerifier{result: verify.Result{
		Status:       verify.StatusFailed,
		Matched:      false,
		OnChainHash:  strings.Repeat("ab", 32),
		CompiledHash: strings.Repeat("cd", 32),
		SourceKind:   verify.SourceGit,
		Diagnostics: []verify.Diagnostic{{
			Code:     "HASH_MISMATCH",
			Severity: verify.SeverityError,
			Message:  "hashes differ",
			Hint:     "check the toolchain",
		}},
	}}
	srv := newVerifyHandler(ms, stub)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/"+verifyContractID+"/verify", verifyRequestBody(t))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", contributorUser)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// A failed verification is a successful request carrying a verdict.
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}

	var resp struct {
		Status      string `json:"status"`
		Diagnostics []struct {
			Code string `json:"code"`
			Hint string `json:"hint"`
		} `json:"diagnostics"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Status != "failed" {
		t.Errorf("status = %q, want failed", resp.Status)
	}
	if len(resp.Diagnostics) != 1 || resp.Diagnostics[0].Code != "HASH_MISMATCH" {
		t.Fatalf("diagnostics = %v, want one HASH_MISMATCH", resp.Diagnostics)
	}
	if resp.Diagnostics[0].Hint == "" {
		t.Error("diagnostic hint was not surfaced")
	}
}

func TestVerifyContractUnknownContract(t *testing.T) {
	srv := newVerifyHandler(seedRoleStore(t), &stubVerifier{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/CUNKNOWN/verify", verifyRequestBody(t))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", contributorUser)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestVerifyContractMissingSource(t *testing.T) {
	ms := seededVerifyStore(t, strings.Repeat("ab", 32))
	srv := newVerifyHandler(ms, &stubVerifier{})

	body, err := json.Marshal(map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/"+verifyContractID+"/verify", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", contributorUser)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422 for a missing source, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestVerifyContractUnconfiguredSandbox(t *testing.T) {
	ms := seededVerifyStore(t, strings.Repeat("ab", 32))
	srv := newVerifyHandler(ms, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/"+verifyContractID+"/verify", verifyRequestBody(t))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", contributorUser)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 when no verifier is configured, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestGetContractVerificationNotYetSubmitted(t *testing.T) {
	ms := seededVerifyStore(t, strings.Repeat("ab", 32))
	srv := newVerifyHandler(ms, &stubVerifier{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+verifyContractID+"/verification", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404 before any submission, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestGetContractVerificationReturnsStoredVerdict(t *testing.T) {
	ms := seededVerifyStore(t, strings.Repeat("ab", 32))
	verifiedAt := time.Date(2025, 3, 4, 5, 6, 7, 0, time.UTC)
	if err := ms.UpsertContractVerification(nil, store.ContractVerification{
		ContractID:     verifyContractID,
		Status:         store.VerificationVerified,
		OnChainHash:    strings.Repeat("ab", 32),
		CompiledHash:   strings.Repeat("ab", 32),
		Matched:        true,
		SourceKind:     store.VerificationSourceArchive,
		SourceRef:      "counter.tar.gz",
		StellarVersion: "stellar 21.0.0",
		RustcVersion:   "rustc 1.84.0",
		CargoVersion:   "cargo 1.84.0",
		BuildLog:       "compiled",
		SubmittedAt:    verifiedAt.Add(-time.Minute),
		VerifiedAt:     &verifiedAt,
		UpdatedAt:      verifiedAt,
		Diagnostics:    []store.VerificationDiagnostic{{Code: "INFO", Severity: "info", Message: "ok"}},
	}); err != nil {
		t.Fatal(err)
	}
	srv := newVerifyHandler(ms, &stubVerifier{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+verifyContractID+"/verification", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var resp struct {
		ContractID string `json:"contract_id"`
		Status     string `json:"status"`
		Matched    bool   `json:"matched"`
		Source     struct {
			Kind string `json:"kind"`
			Ref  string `json:"ref"`
		} `json:"source"`
		Toolchain struct {
			Stellar string `json:"stellar"`
			Rustc   string `json:"rustc"`
		} `json:"toolchain"`
		VerifiedAt  string `json:"verified_at"`
		SubmittedAt string `json:"submitted_at"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.ContractID != verifyContractID || resp.Status != "verified" || !resp.Matched {
		t.Errorf("unexpected verdict: %+v", resp)
	}
	if resp.Source.Kind != "archive" || resp.Source.Ref != "counter.tar.gz" {
		t.Errorf("unexpected source: %+v", resp.Source)
	}
	if resp.Toolchain.Stellar != "stellar 21.0.0" || resp.Toolchain.Rustc != "rustc 1.84.0" {
		t.Errorf("unexpected toolchain: %+v", resp.Toolchain)
	}
	if resp.VerifiedAt != "2025-03-04T05:06:07Z" {
		t.Errorf("verified_at = %q, want RFC3339 UTC", resp.VerifiedAt)
	}
}
