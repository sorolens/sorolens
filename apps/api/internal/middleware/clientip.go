package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

// ClientIPResolver resolves the originating client address of a request. It
// only believes X-Forwarded-For when the immediate peer is a configured
// trusted proxy, so a caller cannot choose the address an API key's allowlist
// is checked against (issue #793). With no trusted proxies every request uses
// its transport peer address.
type ClientIPResolver struct {
	trusted []*net.IPNet
}

// NewClientIPResolver parses trusted proxy CIDR ranges. An empty list trusts
// no proxy. A malformed range is a configuration error and is returned.
func NewClientIPResolver(trustedCIDRs []string) (*ClientIPResolver, error) {
	r := &ClientIPResolver{}
	for _, raw := range trustedCIDRs {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		_, network, err := net.ParseCIDR(s)
		if err != nil {
			return nil, fmt.Errorf("invalid trusted proxy CIDR %q", s)
		}
		if ones, _ := network.Mask.Size(); ones == 0 {
			// Trusting every address would let any caller supply its own
			// X-Forwarded-For, defeating the allowlist.
			return nil, fmt.Errorf("trusted proxy CIDR %q covers every address", s)
		}
		r.trusted = append(r.trusted, network)
	}
	return r, nil
}

// ClientIP returns the client address for r. Behind a trusted proxy it walks
// X-Forwarded-For from right to left and returns the first address that is not
// itself a trusted proxy, which is the furthest hop the proxy vouches for.
// A malformed entry to the right of that hop makes the chain unverifiable, so
// it fails closed and returns nil; if every hop is trusted the peer is
// returned.
func (c *ClientIPResolver) ClientIP(r *http.Request) net.IP {
	if r == nil {
		return nil
	}
	peer := parseIP(r.RemoteAddr)
	if c == nil || peer == nil || !c.trusts(peer) {
		return peer
	}
	var last net.IP
	// A proxy may append a separate X-Forwarded-For line instead of extending
	// the first one, so every line is joined before the right-to-left walk.
	entries := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(entries) - 1; i >= 0; i-- {
		if strings.TrimSpace(entries[i]) == "" {
			continue
		}
		ip := parseIP(entries[i])
		if ip == nil {
			// A hop we cannot parse means we cannot know which address the
			// proxy vouched for, so do not fall back to a guess.
			return nil
		}
		last = ip
		if !c.trusts(ip) {
			return ip
		}
	}
	if last != nil {
		return last
	}
	return peer
}

func (c *ClientIPResolver) trusts(ip net.IP) bool {
	if c == nil {
		return false
	}
	for _, network := range c.trusted {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// parseIP accepts a bare address, a host:port pair, or a bracketed IPv6
// address as found in RemoteAddr, X-Forwarded-For entries and headers.
func parseIP(raw string) net.IP {
	s := strings.TrimSpace(strings.Trim(raw, `"`))
	if s == "" {
		return nil
	}
	if host, _, err := net.SplitHostPort(s); err == nil {
		s = host
	}
	s = strings.Trim(s, "[]")
	return net.ParseIP(s)
}

// PeerIP returns the transport peer address without consulting any forwarded
// header. It is used where trusting a header would weaken a security decision.
func PeerIP(r *http.Request) net.IP {
	if r == nil {
		return nil
	}
	return parseIP(r.RemoteAddr)
}
