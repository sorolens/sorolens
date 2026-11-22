package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

const resourceTrendContractID = "C_RESOURCE_TREND"

func TestContractResourceTrendReturnsDailyAveragesInOneRequest(t *testing.T) {
	now := time.Now().UTC()
	ms := store.NewMockStore()
	if err := ms.BatchInsertInvocations(nil, []store.Invocation{
		{
			TxHash:             "tx-trend-1",
			ContractID:         resourceTrendContractID,
			LedgerClosedAt:     now.Add(-2 * time.Hour),
			CPUInsn:            1000,
			MemByte:            200,
			ResourceFeeCharged: 100,
		},
		{
			TxHash:             "tx-trend-2",
			ContractID:         resourceTrendContractID,
			LedgerClosedAt:     now.Add(-1 * time.Hour),
			CPUInsn:            3000,
			MemByte:            400,
			ResourceFeeCharged: 300,
		},
		{
			TxHash:             "tx-trend-3",
			ContractID:         resourceTrendContractID,
			LedgerClosedAt:     now.AddDate(0, 0, -1),
			CPUInsn:            500,
			MemByte:            100,
			ResourceFeeCharged: 50,
		},
		{
			TxHash:             "tx-other-contract",
			ContractID:         "C_OTHER",
			LedgerClosedAt:     now.Add(-1 * time.Hour),
			CPUInsn:            9999,
			MemByte:            9999,
			ResourceFeeCharged: 9999,
		},
	}); err != nil {
		t.Fatalf("seed invocations: %v", err)
	}

	srv := newTestHandler(ms, true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+resourceTrendContractID+"/resource-trend?days=7", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}

	var body struct {
		Days   int `json:"days"`
		Points []struct {
			Date       string  `json:"date"`
			AvgCPUInsn float64 `json:"avg_cpu_insn"`
			AvgMemByte float64 `json:"avg_mem_byte"`
			AvgFee     float64 `json:"avg_fee"`
			Count      int64   `json:"count"`
		} `json:"points"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	if body.Days != 7 {
		t.Errorf("days = %d, want 7", body.Days)
	}
	if len(body.Points) != 7 {
		t.Fatalf("points = %d, want one per day (7)", len(body.Points))
	}

	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	var todayPoint, yesterdayPoint *struct {
		Date       string  `json:"date"`
		AvgCPUInsn float64 `json:"avg_cpu_insn"`
		AvgMemByte float64 `json:"avg_mem_byte"`
		AvgFee     float64 `json:"avg_fee"`
		Count      int64   `json:"count"`
	}
	for i := range body.Points {
		switch body.Points[i].Date {
		case today:
			todayPoint = &body.Points[i]
		case yesterday:
			yesterdayPoint = &body.Points[i]
		}
	}
	if todayPoint == nil || yesterdayPoint == nil {
		t.Fatalf("expected buckets for %s and %s, got %+v", today, yesterday, body.Points)
	}
	if todayPoint.Count != 2 || todayPoint.AvgCPUInsn != 2000 || todayPoint.AvgMemByte != 300 || todayPoint.AvgFee != 200 {
		t.Errorf("today bucket = %+v, want count 2 and averages 2000/300/200", *todayPoint)
	}
	if yesterdayPoint.Count != 1 || yesterdayPoint.AvgCPUInsn != 500 {
		t.Errorf("yesterday bucket = %+v, want count 1 and cpu 500", *yesterdayPoint)
	}
}

func TestContractResourceTrendDefaultsToThirtyDays(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+resourceTrendContractID+"/resource-trend", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body struct {
		Days   int        `json:"days"`
		Points []struct{} `json:"points"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Days != 30 || len(body.Points) != 30 {
		t.Errorf("days=%d points=%d, want 30 and 30", body.Days, len(body.Points))
	}
}
