package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/cursor"
)

// MaxPageSize is the largest page a list endpoint returns. It mirrors the
// `maximum` on components/parameters/Limit in docs/openapi.yaml, so the spec
// and the handler cannot disagree about what a caller may ask for. A larger
// ?limit= is clamped to this value rather than rejected: an over-large page is
// a client asking for "as much as allowed", while a malformed one is a bug the
// caller should hear about.
const MaxPageSize = 200

// clampLimit bounds n to the inclusive range [1, max]. Non-positive values keep
// their "default page size" meaning and are returned unchanged for the caller's
// default handling to replace.
func clampLimit(n, max int) int {
	if n > max {
		return max
	}
	return n
}

// decodeCursor reads the opaque ?cursor= value shared by the keyset-paginated
// list endpoints. A malformed cursor is rejected with INVALID_CURSOR instead of
// being silently treated as "start of list", which would answer a caller with
// the first page when it asked for a later one.
func decodeCursor(w http.ResponseWriter, r *http.Request) (string, bool) {
	raw, ok := cursor.Decode(r.URL.Query().Get("cursor"))
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidCursor, "invalid cursor")
		return "", false
	}
	return raw, true
}

// networkFilter reads the optional ?network= filter. An unknown network is
// rejected with INVALID_NETWORK so a typo cannot quietly widen the query to
// every network.
func networkFilter(w http.ResponseWriter, r *http.Request) (string, bool) {
	network, ok := networkParam(r)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidNetwork,
			"network must be one of: testnet, mainnet, futurenet, standalone")
		return "", false
	}
	return network, true
}

// pageLimit reads ?limit= for a list endpoint. Absent means def; a value that
// is not a positive integer is rejected with INVALID_LIMIT; anything above
// MaxPageSize is clamped so no request can pull an unbounded number of rows.
func pageLimit(w http.ResponseWriter, r *http.Request, def int) (int, bool) {
	v := strings.TrimSpace(r.URL.Query().Get("limit"))
	if v == "" {
		return def, true
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidLimit,
			"limit must be a positive integer; values above "+strconv.Itoa(MaxPageSize)+" are clamped")
		return 0, false
	}
	return clampLimit(n, MaxPageSize), true
}

// dateParam reads an inclusive ?since=/?until= bound, accepting either an
// RFC3339 timestamp or a bare YYYY-MM-DD date (expanded to the start, or with
// endOfDay the end, of that UTC day). A value that parses as neither is
// rejected with INVALID_DATE.
func dateParam(w http.ResponseWriter, r *http.Request, key string, endOfDay bool) (*time.Time, bool) {
	t, ok := timeParam(r, key, endOfDay)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidDate,
			key+" must be an RFC3339 timestamp or YYYY-MM-DD date")
		return nil, false
	}
	return t, true
}

// enumParam reads an optional enum query parameter. Empty means "no filter".
// Any other value outside allowed is rejected with INVALID_ENUM, so an unknown
// filter value cannot be mistaken for the unfiltered result set.
func enumParam(w http.ResponseWriter, r *http.Request, key string, allowed map[string]bool, hint string) (string, bool) {
	v := strings.TrimSpace(r.URL.Query().Get(key))
	if v == "" {
		return "", true
	}
	if !allowed[v] {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidEnum, hint)
		return "", false
	}
	return v, true
}
