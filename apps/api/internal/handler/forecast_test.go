package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

const forecastContractID = "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

type forecastPoint struct {
	Date  string  `json:"date"`
	Value float64 `json:"value"`
	Lower float64 `json:"lower"`
	Upper float64 `json:"upper"`
}

type forecastSeries struct {
	Metric     string          `json:"metric"`
	Horizon    int             `json:"horizon"`
	DailyCount int             `json:"daily_count"`
	Points     []forecastPoint `json:"points"`
}

type forecastResp struct {
	ContractID   string           `json:"contract_id"`
	LookbackDays int              `json:"lookback_days"`
	Series       []forecastSeries `json:"series"`
}

// seedForecastStore accumulates daily invocations with a gentle upward fee
// trend plus a weekly pattern so the model has signal to learn.
func seedForecastStore(days int) *store.MockStore {
	ms := store.NewMockStore()
	base := time.Now().UTC().AddDate(0, 0, -days)
	for i := 0; i < days; i++ {
		day := base.AddDate(0, 0, i)
		// ~1-3 invocations/day so a weekly pattern survives MockStore's
		// zero-fill aggregation.
		n := 1 + i%3
		for j := 0; j < n; j++ {
			ms.BatchInsertInvocations(nil, []store.Invocation{{
				TxHash:             "tx-forecast-" + day.Format("20060102") + "-" + string(rune('a'+j)),
				ContractID:         forecastContractID,
				Network:            "testnet",
				Ledger:             uint32(i*10 + j),
				LedgerClosedAt:     day.Add(time.Duration(j) * time.Hour),
				Status:             "SUCCESS",
				ResourceFeeCharged: 100000 + int64(i*1000),
			}})
		}
		ms.BatchInsertEvents(nil, []store.Event{{
			ID:             "e-" + day.Format("20060102"),
			ContractID:     forecastContractID,
			Network:        "testnet",
			Ledger:         uint32(i * 10),
			LedgerClosedAt: day,
			TxHash:         "tx-forecast-" + day.Format("20060102") + "-0",
			Type:           "lifecount",
			InsertedAt:     day,
		}})
	}
	return ms
}

func TestContractForecastDefaultHorizon(t *testing.T) {
	srv := newTestHandler(seedForecastStore(90), true, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+forecastContractID+"/forecast", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var resp forecastResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Series) != 3 {
		t.Fatalf("want 3 series (fees, invocations, events), got %d", len(resp.Series))
	}
	metrics := map[string]bool{}
	for _, s := range resp.Series {
		metrics[s.Metric] = true
		if s.Horizon != 30 {
			t.Errorf("series %s: horizon want 30, got %d", s.Metric, s.Horizon)
		}
		if s.DailyCount != 30 {
			t.Errorf("series %s: daily_count want 30, got %d", s.Metric, s.DailyCount)
		}
		if len(s.Points) != 30 {
			t.Errorf("series %s: want 30 points, got %d", s.Metric, len(s.Points))
		}
		for _, p := range s.Points {
			if p.Lower > p.Value || p.Upper < p.Value {
				t.Errorf("series %s %s: CI must contain value (lower=%v value=%v upper=%v)", s.Metric, p.Date, p.Lower, p.Value, p.Upper)
			}
		}
	}
	for _, m := range []string{"fees", "invocations", "events"} {
		if !metrics[m] {
			t.Errorf("missing series %q", m)
		}
	}
}

func TestContractForecastHorizonParam(t *testing.T) {
	srv := newTestHandler(seedForecastStore(90), true, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+forecastContractID+"/forecast?horizon=14d", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var resp forecastResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	for _, s := range resp.Series {
		if s.Horizon != 14 || len(s.Points) != 14 {
			t.Fatalf("series %s: want 14-day horizon, got horizon=%d points=%d", s.Metric, s.Horizon, len(s.Points))
		}
	}
}

func TestContractForecastInvalidHorizon(t *testing.T) {
	srv := newTestHandler(seedForecastStore(90), true, true)

	for _, q := range []string{"horizon=xx", "horizon=-5", "horizon=0"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+forecastContractID+"/forecast?"+q, nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: want 400, got %d", q, w.Code)
		}
	}
}

func TestContractForecastEmptyHistory(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+forecastContractID+"/forecast", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("no-data contract: want 200 with empty/zero series, got %d (%s)", w.Code, w.Body.String())
	}
	var resp forecastResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Series) != 3 {
		t.Fatalf("want 3 series even with no data, got %d", len(resp.Series))
	}
	for _, s := range resp.Series {
		if len(s.Points) != 30 {
			t.Errorf("series %s: want 30 zero points, got %d", s.Metric, len(s.Points))
		}
	}
}

func TestContractForecastRequiresReadScopeKey(t *testing.T) {
	ms := seedForecastStore(60)
	ms.AddAPIKey(store.APIKey{
		ID: "key-read-contracts", Name: "reader", KeyPrefix: "sl_readc",
		KeyHash:   store.HashKey(readContractsKey),
		Scopes:    []string{store.ScopeReadContracts},
		CreatedAt: time.Now().UTC(),
	})
	srv := newTestHandler(ms, true, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+forecastContractID+"/forecast", nil)
	req.Header.Set("Authorization", "Bearer "+readContractsKey)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("read:contracts key should access forecast, want 200, got %d (%s)", w.Code, w.Body.String())
	}
}
