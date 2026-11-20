package middleware

import (
	"encoding/json"
	"net/http"

	chiMiddleware "github.com/go-chi/chi/v5/middleware"
)

type errorEnvelope struct {
	Error struct {
		Code      int    `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	} `json:"error"`
}

func writeError(w http.ResponseWriter, r *http.Request, status int, _, message string) {
	reqID := ""
	if r != nil {
		reqID = chiMiddleware.GetReqID(r.Context())
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorEnvelope{
		Error: struct {
			Code      int    `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		}{
			Code:      status,
			Message:   message,
			RequestID: reqID,
		},
	})
}
