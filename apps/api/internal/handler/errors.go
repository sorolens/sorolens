package handler

import (
	"encoding/json"
	"net/http"

	"github.com/sorolens/sorolens/apps/api/internal/middleware"
)

const (
	CodeNotFound         = "NOT_FOUND"
	CodeInvalidInput     = "INVALID_INPUT"
	CodeInternal         = "INTERNAL"
	CodeForbidden        = "FORBIDDEN"
	CodeRateLimited      = "RATE_LIMITED"
	CodeUnsupportedMedia = "UNSUPPORTED_MEDIA_TYPE"
	CodeUnauthorized     = "UNAUTHORIZED"
)

// MaxPageSize is the upper bound applied to the `limit` query parameter

// on list endpoints (events, invocations, storage and contracts). Requests asking

// for more results than this are clamped to MaxPageSize to avoid unbounded

// table scans and abuse. The value is deliberately conservative.

const MaxPageSize = 100

// DefaultPageSize is used when the `limit` query parameter is omitted.

const DefaultPageSize = 20

type errorBody struct {
	Code      string `json:"code" `
	Message   string `json:"message" `
	RequestID string `json:"request_id" `
}

type errorEnvelope struct {
	Error errorBody `json:"error" `
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorEnvelope{
		Error: errorBody{
			Code:      code,
			Message:   message,
			RequestID: middleware.GetRequestID(r.Context()),
		},
	})
}

// writeInvalidInput responds with 422 Unprocessable Entity and the standard

// error envelope for invalid query parameters (cursor, network, date, limit).

func writeInvalidInput(w http.ResponseWriter, r *http.Request, message string) {
	writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, message)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
