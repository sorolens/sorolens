package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

func TestGetContract_TokenMetadata(t *testing.T) {
	ms := store.NewMockStore()
	sacID := "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
	nonSacID := "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAD2KM"
	accountID := "GA5ZSEJYB37JRC5AVCIA5MOP4RHTM335X2KGX3IHOJAPP5RE34K4KZVN"

	// 1. Seed SAC contract with token metadata
	if err := ms.UpsertContract(nil, store.Contract{
		ID:      sacID,
		Network: "testnet",
		Label:   "USDC",
		Status:  "active",
		Token: &store.TokenMetadata{
			AssetCode: "USDC",
			Issuer:    accountID,
			Decimals:  7,
			Symbol:    "USDC",
			Name:      "USDC:" + accountID,
		},
	}); err != nil {
		t.Fatal(err)
	}

	// 2. Seed non-SAC contract without token metadata
	if err := ms.UpsertContract(nil, store.Contract{
		ID:      nonSacID,
		Network: "testnet",
		Label:   "custom_contract",
		Status:  "active",
	}); err != nil {
		t.Fatal(err)
	}

	for _, endpoint := range []string{"/api/v1/contracts/" + sacID, "/api/v2/contracts/" + sacID} {
		t.Run("SAC_"+endpoint, func(t *testing.T) {
			body := getContractJSON(t, ms, endpoint)
			rawTok, ok := body["token"]
			if !ok || rawTok == nil {
				t.Fatalf("expected token metadata in response, got %+v", body)
			}
			tok, ok := rawTok.(map[string]any)
			if !ok {
				t.Fatalf("expected token to be map, got %T", rawTok)
			}
			if tok["asset_code"] != "USDC" {
				t.Errorf("asset_code: got %v, want USDC", tok["asset_code"])
			}
			if tok["issuer"] != accountID {
				t.Errorf("issuer: got %v, want %s", tok["issuer"], accountID)
			}
			if dec, ok := tok["decimals"].(float64); !ok || uint32(dec) != 7 {
				t.Errorf("decimals: got %v, want 7", tok["decimals"])
			}
			if tok["symbol"] != "USDC" {
				t.Errorf("symbol: got %v, want USDC", tok["symbol"])
			}
		})
	}

	for _, endpoint := range []string{"/api/v1/contracts/" + nonSacID, "/api/v2/contracts/" + nonSacID} {
		t.Run("NonSAC_"+endpoint, func(t *testing.T) {
			body := getContractJSON(t, ms, endpoint)
			if tok, present := body["token"]; present && tok != nil {
				t.Errorf("expected token to be omitted or nil for non-SAC contract, got %v", tok)
			}
		})
	}

	t.Run("ListContractsEnrichesSAC", func(t *testing.T) {
		srv := newTestHandler(ms, true, true)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/contracts", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
		var listResp struct {
			Items []struct {
				ID    string                `json:"id"`
				Token *store.TokenMetadata `json:"token"`
			} `json:"items"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &listResp); err != nil {
			t.Fatal(err)
		}
		for _, item := range listResp.Items {
			if item.ID == sacID {
				if item.Token == nil || item.Token.AssetCode != "USDC" {
					t.Errorf("expected SAC contract %s to have USDC token metadata, got %+v", sacID, item.Token)
				}
			} else if item.ID == nonSacID {
				if item.Token != nil {
					t.Errorf("expected non-SAC contract %s to have nil token, got %+v", nonSacID, item.Token)
				}
			}
		}
	})

	t.Run("SearchContractsByTokenSymbol", func(t *testing.T) {
		srv := newTestHandler(ms, true, true)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/search?q=USDC", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
		var searchResp struct {
			Items []struct {
				ID    string                `json:"id"`
				Token *store.TokenMetadata `json:"token"`
			} `json:"items"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &searchResp); err != nil {
			t.Fatal(err)
		}
		if len(searchResp.Items) == 0 {
			t.Fatalf("expected search to return SAC contract")
		}
		if searchResp.Items[0].ID != sacID || searchResp.Items[0].Token == nil || searchResp.Items[0].Token.AssetCode != "USDC" {
			t.Errorf("unexpected search item: %+v", searchResp.Items[0])
		}
	})
}
