package handler

import (
	"net/http"
)

// Health handles GET /health - always returns 200.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Readyz handles GET /readyz - returns 503 if postgres or redis are unreachable.
func (h *Handler) Readyz(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	dbErr := h.DB.Ping(ctx)
	redisErr := h.Redis.Ping(ctx)

	if dbErr != nil || redisErr != nil {
		details := map[string]string{}
		if dbErr != nil {
			details["postgres"] = dbErr.Error()
		}
		if redisErr != nil {
			details["redis"] = redisErr.Error()
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "unavailable",
			"checks": details,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
