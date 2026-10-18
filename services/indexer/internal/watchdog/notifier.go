package watchdog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"time"
)

// Notification channel types (alert_subscriptions.channel_type, issue #127).
const (
	ChannelWebhook   = "webhook"
	ChannelSlack     = "slack"
	ChannelDiscord   = "discord"
	ChannelPagerDuty = "pagerduty"
)

// PagerDutyEventsURL is the PagerDuty Events API v2 enqueue endpoint, used
// when a pagerduty subscription does not override webhook_url.
const PagerDutyEventsURL = "https://events.pagerduty.com/v2/enqueue"

// AlertSubscriptionStore is the local interface for alert subscription
// storage, kept separate from apps/api to avoid internal package imports.
type AlertSubscriptionStore interface {
	ListByContract(ctx context.Context, contractID string) ([]AlertSubscription, error)
}

// AlertSubscription is the local mirror of the store model.
type AlertSubscription struct {
	ID             string
	ContractID     string
	WebhookURL     string
	SeverityFilter string
	// ChannelType selects the payload format; empty means webhook.
	ChannelType string
	// RoutingKey is the PagerDuty integration key (pagerduty only).
	RoutingKey string
}

// explorerURL links an alert to the transaction that raised it.
func explorerURL(a Alert) string {
	return fmt.Sprintf("https://sorobanexplorer.com/transaction/%s", a.TxHash)
}

// FormatNotification renders the HTTP request body for one subscription's
// channel and returns it with the URL to POST it to.
func FormatNotification(sub AlertSubscription, alert Alert) (string, []byte, error) {
	var (
		payload any
		url     = sub.WebhookURL
	)
	switch sub.ChannelType {
	case "", ChannelWebhook:
		payload = webhookPayload(alert)
	case ChannelSlack:
		payload = slackPayload(alert)
	case ChannelDiscord:
		payload = discordPayload(alert)
	case ChannelPagerDuty:
		if sub.RoutingKey == "" {
			return "", nil, fmt.Errorf("pagerduty subscription %s has no routing key", sub.ID)
		}
		payload = pagerDutyPayload(alert, sub.RoutingKey)
		if url == "" {
			url = PagerDutyEventsURL
		}
	default:
		return "", nil, fmt.Errorf("unknown channel type %q", sub.ChannelType)
	}
	if url == "" {
		return "", nil, fmt.Errorf("subscription %s has no webhook url", sub.ID)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", nil, err
	}
	return url, body, nil
}

// webhookPayload is the generic JSON body (unchanged from the original
// webhook-only notifier so existing receivers keep working).
func webhookPayload(a Alert) map[string]any {
	return map[string]any{
		"contract_id":  a.ContractID,
		"severity":     a.Severity,
		"message":      a.Message,
		"timestamp":    a.Timestamp.Format(time.RFC3339),
		"explorer_url": explorerURL(a),
	}
}

// severityEmoji prefixes Slack text so severity is visible in notifications.
func severityEmoji(severity string) string {
	switch severity {
	case "Critical":
		return ":rotating_light:"
	case "Warning":
		return ":warning:"
	default:
		return ":information_source:"
	}
}

// truncate caps s at n runes, marking the cut with an ellipsis.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// slackPayload is a Slack incoming-webhook message using Block Kit. `text`
// is the fallback shown in push notifications.
func slackPayload(a Alert) map[string]any {
	title := fmt.Sprintf("%s %s alert", severityEmoji(a.Severity), a.Severity)
	return map[string]any{
		"text": fmt.Sprintf("%s on %s: %s", title, a.ContractID, truncate(a.Message, 200)),
		"blocks": []any{
			map[string]any{
				"type": "header",
				"text": map[string]any{"type": "plain_text", "text": truncate(fmt.Sprintf("%s alert", a.Severity), 150), "emoji": true},
			},
			map[string]any{
				"type": "section",
				"fields": []any{
					map[string]any{"type": "mrkdwn", "text": fmt.Sprintf("*Contract*\n`%s`", a.ContractID)},
					map[string]any{"type": "mrkdwn", "text": fmt.Sprintf("*Severity*\n%s %s", severityEmoji(a.Severity), a.Severity)},
				},
			},
			map[string]any{
				"type": "section",
				"text": map[string]any{"type": "mrkdwn", "text": truncate(a.Message, 2900)},
			},
			map[string]any{
				"type": "context",
				"elements": []any{
					map[string]any{"type": "mrkdwn", "text": fmt.Sprintf("Ledger %d · <!date^%d^{date_short_pretty} {time}|%s>",
						a.Ledger, a.Timestamp.Unix(), a.Timestamp.UTC().Format(time.RFC3339))},
				},
			},
			map[string]any{
				"type": "actions",
				"elements": []any{
					map[string]any{
						"type": "button",
						"text": map[string]any{"type": "plain_text", "text": "View transaction"},
						"url":  explorerURL(a),
					},
				},
			},
		},
	}
}

// Discord embed colors per severity.
const (
	discordColorCritical = 0xE5484D
	discordColorWarning  = 0xF5A524
	discordColorInfo     = 0x3B82F6
)

func discordColor(severity string) int {
	switch severity {
	case "Critical":
		return discordColorCritical
	case "Warning":
		return discordColorWarning
	default:
		return discordColorInfo
	}
}

// discordPayload is a Discord webhook message with one embed.
func discordPayload(a Alert) map[string]any {
	return map[string]any{
		"username": "Sorolens",
		"embeds": []any{
			map[string]any{
				"title":       truncate(fmt.Sprintf("%s alert", a.Severity), 256),
				"description": truncate(a.Message, 4000),
				"url":         explorerURL(a),
				"color":       discordColor(a.Severity),
				"timestamp":   a.Timestamp.UTC().Format(time.RFC3339),
				"fields": []any{
					map[string]any{"name": "Contract", "value": "`" + a.ContractID + "`"},
					map[string]any{"name": "Severity", "value": a.Severity, "inline": true},
					map[string]any{"name": "Ledger", "value": fmt.Sprintf("%d", a.Ledger), "inline": true},
				},
				"footer": map[string]any{"text": "Sorolens watchdog"},
			},
		},
	}
}

// pagerDutySeverity maps watchdog severities onto Events API v2 severities.
func pagerDutySeverity(severity string) string {
	switch severity {
	case "Critical":
		return "critical"
	case "Warning":
		return "warning"
	default:
		return "info"
	}
}

// pagerDutyPayload is a PagerDuty Events API v2 trigger event. The dedup key
// is derived from the alert so a re-delivered alert does not open a second
// incident.
func pagerDutyPayload(a Alert, routingKey string) map[string]any {
	return map[string]any{
		"routing_key":  routingKey,
		"event_action": "trigger",
		"dedup_key":    fmt.Sprintf("sorolens:%s:%s", a.ContractID, a.TxHash),
		"payload": map[string]any{
			"summary":   truncate(fmt.Sprintf("[%s] %s: %s", a.Severity, a.ContractID, a.Message), 1024),
			"source":    a.ContractID,
			"severity":  pagerDutySeverity(a.Severity),
			"timestamp": a.Timestamp.UTC().Format(time.RFC3339),
			"component": "sorolens-watchdog",
			"custom_details": map[string]any{
				"contract_id": a.ContractID,
				"message":     a.Message,
				"ledger":      a.Ledger,
				"tx_hash":     a.TxHash,
			},
		},
		"links": []any{
			map[string]any{"href": explorerURL(a), "text": "View transaction"},
		},
	}
}

// ---- SSRF guard -------------------------------------------------------------

// envAllowedHosts names the optional comma-separated host allowlist. When set,
// notifications may only be delivered to the listed hosts ("example.com"
// matches exactly, "*.example.com" matches any subdomain). Blocked address
// ranges stay blocked even for allowlisted hosts.
const envAllowedHosts = "WEBHOOK_ALLOWED_HOSTS"

// maxRedirects bounds how many redirects a delivery follows.
const maxRedirects = 3

// errBlockedDestination is wrapped by every error caused by a forbidden
// delivery destination.
var errBlockedDestination = errors.New("destination is not allowed")

// lookupFunc resolves a hostname to IP addresses. nil selects the system
// resolver; tests inject their own to simulate DNS rebinding.
type lookupFunc func(ctx context.Context, host string) ([]netip.Addr, error)

func defaultLookup(ctx context.Context, host string) ([]netip.Addr, error) {
	return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
}

// Ranges that are not covered by the netip.Addr predicates in isBlockedAddr.
var blockedPrefixes = mustPrefixes(
	"0.0.0.0/8",       // "this network"
	"100.64.0.0/10",   // carrier-grade NAT (includes Alibaba metadata 100.100.100.200)
	"192.0.0.0/24",    // IETF protocol assignments
	"192.0.2.0/24",    // TEST-NET-1
	"198.18.0.0/15",   // benchmarking
	"198.51.100.0/24", // TEST-NET-2
	"203.0.113.0/24",  // TEST-NET-3
	"240.0.0.0/4",     // reserved, includes broadcast
	"100::/64",        // IPv6 discard-only
	"2001::/32",       // Teredo
	"2001:db8::/32",   // IPv6 documentation
)

var (
	nat64Prefix = netip.MustParsePrefix("64:ff9b::/96")
	sixToFour   = netip.MustParsePrefix("2002::/16")
)

func mustPrefixes(ss ...string) []netip.Prefix {
	out := make([]netip.Prefix, len(ss))
	for i, s := range ss {
		out[i] = netip.MustParsePrefix(s)
	}
	return out
}

// isBlockedAddr reports whether a server-side request must never be sent to a.
// IPv4-mapped IPv6 addresses and IPv4 addresses embedded in NAT64 and 6to4
// addresses are judged by the IPv4 address they carry.
func isBlockedAddr(a netip.Addr) bool {
	if !a.IsValid() {
		return true
	}
	a = a.Unmap().WithZone("")
	if a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast() ||
		a.IsLinkLocalMulticast() || a.IsInterfaceLocalMulticast() ||
		a.IsMulticast() || a.IsUnspecified() {
		return true
	}
	for _, p := range blockedPrefixes {
		if p.Contains(a) {
			return true
		}
	}
	if a.Is6() {
		b := a.As16()
		switch {
		case nat64Prefix.Contains(a):
			return isBlockedAddr(netip.AddrFrom4([4]byte(b[12:16])))
		case sixToFour.Contains(a):
			return isBlockedAddr(netip.AddrFrom4([4]byte(b[2:6])))
		}
	}
	return false
}

// hostAllowlist is a set of permitted hosts; the zero value permits every host.
type hostAllowlist []string

func parseAllowlist(s string) hostAllowlist {
	var out hostAllowlist
	for _, h := range strings.Split(s, ",") {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
			out = append(out, h)
		}
	}
	return out
}

func allowedHostsFromEnv() hostAllowlist {
	return parseAllowlist(os.Getenv(envAllowedHosts))
}

func (l hostAllowlist) allows(host string) bool {
	if len(l) == 0 {
		return true
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	for _, entry := range l {
		if suffix, ok := strings.CutPrefix(entry, "*."); ok {
			if strings.HasSuffix(host, "."+suffix) {
				return true
			}
		} else if host == entry {
			return true
		}
	}
	return false
}

// resolveHost returns the addresses host points at. An IP literal is returned
// as-is without a DNS query.
func resolveHost(ctx context.Context, host string, lookup lookupFunc) ([]netip.Addr, error) {
	if ip, err := netip.ParseAddr(host); err == nil {
		return []netip.Addr{ip}, nil
	}
	if lookup == nil {
		lookup = defaultLookup
	}
	addrs, err := lookup(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve %q: %w", host, err)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("resolve %q: no addresses", host)
	}
	return addrs, nil
}

// newGuardedClient returns an http.Client for delivering notifications. Every
// connection, including each redirect hop, resolves the host itself, refuses
// forbidden addresses and dials the exact address it checked, so a DNS answer
// that changes after a subscription was created (rebinding) cannot reach an
// internal service. A host with even one forbidden record is refused.
// Environment proxies are ignored because a proxy would connect on the
// caller's behalf and bypass the check. The resolver and the address policy are
// parameters so tests can inject their own.
func newGuardedClient(timeout time.Duration, allow hostAllowlist, lookup lookupFunc, blocked func(netip.Addr) bool) *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			if !allow.allows(host) {
				return nil, fmt.Errorf("%w: host %q is not on the allowlist", errBlockedDestination, host)
			}
			addrs, err := resolveHost(ctx, host, lookup)
			if err != nil {
				return nil, err
			}
			for _, a := range addrs {
				if blocked(a) {
					return nil, fmt.Errorf("%w: %q resolves to a private, loopback, link-local or metadata address", errBlockedDestination, host)
				}
			}
			var lastErr error
			for _, a := range addrs {
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(a.String(), port))
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			return nil, lastErr
		},
		TLSHandshakeTimeout: 10 * time.Second,
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("%w: redirect to scheme %q", errBlockedDestination, req.URL.Scheme)
			}
			if !allow.allows(req.URL.Hostname()) {
				return fmt.Errorf("%w: redirect to host %q is not on the allowlist", errBlockedDestination, req.URL.Hostname())
			}
			// Private targets are refused by the dialer when the hop connects.
			return nil
		},
	}
}

// httpClient is the notifier's HTTP client; tests may replace it. It refuses
// loopback, private, link-local and metadata destinations at dial time (so a
// DNS answer that changes after the subscription was created cannot reach
// internal services), limits redirects, and honours WEBHOOK_ALLOWED_HOSTS.
var httpClient = newGuardedClient(10*time.Second, allowedHostsFromEnv(), nil, isBlockedAddr)

// SetHTTPClient replaces the delivery client and returns a func that restores
// the previous one. It exists for tests in other packages that deliver to
// loopback servers, which the default client refuses.
func SetHTTPClient(c *http.Client) (restore func()) {
	prev := httpClient
	httpClient = c
	return func() { httpClient = prev }
}

// DispatchAlerts queries matching subscriptions for a critical alert and
// posts a channel-formatted notification to each one. It retries once on
// 5xx responses, logs and skips on 4xx. Each request has a 10-second
// timeout.
func DispatchAlerts(ctx context.Context, alert Alert, subStore AlertSubscriptionStore, logger *slog.Logger) {
	if alert.Severity != "Critical" {
		return
	}

	subs, err := subStore.ListByContract(ctx, alert.ContractID)
	if err != nil {
		logger.Error("dispatch alerts: list subscriptions", "err", err, "contract_id", alert.ContractID)
		return
	}

	for _, sub := range subs {
		if sub.SeverityFilter != "Critical" && sub.SeverityFilter != alert.Severity {
			continue
		}
		go deliver(ctx, sub, alert, logger)
	}
}

// deliver sends one notification, retrying once on a 5xx. Each attempt
// builds a fresh request so the body is re-sent in full.
func deliver(ctx context.Context, sub AlertSubscription, alert Alert, logger *slog.Logger) {
	url, body, err := FormatNotification(sub, alert)
	if err != nil {
		logger.Error("dispatch alerts: format", "err", err, "subscription_id", sub.ID, "channel", sub.ChannelType)
		return
	}
	for attempt := 1; attempt <= 2; attempt++ {
		status, err := post(ctx, url, body)
		switch {
		case err != nil:
			logger.Error("dispatch alerts: request failed", "err", err, "subscription_id", sub.ID, "attempt", attempt)
			return
		case status >= 500:
			logger.Error("dispatch alerts: server error", "status", status, "subscription_id", sub.ID, "attempt", attempt)
			continue
		case status >= 400:
			logger.Error("dispatch alerts: client error", "status", status, "subscription_id", sub.ID)
		}
		return
	}
}

func post(ctx context.Context, url string, body []byte) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}
