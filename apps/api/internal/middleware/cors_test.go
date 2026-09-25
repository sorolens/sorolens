package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestCORSAllowsAnyOriginByDefault(t *testing.T) {
	t.Parallel()

	nextHit := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHit = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://anything.example")
	rec := httptest.NewRecorder()

	NewCORS(nil)(next).ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Allow-Origin = %q, want *", got)
	}
	if !nextHit {
		t.Error("next handler was not called")
	}
}

func TestCORSReflectsAllowedOrigin(t *testing.T) {
	t.Parallel()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := NewCORS([]string{"https://sorolens-web-iota.vercel.app"})

	tests := []struct {
		name       string
		origin     string
		wantOrigin string
	}{
		{
			name:       "allowed origin is reflected",
			origin:     "https://sorolens-web-iota.vercel.app",
			wantOrigin: "https://sorolens-web-iota.vercel.app",
		},
		{
			name:       "unknown origin is not reflected",
			origin:     "https://evil.example",
			wantOrigin: "",
		},
		{
			name:       "missing origin header is not reflected",
			origin:     "",
			wantOrigin: "",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			rec := httptest.NewRecorder()

			mw(next).ServeHTTP(rec, req)

			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tc.wantOrigin {
				t.Errorf("Allow-Origin = %q, want %q", got, tc.wantOrigin)
			}
			if got := rec.Header().Get("Vary"); got != "Origin" {
				t.Errorf("Vary = %q, want Origin", got)
			}
		})
	}
}

func TestCORSHandlesPreflight(t *testing.T) {
	t.Parallel()

	nextHit := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHit = true
	})

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/contracts", nil)
	req.Header.Set("Origin", "https://sorolens-web-iota.vercel.app")
	rec := httptest.NewRecorder()

	NewCORS([]string{"https://sorolens-web-iota.vercel.app"})(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if nextHit {
		t.Error("preflight request reached the next handler")
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("Allow-Methods header is empty")
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Error("Allow-Headers header is empty")
	}
}

func TestAllowedOriginsFromEnv(t *testing.T) {
	tests := []struct {
		name  string
		value string
		set   bool
		want  []string
	}{
		{name: "unset means allow any", set: false, want: nil},
		{name: "empty means allow any", set: true, value: "", want: nil},
		{name: "wildcard means allow any", set: true, value: "*", want: nil},
		{name: "only separators means allow any", set: true, value: " , ,", want: nil},
		{
			name:  "comma separated list is trimmed",
			set:   true,
			value: " https://a.example , https://b.example ",
			want:  []string{"https://a.example", "https://b.example"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.set {
				t.Setenv(AllowedOriginsEnv, tc.value)
			} else {
				// Exercise the "variable not present" branch.
				original, had := os.LookupEnv(AllowedOriginsEnv)
				if err := os.Unsetenv(AllowedOriginsEnv); err != nil {
					t.Fatalf("unset %s: %v", AllowedOriginsEnv, err)
				}
				t.Cleanup(func() {
					if had {
						_ = os.Setenv(AllowedOriginsEnv, original)
					}
				})
			}

			got := AllowedOriginsFromEnv()
			if len(got) != len(tc.want) {
				t.Fatalf("AllowedOriginsFromEnv() = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("origin[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}
