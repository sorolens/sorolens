// Package reportdigest builds and sends the scheduled email digests
// subscribed to via POST /api/v1/reports/subscriptions (issue #330).
//
// RunDigest is the entry point a cron invokes (daily at 08:00 UTC, and weekly
// on the subscriber's chosen day). It is deliberately independent of the HTTP
// Handler so it can run from a separate scheduler process, and it takes a
// Mailer interface so tests exercise the full path with a fake SMTP client.
package reportdigest

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// Mailer sends one rendered email. The SMTP implementation lives alongside
// this file; tests supply a fake.
type Mailer interface {
	Send(ctx context.Context, to, subject, htmlBody, textBody string) error
}

// DigestStore is the read surface RunDigest needs. Both *store.postgresStore
// and *store.MockStore satisfy it.
type DigestStore interface {
	ListDueReportSubscriptions(ctx context.Context, frequency string, weekday int) ([]store.ReportSubscription, error)
	ListContracts(ctx context.Context, cursor string, limit int, f store.ContractFilters) ([]store.Contract, string, error)
	GetContractStats(ctx context.Context, contractID, window string) (store.ContractStats, error)
}

// Config parameterises a digest run.
type Config struct {
	// Frequency is "daily" or "weekly".
	Frequency string
	// BaseURL is the public API origin, used to build the unsubscribe link,
	// e.g. "https://sorolens.dev".
	BaseURL string
	// SigningKey signs the one-click unsubscribe token. When empty the token
	// is the bare subscription id (matches the handler's unsigned mode).
	SigningKey []byte
	// MaxContracts caps how many tracked contracts appear in a digest.
	MaxContracts int
}

type contractLine struct {
	Contract store.Contract
	Stats    store.ContractStats
}

// RunDigest sends the digest to every subscription due now for cfg.Frequency.
// It returns the number of emails sent. Subscriptions receive nothing when no
// contracts are tracked (there is nothing to report).
func RunDigest(ctx context.Context, s DigestStore, m Mailer, cfg Config, now time.Time) (int, error) {
	if !store.ValidReportFrequencies[cfg.Frequency] {
		return 0, fmt.Errorf("reportdigest: invalid frequency %q", cfg.Frequency)
	}

	subs, err := s.ListDueReportSubscriptions(ctx, cfg.Frequency, int(now.UTC().Weekday()))
	if err != nil {
		return 0, fmt.Errorf("reportdigest: list due: %w", err)
	}
	if len(subs) == 0 {
		return 0, nil
	}

	lines, err := gatherContractLines(ctx, s, cfg)
	if err != nil {
		return 0, err
	}
	// Skip entirely when there are no tracked contracts.
	if len(lines) == 0 {
		return 0, nil
	}

	subject := digestSubject(cfg.Frequency)
	textBody, htmlBodyTmpl := renderBodies(cfg.Frequency, lines, now)

	sent := 0
	for _, sub := range subs {
		unsub := unsubscribeURL(cfg.BaseURL, cfg.SigningKey, sub.ID)
		htmlBody := strings.ReplaceAll(htmlBodyTmpl, unsubPlaceholder, html.EscapeString(unsub))
		text := textBody + "\n\nUnsubscribe: " + unsub + "\n"
		if err := m.Send(ctx, sub.Email, subject, htmlBody, text); err != nil {
			return sent, fmt.Errorf("reportdigest: send to %s: %w", sub.Email, err)
		}
		sent++
	}
	return sent, nil
}

func gatherContractLines(ctx context.Context, s DigestStore, cfg Config) ([]contractLine, error) {
	limit := cfg.MaxContracts
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	contracts, _, err := s.ListContracts(ctx, "", limit, store.ContractFilters{})
	if err != nil {
		return nil, fmt.Errorf("reportdigest: list contracts: %w", err)
	}
	window := "24h"
	if cfg.Frequency == store.ReportWeekly {
		window = "7d"
	}
	lines := make([]contractLine, 0, len(contracts))
	for _, c := range contracts {
		stats, err := s.GetContractStats(ctx, c.ID, window)
		if err != nil {
			return nil, fmt.Errorf("reportdigest: stats for %s: %w", c.ID, err)
		}
		lines = append(lines, contractLine{Contract: c, Stats: stats})
	}
	return lines, nil
}

const unsubPlaceholder = "{{UNSUBSCRIBE_URL}}"

func digestSubject(freq string) string {
	if freq == store.ReportWeekly {
		return "Your weekly Sorolens digest"
	}
	return "Your daily Sorolens digest"
}

// renderBodies returns the plain-text body and an HTML body containing the
// unsubPlaceholder token, which the caller replaces per-recipient.
func renderBodies(freq string, lines []contractLine, now time.Time) (string, string) {
	period := "24 hours"
	if freq == store.ReportWeekly {
		period = "7 days"
	}

	var text strings.Builder
	var htmlB strings.Builder
	fmt.Fprintf(&text, "Sorolens digest — activity over the last %s (generated %s)\n\n",
		period, now.UTC().Format(time.RFC1123))
	htmlB.WriteString("<h1>Sorolens digest</h1>")
	fmt.Fprintf(&htmlB, "<p>Activity over the last %s.</p><ul>", html.EscapeString(period))

	for _, l := range lines {
		name := l.Contract.Label
		if name == "" {
			name = l.Contract.ID
		}
		fmt.Fprintf(&text, "%s (%s)\n  events: %d  invocations: %d  storage entries: %d\n",
			name, l.Contract.ID, l.Stats.WindowEventCount, l.Stats.WindowInvocationCount, l.Stats.StorageCount)
		fmt.Fprintf(&htmlB,
			"<li><strong>%s</strong> <code>%s</code><br>events: %d · invocations: %d · storage entries: %d</li>",
			html.EscapeString(name), html.EscapeString(l.Contract.ID),
			l.Stats.WindowEventCount, l.Stats.WindowInvocationCount, l.Stats.StorageCount)
	}

	htmlB.WriteString("</ul>")
	fmt.Fprintf(&htmlB, `<p style="font-size:12px;color:#888"><a href="%s">Unsubscribe</a> from these emails.</p>`, unsubPlaceholder)
	return text.String(), htmlB.String()
}

func unsubscribeURL(baseURL string, key []byte, id string) string {
	token := id
	if len(key) > 0 {
		mac := hmac.New(sha256.New, key)
		mac.Write([]byte(id))
		token = id + "." + hex.EncodeToString(mac.Sum(nil))
	}
	return strings.TrimRight(baseURL, "/") + "/api/v1/reports/unsubscribe?token=" + token
}
