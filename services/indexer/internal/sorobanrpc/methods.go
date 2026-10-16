package sorobanrpc

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"

	"github.com/sorolens/sorolens/services/indexer/internal/poller"
)

// Compile-time checks: the poller reaches these through its interfaces.
var (
	_ poller.RPCClient         = (*Client)(nil)
	_ poller.TransactionLister = (*Client)(nil)
)

// ---- wire types -----------------------------------------------------------

type pagination struct {
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

type eventFilter struct {
	Type        string   `json:"type,omitempty"`
	ContractIDs []string `json:"contractIds,omitempty"`
}

type getEventsParams struct {
	StartLedger uint32        `json:"startLedger,omitempty"`
	EndLedger   uint32        `json:"endLedger,omitempty"`
	Filters     []eventFilter `json:"filters"`
	Pagination  *pagination   `json:"pagination,omitempty"`
}

type wireEvent struct {
	Type                     string   `json:"type"`
	Ledger                   uint32   `json:"ledger"`
	LedgerClosedAt           string   `json:"ledgerClosedAt"`
	ContractID               string   `json:"contractId"`
	ID                       string   `json:"id"`
	InSuccessfulContractCall bool     `json:"inSuccessfulContractCall"`
	TxHash                   string   `json:"txHash"`
	Topic                    []string `json:"topic"`
	Value                    string   `json:"value"`
	TransactionIndex         int      `json:"transactionIndex"`
}

type wireEventsResult struct {
	Events       []wireEvent `json:"events"`
	LatestLedger uint32      `json:"latestLedger"`
	Cursor       string      `json:"cursor"`
}

// flexInt64 decodes a JSON number or a numeric string. Soroban RPC has sent
// timestamps both ways across releases.
type flexInt64 int64

func (f *flexInt64) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid integer %s: %w", b, err)
	}
	*f = flexInt64(n)
	return nil
}

type wireTransaction struct {
	Status           string    `json:"status"`
	Ledger           uint32    `json:"ledger"`
	CreatedAt        flexInt64 `json:"createdAt"`
	ApplicationOrder int       `json:"applicationOrder"`
	ResultXDR        string    `json:"resultXdr"`
}

type wireLedgerEntry struct {
	Key                   string `json:"key"`
	XDR                   string `json:"xdr"`
	LastModifiedLedgerSeq uint32 `json:"lastModifiedLedgerSeq"`
}

type wireLedgerEntriesResult struct {
	Entries      []wireLedgerEntry `json:"entries"`
	LatestLedger uint32            `json:"latestLedger"`
}

type wireLedgerTransaction struct {
	Status      string `json:"status"`
	TxHash      string `json:"txHash"`
	Ledger      uint32 `json:"ledger"`
	EnvelopeXDR string `json:"envelopeXdr"`
}

type wireTransactionsResult struct {
	Transactions []wireLedgerTransaction `json:"transactions"`
	LatestLedger uint32                  `json:"latestLedger"`
	Cursor       string                  `json:"cursor"`
}

// ---- poller.RPCClient -----------------------------------------------------

// GetLatestLedger returns the newest ledger the node has ingested.
func (c *Client) GetLatestLedger(ctx context.Context) (*poller.LatestLedger, error) {
	var res struct {
		Sequence        uint32 `json:"sequence"`
		ProtocolVersion int    `json:"protocolVersion"`
	}
	if err := c.call(ctx, "getLatestLedger", nil, &res); err != nil {
		return nil, err
	}
	return &poller.LatestLedger{Sequence: res.Sequence, ProtocolVersion: res.ProtocolVersion}, nil
}

// GetEvents returns every event in [startLedger, endLedger) matching filters,
// following getEvents pagination until the window is exhausted. endLedger 0
// leaves the range open-ended on the node side.
func (c *Client) GetEvents(ctx context.Context, startLedger, endLedger uint32, filters []poller.EventFilter) (*poller.GetEventsResult, error) {
	wf := make([]eventFilter, 0, len(filters))
	for _, f := range filters {
		wf = append(wf, eventFilter{Type: f.Type, ContractIDs: f.ContractIDs})
	}

	out := &poller.GetEventsResult{}
	params := getEventsParams{
		StartLedger: startLedger,
		EndLedger:   endLedger,
		Filters:     wf,
		Pagination:  &pagination{Limit: eventsPageLimit},
	}
	for page := 0; page < maxEventPages; page++ {
		var res wireEventsResult
		if err := c.call(ctx, "getEvents", params, &res); err != nil {
			return nil, err
		}
		for _, e := range res.Events {
			out.Events = append(out.Events, poller.RPCEvent{
				ID:                       e.ID,
				ContractID:               e.ContractID,
				Ledger:                   e.Ledger,
				LedgerClosedAt:           e.LedgerClosedAt,
				TxHash:                   e.TxHash,
				Type:                     e.Type,
				Topic:                    e.Topic,
				Value:                    e.Value,
				InSuccessfulContractCall: e.InSuccessfulContractCall,
				TransactionIndex:         e.TransactionIndex,
			})
		}
		out.LatestLedger = res.LatestLedger
		out.Cursor = res.Cursor

		// A short page, or no cursor to continue from, ends the window.
		if len(res.Events) < eventsPageLimit || res.Cursor == "" {
			return out, nil
		}
		// Later pages are addressed by cursor alone; the RPC rejects a
		// startLedger alongside one.
		params = getEventsParams{
			Filters:    wf,
			Pagination: &pagination{Cursor: res.Cursor, Limit: eventsPageLimit},
		}
	}
	return nil, fmt.Errorf("soroban rpc getEvents: more than %d pages in [%d,%d); shrink the ledger window", maxEventPages, startLedger, endLedger)
}

// GetTransaction returns the outcome of the transaction with the given hex
// hash. It returns ErrTransactionNotFound when the node does not know it.
func (c *Client) GetTransaction(ctx context.Context, hash string) (*poller.TransactionResult, error) {
	var res wireTransaction
	if err := c.call(ctx, "getTransaction", map[string]string{"hash": hash}, &res); err != nil {
		return nil, err
	}
	if res.Status == "NOT_FOUND" {
		return nil, fmt.Errorf("%w: %s", ErrTransactionNotFound, hash)
	}
	out := &poller.TransactionResult{
		Status:           res.Status,
		Ledger:           res.Ledger,
		ApplicationOrder: res.ApplicationOrder,
		ResultXDR:        res.ResultXDR,
	}
	if res.CreatedAt > 0 {
		out.LedgerClosedAt = time.Unix(int64(res.CreatedAt), 0).UTC()
	}
	return out, nil
}

// GetLedgerEntries fetches the ledger entries for base64 XDR LedgerKeys.
// Archived or missing keys are simply absent from the result.
func (c *Client) GetLedgerEntries(ctx context.Context, keys []string) (*poller.GetLedgerEntriesResult, error) {
	var res wireLedgerEntriesResult
	if err := c.call(ctx, "getLedgerEntries", map[string][]string{"keys": keys}, &res); err != nil {
		return nil, err
	}
	out := &poller.GetLedgerEntriesResult{LatestLedger: res.LatestLedger}
	found := make(map[string]bool, len(res.Entries))
	for _, e := range res.Entries {
		out.Entries = append(out.Entries, poller.LedgerEntry{
			Key:                   e.Key,
			XDR:                   e.XDR,
			LastModifiedLedgerSeq: e.LastModifiedLedgerSeq,
		})
		found[e.Key] = true
	}
	seen := make(map[string]bool, len(keys))
	for _, k := range keys {
		if seen[k] {
			out.DuplicatedKeys = append(out.DuplicatedKeys, k)
			continue
		}
		seen[k] = true
		if !found[k] {
			out.KeysNotFound = append(out.KeysNotFound, k)
		}
	}
	return out, nil
}

// GetTransactions returns one getTransactions page, starting at startLedger
// for the first page or after cursor for later ones.
func (c *Client) GetTransactions(ctx context.Context, startLedger uint32, cursor string, limit int) (*poller.GetTransactionsResult, error) {
	params := struct {
		StartLedger uint32      `json:"startLedger,omitempty"`
		Pagination  *pagination `json:"pagination,omitempty"`
	}{Pagination: &pagination{Cursor: cursor, Limit: limit}}
	if cursor == "" {
		params.StartLedger = startLedger
	}
	var res wireTransactionsResult
	if err := c.call(ctx, "getTransactions", params, &res); err != nil {
		return nil, err
	}
	out := &poller.GetTransactionsResult{LatestLedger: res.LatestLedger, Cursor: res.Cursor}
	for _, t := range res.Transactions {
		out.Transactions = append(out.Transactions, poller.LedgerTransaction{
			Status:      t.Status,
			Ledger:      t.Ledger,
			TxHash:      t.TxHash,
			EnvelopeXDR: t.EnvelopeXDR,
		})
	}
	return out, nil
}

// GetContractWasmHash returns the lowercase hex Wasm hash a contract instance
// currently points at. contractID may be a C... strkey or 64 hex characters.
// It returns "" with no error when the instance is missing, archived, or is a
// Stellar Asset Contract (which has no Wasm).
func (c *Client) GetContractWasmHash(ctx context.Context, contractID string) (string, error) {
	key, err := ContractInstanceLedgerKey(contractID)
	if err != nil {
		return "", err
	}
	res, err := c.GetLedgerEntries(ctx, []string{key})
	if err != nil {
		return "", err
	}
	for _, e := range res.Entries {
		if h, ok := wasmHashFromInstanceEntry(e.XDR); ok {
			return h, nil
		}
	}
	return "", nil
}

// ContractInstanceLedgerKey builds the base64 XDR LedgerKey of a contract's
// persistent instance entry.
func ContractInstanceLedgerKey(contractID string) (string, error) {
	raw, err := decodeContractID(contractID)
	if err != nil {
		return "", err
	}
	var cid xdr.ContractId
	copy(cid[:], raw)
	key := xdr.LedgerKey{
		Type: xdr.LedgerEntryTypeContractData,
		ContractData: &xdr.LedgerKeyContractData{
			Contract: xdr.ScAddress{
				Type:       xdr.ScAddressTypeScAddressTypeContract,
				ContractId: &cid,
			},
			Key:        xdr.ScVal{Type: xdr.ScValTypeScvLedgerKeyContractInstance},
			Durability: xdr.ContractDataDurabilityPersistent,
		},
	}
	return xdr.MarshalBase64(key)
}

func decodeContractID(id string) ([]byte, error) {
	if strings.HasPrefix(id, "C") && len(id) == 56 {
		raw, err := strkey.Decode(strkey.VersionByteContract, id)
		if err != nil {
			return nil, fmt.Errorf("soroban rpc: invalid contract id %q: %w", id, err)
		}
		return raw, nil
	}
	raw, err := hex.DecodeString(id)
	if err != nil || len(raw) != 32 {
		return nil, fmt.Errorf("soroban rpc: contract id %q is neither a C... strkey nor 32 hex bytes", id)
	}
	return raw, nil
}

func wasmHashFromInstanceEntry(entryXDR string) (string, bool) {
	var data xdr.LedgerEntryData
	if err := xdr.SafeUnmarshalBase64(entryXDR, &data); err != nil {
		return "", false
	}
	cd, ok := data.GetContractData()
	if !ok {
		return "", false
	}
	inst, ok := cd.Val.GetInstance()
	if !ok {
		return "", false
	}
	h, ok := inst.Executable.GetWasmHash()
	if !ok {
		return "", false
	}
	return hex.EncodeToString(h[:]), true
}
