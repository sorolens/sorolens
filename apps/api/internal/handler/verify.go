package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
	"github.com/sorolens/sorolens/apps/api/internal/verify"
)

// ContractVerifier runs a deterministic rebuild of submitted contract source
// and compares the resulting Wasm hash against the on-chain hash. It is an
// interface so tests can substitute a stub, and so deployments without a build
// toolchain (for example the Vercel serverless entrypoint) can leave it nil and
// answer 503 instead of pretending to verify.
type ContractVerifier interface {
	Verify(ctx context.Context, req verify.Request) (verify.Result, error)
}

// maxVerifyRequestBytes caps the decoded JSON body. A base64 source archive is
// ~33% larger than the raw bytes, so this sits above verify's 8 MiB default.
const maxVerifyRequestBytes = 16 << 20

type verifySourceInput struct {
	Kind          string `json:"kind"`
	Filename      string `json:"filename"`
	ContentBase64 string `json:"content_base64"`
	URL           string `json:"url"`
	Commit        string `json:"commit"`
	Subdir        string `json:"subdir"`
}

type verifyExpectedInput struct {
	StellarVersion string `json:"stellar_version"`
	RustcVersion   string `json:"rustc_version"`
}

type verifyRequest struct {
	Source   *verifySourceInput   `json:"source"`
	Artifact string               `json:"artifact"`
	Expected *verifyExpectedInput `json:"expected"`
}

type verificationDiagnosticResponse struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Hint     string `json:"hint,omitempty"`
}

type verificationToolchainResponse struct {
	Stellar string `json:"stellar,omitempty"`
	Rustc   string `json:"rustc,omitempty"`
	Cargo   string `json:"cargo,omitempty"`
}

type verificationSourceResponse struct {
	Kind   string `json:"kind"`
	Ref    string `json:"ref,omitempty"`
	Digest string `json:"digest,omitempty"`
}

type verificationResponse struct {
	ContractID       string                           `json:"contract_id"`
	Status           string                           `json:"status"`
	Matched          bool                             `json:"matched"`
	OnChainHash      string                           `json:"on_chain_hash,omitempty"`
	CompiledWasmHash string                           `json:"compiled_wasm_hash,omitempty"`
	Source           verificationSourceResponse       `json:"source"`
	Toolchain        verificationToolchainResponse    `json:"toolchain"`
	Diagnostics      []verificationDiagnosticResponse `json:"diagnostics"`
	BuildLog         string                           `json:"build_log,omitempty"`
	SubmittedAt      string                           `json:"submitted_at"`
	VerifiedAt       string                           `json:"verified_at,omitempty"`
	UpdatedAt        string                           `json:"updated_at"`
}

func verificationFromStore(v store.ContractVerification) verificationResponse {
	resp := verificationResponse{
		ContractID:       v.ContractID,
		Status:           v.Status,
		Matched:          v.Matched,
		OnChainHash:      v.OnChainHash,
		CompiledWasmHash: v.CompiledHash,
		Source: verificationSourceResponse{
			Kind:   v.SourceKind,
			Ref:    v.SourceRef,
			Digest: v.SourceDigest,
		},
		Toolchain: verificationToolchainResponse{
			Stellar: v.StellarVersion,
			Rustc:   v.RustcVersion,
			Cargo:   v.CargoVersion,
		},
		Diagnostics: make([]verificationDiagnosticResponse, 0, len(v.Diagnostics)),
		BuildLog:    v.BuildLog,
		SubmittedAt: v.SubmittedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   v.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if v.VerifiedAt != nil {
		resp.VerifiedAt = v.VerifiedAt.UTC().Format(time.RFC3339)
	}
	for _, d := range v.Diagnostics {
		resp.Diagnostics = append(resp.Diagnostics, verificationDiagnosticResponse{
			Code:     d.Code,
			Severity: d.Severity,
			Message:  d.Message,
			Hint:     d.Hint,
		})
	}
	return resp
}

func buildVerifyRequest(req verifyRequest, contractID, onChainHash string) (verify.Request, error) {
	out := verify.Request{
		ContractID:  contractID,
		OnChainHash: onChainHash,
		Artifact:    strings.TrimSpace(req.Artifact),
	}
	if req.Source == nil {
		return out, errors.New("source is required")
	}
	out.Source.Subdir = strings.TrimSpace(req.Source.Subdir)
	if req.Expected != nil {
		out.Expected = verify.ExpectedToolchain{
			StellarVersion: strings.TrimSpace(req.Expected.StellarVersion),
			RustcVersion:   strings.TrimSpace(req.Expected.RustcVersion),
		}
	}

	switch kind := strings.ToLower(strings.TrimSpace(req.Source.Kind)); kind {
	case verify.SourceArchive:
		raw := strings.TrimSpace(req.Source.ContentBase64)
		if raw == "" {
			return out, errors.New("source.content_base64 is required for an archive source")
		}
		content, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return out, errors.New("source.content_base64 must be standard base64")
		}
		out.Source.Kind = verify.SourceKind(verify.SourceArchive)
		out.Source.Filename = strings.TrimSpace(req.Source.Filename)
		out.Source.Content = content
	case verify.SourceGit:
		if strings.TrimSpace(req.Source.URL) == "" {
			return out, errors.New("source.url is required for a git source")
		}
		if strings.TrimSpace(req.Source.Commit) == "" {
			return out, errors.New("source.commit is required for a git source")
		}
		out.Source.Kind = verify.SourceKind(verify.SourceGit)
		out.Source.GitURL = strings.TrimSpace(req.Source.URL)
		out.Source.GitCommit = strings.TrimSpace(req.Source.Commit)
	default:
		return out, fmt.Errorf("source.kind must be %q or %q", verify.SourceArchive, verify.SourceGit)
	}
	return out, nil
}

func verificationRecordFromResult(req verify.Request, res verify.Result) store.ContractVerification {
	now := time.Now().UTC()
	verifiedAt := res.VerifiedAt
	if verifiedAt.IsZero() {
		verifiedAt = now
	}
	submittedAt := res.SubmittedAt
	if submittedAt.IsZero() {
		submittedAt = now
	}
	status := res.Status
	if status == "" {
		status = store.VerificationFailed
	}
	record := store.ContractVerification{
		ContractID:     req.ContractID,
		Status:         status,
		OnChainHash:    res.OnChainHash,
		CompiledHash:   res.CompiledHash,
		Matched:        res.Matched,
		SourceKind:     res.SourceKind,
		SourceRef:      res.SourceRef,
		SourceDigest:   res.SourceDigest,
		StellarVersion: res.StellarVersion,
		RustcVersion:   res.RustcVersion,
		CargoVersion:   res.CargoVersion,
		BuildLog:       res.BuildLog,
		SubmittedAt:    submittedAt,
		VerifiedAt:     &verifiedAt,
		UpdatedAt:      now,
		Diagnostics:    make([]store.VerificationDiagnostic, 0, len(res.Diagnostics)),
	}
	for _, d := range res.Diagnostics {
		record.Diagnostics = append(record.Diagnostics, store.VerificationDiagnostic{
			Code:     d.Code,
			Severity: d.Severity,
			Message:  d.Message,
			Hint:     d.Hint,
		})
	}
	return record
}

// VerifyContract handles POST /api/v1/contracts/{id}/verify.
//
// The body names the source to rebuild, either an uploaded archive
// (`{"source":{"kind":"archive","content_base64":"...","filename":"..."}}`)
// or a pinned Git commit (`{"source":{"kind":"git","url":"...","commit":"..."}}`).
// The verdict is always reported in the response body with `status` set to
// "verified" or "failed"; a failed verification is a successful request that
// carries actionable diagnostics, so it returns 200 rather than an error code
// the client could mistake for malformed input.
func (h *Handler) VerifyContract(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")

	if h.Verifier == nil {
		writeError(w, r, http.StatusServiceUnavailable, CodeInternal,
			"source verification is unavailable on this deployment: no build sandbox is configured")
		return
	}

	var req verifyRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxVerifyRequestBytes))
	if err := dec.Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}

	c, err := h.Store.GetContract(r.Context(), contractID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "contract not found")
		return
	}
	if err != nil {
		h.Logger.Error("get contract for verification", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to load contract")
		return
	}

	verifyReq, err := buildVerifyRequest(req, contractID, c.WasmHash)
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, err.Error())
		return
	}
	verifyReq.SubmittedAt = time.Now().UTC()

	result, err := h.Verifier.Verify(r.Context(), verifyReq)
	if err != nil {
		h.Logger.Error("verify contract source", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal,
			"verification could not be completed: "+err.Error())
		return
	}

	record := verificationRecordFromResult(verifyReq, result)
	if err := h.Store.UpsertContractVerification(r.Context(), record); err != nil {
		h.Logger.Error("persist contract verification", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to persist verification result")
		return
	}

	writeJSON(w, http.StatusOK, verificationFromStore(record))
}

// GetContractVerification handles GET /api/v1/contracts/{id}/verification.
// It serves the cached verdict so the dashboard can render a verified badge
// without triggering a rebuild.
func (h *Handler) GetContractVerification(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")

	if _, err := h.Store.GetContract(r.Context(), contractID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "contract not found")
			return
		}
		h.Logger.Error("get contract for verification record", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to load contract")
		return
	}

	v, err := h.Store.GetContractVerification(r.Context(), contractID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, CodeNotFound,
			"contract has not been submitted for source verification")
		return
	}
	if err != nil {
		h.Logger.Error("get contract verification", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to load verification record")
		return
	}

	writeJSON(w, http.StatusOK, verificationFromStore(v))
}
