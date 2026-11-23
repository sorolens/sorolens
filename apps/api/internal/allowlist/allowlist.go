// Package allowlist validates and matches the per-key network and browser
// origin restrictions added in issue #793.
//
// A key stores two optional lists: allowed_cidrs (source addresses) and
// allowed_origins (browser Origins). An empty list means "no restriction", so
// keys created before this feature keep working unchanged.
package allowlist

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

var (
	// ErrInvalidCIDR reports an allowed_cidrs entry that is neither a CIDR
	// range nor an IP address.
	ErrInvalidCIDR = errors.New("invalid CIDR range")
	// ErrInvalidOrigin reports an allowed_origins entry that is not a bare
	// http(s) origin.
	ErrInvalidOrigin = errors.New("invalid origin")
)

// ParseCIDRs validates and canonicalizes a list of source networks.
//
// Each entry is a CIDR range ("203.0.113.0/24", "2001:db8::/32") or a bare IP
// address, which is stored as a single-host range (/32 or /128). Host bits are
// cleared (ParseCIDR semantics), duplicates are dropped and input order is
// preserved, so the stored value is exactly what the matcher compares against.
func ParseCIDRs(entries []string) ([]string, error) {
	out := make([]string, 0, len(entries))
	seen := make(map[string]bool, len(entries))
	for _, raw := range entries {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			return nil, fmt.Errorf("%w: allowed_cidrs contains an empty entry; expected a range such as 203.0.113.0/24", ErrInvalidCIDR)
		}
		normalized, err := normalizeCIDR(entry)
		if err != nil {
			return nil, err
		}
		if seen[normalized] {
			continue
		}
		seen[normalized] = true
		out = append(out, normalized)
	}
	return out, nil
}

func normalizeCIDR(entry string) (string, error) {
	if strings.Contains(entry, "/") {
		_, network, err := net.ParseCIDR(entry)
		if err != nil {
			return "", fmt.Errorf("%w: %q is not a valid CIDR range", ErrInvalidCIDR, entry)
		}
		return network.String(), nil
	}
	ip := net.ParseIP(entry)
	if ip == nil {
		return "", fmt.Errorf("%w: %q is not an IP address or CIDR range", ErrInvalidCIDR, entry)
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.String() + "/32", nil
	}
	return ip.String() + "/128", nil
}

// ParseOrigins validates and canonicalizes a list of browser Origins.
//
// An origin is a scheme and host only ("https://app.example.com",
// "http://localhost:3000"). Paths, queries, credentials and wildcards are
// rejected: a suffix or wildcard match would let one entry cover hosts the
// operator did not intend. Scheme and host are lower-cased, duplicates are
// dropped and input order is preserved.
func ParseOrigins(entries []string) ([]string, error) {
	out := make([]string, 0, len(entries))
	seen := make(map[string]bool, len(entries))
	for _, raw := range entries {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			return nil, fmt.Errorf("%w: allowed_origins contains an empty entry; expected an origin such as https://app.example.com", ErrInvalidOrigin)
		}
		normalized, err := normalizeOrigin(entry)
		if err != nil {
			return nil, err
		}
		if seen[normalized] {
			continue
		}
		seen[normalized] = true
		out = append(out, normalized)
	}
	return out, nil
}

func normalizeOrigin(entry string) (string, error) {
	parsed, err := url.Parse(entry)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("%w: %q must look like https://app.example.com", ErrInvalidOrigin, entry)
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
	default:
		return "", fmt.Errorf("%w: %q must use http or https", ErrInvalidOrigin, entry)
	}
	if strings.Contains(parsed.Host, "*") {
		return "", fmt.Errorf("%w: %q must not contain a wildcard; list each origin explicitly", ErrInvalidOrigin, entry)
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		(parsed.Path != "" && parsed.Path != "/") {
		return "", fmt.Errorf("%w: %q must be a scheme and host only, with no path or query", ErrInvalidOrigin, entry)
	}
	return strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host), nil
}

// IPAllowed reports whether clientIP satisfies the source restrictions. An
// empty list is unrestricted; an address that cannot be parsed is never
// allowed, so a broken RemoteAddr fails closed.
func IPAllowed(entries []string, clientIP string) bool {
	if len(entries) == 0 {
		return true
	}
	ip := net.ParseIP(strings.Trim(strings.TrimSpace(clientIP), "[]"))
	if ip == nil {
		return false
	}
	for _, entry := range entries {
		if ipInRange(ip, entry) {
			return true
		}
	}
	return false
}

func ipInRange(ip net.IP, entry string) bool {
	if strings.Contains(entry, "/") {
		_, network, err := net.ParseCIDR(entry)
		if err != nil {
			return false
		}
		return network.Contains(ip)
	}
	other := net.ParseIP(entry)
	return other != nil && other.Equal(ip)
}

// OriginAllowed reports whether a request carrying origin may use the key.
//
// An empty list is unrestricted. A request without an Origin header is not a
// browser cross-origin request (server-side clients do not send one), so it is
// allowed; the literal "null" origin is not, unless it was explicitly listed.
func OriginAllowed(entries []string, origin string) bool {
	if len(entries) == 0 {
		return true
	}
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return true
	}
	origin = normalizeOriginForCompare(origin)
	for _, entry := range entries {
		if normalizeOriginForCompare(entry) == origin {
			return true
		}
	}
	return false
}

func normalizeOriginForCompare(origin string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(origin), "/"))
}
