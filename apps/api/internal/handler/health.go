package handler

import (
	"net/http"
	"time"
)

// Health handles GET /health - always returns 200.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// HealthCheck handles GET /api/v1/health. It is a lightweight liveness probe
// that also reports whether the API's dependencies are reachable. It always
// responds 200 so deployment platforms and uptime monitors can distinguish a
// live process from a dead one; the db and redis fields carry the
// per-dependency state ("connected" or "unreachable") and status is
// "degraded" when any dependency is down.
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	dbStatus := "connected"
	if err := h.DB.Ping(ctx); err != nil {
		dbStatus = "unreachable"
	}

	redisStatus := "connected"
	if err := h.Redis.Ping(ctx); err != nil {
		redisStatus = "unreachable"
	}

	status := "ok"
	if dbStatus != "connected" || redisStatus != "connected" {
		status = "degraded"
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":    status,
		"db":        dbStatus,
		"redis":     redisStatus,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
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
