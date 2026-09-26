// Package changelog implements the HTTP handlers for the per-contract
// Wasm hash changelog feature (issue #276).
//
// Routes:
//
//	GET /contracts/{id}/changelog        – JSON array of ContractVersion records
//	GET /contracts/{id}/changelog/feed   – Atom 1.0 XML feed
//	GET /contracts/{id}/changelog/badge  – SVG badge with latest wasm hash slice
package changelog

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// Handler bundles the changelog HTTP handler methods.
type Handler struct {
	store store.Store
}

// New returns a Handler wired to the given store.
func New(s store.Store) *Handler {
	return &Handler{store: s}
}

// ---- JSON changelog -------------------------------------------------------

// versionJSON is the on-wire representation of a single changelog entry.
type versionJSON struct {
	ID                int64     `json:"id"`
	ContractID        string    `json:"contract_id"`
	WasmHash          string    `json:"wasm_hash"`
	FirstSeenLedger   int64     `json:"first_seen_ledger"`
	TxHash            string    `json:"tx_hash,omitempty"`
	VerifiedSourceRef string    `json:"verified_source_ref,omitempty"`
	RecordedAt        time.Time `json:"recorded_at"`
}

// GetChangelog handles GET /contracts/{id}/changelog.
// It returns the full Wasm hash history for the given contract as a JSON
// array sorted chronologically (oldest first).
func (h *Handler) GetChangelog(w http.ResponseWriter, r *http.Request) {
	contractID := extractContractID(r)
	if contractID == "" {
		http.Error(w, "missing contract id", http.StatusBadRequest)
		return
	}

	// Confirm contract exists so we return 404 instead of an empty array.
	if _, err := h.store.GetContract(r.Context(), contractID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "contract not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	versions, err := h.store.ListContractVersions(r.Context(), contractID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	out := make([]versionJSON, 0, len(versions))
	for _, v := range versions {
		out = append(out, versionJSON{
			ID:                v.ID,
			ContractID:        v.ContractID,
			WasmHash:          v.WasmHash,
			FirstSeenLedger:   v.FirstSeenLedger,
			TxHash:            v.TxHash,
			VerifiedSourceRef: v.VerifiedSourceRef,
			RecordedAt:        v.RecordedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out) //nolint:errcheck
}

// ---- Atom 1.0 feed ---------------------------------------------------------

type atomFeed struct {
	XMLName xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
	Title   string      `xml:"title"`
	ID      string      `xml:"id"`
	Updated string      `xml:"updated"`
	Link    atomLink    `xml:"link"`
	Entries []atomEntry `xml:"entry"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr,omitempty"`
}

type atomEntry struct {
	Title   string   `xml:"title"`
	ID      string   `xml:"id"`
	Updated string   `xml:"updated"`
	Content string   `xml:"content"`
	Link    atomLink `xml:"link"`
}

// GetFeed handles GET /contracts/{id}/changelog/feed.
// It returns an Atom 1.0 XML feed suitable for RSS readers.
func (h *Handler) GetFeed(w http.ResponseWriter, r *http.Request) {
	contractID := extractContractID(r)
	if contractID == "" {
		http.Error(w, "missing contract id", http.StatusBadRequest)
		return
	}

	if _, err := h.store.GetContract(r.Context(), contractID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "contract not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	versions, err := h.store.ListContractVersions(r.Context(), contractID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	selfURL := fmt.Sprintf("%s/contracts/%s/changelog/feed", baseURL(r), contractID)

	updated := time.Now().UTC().Format(time.RFC3339)
	if len(versions) > 0 {
		updated = versions[len(versions)-1].RecordedAt.UTC().Format(time.RFC3339)
	}

	feed := atomFeed{
		Title:   fmt.Sprintf("Wasm changelog for %s", contractID),
		ID:      selfURL,
		Updated: updated,
		Link:    atomLink{Href: selfURL, Rel: "self"},
	}

	for _, v := range versions {
		txInfo := "no tx linked"
		if v.TxHash != "" {
			txInfo = "tx: " + v.TxHash
		}
		verifiedInfo := ""
		if v.VerifiedSourceRef != "" {
			verifiedInfo = fmt.Sprintf(" | source: %s", v.VerifiedSourceRef)
		}
		feed.Entries = append(feed.Entries, atomEntry{
			Title:   fmt.Sprintf("Wasm %s (ledger %d)", shortHash(v.WasmHash), v.FirstSeenLedger),
			ID:      fmt.Sprintf("%s/contracts/%s/changelog#%d", baseURL(r), contractID, v.ID),
			Updated: v.RecordedAt.UTC().Format(time.RFC3339),
			Content: fmt.Sprintf("hash=%s | first_seen_ledger=%d | %s%s",
				v.WasmHash, v.FirstSeenLedger, txInfo, verifiedInfo),
			Link: atomLink{Href: fmt.Sprintf("%s/contracts/%s/changelog", baseURL(r), contractID)},
		})
	}

	w.Header().Set("Content-Type", "application/atom+xml; charset=utf-8")
	w.Write([]byte(xml.Header))    //nolint:errcheck
	xml.NewEncoder(w).Encode(feed) //nolint:errcheck
}

// ---- SVG badge -------------------------------------------------------------

const badgeSVGTemplate = `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="20">
  <linearGradient id="s" x2="0" y2="100%%">
    <stop offset="0" stop-color="#bbb" stop-opacity=".1"/>
    <stop offset="1" stop-opacity=".1"/>
  </linearGradient>
  <rect rx="3" width="%d" height="20" fill="#555"/>
  <rect rx="3" x="%d" width="%d" height="20" fill="#007ec6"/>
  <rect rx="3" width="%d" height="20" fill="url(#s)"/>
  <g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="11">
    <text x="%d" y="15" fill="#010101" fill-opacity=".3">wasm</text>
    <text x="%d" y="14">wasm</text>
    <text x="%d" y="15" fill="#010101" fill-opacity=".3">%s</text>
    <text x="%d" y="14">%s</text>
  </g>
</svg>`

// GetBadge handles GET /contracts/{id}/changelog/badge.
// It returns an SVG badge showing the first 8 characters of the latest
// known Wasm hash, suitable for embedding in READMEs.
func (h *Handler) GetBadge(w http.ResponseWriter, r *http.Request) {
	contractID := extractContractID(r)
	if contractID == "" {
		http.Error(w, "missing contract id", http.StatusBadRequest)
		return
	}

	latest, err := h.store.GetLatestContractVersion(r.Context(), contractID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// Contract exists but no versions yet; show an "unknown" badge.
			writeBadge(w, "unknown")
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeBadge(w, shortHash(latest.WasmHash))
}

func writeBadge(w http.ResponseWriter, hashSlice string) {
	const labelW = 38
	valueW := 6*len(hashSlice) + 10
	totalW := labelW + valueW
	valueX := labelW + valueW/2

	svg := fmt.Sprintf(badgeSVGTemplate,
		totalW, totalW,
		labelW, valueW,
		totalW,
		labelW/2+1, labelW/2,
		valueX+1, hashSlice,
		valueX, hashSlice,
	)
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "no-cache, max-age=0")
	fmt.Fprint(w, svg) //nolint:errcheck
}

// ---- helpers ---------------------------------------------------------------

// extractContractID extracts the contract ID from the URL path.
// It supports both /contracts/{id}/changelog and /contracts/{id}/changelog/feed.
func extractContractID(r *http.Request) string {
	// URL pattern: /contracts/<id>/changelog[/...]
	// We split on "/" and find the segment after "contracts".
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	for i, p := range parts {
		if p == "contracts" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// baseURL returns the scheme+host of the incoming request for building
// self-links in the Atom feed.
func baseURL(r *http.Request) string {
	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	return scheme + "://" + r.Host
}

// shortHash returns the first 8 characters of a hex hash string.
func shortHash(h string) string {
	if len(h) <= 8 {
		return h
	}
	return h[:8]
}
