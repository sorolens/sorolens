package handler

import (
	"encoding/json"
	"github.com/sorolens/sorolens/apps/api/internal/apierror"
	"net/http"
)

// Error codes are re-exported from the shared apierror package so handlers

// and middleware name the same set of codes.

const (
	CodeNotFound         = apierror.CodeNotFound
	CodeInvalidInput     = apierror.CodeInvalidInput
	CodeInternal         = apierror.CodeInternal
	CodeForbidden        = apierror.CodeForbidden
	CodeRateLimited      = apierror.CodeRateLimited
	CodeUnsupportedMedia = apierror.CodeUnsupportedMedia
	CodeUnauthorized     = apierror.CodeUnauthorized
)

// errorEnvelope is kept as an alias for the shared type so existing

// handler code that references it continues to compile.

type errorEnvelope = apierror.Envelope

// writeError writes the single JSON error envelope shared by all handlers

// and middleware.

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	apierror.WriteError(w, r, status, code, message)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
