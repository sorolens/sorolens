package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

func TestSubscriptionHandlers(t *testing.T) {
	ms := store.NewMockStore()
	srv := newTestHandler(ms, true, true)

	// 1. Create subscription
	createBody, _ := json.Marshal(map[string]string{
		"contract_id":     "CONTRACT_SUB_1",
		"webhook_url":     "https://example.com/webhook",
		"severity_filter": "Critical",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create subscription: want 201, got %d (%s)", w.Code, w.Body.String())
	}

	var created map[string]any
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	subID, ok := created["id"].(string)
	if !ok || subID == "" {
		t.Fatalf("expected string id in response, got %v", created["id"])
	}

	// 2. List subscriptions
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions", nil)
	wList := httptest.NewRecorder()
	srv.ServeHTTP(wList, reqList)

	if wList.Code != http.StatusOK {
		t.Fatalf("list subscriptions: want 200, got %d", wList.Code)
	}
	var listResp struct {
		Subscriptions []map[string]any `json:"subscriptions"`
	}
	if err := json.NewDecoder(wList.Body).Decode(&listResp); err != nil {
		t.Fatal(err)
	}
	if len(listResp.Subscriptions) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(listResp.Subscriptions))
	}

	// Add a delivery history item to MockStore
	now := time.Now().UTC()
	del := store.WebhookDelivery{
		ID:             "del_test_100",
		SubscriptionID: subID,
		Payload:        `{"alert":"critical"}`,
		Status:         "success",
		Attempt:        1,
		MaxAttempts:    5,
		NextAttemptAt:  now,
		ResponseCode:   200,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	_ = ms.CreateDelivery(context.Background(), del)
	_ = ms.UpdateDeliveryStatus(context.Background(), subID, "success", now)

	// 3. Get subscription delivery history
	reqDeliv := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions/"+subID+"/deliveries?page=1&limit=10", nil)
	wDeliv := httptest.NewRecorder()
	srv.ServeHTTP(wDeliv, reqDeliv)

	if wDeliv.Code != http.StatusOK {
		t.Fatalf("get deliveries: want 200, got %d (%s)", wDeliv.Code, wDeliv.Body.String())
	}
	var delivResp struct {
		Deliveries []store.WebhookDelivery `json:"deliveries"`
		Page       int                     `json:"page"`
		Limit      int                     `json:"limit"`
		Total      int                     `json:"total"`
	}
	if err := json.NewDecoder(wDeliv.Body).Decode(&delivResp); err != nil {
		t.Fatal(err)
	}
	if delivResp.Total != 1 || len(delivResp.Deliveries) != 1 {
		t.Fatalf("expected total=1 and 1 delivery, got total=%d len=%d", delivResp.Total, len(delivResp.Deliveries))
	}
	if delivResp.Deliveries[0].Status != "success" {
		t.Errorf("expected delivery status 'success', got %q", delivResp.Deliveries[0].Status)
	}

	// 4. Delete subscription
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/v1/subscriptions/"+subID, nil)
	wDel := httptest.NewRecorder()
	srv.ServeHTTP(wDel, reqDel)

	if wDel.Code != http.StatusNoContent {
		t.Fatalf("delete subscription: want 240/204, got %d", wDel.Code)
	}
}
