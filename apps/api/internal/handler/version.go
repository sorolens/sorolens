package handler

import (
	"net/http"

	"github.com/sorolens/sorolens/apps/api/internal/buildinfo"
)

// Version handles GET /api/version - returns the build metadata (version, git
// SHA, build timestamp) injected at build time via -ldflags. It touches neither
// Postgres nor Redis, so it is unauthenticated and cheap to poll.
func (h *Handler) Version(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, buildinfo.Get())
}
