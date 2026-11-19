package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stellar/go-stellar-sdk/xdr"

	"github.com/sorolens/sorolens/services/indexer/internal/history"
	"github.com/sorolens/sorolens/services/indexer/internal/poller"
)

const importContract = "CCLV77FYLRJMBGTILTKVIM76SI6JY56H7KTT47HE26UF4F265UYRDSR4"

type importFakeStore struct {
	target history.Target
	stored []poller.Event
}

func (s *importFakeStore) ImportTarget(context.Context, string) (history.Target, error) {
	return s.target, nil
}

func (s *importFakeStore) CommitHistoryBatch(_ context.Context, events []poller.Event, _ []poller.Invocation) ([]poller.FailedEvent, error) {
	s.stored = append(s.stored, events...)
	return nil, nil
}

type importFakeRPC struct {
	poller.RPCClient
	latest uint32
}

func (r importFakeRPC) GetLatestLedger(context.Context) (*poller.LatestLedger, error) {
	return &poller.LatestLedger{Sequence: r.latest}, nil
}

type importFakeReader struct {
	rows []history.HubbleRow
	got  []map[string]any
}

func (r *importFakeReader) Rows(_ context.Context, _ string, params map[string]any) ([]history.HubbleRow, error) {
	r.got = append(r.got, params)
	out := r.rows
	r.rows = nil
	return out, nil
}

func eventXDR(t *testing.T) string {
	t.Helper()
	sym := xdr.ScSymbol("mint")
	b64, err := xdr.MarshalBase64(xdr.ContractEvent{
		Type: xdr.ContractEventTypeContract,
		Body: xdr.ContractEventBody{V: 0, V0: &xdr.ContractEventV0{
			Topics: []xdr.ScVal{{Type: xdr.ScValTypeScvSymbol, Sym: &sym}},
			Data:   xdr.ScVal{Type: xdr.ScValTypeScvVoid},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return b64
}

func TestRunImportStoresHubbleEvents(t *testing.T) {
	store := &importFakeStore{target: history.Target{Network: "mainnet", RPCFloorLedger: 60_000_000}}
	reader := &importFakeReader{rows: []history.HubbleRow{{
		LedgerSequence: 55_000_010, TransactionID: 236223201280 << 3, ContractID: importContract,
		ContractEventXDR: eventXDR(t), TransactionHash: "aa",
		ClosedAt: time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC),
	}}}
	flags := &importFlags{contract: importContract, from: 55_000_000, to: 55_000_100, batch: 1000, billingProject: "mine"}
	var out bytes.Buffer

	err := runImport(context.Background(), flags, store,
		map[string]poller.RPCClient{"mainnet": importFakeRPC{latest: 61_000_000}},
		func(_ context.Context, cfg history.HubbleConfig) (history.RowReader, error) {
			if cfg.BillingProject != "mine" {
				t.Fatalf("config = %+v", cfg)
			}
			return reader, nil
		}, &out, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	if len(store.stored) != 1 || store.stored[0].Network != "mainnet" || store.stored[0].Type != "contract" {
		t.Fatalf("stored = %+v", store.stored)
	}
	var res history.Result
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("summary %q: %v", out.String(), err)
	}
	if res.Events != 1 || res.Batches != 1 || res.Network != "mainnet" {
		t.Fatalf("summary = %+v", res)
	}
}

func TestRunImportFailsClearly(t *testing.T) {
	ctx := context.Background()
	store := &importFakeStore{target: history.Target{Network: "mainnet"}}
	rpcs := map[string]poller.RPCClient{"": importFakeRPC{latest: 10}}
	reader := func(context.Context, history.HubbleConfig) (history.RowReader, error) {
		return &importFakeReader{}, nil
	}
	discard := slog.New(slog.DiscardHandler)
	ok := importFlags{contract: importContract, from: 1, to: 2, billingProject: "mine"}

	cases := map[string]struct {
		flags   importFlags
		rpcs    map[string]poller.RPCClient
		network string
		reader  func(context.Context, history.HubbleConfig) (history.RowReader, error)
		want    string
	}{
		"no range":           {flags: importFlags{contract: importContract}, want: "-import-from"},
		"no billing project": {flags: importFlags{contract: importContract, from: 1, to: 2}, want: "billing project"},
		"testnet public":     {flags: ok, network: "testnet", want: "only holds mainnet"},
		"no rpc":             {flags: ok, rpcs: map[string]poller.RPCClient{}, want: "no Soroban RPC"},
		"no credentials": {flags: ok, reader: func(context.Context, history.HubbleConfig) (history.RowReader, error) {
			return nil, errors.New("no Google Cloud credentials")
		}, want: "credentials"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			st := *store
			if tc.network != "" {
				st.target.Network = tc.network
			}
			r := rpcs
			if tc.rpcs != nil {
				r = tc.rpcs
			}
			rd := reader
			if tc.reader != nil {
				rd = tc.reader
			}
			f := tc.flags
			err := runImport(ctx, &f, &st, r, rd, &bytes.Buffer{}, discard)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}
