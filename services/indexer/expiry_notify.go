package main

import (
	"context"
	"log/slog"

	"github.com/sorolens/sorolens/services/indexer/internal/expiryalert"
	"github.com/sorolens/sorolens/services/indexer/internal/pgstore"
	"github.com/sorolens/sorolens/services/indexer/internal/poller"
	"github.com/sorolens/sorolens/services/indexer/internal/watchdog"
)

// main.go type-asserts the live store against these surfaces before enabling the
// job. Asserting them at compile time keeps that runtime check from silently
// evaluating to false — which would disable storage expiry alerting (issue #510)
// with nothing but a missing log line to show for it.
var (
	_ expiryalert.Store  = (*pgstore.Store)(nil)
	_ subscriptionLister = (*pgstore.Store)(nil)
)

// subscriptionLister is the read surface expiry alerting needs to reach the
// notification channels a user configured for a contract (issue #510). It is
// satisfied by *pgstore.Store.ListAlertSubscriptions, and kept narrow so the
// dispatch path can be tested with a mock.
type subscriptionLister interface {
	ListAlertSubscriptions(ctx context.Context, contractID string) ([]pgstore.AlertSubscription, error)
}

// expiryNotifier is the production expiryalert.Notifier: it turns the runner's
// poller.Alert into the watchdog.Alert the shared notifier understands and hands
// it to watchdog.DispatchAlerts, which posts one channel-formatted notification
// per matching alert subscription (webhook, Slack, Discord, PagerDuty).
//
// Nothing is delivered unless the user registered a subscription for the
// contract, and DispatchAlerts only dispatches Critical severity — which is
// exactly what the expiry runner raises. This is the same notifier the watchdog
// feed uses, so expiry summaries arrive on the channels #510 promises without a
// second delivery implementation.
type expiryNotifier struct {
	subs subscriptionLister
	log  *slog.Logger
}

// newExpiryNotifier wires the notifier to a subscription store. A nil lister
// (a deployment whose store has no alert_subscriptions surface) yields a
// notifier that only logs, so expiry alerting can never block on delivery.
func newExpiryNotifier(subs subscriptionLister, log *slog.Logger) *expiryNotifier {
	if log == nil {
		log = slog.Default()
	}
	return &expiryNotifier{subs: subs, log: log}
}

// Notify implements expiryalert.Notifier.
func (n *expiryNotifier) Notify(ctx context.Context, a poller.Alert) {
	if n == nil || n.subs == nil {
		return
	}
	watchdog.DispatchAlerts(ctx, watchdog.Alert{
		ContractID: a.ContractID,
		Severity:   a.Severity,
		Message:    a.Message,
		Ledger:     a.Ledger,
		TxHash:     a.TxHash,
		Timestamp:  a.Timestamp,
	}, &contractSubscriptions{lister: n.subs}, n.log)
}

// contractSubscriptions adapts the indexer store's ListAlertSubscriptions to
// watchdog.AlertSubscriptionStore. The two AlertSubscription shapes are local
// mirrors of the same row (the indexer cannot import apps/api across module
// boundaries), so only the type name differs and every field carries over.
type contractSubscriptions struct {
	lister subscriptionLister
}

// ListByContract implements watchdog.AlertSubscriptionStore.
func (c *contractSubscriptions) ListByContract(ctx context.Context, contractID string) ([]watchdog.AlertSubscription, error) {
	rows, err := c.lister.ListAlertSubscriptions(ctx, contractID)
	if err != nil {
		return nil, err
	}
	out := make([]watchdog.AlertSubscription, 0, len(rows))
	for _, r := range rows {
		out = append(out, watchdog.AlertSubscription{
			ID:             r.ID,
			ContractID:     r.ContractID,
			WebhookURL:     r.WebhookURL,
			SeverityFilter: r.SeverityFilter,
			ChannelType:    r.ChannelType,
			RoutingKey:     r.RoutingKey,
		})
	}
	return out, nil
}
