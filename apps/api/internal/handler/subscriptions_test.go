package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

const subsPath = "/api/v1/watchdog/subscriptions"

type subscriptionBody struct {
	ID             string `json:"id"`
	ChannelType    string `json:"channel_type"`
	WebhookURL     string `json:"webhook_url"`
	HasRoutingKey  bool   `json:"has_routing_key"`
	SeverityFilter string `json:"severity_filter"`
}

// seedMonitored registers C1 with the watchdog so subscriptions can target it.
func seedMonitored(t *testing.T, ms *store.MockStore) {
	t.Helper()
	if err := ms.UpsertMonitoredContract(context.Background(), store.MonitoredContract{
		ContractID: "C1", Network: "testnet", Name: "svc", Status: "Healthy",
	}); err != nil {
		t.Fatal(err)
	}
}

func TestCreateSubscriptionPerChannel(t *testing.T) {
	ms := seedRBACUsers(t)
	seedMonitored(t, ms)
	srv := newTestHandler(ms, true, true)

	cases := []struct {
		name, body string
		want       subscriptionBody
	}{
		{
			"webhook default",
			`{"contract_id":"C1","webhook_url":"https://example.com/hook"}`,
			subscriptionBody{ChannelType: "webhook", WebhookURL: "https://example.com/hook", SeverityFilter: "Critical"},
		},
		{
			"slack masks token",
			`{"contract_id":"C1","channel_type":"slack","webhook_url":"https://hooks.slack.com/services/T0/B0/secret"}`,
			subscriptionBody{ChannelType: "slack", WebhookURL: "https://hooks.slack.com/services/***", SeverityFilter: "Critical"},
		},
		{
			"discord masks token",
			`{"contract_id":"C1","channel_type":"discord","webhook_url":"https://discord.com/api/webhooks/1/secret","severity_filter":"Warning"}`,
			subscriptionBody{ChannelType: "discord", WebhookURL: "https://discord.com/api/***", SeverityFilter: "Warning"},
		},
		{
			"pagerduty hides routing key",
			`{"contract_id":"C1","channel_type":"PagerDuty","routing_key":"R0UT1NG"}`,
			subscriptionBody{ChannelType: "pagerduty", HasRoutingKey: true, SeverityFilter: "Critical"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doRequestAsUser(srv, http.MethodPost, subsPath, "", contributorUser, tc.body)
			if w.Code != http.StatusCreated {
				t.Fatalf("want 201, got %d (%s)", w.Code, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "secret") || strings.Contains(w.Body.String(), "R0UT1NG") {
				t.Fatalf("response leaks a secret: %s", w.Body.String())
			}
			var got subscriptionBody
			if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
				t.Fatal(err)
			}
			got.ID = ""
			if got != tc.want {
				t.Fatalf("want %+v, got %+v", tc.want, got)
			}
		})
	}

	// The store keeps the real secrets for the notifier.
	subs, _ := ms.ListAll(context.Background())
	var sawKey bool
	for _, s := range subs {
		if s.ChannelType == "pagerduty" && s.RoutingKey == "R0UT1NG" {
			sawKey = true
		}
		if s.ChannelType == "slack" && s.WebhookURL != "https://hooks.slack.com/services/T0/B0/secret" {
			t.Errorf("stored slack url altered: %s", s.WebhookURL)
		}
	}
	if !sawKey {
		t.Error("routing key not stored")
	}
}

func TestCreateSubscriptionValidation(t *testing.T) {
	ms := seedRBACUsers(t)
	seedMonitored(t, ms)
	srv := newTestHandler(ms, true, true)
	cases := map[string]string{
		"unmonitored contract":   `{"contract_id":"CNOPE","webhook_url":"https://x.io"}`,
		"unknown channel":        `{"contract_id":"C1","channel_type":"sms","webhook_url":"https://x.io"}`,
		"missing contract":       `{"webhook_url":"https://x.io"}`,
		"bad severity":           `{"contract_id":"C1","webhook_url":"https://x.io","severity_filter":"Panic"}`,
		"webhook without url":    `{"contract_id":"C1"}`,
		"webhook bad scheme":     `{"contract_id":"C1","webhook_url":"ftp://x.io"}`,
		"slack over http":        `{"contract_id":"C1","channel_type":"slack","webhook_url":"http://hooks.slack.com/x"}`,
		"discord without url":    `{"contract_id":"C1","channel_type":"discord"}`,
		"pagerduty without key":  `{"contract_id":"C1","channel_type":"pagerduty"}`,
		"routing key on webhook": `{"contract_id":"C1","webhook_url":"https://x.io","routing_key":"k"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if w := doRequestAsUser(srv, http.MethodPost, subsPath, "", contributorUser, body); w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("want 422, got %d (%s)", w.Code, w.Body.String())
			}
		})
	}
}

func TestSubscriptionRoutesRolesAndDelete(t *testing.T) {
	ms := seedRBACUsers(t)
	seedMonitored(t, ms)
	if err := ms.Create(context.Background(), store.AlertSubscription{
		ID: "sub_1", ContractID: "C1", WebhookURL: "https://x.io", SeverityFilter: "Critical",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	srv := newTestHandler(ms, true, true)

	for _, tc := range []struct {
		method, path, user string
		want               int
	}{
		{http.MethodGet, subsPath, "", http.StatusUnauthorized},
		{http.MethodGet, subsPath, viewerUser, http.StatusForbidden},
		{http.MethodPost, subsPath, viewerUser, http.StatusForbidden},
		{http.MethodDelete, subsPath + "/sub_1", viewerUser, http.StatusForbidden},
		{http.MethodGet, subsPath, contributorUser, http.StatusOK},
		{http.MethodDelete, subsPath + "/sub_1", contributorUser, http.StatusNoContent},
		{http.MethodDelete, subsPath + "/sub_1", contributorUser, http.StatusNotFound},
	} {
		body := ""
		if tc.method == http.MethodPost {
			body = `{"contract_id":"C1","webhook_url":"https://x.io"}`
		}
		if w := doRequestAsUser(srv, tc.method, tc.path, "", tc.user, body); w.Code != tc.want {
			t.Errorf("%s %s as %q: want %d, got %d", tc.method, tc.path, tc.user, tc.want, w.Code)
		}
	}
}
