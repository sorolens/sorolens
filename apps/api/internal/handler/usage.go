package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/middleware"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// GetUsage handles GET /api/v1/usage.
// It returns aggregated request counts, endpoint breakdown, daily timeline,
// and monthly quota usage for the caller's key, or for any key if requested
// by an admin (issue #792).
func (h *Handler) GetUsage(w http.ResponseWriter, r *http.Request) {
	id, ok := middleware.IdentityFromRequest(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, CodeUnauthorized, "authentication required")
		return
	}

	keyID := r.URL.Query().Get("key_id")
	if keyID == "" {
		if id.APIKey != nil {
			keyID = id.APIKey.ID
		} else {
			writeError(w, r, http.StatusBadRequest, CodeInvalidInput, "key_id query parameter is required")
			return
		}
	} else if id.APIKey == nil || id.APIKey.ID != keyID {
		// Target key is different from caller's key; caller must hold admin scope or role.
		if !id.IsAdminKey() && id.Role != "admin" {
			writeError(w, r, http.StatusForbidden, CodeForbidden, "admin scope required to view usage for another key")
			return
		}
	}

	now := time.Now().UTC()
	from := now.AddDate(0, 0, -30)
	to := now

	if fromParam := r.URL.Query().Get("from"); fromParam != "" {
		t, err := parseDateOrRFC3339(fromParam)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, CodeInvalidInput, "invalid 'from' timestamp; use RFC3339 or YYYY-MM-DD")
			return
		}
		from = t
	}

	if toParam := r.URL.Query().Get("to"); toParam != "" {
		t, err := parseDateOrRFC3339(toParam)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, CodeInvalidInput, "invalid 'to' timestamp; use RFC3339 or YYYY-MM-DD")
			return
		}
		to = t
	}

	summary, err := h.Store.GetKeyUsage(r.Context(), keyID, from, to)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "API key not found")
			return
		}
		h.Logger.Error("get key usage", "key_id", keyID, "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to get key usage")
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func parseDateOrRFC3339(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, errors.New("unrecognized time format")
}
