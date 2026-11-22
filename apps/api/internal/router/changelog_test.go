package router_test

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/handler"
	"github.com/sorolens/sorolens/apps/api/internal/router"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

const (
	changelogContractID = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
	changelogEmptyID    = "CEMPTYCHANGELOGCONTRACT"
)

// changelogServer serves the full router over a mock store holding one
// contract with two recorded Wasm hashes and one contract with none.
func changelogServer(t *testing.T) *httptest.Server {
	t.Helper()
	st := store.NewMockStore()
	ctx := context.Background()
	for _, id := range []string{changelogContractID, changelogEmptyID} {
		if err := st.UpsertContract(ctx, store.Contract{ID: id, Network: "testnet", Status: "active"}); err != nil {
			t.Fatal(err)
		}
	}
	for _, v := range []store.ContractVersion{
		{ID: 1, ContractID: changelogContractID, WasmHash: strings.Repeat("ab", 32), FirstSeenLedger: 100, RecordedAt: time.Now().Add(-time.Hour)},
		{ID: 2, ContractID: changelogContractID, WasmHash: strings.Repeat("de", 32), FirstSeenLedger: 200, RecordedAt: time.Now()},
	} {
		if err := st.RecordContractVersion(ctx, v); err != nil {
			t.Fatal(err)
		}
	}
	srv := httptest.NewServer(router.New(&handler.Handler{
		Store:       st,
		DB:          &store.MockPinger{Healthy: true},
		Redis:       &store.MockPinger{Healthy: true},
		RedisClient: noLimitRedis{},
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, 1<<20))
	t.Cleanup(srv.Close)
	return srv
}

func changelogGet(t *testing.T, url string, header http.Header) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return resp, body
}

// TestRouter_ChangelogRoutesAreRegistered checks each changelog endpoint is
// reachable through the router without a credential, like the rest of the
// public per-contract read surface, and answers with its own content type.
func TestRouter_ChangelogRoutesAreRegistered(t *testing.T) {
	srv := changelogServer(t)
	base := srv.URL + "/api/v1/contracts/" + changelogContractID

	tests := []struct {
		path        string
		contentType string
		contains    string
	}{
		{"/changelog", "application/json", `"wasm_hash":"` + strings.Repeat("de", 32) + `"`},
		{"/changelog/feed", "application/atom+xml", "<feed"},
		{"/changelog/badge", "image/svg+xml", "dededede"},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			resp, body := changelogGet(t, base+tc.path, nil)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d (%s), want 200", resp.StatusCode, body)
			}
			if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, tc.contentType) {
				t.Errorf("Content-Type = %q, want %s", ct, tc.contentType)
			}
			if !strings.Contains(string(body), tc.contains) {
				t.Errorf("body does not contain %q:\n%s", tc.contains, body)
			}
		})
	}
}

// TestRouter_ChangelogFeedLinksIncludeAPIPrefix guards the feed's links:
// they must point back at the /api/v1 routes, not at a bare /contracts path.
func TestRouter_ChangelogFeedLinksIncludeAPIPrefix(t *testing.T) {
	srv := changelogServer(t)
	resp, body := changelogGet(t, srv.URL+"/api/v1/contracts/"+changelogContractID+"/changelog/feed", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d (%s), want 200", resp.StatusCode, body)
	}
	var feed struct {
		ID   string `xml:"id"`
		Link struct {
			Href string `xml:"href,attr"`
		} `xml:"link"`
		Entries []struct {
			ID   string `xml:"id"`
			Link struct {
				Href string `xml:"href,attr"`
			} `xml:"link"`
		} `xml:"entry"`
	}
	if err := xml.Unmarshal(body, &feed); err != nil {
		t.Fatalf("parse feed: %v\n%s", err, body)
	}
	self := srv.URL + "/api/v1/contracts/" + changelogContractID + "/changelog/feed"
	if feed.ID != self || feed.Link.Href != self {
		t.Errorf("feed id/self link = %q / %q, want %q", feed.ID, feed.Link.Href, self)
	}
	if len(feed.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(feed.Entries))
	}
	entryLink := srv.URL + "/api/v1/contracts/" + changelogContractID + "/changelog"
	for _, e := range feed.Entries {
		if e.Link.Href != entryLink || !strings.HasPrefix(e.ID, entryLink+"#") {
			t.Errorf("entry id/link = %q / %q, want under %q", e.ID, e.Link.Href, entryLink)
		}
	}
}

// TestRouter_ChangelogUnknownContract checks the JSON and feed endpoints
// answer an untracked contract with the standard 404 error envelope, carrying
// the request ID the RequestID middleware assigned, while the badge still
// renders so an embedded image never breaks.
func TestRouter_ChangelogUnknownContract(t *testing.T) {
	srv := changelogServer(t)
	base := srv.URL + "/api/v1/contracts/CUNKNOWN"

	for _, path := range []string{"/changelog", "/changelog/feed"} {
		t.Run(path, func(t *testing.T) {
			resp, body := changelogGet(t, base+path, nil)
			if resp.StatusCode != http.StatusNotFound {
				t.Fatalf("status = %d (%s), want 404", resp.StatusCode, body)
			}
			if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}
			var env struct {
				Error struct {
					Code      string `json:"code"`
					Message   string `json:"message"`
					RequestID string `json:"request_id"`
				} `json:"error"`
			}
			if err := json.Unmarshal(body, &env); err != nil {
				t.Fatalf("decode error body: %v\n%s", err, body)
			}
			if env.Error.Code != "NOT_FOUND" || env.Error.Message != "contract not found" {
				t.Errorf("error = %+v, want NOT_FOUND / contract not found", env.Error)
			}
			if rid := resp.Header.Get("X-Request-ID"); rid == "" || env.Error.RequestID != rid {
				t.Errorf("request_id = %q, want X-Request-ID %q", env.Error.RequestID, rid)
			}
		})
	}

	t.Run("/changelog/badge", func(t *testing.T) {
		resp, body := changelogGet(t, base+"/changelog/badge", nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d (%s), want 200", resp.StatusCode, body)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "image/svg+xml" {
			t.Errorf("Content-Type = %q, want image/svg+xml", ct)
		}
		if !strings.Contains(string(body), "unknown") {
			t.Errorf("badge does not show 'unknown':\n%s", body)
		}
	})
}

// TestRouter_ChangelogEmptyHistory checks a tracked contract with no
// recorded versions gets empty results rather than an error.
func TestRouter_ChangelogEmptyHistory(t *testing.T) {
	srv := changelogServer(t)
	base := srv.URL + "/api/v1/contracts/" + changelogEmptyID

	resp, body := changelogGet(t, base+"/changelog", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("changelog status = %d (%s), want 200", resp.StatusCode, body)
	}
	if got := strings.TrimSpace(string(body)); got != "[]" {
		t.Errorf("changelog body = %s, want []", got)
	}

	resp, body = changelogGet(t, base+"/changelog/feed", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("feed status = %d (%s), want 200", resp.StatusCode, body)
	}
	if strings.Contains(string(body), "<entry>") {
		t.Errorf("feed has entries for an empty history:\n%s", body)
	}
}

// TestRouter_ChangelogCachingHeaders checks the global caching middleware
// applies to the changelog like any other read: a weak ETag that answers a
// matching If-None-Match with 304. The badge keeps its own no-cache header
// through the middleware stack so READMEs revalidate on every view.
func TestRouter_ChangelogCachingHeaders(t *testing.T) {
	srv := changelogServer(t)
	base := srv.URL + "/api/v1/contracts/" + changelogContractID

	resp, body := changelogGet(t, base+"/changelog", nil)
	etag := resp.Header.Get("ETag")
	if resp.StatusCode != http.StatusOK || etag == "" {
		t.Fatalf("status = %d, ETag = %q (%s); want 200 with an ETag", resp.StatusCode, etag, body)
	}
	resp, body = changelogGet(t, base+"/changelog", http.Header{"If-None-Match": {etag}})
	if resp.StatusCode != http.StatusNotModified {
		t.Errorf("conditional GET status = %d, want 304", resp.StatusCode)
	}
	if len(body) != 0 {
		t.Errorf("304 carried a body: %s", body)
	}

	resp, _ = changelogGet(t, base+"/changelog/badge", nil)
	if cc := resp.Header.Get("Cache-Control"); cc != "no-cache, max-age=0" {
		t.Errorf("badge Cache-Control = %q, want no-cache, max-age=0", cc)
	}
}
