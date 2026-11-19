package history

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/sorolens/sorolens/services/indexer/internal/poller"
)

const contractID = "CCLV77FYLRJMBGTILTKVIM76SI6JY56H7KTT47HE26UF4F265UYRDSR4"

var t0 = time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)

// fakeSource returns one event per ledger divisible by 10 in each query.
type fakeSource struct {
	queries []Query
	failAt  int
	foreign bool
}

func (f *fakeSource) Events(_ context.Context, q Query) ([]poller.Event, error) {
	f.queries = append(f.queries, q)
	if f.failAt > 0 && len(f.queries) == f.failAt {
		return nil, errors.New("quota exceeded")
	}
	var out []poller.Event
	for l := q.FromLedger; l <= q.ToLedger; l++ {
		if l%10 != 0 {
			continue
		}
		id := contractID
		if f.foreign {
			id = "COTHER"
		}
		out = append(out, poller.Event{
			ID:             EventID(int64(l)<<32|1<<12, 0),
			ContractID:     id,
			Ledger:         l,
			LedgerClosedAt: t0.Add(time.Duration(l) * 5 * time.Second),
			Type:           "contract",
		})
	}
	return out, nil
}

// fakeStore keeps events by ID, the way the events table's primary key and
// ON CONFLICT DO NOTHING do.
type fakeStore struct {
	target  Target
	err     error
	rows    map[string]poller.Event
	commits int
}

func (s *fakeStore) ImportTarget(context.Context, string) (Target, error) {
	return s.target, s.err
}

func (s *fakeStore) CommitHistoryBatch(_ context.Context, events []poller.Event, invocations []poller.Invocation) ([]poller.FailedEvent, error) {
	if len(invocations) != 0 {
		return nil, fmt.Errorf("unexpected invocations")
	}
	if s.rows == nil {
		s.rows = map[string]poller.Event{}
	}
	s.commits++
	for _, e := range events {
		if _, ok := s.rows[e.ID]; !ok {
			s.rows[e.ID] = e
		}
	}
	return nil, nil
}

func TestImportBatchesTheRange(t *testing.T) {
	src := &fakeSource{}
	st := &fakeStore{target: Target{Network: "mainnet"}}
	im := &Importer{Source: src, Store: st, BatchLedgers: 100}

	res, err := im.Import(context.Background(), Request{ContractID: contractID, FromLedger: 1001, ToLedger: 1250})
	if err != nil {
		t.Fatal(err)
	}
	want := []Query{
		{contractID, "mainnet", 1001, 1100},
		{contractID, "mainnet", 1101, 1200},
		{contractID, "mainnet", 1201, 1250},
	}
	if fmt.Sprint(src.queries) != fmt.Sprint(want) {
		t.Fatalf("queries = %v, want %v", src.queries, want)
	}
	if res.Batches != 3 || res.Events != 25 || len(st.rows) != 25 {
		t.Fatalf("result = %+v, stored %d", res, len(st.rows))
	}
	for _, e := range st.rows {
		if e.Network != "mainnet" {
			t.Fatalf("event stored without the contract's network: %+v", e)
		}
	}
}

func TestImportIsIdempotent(t *testing.T) {
	st := &fakeStore{target: Target{Network: "mainnet"}}
	req := Request{ContractID: contractID, FromLedger: 1, ToLedger: 500}
	for i := 0; i < 2; i++ {
		im := &Importer{Source: &fakeSource{}, Store: st, BatchLedgers: 64}
		if _, err := im.Import(context.Background(), req); err != nil {
			t.Fatal(err)
		}
	}
	if len(st.rows) != 50 {
		t.Fatalf("stored %d events after two runs, want 50", len(st.rows))
	}
}

func TestImportClipsAtTheRPCFloor(t *testing.T) {
	src := &fakeSource{}
	st := &fakeStore{target: Target{Network: "mainnet", RPCFloorLedger: 1200}}
	res, err := (&Importer{Source: src, Store: st}).Import(context.Background(),
		Request{ContractID: contractID, FromLedger: 1000, ToLedger: 5000})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Clipped || res.ToLedger != 1199 {
		t.Fatalf("result = %+v, want clipped at 1199", res)
	}
	if last := src.queries[len(src.queries)-1]; last.ToLedger != 1199 {
		t.Fatalf("last query = %+v", last)
	}
}

func TestImportSkipsARangeTheRPCCovers(t *testing.T) {
	src := &fakeSource{}
	st := &fakeStore{target: Target{Network: "mainnet", RPCFloorLedger: 900}}
	res, err := (&Importer{Source: src, Store: st}).Import(context.Background(),
		Request{ContractID: contractID, FromLedger: 1000, ToLedger: 2000})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Skipped || len(src.queries) != 0 {
		t.Fatalf("result = %+v, queries %v", res, src.queries)
	}
}

func TestImportStopsOnSourceError(t *testing.T) {
	st := &fakeStore{target: Target{Network: "mainnet"}}
	src := &fakeSource{failAt: 2}
	res, err := (&Importer{Source: src, Store: st, BatchLedgers: 100}).Import(context.Background(),
		Request{ContractID: contractID, FromLedger: 1, ToLedger: 1000})
	if err == nil || res.Batches != 1 || st.commits != 1 {
		t.Fatalf("err = %v, result = %+v, commits = %d", err, res, st.commits)
	}
}

func TestImportRejectsRowsOfAnotherContract(t *testing.T) {
	st := &fakeStore{target: Target{Network: "mainnet"}}
	_, err := (&Importer{Source: &fakeSource{foreign: true}, Store: st}).Import(context.Background(),
		Request{ContractID: contractID, FromLedger: 1, ToLedger: 100})
	if err == nil || st.commits != 0 {
		t.Fatalf("err = %v, commits = %d", err, st.commits)
	}
}

func TestImportValidatesTheRequest(t *testing.T) {
	st := &fakeStore{}
	im := &Importer{Source: &fakeSource{}, Store: st}
	for _, req := range []Request{
		{FromLedger: 1, ToLedger: 2},
		{ContractID: contractID, ToLedger: 2},
		{ContractID: contractID, FromLedger: 5, ToLedger: 2},
	} {
		if _, err := im.Import(context.Background(), req); err == nil {
			t.Fatalf("Import(%+v) succeeded", req)
		}
	}
	if _, err := (&Importer{}).Import(context.Background(), Request{}); err == nil {
		t.Fatal("an importer without a source succeeded")
	}
	st.err = errors.New("contract is not tracked")
	if _, err := im.Import(context.Background(), Request{ContractID: contractID, FromLedger: 1, ToLedger: 2}); err == nil {
		t.Fatal("an untracked contract was imported")
	}
}

func TestImportHonoursCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	st := &fakeStore{target: Target{Network: "mainnet"}}
	if _, err := (&Importer{Source: &fakeSource{}, Store: st}).Import(ctx,
		Request{ContractID: contractID, FromLedger: 1, ToLedger: 100}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}
