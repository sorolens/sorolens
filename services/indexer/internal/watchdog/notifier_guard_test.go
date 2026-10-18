package watchdog

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestIsBlockedAddr(t *testing.T) {
	cases := []struct {
		addr    string
		blocked bool
	}{
		// IPv4
		{"127.0.0.1", true},
		{"127.255.255.254", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.168.1.1", true},
		{"169.254.169.254", true}, // AWS/GCP/Azure metadata
		{"169.254.0.1", true},
		{"100.100.100.200", true}, // Alibaba metadata (CGNAT)
		{"100.64.0.1", true},
		{"0.0.0.0", true},
		{"0.1.2.3", true},
		{"224.0.0.1", true},
		{"255.255.255.255", true},
		{"240.0.0.1", true},
		{"192.0.2.1", true},
		{"198.18.0.1", true},
		{"8.8.8.8", false},
		{"93.184.216.34", false},
		{"172.15.0.1", false},
		{"172.32.0.1", false},
		{"100.63.255.255", false},
		// IPv6
		{"::1", true},
		{"::", true},
		{"fe80::1", true},
		{"fe80::1%eth0", true},
		{"fc00::1", true},
		{"fd00:ec2::254", true}, // AWS IPv6 metadata
		{"ff02::1", true},
		{"2001:db8::1", true},
		{"2001::1", true},
		{"2606:4700:4700::1111", false},
		{"2001:4860:4860::8888", false},
		// IPv4 carried inside IPv6
		{"::ffff:127.0.0.1", true},
		{"::ffff:169.254.169.254", true},
		{"::ffff:10.0.0.1", true},
		{"::ffff:8.8.8.8", false},
		{"64:ff9b::7f00:1", true},    // NAT64 -> 127.0.0.1
		{"64:ff9b::a9fe:a9fe", true}, // NAT64 -> 169.254.169.254
		{"64:ff9b::808:808", false},  // NAT64 -> 8.8.8.8
		{"2002:7f00:1::", true},      // 6to4 -> 127.0.0.1
		{"2002:a9fe:a9fe::", true},   // 6to4 -> 169.254.169.254
		{"2002:808:808::", false},    // 6to4 -> 8.8.8.8
	}
	for _, tc := range cases {
		t.Run(tc.addr, func(t *testing.T) {
			if got := isBlockedAddr(netip.MustParseAddr(tc.addr)); got != tc.blocked {
				t.Fatalf("isBlockedAddr(%s) = %v, want %v", tc.addr, got, tc.blocked)
			}
		})
	}
	if !isBlockedAddr(netip.Addr{}) {
		t.Error("the zero Addr must be blocked")
	}
}

func fixedLookup(m map[string][]string) lookupFunc {
	return func(_ context.Context, host string) ([]netip.Addr, error) {
		ips, ok := m[host]
		if !ok {
			return nil, errors.New("no such host")
		}
		out := make([]netip.Addr, len(ips))
		for i, s := range ips {
			out[i] = netip.MustParseAddr(s)
		}
		return out, nil
	}
}

func TestParseAllowlist(t *testing.T) {
	l := parseAllowlist(" Hooks.Example.com, *.slack.com ,,")
	if len(l) != 2 || l[0] != "hooks.example.com" || l[1] != "*.slack.com" {
		t.Fatalf("parseAllowlist = %v", l)
	}
	if !l.allows("HOOKS.example.com.") || !l.allows("a.slack.com") || l.allows("slack.com") || l.allows("evil.com") {
		t.Error("Allows disagrees with the parsed list")
	}
	if !parseAllowlist("").allows("anything.example") {
		t.Error("an empty allowlist must permit all hosts")
	}
}

func TestAllowedHostsFromEnv(t *testing.T) {
	t.Setenv(envAllowedHosts, "a.example.com")
	if l := allowedHostsFromEnv(); !l.allows("a.example.com") || l.allows("b.example.com") {
		t.Fatalf("allowedHostsFromEnv = %v", l)
	}
}

func get(t *testing.T, c *http.Client, url string) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	return c.Do(req)
}

func allowAll(netip.Addr) bool { return false }

// The default policy refuses a real loopback server: this is the end-to-end
// check that the guarded client is wired to isBlockedAddr.
func TestGuardedClientRefusesLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("request reached a loopback server")
	}))
	defer srv.Close()

	_, err := get(t, newGuardedClient(2*time.Second, nil, nil, isBlockedAddr), srv.URL)
	if !errors.Is(err, errBlockedDestination) {
		t.Fatalf("err = %v, want errBlockedDestination", err)
	}
}

func TestGuardedClientDialsTheCheckedAddress(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()
	_, port, _ := net.SplitHostPort(srv.Listener.Addr().String())

	lookup := fixedLookup(map[string][]string{"hook.test": {"127.0.0.1"}})
	resp, err := get(t, newGuardedClient(2*time.Second, nil, lookup, allowAll), "http://hook.test:"+port+"/")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

// A host that validated as public at creation but now resolves inside must be
// refused when the delivery dials (DNS rebinding).
func TestGuardedClientRefusesRebinding(t *testing.T) {
	answers := [][]netip.Addr{
		{netip.MustParseAddr("93.184.216.34")},
		{netip.MustParseAddr("169.254.169.254")},
	}
	var calls atomic.Int32
	lookup := func(context.Context, string) ([]netip.Addr, error) {
		i := int(calls.Add(1)) - 1
		if i >= len(answers) {
			i = len(answers) - 1
		}
		return answers[i], nil
	}

	// The first answer (as seen when the subscription was created) is public;
	// the one the delivery gets is the metadata address.
	if _, err := resolveHost(context.Background(), "rebind.test", lookup); err != nil {
		t.Fatal(err)
	}
	_, err := get(t, newGuardedClient(2*time.Second, nil, lookup, isBlockedAddr), "http://rebind.test/hook")
	if !errors.Is(err, errBlockedDestination) {
		t.Fatalf("delivery err = %v, want errBlockedDestination", err)
	}
}

func TestGuardedClientRefusesBadRecordsAndLiterals(t *testing.T) {
	lookup := fixedLookup(map[string][]string{
		"mixed.test": {"93.184.216.34", "10.0.0.1"},
		"v6.test":    {"fd00:ec2::254"},
	})
	c := newGuardedClient(2*time.Second, nil, lookup, isBlockedAddr)
	for _, host := range []string{"mixed.test", "v6.test", "169.254.169.254", "[::1]", "[::ffff:127.0.0.1]"} {
		if _, err := get(t, c, "http://"+host+"/"); !errors.Is(err, errBlockedDestination) {
			t.Errorf("%s: err = %v, want errBlockedDestination", host, err)
		}
	}
}

func TestGuardedClientRefusesRedirectToPrivateAddress(t *testing.T) {
	var reached atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
			return
		}
		reached.Store(true)
	}))
	defer srv.Close()

	// 127.0.0.1 (the test server) is permitted; the redirect target is not.
	policy := func(a netip.Addr) bool { return a != netip.MustParseAddr("127.0.0.1") }
	_, err := get(t, newGuardedClient(2*time.Second, nil, nil, policy), srv.URL+"/start")
	if !errors.Is(err, errBlockedDestination) {
		t.Fatalf("err = %v, want errBlockedDestination", err)
	}
	if reached.Load() {
		t.Error("redirect target was contacted")
	}
}

func TestGuardedClientLimitsRedirects(t *testing.T) {
	var hops atomic.Int32
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hops.Add(1)
		http.Redirect(w, r, srv.URL+"/again", http.StatusFound)
	}))
	defer srv.Close()

	_, err := get(t, newGuardedClient(2*time.Second, nil, nil, allowAll), srv.URL)
	if err == nil || !strings.Contains(err.Error(), "redirects") {
		t.Fatalf("err = %v, want redirect-limit error", err)
	}
	if got := hops.Load(); got != maxRedirects+1 {
		t.Fatalf("server saw %d requests, want %d", got, maxRedirects+1)
	}
}

func TestGuardedClientRefusesRedirectOffAllowlist(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://other.example.net/", http.StatusFound)
	}))
	defer srv.Close()

	c := newGuardedClient(2*time.Second, hostAllowlist{"127.0.0.1"}, nil, allowAll)
	if _, err := get(t, c, srv.URL); !errors.Is(err, errBlockedDestination) {
		t.Fatalf("err = %v, want errBlockedDestination", err)
	}
}

func TestGuardedClientRefusesNonHTTPRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "gopher://127.0.0.1/")
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()

	if _, err := get(t, newGuardedClient(2*time.Second, nil, nil, allowAll), srv.URL); err == nil {
		t.Fatal("expected the non-http redirect to be refused")
	}
}
