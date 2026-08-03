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
	CodeRateLimited      = "RATE_LIMITED"
	CodeUnsupportedMedia = "UNSUPPORTED_MEDIA_TYPE"
)

type errorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
