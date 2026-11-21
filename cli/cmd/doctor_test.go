package cmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const doctorTestKey = "sl_doctorTestSecretKey0123456789abcdef"

// mockAPI is a configurable stand-in for the Sorolens API.
type mockAPI struct {
	version       string // body of GET /api/version; "" answers 404
	contracts     int    // status of GET /api/v1/contracts
	watchdog      int    // status of GET /api/v1/watchdog/stats
	missingScope  string // "required" field of a 403 body
	health        int    // status of GET /health
	dateOffset    time.Duration
	omitDate      bool
	sawKeyOnProbe atomic.Bool // a key was sent to an unauthenticated probe
}

func newMockAPI() *mockAPI {
	return &mockAPI{version: "0.1.0", contracts: 200, watchdog: 200, health: 200}
}

func (m *mockAPI) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.omitDate {
			w.Header()["Date"] = nil
		} else {
			w.Header().Set("Date", time.Now().Add(m.dateOffset).UTC().Format(http.TimeFormat))
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/health":
			if r.Header.Get("Authorization") != "" || r.Header.Get("X-API-Key") != "" {
				m.sawKeyOnProbe.Store(true)
			}
			w.WriteHeader(m.health)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/api/version":
			if m.version == "" {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"version": m.version, "git_sha": "abc1234", "built_at": "dev"})
		case "/api/v1/contracts":
			m.writeScoped(w, m.contracts, "read:contracts")
		case "/api/v1/watchdog/stats":
			m.writeScoped(w, m.watchdog, "read:watchdog")
		default:
			http.NotFound(w, r)
		}
	})
}

func (m *mockAPI) writeScoped(w http.ResponseWriter, status int, scope string) {
	w.WriteHeader(status)
	switch status {
	case http.StatusUnauthorized:
		_, _ = w.Write([]byte(`{"error":"invalid API key"}`))
	case http.StatusForbidden:
		req := scope
		if m.missingScope != "" {
			req = m.missingScope
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "missing scope", "required": req})
	default:
		_, _ = w.Write([]byte(`{"contracts":[]}`))
	}
}

func setupDoctor(t *testing.T) {
	t.Helper()
	useTempHome(t)
	useFakeStore(t)
	t.Setenv("SOROLENS_API_KEY", "")
	t.Setenv("SOROLENS_OUTPUT", "")
	t.Setenv("SOROLENS_OUTPUT_FORMAT", "")
	t.Setenv("NO_COLOR", "1")
	oldTransport, oldNow := doctorTransport, doctorNow
	t.Cleanup(func() { doctorTransport, doctorNow = oldTransport, oldNow })
}

// runDoctorCmd runs `sorolens doctor` with args and returns its output and
// error.
func runDoctorCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	rootCmd.SetArgs(append([]string{"doctor"}, args...))
	return captureOutput(t, rootCmd.Execute)
}

// runDoctorJSON runs `sorolens doctor --json` and decodes the report.
func runDoctorJSON(t *testing.T, args ...string) (doctorReport, string, error) {
	t.Helper()
	out, err := runDoctorCmd(t, append([]string{"--json"}, args...)...)
	var rep doctorReport
	if jerr := json.Unmarshal([]byte(out), &rep); jerr != nil {
		t.Fatalf("doctor --json is not valid JSON: %v\n%s", jerr, out)
	}
	return rep, out, err
}

func checkByName(t *testing.T, rep doctorReport, name string) doctorCheck {
	t.Helper()
	for _, c := range rep.Checks {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("check %q missing from report: %+v", name, rep.Checks)
	return doctorCheck{}
}

func assertCheck(t *testing.T, rep doctorReport, name, status, msgContains string) doctorCheck {
	t.Helper()
	c := checkByName(t, rep, name)
	if c.Status != status {
		t.Fatalf("check %s: status %q, want %q (message: %s)", name, c.Status, status, c.Message)
	}
	if msgContains != "" && !strings.Contains(c.Message, msgContains) {
		t.Fatalf("check %s: message %q does not contain %q", name, c.Message, msgContains)
	}
	if status != doctorPass && c.Hint == "" {
		t.Fatalf("check %s: %s without a hint", name, status)
	}
	return c
}

func assertFailedExit(t *testing.T, err error) {
	t.Helper()
	var df errDoctorFailed
	if !errors.As(err, &df) {
		t.Fatalf("want a doctor failure error (non-zero exit), got %v", err)
	}
}

func TestDoctorAllChecksPass(t *testing.T) {
	setupDoctor(t)
	m := newMockAPI()
	srv := httptest.NewServer(m.handler())
	defer srv.Close()

	rep, out, err := runDoctorJSON(t, "--api-url", srv.URL, "--api-key", doctorTestKey)
	if err != nil {
		t.Fatalf("doctor failed: %v\n%s", err, out)
	}
	if !rep.OK || rep.Summary.Fail != 0 || rep.Summary.Warn != 0 || rep.Summary.Pass != 6 {
		t.Fatalf("want 6 passing checks, got %+v\n%s", rep.Summary, out)
	}
	want := []string{checkConfig, checkReach, checkTLS, checkAuth, checkVersion, checkClock}
	for i, c := range rep.Checks {
		if c.Name != want[i] {
			t.Fatalf("check %d is %q, want %q", i, c.Name, want[i])
		}
	}
	if rep.SchemaVersion != doctorSchemaVersion {
		t.Fatalf("schema_version %d", rep.SchemaVersion)
	}
	assertCheck(t, rep, checkAuth, doctorPass, "read:watchdog")
	assertCheck(t, rep, checkVersion, doctorPass, "0.1.0")
	if m.sawKeyOnProbe.Load() {
		t.Fatal("API key was sent to the unauthenticated /health probe")
	}
	if strings.Contains(out, doctorTestKey) {
		t.Fatal("API key leaked into --json output")
	}
}

func TestDoctorJSONSchemaIsStable(t *testing.T) {
	setupDoctor(t)
	srv := httptest.NewServer(newMockAPI().handler())
	defer srv.Close()

	_, out, err := runDoctorJSON(t, "--api-url", srv.URL, "--api-key", doctorTestKey)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"schema_version", "ok", "cli_version", "api_url", "profile", "summary", "checks"} {
		if _, ok := doc[k]; !ok {
			t.Errorf("top-level field %q missing", k)
		}
	}
	summary := doc["summary"].(map[string]any)
	for _, k := range []string{"pass", "warn", "fail"} {
		if _, ok := summary[k]; !ok {
			t.Errorf("summary field %q missing", k)
		}
	}
	for _, raw := range doc["checks"].([]any) {
		c := raw.(map[string]any)
		for _, k := range []string{"name", "status", "message"} {
			if _, ok := c[k]; !ok {
				t.Errorf("check field %q missing in %v", k, c)
			}
		}
	}
}

func TestDoctorUnreachable(t *testing.T) {
	setupDoctor(t)
	srv := httptest.NewServer(newMockAPI().handler())
	url := srv.URL
	srv.Close() // nothing listens there any more

	rep, out, err := runDoctorJSON(t, "--api-url", url, "--api-key", doctorTestKey, "--timeout", "2s")
	assertFailedExit(t, err)
	assertCheck(t, rep, checkReach, doctorFail, "Cannot reach")
	for _, name := range []string{checkTLS, checkAuth, checkVersion, checkClock} {
		assertCheck(t, rep, name, doctorWarn, "Not checked")
	}
	if rep.OK {
		t.Fatal("ok must be false when a check fails")
	}
	if strings.Contains(out, doctorTestKey) {
		t.Fatal("API key leaked")
	}
}

func TestDoctorUnauthorised(t *testing.T) {
	setupDoctor(t)
	m := newMockAPI()
	m.contracts = http.StatusUnauthorized
	srv := httptest.NewServer(m.handler())
	defer srv.Close()

	rep, _, err := runDoctorJSON(t, "--api-url", srv.URL, "--api-key", doctorTestKey)
	assertFailedExit(t, err)
	assertCheck(t, rep, checkAuth, doctorFail, "rejected")
}

func TestDoctorMissingScope(t *testing.T) {
	setupDoctor(t)
	m := newMockAPI()
	m.contracts = http.StatusForbidden
	srv := httptest.NewServer(m.handler())
	defer srv.Close()

	rep, _, err := runDoctorJSON(t, "--api-url", srv.URL, "--api-key", doctorTestKey)
	assertFailedExit(t, err)
	assertCheck(t, rep, checkAuth, doctorFail, "read:contracts")
}

func TestDoctorMissingWatchdogScopeWarns(t *testing.T) {
	setupDoctor(t)
	m := newMockAPI()
	m.watchdog = http.StatusForbidden
	srv := httptest.NewServer(m.handler())
	defer srv.Close()

	rep, _, err := runDoctorJSON(t, "--api-url", srv.URL, "--api-key", doctorTestKey)
	if err != nil {
		t.Fatalf("a warning must not fail the run: %v", err)
	}
	assertCheck(t, rep, checkAuth, doctorWarn, "read:watchdog")
}

func TestDoctorNoAPIKeyWarns(t *testing.T) {
	setupDoctor(t)
	srv := httptest.NewServer(newMockAPI().handler())
	defer srv.Close()

	rep, _, err := runDoctorJSON(t, "--api-url", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	assertCheck(t, rep, checkAuth, doctorWarn, "No API key")
}

func TestDoctorVersionMismatch(t *testing.T) {
	cases := []struct {
		version, status, contains string
	}{
		{"1.2.0", doctorFail, "newer than this CLI supports"},
		{"v0.0.9", doctorFail, "older than this CLI supports"},
		{"0.4.2-rc.1", doctorPass, "0.4.2-rc.1"},
		{"dev", doctorWarn, "development build"},
		{"", doctorWarn, "does not report its version"},
	}
	for _, tc := range cases {
		t.Run(tc.version, func(t *testing.T) {
			setupDoctor(t)
			m := newMockAPI()
			m.version = tc.version
			srv := httptest.NewServer(m.handler())
			defer srv.Close()

			rep, _, err := runDoctorJSON(t, "--api-url", srv.URL, "--api-key", doctorTestKey)
			assertCheck(t, rep, checkVersion, tc.status, tc.contains)
			if tc.status == doctorFail {
				assertFailedExit(t, err)
			} else if err != nil {
				t.Fatalf("unexpected failure: %v", err)
			}
		})
	}
}

func TestDoctorClockSkew(t *testing.T) {
	cases := []struct {
		name     string
		offset   time.Duration
		status   string
		contains string
	}{
		{"in sync", 0, doctorPass, "within"},
		{"server ahead 2m", 2 * time.Minute, doctorWarn, "behind the server"},
		{"server behind 10m", -10 * time.Minute, doctorFail, "ahead of the server"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupDoctor(t)
			m := newMockAPI()
			m.dateOffset = tc.offset
			srv := httptest.NewServer(m.handler())
			defer srv.Close()

			rep, _, err := runDoctorJSON(t, "--api-url", srv.URL, "--api-key", doctorTestKey)
			assertCheck(t, rep, checkClock, tc.status, tc.contains)
			if tc.status == doctorFail {
				assertFailedExit(t, err)
			}
		})
	}
}

func TestDoctorClockSkewFromLocalClock(t *testing.T) {
	setupDoctor(t)
	doctorNow = func() time.Time { return time.Now().Add(-time.Hour) }
	srv := httptest.NewServer(newMockAPI().handler())
	defer srv.Close()

	rep, _, err := runDoctorJSON(t, "--api-url", srv.URL, "--api-key", doctorTestKey)
	assertFailedExit(t, err)
	assertCheck(t, rep, checkClock, doctorFail, "behind the server")
}

func TestDoctorMissingDateHeaderWarns(t *testing.T) {
	setupDoctor(t)
	m := newMockAPI()
	m.omitDate = true
	srv := httptest.NewServer(m.handler())
	defer srv.Close()

	rep, _, _ := runDoctorJSON(t, "--api-url", srv.URL, "--api-key", doctorTestKey)
	c := checkByName(t, rep, checkClock)
	if c.Status != doctorWarn {
		t.Fatalf("clock without a Date header: %+v", c)
	}
}

func TestDoctorUntrustedCertificateFails(t *testing.T) {
	setupDoctor(t)
	srv := httptest.NewTLSServer(newMockAPI().handler())
	defer srv.Close()

	rep, _, err := runDoctorJSON(t, "--api-url", srv.URL, "--api-key", doctorTestKey)
	assertFailedExit(t, err)
	assertCheck(t, rep, checkReach, doctorFail, "")
	assertCheck(t, rep, checkTLS, doctorFail, "TLS handshake failed")
}

func TestDoctorTrustedCertificatePasses(t *testing.T) {
	setupDoctor(t)
	srv := httptest.NewTLSServer(newMockAPI().handler())
	defer srv.Close()
	doctorTransport = srv.Client().Transport

	rep, out, err := runDoctorJSON(t, "--api-url", srv.URL, "--api-key", doctorTestKey)
	if err != nil {
		t.Fatalf("doctor failed: %v\n%s", err, out)
	}
	assertCheck(t, rep, checkTLS, doctorPass, "TLS 1.")
}

func TestDoctorPlainHTTPToRemoteHostWarns(t *testing.T) {
	setupDoctor(t)
	run := &doctorRun{redact: newRedactor("")}
	globalConfig.APIURL = "http://api.example.com"
	if !run.checkURL(true) {
		t.Fatal("valid URL rejected")
	}
	run.checkTLS(&probeResult{})
	if c := run.checks[0]; c.Status != doctorWarn || !strings.Contains(c.Message, "plain HTTP") {
		t.Fatalf("plain HTTP to a remote host: %+v", c)
	}
}

func TestDoctorInvalidConfigFileFails(t *testing.T) {
	setupDoctor(t)
	home := useTempHome(t)
	// Broken YAML that also holds a key: neither may stop the run, and the
	// key must not be echoed through the parse error.
	writeCfg(t, home, "api_key: "+doctorTestKey+"\nprofiles: [unclosed\n")
	srv := httptest.NewServer(newMockAPI().handler())
	defer srv.Close()

	rep, out, err := runDoctorJSON(t, "--api-url", srv.URL)
	assertFailedExit(t, err)
	assertCheck(t, rep, checkConfig, doctorFail, "Cannot use")
	assertCheck(t, rep, checkReach, doctorPass, "")
	if strings.Contains(out, doctorTestKey) {
		t.Fatal("API key from the config file leaked")
	}
}

func TestDoctorUnknownProfileFails(t *testing.T) {
	setupDoctor(t)
	srv := httptest.NewServer(newMockAPI().handler())
	defer srv.Close()

	rep, _, err := runDoctorJSON(t, "--api-url", srv.URL, "--profile", "nope")
	assertFailedExit(t, err)
	assertCheck(t, rep, checkConfig, doctorFail, `"nope"`)
}

func TestDoctorInvalidAPIURLFails(t *testing.T) {
	setupDoctor(t)
	rep, _, err := runDoctorJSON(t, "--api-url", "localhost:8080")
	assertFailedExit(t, err)
	assertCheck(t, rep, checkConfig, doctorFail, "not a valid")
	assertCheck(t, rep, checkReach, doctorWarn, "Not checked")
}

func TestDoctorPlaintextConfigKeyWarns(t *testing.T) {
	setupDoctor(t)
	home := useTempHome(t)
	srv := httptest.NewServer(newMockAPI().handler())
	defer srv.Close()
	writeCfg(t, home, "api_url: "+srv.URL+"\napi_key: "+doctorTestKey+"\n")

	rep, out, err := runDoctorJSON(t)
	if err != nil {
		t.Fatalf("doctor failed: %v\n%s", err, out)
	}
	assertCheck(t, rep, checkConfig, doctorWarn, "plain text")
	assertCheck(t, rep, checkAuth, doctorPass, "")
	if strings.Contains(out, doctorTestKey) {
		t.Fatal("API key leaked")
	}
}

func TestDoctorProfileKeyFromStore(t *testing.T) {
	setupDoctor(t)
	home := useTempHome(t)
	store := useFakeStore(t)
	store.keys["prod"] = doctorTestKey
	srv := httptest.NewServer(newMockAPI().handler())
	defer srv.Close()
	writeCfg(t, home, "default_profile: prod\nprofiles:\n  prod:\n    api_url: "+srv.URL+"\n    key_ref: keyring\n")

	rep, out, err := runDoctorJSON(t)
	if err != nil {
		t.Fatalf("doctor failed: %v\n%s", err, out)
	}
	if rep.Profile != "prod" {
		t.Fatalf("profile %q", rep.Profile)
	}
	assertCheck(t, rep, checkConfig, doctorPass, `profile "prod"`)
	assertCheck(t, rep, checkAuth, doctorPass, "")
}

func TestDoctorNoSecretInAnyOutput(t *testing.T) {
	setupDoctor(t)
	// The server rejects the key and the URL carries credentials too: no
	// output format may print any of them.
	m := newMockAPI()
	m.contracts = http.StatusUnauthorized
	srv := httptest.NewServer(m.handler())
	defer srv.Close()
	apiURL := strings.Replace(srv.URL, "http://", "http://admin:hunter2pass@", 1) + "?api_key=" + doctorTestKey

	for _, args := range [][]string{{}, {"--json"}, {"--output", "yaml"}} {
		out, _ := runDoctorCmd(t, append(args, "--api-url", apiURL, "--api-key", doctorTestKey)...)
		for _, secret := range []string{doctorTestKey, "hunter2pass"} {
			if strings.Contains(out, secret) {
				t.Fatalf("output %v contains a secret:\n%s", args, out)
			}
		}
	}
}

func TestDoctorRedactor(t *testing.T) {
	r := newRedactor(doctorTestKey)
	in := "key " + doctorTestKey + " Authorization: Bearer abc.def-123 url?api_key=xyz&token=t0k other sl_ABCDEFGHIJKLMNOPQRSTUV"
	out := r(in)
	for _, s := range []string{doctorTestKey, "abc.def-123", "xyz", "t0k", "sl_ABCDEFGHIJKLMNOPQRSTUV"} {
		if strings.Contains(out, s) {
			t.Fatalf("redactor left %q in %q", s, out)
		}
	}
	if got := redactURL("https://user:pw@api.example.com/x?api_key=k&a=1"); strings.Contains(got, "pw") || strings.Contains(got, "api_key") || got != "https://api.example.com/x" {
		t.Fatalf("redactURL: %q", got)
	}
}

func TestDoctorHumanOutput(t *testing.T) {
	setupDoctor(t)
	m := newMockAPI()
	m.version = "9.0.0"
	srv := httptest.NewServer(m.handler())
	defer srv.Close()

	out, err := runDoctorCmd(t, "--api-url", srv.URL, "--api-key", doctorTestKey)
	assertFailedExit(t, err)
	for _, want := range []string{"PASS  config", "FAIL  version", "hint: Upgrade the sorolens CLI.", "5 passed, 0 warnings, 1 failed"} {
		if !strings.Contains(out, want) {
			t.Fatalf("human output missing %q:\n%s", want, out)
		}
	}
}

func TestParseSemver(t *testing.T) {
	cases := map[string]bool{"0.1.0": true, "v1.2.3": true, "1.2.3-rc.1+build": true, "1.2": false, "x.y.z": false, "dev": false}
	for in, ok := range cases {
		if _, got := parseSemver(in); got != ok {
			t.Errorf("parseSemver(%q) ok=%v, want %v", in, got, ok)
		}
	}
	a, _ := parseSemver("0.9.9")
	b, _ := parseSemver("1.0.0")
	if compareSemver(a, b) >= 0 || compareSemver(b, a) <= 0 || compareSemver(a, a) != 0 {
		t.Fatal("compareSemver ordering")
	}
}
