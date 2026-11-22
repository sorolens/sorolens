package history

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stellar/go-stellar-sdk/xdr"
)

func contractEventXDR(t *testing.T, typ xdr.ContractEventType) (string, []string, string) {
	t.Helper()
	sym := xdr.ScSymbol("transfer")
	amount := xdr.Uint32(7)
	topics := []xdr.ScVal{{Type: xdr.ScValTypeScvSymbol, Sym: &sym}}
	data := xdr.ScVal{Type: xdr.ScValTypeScvU32, U32: &amount}
	ev := xdr.ContractEvent{
		Type: typ,
		Body: xdr.ContractEventBody{V: 0, V0: &xdr.ContractEventV0{Topics: topics, Data: data}},
	}
	b64, err := xdr.MarshalBase64(ev)
	if err != nil {
		t.Fatal(err)
	}
	topic, _ := xdr.MarshalBase64(topics[0])
	value, _ := xdr.MarshalBase64(data)
	return b64, []string{topic}, value
}

func TestHubbleRowToEvent(t *testing.T) {
	raw, topics, value := contractEventXDR(t, xdr.ContractEventTypeContract)
	closed := time.Date(2024, 3, 1, 12, 0, 0, 0, time.FixedZone("x", 3600))
	row := HubbleRow{
		LedgerSequence:           51_000_000,
		ClosedAt:                 closed,
		TransactionHash:          "abc",
		TransactionID:            219043332096 << 4,
		EventIndex:               2,
		ContractID:               contractID,
		ContractEventXDR:         raw,
		InSuccessfulContractCall: true,
	}
	ev, err := row.ToEvent()
	if err != nil {
		t.Fatal(err)
	}
	if ev.ID != EventID(row.TransactionID, 2) || len(ev.ID) != 30 {
		t.Fatalf("id = %q", ev.ID)
	}
	if ev.Type != "contract" || ev.Ledger != 51_000_000 || ev.TxHash != "abc" || !ev.InSuccessfulCall {
		t.Fatalf("event = %+v", ev)
	}
	if ev.LedgerClosedAt.Location() != time.UTC || !ev.LedgerClosedAt.Equal(closed) {
		t.Fatalf("closed at = %v", ev.LedgerClosedAt)
	}
	if len(ev.TopicXDR) != 1 || ev.TopicXDR[0] != topics[0] || ev.ValueXDR != value {
		t.Fatalf("topics %v value %q", ev.TopicXDR, ev.ValueXDR)
	}
}

func TestHubbleRowEventTypes(t *testing.T) {
	for typ, want := range map[xdr.ContractEventType]string{
		xdr.ContractEventTypeSystem:     "system",
		xdr.ContractEventTypeDiagnostic: "diagnostic",
	} {
		raw, _, _ := contractEventXDR(t, typ)
		ev, err := HubbleRow{LedgerSequence: 1, TransactionID: 1, ContractEventXDR: raw}.ToEvent()
		if err != nil || ev.Type != want {
			t.Fatalf("type %v: %q, %v", typ, ev.Type, err)
		}
	}
}

func TestHubbleRowRejectsBadRows(t *testing.T) {
	raw, _, _ := contractEventXDR(t, xdr.ContractEventTypeContract)
	for name, row := range map[string]HubbleRow{
		"no ledger":       {TransactionID: 1, ContractEventXDR: raw},
		"no tx id":        {LedgerSequence: 1, ContractEventXDR: raw},
		"negative index":  {LedgerSequence: 1, TransactionID: 1, EventIndex: -1, ContractEventXDR: raw},
		"bad xdr":         {LedgerSequence: 1, TransactionID: 1, ContractEventXDR: "not xdr"},
		"ledger too high": {LedgerSequence: 1 << 33, TransactionID: 1, ContractEventXDR: raw},
	} {
		if _, err := row.ToEvent(); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestEventIDMatchesRPCFormat(t *testing.T) {
	// getEvents ids: the operation's TOID padded to 19 digits, a dash, and
	// the event index padded to 10.
	if got := EventID(4294971392, 1); got != "0000000004294971392-0000000001" {
		t.Fatalf("EventID = %q", got)
	}
}

func TestLedgerTimeBoundsCoverRealCloseTimes(t *testing.T) {
	anchor := uint32(50_000_000)
	anchorTime := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	for _, per := range []time.Duration{4 * time.Second, 5 * time.Second, 7 * time.Second} {
		at := func(l uint32) time.Time {
			return anchorTime.Add(time.Duration(int64(l)-int64(anchor)) * per)
		}
		for _, r := range [][2]uint32{{49_000_000, 49_030_000}, {50_010_000, 50_040_000}, {49_990_000, 50_010_000}} {
			lo, hi := LedgerTimeBounds(r[0], r[1], anchor, anchorTime)
			if !lo.Before(at(r[0])) || !hi.After(at(r[1])) {
				t.Fatalf("%v at %v/ledger: bounds %v..%v miss %v..%v", r, per, lo, hi, at(r[0]), at(r[1]))
			}
		}
	}
}

func TestHubbleConfig(t *testing.T) {
	cfg := HubbleConfig{BillingProject: "mine"}
	if got := cfg.Table(); got != "`crypto-stellar.crypto_stellar.history_contract_events`" {
		t.Fatalf("Table = %s", got)
	}
	if err := cfg.Validate("mainnet"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate("testnet"); err == nil {
		t.Fatal("the public dataset was accepted for testnet")
	}
	if err := (HubbleConfig{BillingProject: "mine", Dataset: "my_testnet"}).Validate("testnet"); err != nil {
		t.Fatal(err)
	}
	if err := (HubbleConfig{}).Validate("mainnet"); err == nil {
		t.Fatal("a config without a billing project was accepted")
	}
	if err := (HubbleConfig{BillingProject: "x", Dataset: "a` OR 1=1"}).Validate("mainnet"); err == nil {
		t.Fatal("an injected dataset name was accepted")
	}
	q := HubbleQuery(HubbleConfig{Project: "p", Dataset: "d"})
	for _, want := range []string{"`p.d.history_contract_events`", "@contract_id", "@from_ledger", "@to_time", "successful"} {
		if !strings.Contains(q, want) {
			t.Fatalf("query lacks %q:\n%s", want, q)
		}
	}
}

type fakeReader struct {
	params []map[string]any
	rows   [][]HubbleRow
}

func (f *fakeReader) Rows(_ context.Context, _ string, params map[string]any) ([]HubbleRow, error) {
	f.params = append(f.params, params)
	if len(f.rows) == 0 {
		return nil, nil
	}
	out := f.rows[0]
	f.rows = f.rows[1:]
	return out, nil
}

func TestHubbleSourceMovesItsAnchor(t *testing.T) {
	raw, _, _ := contractEventXDR(t, xdr.ContractEventTypeContract)
	rowAt := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	reader := &fakeReader{rows: [][]HubbleRow{{
		{LedgerSequence: 1_000_500, ClosedAt: rowAt, TransactionID: 9, ContractID: contractID, ContractEventXDR: raw},
	}}}
	src := NewHubbleSource(HubbleConfig{BillingProject: "mine"}, reader, 9_000_000, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))

	ctx := context.Background()
	events, err := src.Events(ctx, Query{ContractID: contractID, Network: "mainnet", FromLedger: 1_000_000, ToLedger: 1_029_999})
	if err != nil || len(events) != 1 {
		t.Fatalf("events = %v, err = %v", events, err)
	}
	if _, err := src.Events(ctx, Query{ContractID: contractID, Network: "mainnet", FromLedger: 1_030_000, ToLedger: 1_059_999}); err != nil {
		t.Fatal(err)
	}

	first, second := reader.params[0], reader.params[1]
	if first["contract_id"] != contractID || first["from_ledger"] != int64(1_000_000) || first["to_ledger"] != int64(1_029_999) {
		t.Fatalf("params = %v", first)
	}
	// Anchored on the row it found, the second batch's window is a couple
	// of days wide instead of the first one's year and more.
	wide := first["to_time"].(time.Time).Sub(first["from_time"].(time.Time))
	tight := second["to_time"].(time.Time).Sub(second["from_time"].(time.Time))
	if tight*50 > wide || tight > 5*24*time.Hour {
		t.Fatalf("first window %v, second %v", wide, tight)
	}
}

func TestHubbleSourceNeedsAnAnchor(t *testing.T) {
	src := NewHubbleSource(HubbleConfig{BillingProject: "mine"}, &fakeReader{}, 0, time.Time{})
	if _, err := src.Events(context.Background(), Query{Network: "mainnet", FromLedger: 1, ToLedger: 2}); err == nil {
		t.Fatal("a source without an anchor ran a query")
	}
}
