package store_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// TestCoreStorePostgres covers the remaining SQL in postgres.go: labels,
// contract CRUD/search/bulk actions, the batch inserters, sync state and
// indexer cursors, the transactional batch, global stats, watchlist and
// contract versions.
func TestCoreStorePostgres(t *testing.T) {
	pool := pgPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `TRUNCATE TABLE contracts, users, labels_public, labels_workspace,
		watchlist_items, indexer_cursors, contract_versions, events, invocations,
		call_edges, storage_entries, storage_entry_history, sync_state,
		contract_health_scores, performance_baselines, monitored_contracts CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	s := store.NewFullStore(pool)

	// ---- labels --------------------------------------------------------
	for _, id := range []string{"user-1", "user-2"} {
		if err := s.UpsertUser(ctx, store.User{ID: id}); err != nil {
			t.Fatalf("UpsertUser(%s): %v", id, err)
		}
	}
	publicAlpha := strings.Repeat("A", 56)
	publicShared := strings.Repeat("B", 56)
	workspaceAlpha := strings.Repeat("C", 56)
	workspaceShared := strings.Repeat("D", 56)
	workspaceBeta := strings.Repeat("E", 56)
	labels := []store.Label{
		{Label: "alpha", Value: publicAlpha, Public: true},
		{Label: "shared", Value: publicShared, Public: true},
		{Label: "alpha", Value: workspaceAlpha, WorkspaceID: "user-1"},
		{Label: "shared", Value: workspaceShared, WorkspaceID: "user-1"},
		{Label: "beta", Value: workspaceBeta, WorkspaceID: "user-2"},
	}
	for _, l := range labels {
		if err := s.UpsertLabel(ctx, l); err != nil {
			t.Fatalf("UpsertLabel(%s/%s): %v", l.WorkspaceID, l.Label, err)
		}
	}
	listed, err := s.ListLabels(ctx, "user-1", "")
	if err != nil {
		t.Fatalf("ListLabels: %v", err)
	}
	if len(listed) != 4 {
		t.Fatalf("ListLabels(user-1) = %d rows, want 4 (2 public + 2 workspace)", len(listed))
	}
	// ORDER BY label, and workspace-2 labels are not visible to user-1.
	for i := 1; i < len(listed); i++ {
		if listed[i-1].Label > listed[i].Label {
			t.Errorf("ListLabels is not sorted: %v", labelNames(listed))
		}
	}
	matched, err := s.ListLabels(ctx, "user-1", "sha")
	if err != nil || len(matched) != 2 {
		t.Fatalf("ListLabels(query=sha) = %d rows, %v", len(matched), err)
	}
	resolved, err := s.ResolveLabel(ctx, "user-1", "ALPHA")
	if err != nil {
		t.Fatalf("ResolveLabel: %v", err)
	}
	if resolved.Public || resolved.Value != workspaceAlpha {
		t.Errorf("ResolveLabel picked %+v, want the workspace-scoped override", resolved)
	}
	resolved, err = s.ResolveLabel(ctx, "user-2", "alpha")
	if err != nil {
		t.Fatalf("ResolveLabel(public fallback): %v", err)
	}
	if !resolved.Public || resolved.Value != publicAlpha {
		t.Errorf("ResolveLabel fallback = %+v", resolved)
	}
	if _, err := s.ResolveLabel(ctx, "user-1", "missing"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("ResolveLabel(unknown): want ErrNotFound, got %v", err)
	}
	updatedPublic := strings.Repeat("F", 56)
	if err := s.UpsertLabel(ctx, store.Label{Label: "alpha", Value: updatedPublic, Public: true}); err != nil {
		t.Fatalf("UpsertLabel update: %v", err)
	}
	resolved, err = s.ResolveLabel(ctx, "user-2", "alpha")
	if err != nil || resolved.Value != updatedPublic {
		t.Errorf("public label update = %+v, %v", resolved, err)
	}

	// ---- contracts -----------------------------------------------------
	t1 := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	t3 := t1.Add(2 * time.Hour)
	contracts := []store.Contract{
		{ID: "c-a", Network: "testnet", Label: "alpha-contract", Status: "active", AddedAt: t1},
		{ID: "c-b", Network: "mainnet", Label: "beta-contract", Status: "active", AddedAt: t2},
		{ID: "c-c", Network: "testnet", Label: "gamma-contract", Status: "paused", AddedAt: t3},
	}
	for _, c := range contracts {
		if err := s.UpsertContract(ctx, c); err != nil {
			t.Fatalf("UpsertContract(%s): %v", c.ID, err)
		}
	}
	if err := s.UpsertContract(ctx, store.Contract{ID: "c-a", Network: "testnet", Label: "alpha-renamed",
		Status: "active", AddedAt: t1}); err != nil {
		t.Fatalf("UpsertContract update: %v", err)
	}
	got, err := s.GetContract(ctx, "c-a")
	if err != nil {
		t.Fatalf("GetContract: %v", err)
	}
	if got.Label != "alpha-renamed" || got.Tags == nil || len(got.Tags) != 0 {
		t.Errorf("GetContract = %+v, want updated label and empty non-nil tags", got)
	}
	if _, err := s.GetContract(ctx, "c-missing"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("GetContract(unknown): want ErrNotFound, got %v", err)
	}

	if err := s.AddContractTag(ctx, "c-a", "defi"); err != nil {
		t.Fatalf("AddContractTag: %v", err)
	}
	if err := s.AddContractTag(ctx, "c-a", "core"); err != nil {
		t.Fatalf("AddContractTag: %v", err)
	}
	if err := s.AddContractTag(ctx, "c-a", "defi"); err != nil {
		t.Fatalf("duplicate AddContractTag: %v", err)
	}
	got, err = s.GetContract(ctx, "c-a")
	if err != nil {
		t.Fatalf("GetContract with tags: %v", err)
	}
	if strings.Join(got.Tags, ",") != "core,defi" {
		t.Errorf("tags = %v, want [core defi]", got.Tags)
	}

	testnet, next, err := s.ListContracts(ctx, "", 50, store.ContractFilters{Network: "testnet"})
	if err != nil || len(testnet) != 2 || next != "" {
		t.Fatalf("ListContracts(network) = %v, %v, next %q", contractIDs(testnet), err, next)
	}
	if strings.Join(contractIDs(testnet), ",") != "c-a,c-c" {
		t.Errorf("testnet contracts = %v", contractIDs(testnet))
	}
	if testnet[0].Tags == nil || strings.Join(testnet[0].Tags, ",") != "core,defi" {
		t.Errorf("tags not attached to the list page: %+v", testnet[0].Tags)
	}
	active, _, err := s.ListContracts(ctx, "", 50, store.ContractFilters{Status: "active"})
	if err != nil || len(active) != 2 {
		t.Fatalf("ListContracts(status) = %v, %v", contractIDs(active), err)
	}
	byAdded, _, err := s.ListContracts(ctx, "", 50, store.ContractFilters{Sort: "added_at", SortDir: "desc"})
	if err != nil || strings.Join(contractIDs(byAdded), ",") != "c-c,c-b,c-a" {
		t.Fatalf("ListContracts(sorted) = %v, %v", contractIDs(byAdded), err)
	}
	page1, cursor, err := s.ListContracts(ctx, "", 2, store.ContractFilters{})
	if err != nil || len(page1) != 2 || cursor != "c-b" {
		t.Fatalf("ListContracts page 1 = %v, cursor %q, %v", contractIDs(page1), cursor, err)
	}
	page2, cursor2, err := s.ListContracts(ctx, cursor, 2, store.ContractFilters{})
	if err != nil || len(page2) != 1 || page2[0].ID != "c-c" || cursor2 != "" {
		t.Fatalf("ListContracts page 2 = %v, cursor %q, %v", contractIDs(page2), cursor2, err)
	}
	byName, err := s.SearchContracts(ctx, "gamma", 10)
	if err != nil || len(byName) != 1 || byName[0].ID != "c-c" {
		t.Errorf("SearchContracts(label) = %v, %v", contractIDs(byName), err)
	}
	byID, err := s.SearchContracts(ctx, "c-", 10)
	if err != nil || len(byID) != 3 {
		t.Errorf("SearchContracts(id) = %v, %v", contractIDs(byID), err)
	}
	if all, err := s.SearchContracts(ctx, "", 10); err != nil || all == nil || len(all) != 0 {
		t.Errorf("SearchContracts(empty) = %v, %v, want an empty non-nil slice", all, err)
	}
	if updated, err := s.SetContractLabel(ctx, []string{"c-a", "c-c"}, "prod"); err != nil || updated != 2 {
		t.Fatalf("SetContractLabel = %d, %v", updated, err)
	}
	if updated, err := s.SetContractLabel(ctx, nil, "x"); err != nil || updated != 0 {
		t.Errorf("SetContractLabel(empty) = %d, %v", updated, err)
	}
	if byProd, err := s.SearchContracts(ctx, "prod", 10); err != nil || len(byProd) != 2 {
		t.Errorf("SearchContracts(updated label) = %v, %v", contractIDs(byProd), err)
	}

	// ---- batch inserts -------------------------------------------------
	base := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	events := []store.Event{
		{ID: "ev-1", ContractID: "c-a", Network: "testnet", Ledger: 1, LedgerClosedAt: base,
			TxHash: "tx-1", Type: "system", TopicXDR: []string{"t0"}, ValueXDR: "v1"},
		{ID: "ev-2", ContractID: "c-a", Network: "testnet", Ledger: 2, LedgerClosedAt: base.Add(time.Hour),
			TxHash: "shared-tx", Type: "contract", TopicXDR: []string{"t1"},
			TopicDecoded: []any{"transfer", float64(5)}, ValueDecoded: map[string]any{"amount": "5"},
			InSuccessfulCall: true},
		{ID: "ev-3", ContractID: "c-a", Network: "testnet", Ledger: 3, LedgerClosedAt: base.Add(2 * time.Hour),
			TxHash: "shared-tx", Type: "contract", TopicXDR: []string{"t2"},
			TopicDecoded: []any{"transfer", float64(6)}},
	}
	if err := s.BatchInsertEvents(ctx, events); err != nil {
		t.Fatalf("BatchInsertEvents: %v", err)
	}
	duplicate := events[0]
	duplicate.Type = "overwritten"
	if err := s.BatchInsertEvents(ctx, []store.Event{duplicate}); err != nil {
		t.Fatalf("duplicate BatchInsertEvents: %v", err)
	}
	storedEvents, _, err := s.ListEvents(ctx, "c-a", "", 50, store.EventFilters{})
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(storedEvents) != 3 || storedEvents[0].Type != "system" {
		t.Fatalf("events after duplicate insert = %d, first type %q", len(storedEvents), storedEvents[0].Type)
	}
	if len(storedEvents[1].TopicXDR) != 1 || storedEvents[1].TopicXDR[0] != "t1" ||
		len(storedEvents[1].TopicDecoded) != 2 || storedEvents[1].TopicDecoded[0] != "transfer" {
		t.Errorf("decoded JSON did not round-trip: %+v", storedEvents[1])
	}
	decodedValue, ok := storedEvents[1].ValueDecoded.(map[string]any)
	if !ok || decodedValue["amount"] != "5" {
		t.Errorf("value_decoded = %#v", storedEvents[1].ValueDecoded)
	}

	invocations := []store.Invocation{
		{TxHash: "tx-1", ContractID: "c-a", Network: "testnet", Ledger: 1, LedgerClosedAt: base,
			Status: "SUCCESS", FunctionName: "transfer", CPUInsn: 100, MemByte: 10, ResourceFeeCharged: 5},
		{TxHash: "shared-tx", ContractID: "c-a", Network: "testnet", Ledger: 3, LedgerClosedAt: base.Add(2 * time.Hour),
			Status: "FAILED", FunctionName: "transfer_from", CPUInsn: 200, MemByte: 20, ResourceFeeCharged: 15},
		{TxHash: "tx-old", ContractID: "c-a", Network: "testnet", Ledger: 4, LedgerClosedAt: base,
			Status: "SUCCESS", FunctionName: "transfer"},
	}
	if err := s.BatchInsertInvocations(ctx, invocations); err != nil {
		t.Fatalf("BatchInsertInvocations: %v", err)
	}
	if err := s.BatchInsertInvocations(ctx, invocations[:1]); err != nil {
		t.Fatalf("duplicate BatchInsertInvocations: %v", err)
	}
	var invCount int64
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM invocations WHERE contract_id = 'c-a'`).Scan(&invCount); err != nil {
		t.Fatal(err)
	}
	if invCount != 3 {
		t.Errorf("invocations after duplicate insert = %d, want 3", invCount)
	}

	edges := []store.CallEdge{
		{TxHash: "shared-tx", ParentSpanID: "0", ChildSpanID: "0.0", CalleeContractID: "c-b",
			FunctionName: "transfer", CPU: 10, Mem: 20, FeeShare: 3, Depth: 1,
			Network: "testnet", Ledger: 3, LedgerClosedAt: base.Add(2 * time.Hour)},
		{TxHash: "shared-tx", ParentSpanID: "0", ChildSpanID: "0.1", Depth: 1,
			Network: "testnet", Ledger: 3, LedgerClosedAt: base.Add(2 * time.Hour)},
	}
	if err := s.BatchInsertCallEdges(ctx, edges); err != nil {
		t.Fatalf("BatchInsertCallEdges: %v", err)
	}
	if err := s.BatchInsertCallEdges(ctx, edges); err != nil {
		t.Fatalf("duplicate BatchInsertCallEdges: %v", err)
	}
	var edgeCount int64
	var nullCallee int64
	if err := pool.QueryRow(ctx, `SELECT COUNT(*), COUNT(*) FILTER (WHERE callee_contract_id IS NULL)
		FROM call_edges WHERE tx_hash = 'shared-tx'`).Scan(&edgeCount, &nullCallee); err != nil {
		t.Fatal(err)
	}
	if edgeCount != 2 || nullCallee != 1 {
		t.Errorf("call edges = %d rows, %d null callee; want 2 and 1", edgeCount, nullCallee)
	}

	storage := []store.StorageEntry{
		{ContractID: "c-a", Network: "testnet", KeyXDR: "k1", KeyDecoded: map[string]any{"k": "one"},
			ValueXDR: "v1", ValueDecoded: map[string]any{"v": 1}, Durability: "persistent",
			LiveUntilLedger: 100, LastModifiedLedger: 10, Status: "live"},
	}
	if err := s.UpsertStorageEntries(ctx, storage); err != nil {
		t.Fatalf("UpsertStorageEntries: %v", err)
	}
	storage[0].ValueXDR = "v2"
	storage[0].ValueDecoded = map[string]any{"v": 2}
	storage[0].LastModifiedLedger = 20
	if err := s.UpsertStorageEntries(ctx, storage); err != nil {
		t.Fatalf("second UpsertStorageEntries: %v", err)
	}
	var currentValue string
	if err := pool.QueryRow(ctx, `SELECT value_xdr FROM storage_entries
		WHERE contract_id = 'c-a' AND key_xdr = 'k1'`).Scan(&currentValue); err != nil {
		t.Fatal(err)
	}
	if currentValue != "v2" {
		t.Errorf("storage_entries = %q, want the upserted v2", currentValue)
	}
	var historyRows int64
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM storage_entry_history
		WHERE contract_id = 'c-a' AND key_xdr = 'k1'`).Scan(&historyRows); err != nil {
		t.Fatal(err)
	}
	if historyRows != 2 {
		t.Errorf("storage history = %d rows, want one per last_modified_ledger", historyRows)
	}

	// ---- sync state and cursors ---------------------------------------
	syncState, err := s.GetSyncState(ctx, "c-a")
	if err != nil {
		t.Fatalf("GetSyncState default: %v", err)
	}
	if syncState.ContractID != "c-a" || syncState.LastLedger != 0 {
		t.Errorf("default sync state = %+v", syncState)
	}
	if err := s.UpsertSyncState(ctx, store.SyncState{
		ContractID: "c-a", LastLedger: 42, ErrorMessage: "boom",
	}); err != nil {
		t.Fatalf("UpsertSyncState: %v", err)
	}
	syncState, err = s.GetSyncState(ctx, "c-a")
	if err != nil || syncState.LastLedger != 42 || syncState.ErrorMessage != "boom" {
		t.Fatalf("GetSyncState = %+v, %v", syncState, err)
	}

	if cursor, err := s.GetIndexerCursor(ctx, "mainnet"); err != nil || cursor != 0 {
		t.Fatalf("empty cursor = %d, %v", cursor, err)
	}
	for _, ledger := range []uint32{10, 5, 20} {
		if err := s.SetIndexerCursor(ctx, "mainnet", ledger); err != nil {
			t.Fatalf("SetIndexerCursor(%d): %v", ledger, err)
		}
	}
	if cursor, err := s.GetIndexerCursor(ctx, "mainnet"); err != nil || cursor != 20 {
		t.Errorf("cursor after 10,5,20 = %d, %v; want monotonically 20", cursor, err)
	}
	if err := s.SetIndexerCursor(ctx, "", 7); err != nil {
		t.Fatalf("SetIndexerCursor(default network): %v", err)
	}
	if cursor, _ := s.GetIndexerCursor(ctx, ""); cursor != 7 {
		t.Errorf("default-network cursor = %d, want 7", cursor)
	}

	// ---- transactional batch (#506 surface in the API store) ----------
	badInvocations := []store.Invocation{{
		TxHash: "tx-bad", ContractID: "c-missing", Network: "futurenet", Ledger: 50,
		LedgerClosedAt: base, Status: "SUCCESS",
	}}
	err = s.BatchInsertWithCursor(ctx, "futurenet", 50,
		[]store.Event{{ID: "ev-batch-bad", ContractID: "c-a", Network: "futurenet", Ledger: 50,
			LedgerClosedAt: base, TxHash: "tx-bad", Type: "contract"}},
		badInvocations, nil, store.SyncState{ContractID: "c-a", LastLedger: 50})
	if err == nil {
		t.Fatal("BatchInsertWithCursor succeeded with a missing invocation contract")
	}
	var badEvents, badState, badCursor int64
	if err := pool.QueryRow(ctx, `SELECT
		(SELECT COUNT(*) FROM events WHERE id = 'ev-batch-bad'),
		(SELECT COUNT(*) FROM sync_state WHERE contract_id = 'c-a' AND last_ledger = 50),
		(SELECT COUNT(*) FROM indexer_cursors WHERE network = 'futurenet')`).
		Scan(&badEvents, &badState, &badCursor); err != nil {
		t.Fatal(err)
	}
	if badEvents != 0 || badState != 0 || badCursor != 0 {
		t.Errorf("failed transaction left half-writes: events=%d state=%d cursor=%d", badEvents, badState, badCursor)
	}

	goodInvocations := []store.Invocation{{
		TxHash: "tx-batch", ContractID: "c-a", Network: "futurenet", Ledger: 51,
		LedgerClosedAt: base, Status: "SUCCESS", FunctionName: "f",
	}}
	goodEdges := []store.CallEdge{{
		TxHash: "tx-batch", ParentSpanID: "0", ChildSpanID: "0.0", Depth: 1,
		Network: "futurenet", Ledger: 51, LedgerClosedAt: base,
	}}
	goodEvents := []store.Event{{ID: "ev-batch", ContractID: "c-a", Network: "futurenet", Ledger: 51,
		LedgerClosedAt: base, TxHash: "tx-batch", Type: "contract"}}
	for i := 0; i < 2; i++ {
		if err := s.BatchInsertWithCursor(ctx, "futurenet", 51, goodEvents, goodInvocations,
			goodEdges, store.SyncState{ContractID: "c-a", LastLedger: 51}); err != nil {
			t.Fatalf("BatchInsertWithCursor attempt %d: %v", i+1, err)
		}
	}
	if err := s.BatchInsertWithCursor(ctx, "futurenet", 40, nil, nil, nil,
		store.SyncState{}); err != nil {
		t.Fatalf("BatchInsertWithCursor lower ledger: %v", err)
	}
	var batchEvents, batchInvocations, batchEdges int64
	if err := pool.QueryRow(ctx, `SELECT
		(SELECT COUNT(*) FROM events WHERE id = 'ev-batch'),
		(SELECT COUNT(*) FROM invocations WHERE tx_hash = 'tx-batch'),
		(SELECT COUNT(*) FROM call_edges WHERE tx_hash = 'tx-batch')`).
		Scan(&batchEvents, &batchInvocations, &batchEdges); err != nil {
		t.Fatal(err)
	}
	if batchEvents != 1 || batchInvocations != 1 || batchEdges != 1 {
		t.Errorf("retry duplicated rows: events=%d invocations=%d edges=%d", batchEvents, batchInvocations, batchEdges)
	}
	syncState, _ = s.GetSyncState(ctx, "c-a")
	if syncState.LastLedger != 51 {
		t.Errorf("sync state after batch = %d, want 51", syncState.LastLedger)
	}
	if cursor, _ := s.GetIndexerCursor(ctx, "futurenet"); cursor != 51 {
		t.Errorf("cursor after lower batch = %d, want 51 (GREATEST)", cursor)
	}

	// ---- global stats --------------------------------------------------
	stats, err := s.GetGlobalStats(ctx)
	if err != nil {
		t.Fatalf("GetGlobalStats: %v", err)
	}
	if stats.TrackedContracts != 3 {
		t.Errorf("tracked contracts = %d, want 3", stats.TrackedContracts)
	}
	if stats.TotalEvents == 0 || stats.TotalInvocations == 0 || stats.TotalStorageEntries != 1 {
		t.Errorf("global stats = %+v", stats)
	}

	// ---- watchlist -----------------------------------------------------
	if err := s.AddToWatchlist(ctx, "user-1", "c-a"); err != nil {
		t.Fatalf("AddToWatchlist: %v", err)
	}
	if err := s.AddToWatchlist(ctx, "user-1", "c-a"); err != nil {
		t.Fatalf("duplicate AddToWatchlist: %v", err)
	}
	watchlist, err := s.ListWatchlist(ctx, "user-1")
	if err != nil || strings.Join(watchlist, ",") != "c-a" {
		t.Fatalf("ListWatchlist = %v, %v", watchlist, err)
	}
	if ok, err := s.IsInWatchlist(ctx, "user-1", "c-a"); err != nil || !ok {
		t.Errorf("IsInWatchlist = %v, %v", ok, err)
	}
	if err := s.RemoveFromWatchlist(ctx, "user-1", "c-a"); err != nil {
		t.Fatalf("RemoveFromWatchlist: %v", err)
	}
	if err := s.RemoveFromWatchlist(ctx, "user-1", "c-a"); err != nil {
		t.Errorf("second RemoveFromWatchlist: %v", err)
	}
	if ok, _ := s.IsInWatchlist(ctx, "user-1", "c-a"); ok {
		t.Error("contract still in the watchlist after removal")
	}

	// ---- contract versions --------------------------------------------
	versions := []store.ContractVersion{
		{ContractID: "c-a", WasmHash: "hash-1", FirstSeenLedger: 10, TxHash: "tx-1", VerifiedSourceRef: "src-1"},
		{ContractID: "c-a", WasmHash: "hash-2", FirstSeenLedger: 20},
	}
	for _, v := range versions {
		if err := s.RecordContractVersion(ctx, v); err != nil {
			t.Fatalf("RecordContractVersion: %v", err)
		}
	}
	if err := s.RecordContractVersion(ctx, versions[0]); err != nil {
		t.Fatalf("duplicate RecordContractVersion: %v", err)
	}
	listedVersions, err := s.ListContractVersions(ctx, "c-a")
	if err != nil || len(listedVersions) != 2 {
		t.Fatalf("ListContractVersions = %d, %v", len(listedVersions), err)
	}
	if listedVersions[0].WasmHash != "hash-1" || listedVersions[1].WasmHash != "hash-2" {
		t.Errorf("versions not oldest-first: %+v", listedVersions)
	}
	if listedVersions[0].TxHash != "tx-1" || listedVersions[0].VerifiedSourceRef != "src-1" {
		t.Errorf("optional version fields = %+v", listedVersions[0])
	}
	latest, err := s.GetLatestContractVersion(ctx, "c-a")
	if err != nil || latest.WasmHash != "hash-2" || latest.TxHash != "" {
		t.Errorf("GetLatestContractVersion = %+v, %v", latest, err)
	}
	if _, err := s.GetLatestContractVersion(ctx, "c-missing"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("GetLatestContractVersion(unknown): want ErrNotFound, got %v", err)
	}

	// ---- global search -------------------------------------------------
	searchEvents, err := s.SearchEvents(ctx, "shared", 10)
	if err != nil {
		t.Fatalf("SearchEvents: %v", err)
	}
	if len(searchEvents) != 1 || searchEvents[0].ID != "ev-3" {
		t.Errorf("SearchEvents = %v, want the newest event of the tx", searchEventIDs(searchEvents))
	}
	searchFunctions, err := s.SearchFunctions(ctx, "transfer", 10)
	if err != nil {
		t.Fatalf("SearchFunctions: %v", err)
	}
	if len(searchFunctions) != 2 {
		t.Fatalf("SearchFunctions = %d rows, want one per distinct name", len(searchFunctions))
	}
	if searchFunctions[0].Name != "transfer_from" || searchFunctions[1].Name != "transfer" {
		t.Errorf("search functions = %v", searchFunctionNames(searchFunctions))
	}
	if empty, err := s.SearchEvents(ctx, "", 10); err != nil || len(empty) != 0 {
		t.Errorf("SearchEvents(empty) = %v, %v", empty, err)
	}
	if empty, err := s.SearchFunctions(ctx, "", 10); err != nil || len(empty) != 0 {
		t.Errorf("SearchFunctions(empty) = %v, %v", empty, err)
	}

	// ---- bulk untrack --------------------------------------------------
	if err := s.UpsertContract(ctx, store.Contract{ID: "c-del", Network: "testnet", Status: "active", AddedAt: t1}); err != nil {
		t.Fatalf("UpsertContract(c-del): %v", err)
	}
	if err := s.BatchInsertEvents(ctx, []store.Event{{ID: "ev-del", ContractID: "c-del", Network: "testnet",
		Ledger: 1, LedgerClosedAt: base, TxHash: "tx-del", Type: "contract"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.BatchInsertInvocations(ctx, []store.Invocation{{TxHash: "tx-del", ContractID: "c-del",
		Network: "testnet", Ledger: 1, LedgerClosedAt: base, Status: "SUCCESS"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertStorageEntries(ctx, []store.StorageEntry{{ContractID: "c-del", Network: "testnet",
		KeyXDR: "k-del", Durability: "persistent", LastModifiedLedger: 1, Status: "live"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertSyncState(ctx, store.SyncState{ContractID: "c-del", LastLedger: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertContractHealthScore(ctx, store.ContractHealthScore{ContractID: "c-del", Score: 10}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO performance_baselines
		(contract_id, function_name, avg_cpu, avg_mem, avg_fee, snapshot_date)
		VALUES ('c-del', 'f', 1, 1, 1, CURRENT_DATE)`); err != nil {
		t.Fatalf("seed baseline: %v", err)
	}
	if deleted, err := s.DeleteContracts(ctx, nil); err != nil || deleted != 0 {
		t.Errorf("DeleteContracts(empty) = %d, %v", deleted, err)
	}
	deleted, err := s.DeleteContracts(ctx, []string{"c-del"})
	if err != nil || deleted != 1 {
		t.Fatalf("DeleteContracts = %d, %v", deleted, err)
	}
	if _, err := s.GetContract(ctx, "c-del"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("deleted contract still readable: %v", err)
	}
	var leftover int64
	if err := pool.QueryRow(ctx, `SELECT
		(SELECT COUNT(*) FROM events WHERE contract_id = 'c-del') +
		(SELECT COUNT(*) FROM invocations WHERE contract_id = 'c-del') +
		(SELECT COUNT(*) FROM storage_entries WHERE contract_id = 'c-del') +
		(SELECT COUNT(*) FROM storage_entry_history WHERE contract_id = 'c-del') +
		(SELECT COUNT(*) FROM sync_state WHERE contract_id = 'c-del') +
		(SELECT COUNT(*) FROM contract_health_scores WHERE contract_id = 'c-del') +
		(SELECT COUNT(*) FROM performance_baselines WHERE contract_id = 'c-del')`).Scan(&leftover); err != nil {
		t.Fatal(err)
	}
	if leftover != 0 {
		t.Errorf("bulk delete left %d child rows", leftover)
	}

	// ---- partition helpers --------------------------------------------
	var partitioned int64
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM pg_inherits WHERE inhparent = 'events'::regclass`).
		Scan(&partitioned); err != nil {
		t.Fatal(err)
	}
	if partitioned == 0 {
		t.Log("events is not partitioned in this database; skipping partition creation (pre-existing migration 000003 issue)")
	} else if err := s.CreateMonthlyPartitionIfNotExists(ctx, 2031, 1); err != nil {
		t.Errorf("CreateMonthlyPartitionIfNotExists: %v", err)
	}
}

func labelNames(labels []store.Label) []string {
	out := make([]string, len(labels))
	for i, l := range labels {
		out[i] = l.Label
	}
	return out
}

func contractIDs(contracts []store.Contract) []string {
	out := make([]string, len(contracts))
	for i, c := range contracts {
		out[i] = c.ID
	}
	return out
}

func searchEventIDs(events []store.Event) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = e.ID
	}
	return out
}

func searchFunctionNames(matches []store.FunctionMatch) []string {
	out := make([]string, len(matches))
	for i, m := range matches {
		out[i] = m.Name
	}
	return out
}
