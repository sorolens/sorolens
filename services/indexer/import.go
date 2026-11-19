package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/sorolens/sorolens/services/indexer/internal/history"
	"github.com/sorolens/sorolens/services/indexer/internal/pgstore"
	"github.com/sorolens/sorolens/services/indexer/internal/poller"
)

// importFlags are the settings of -mode=import, which copies a contract's
// events older than the RPC retention window from SDF's Hubble dataset in
// BigQuery. The mode is off unless asked for: the poller never imports.
type importFlags struct {
	contract       string
	from, to       uint
	batch          uint
	billingProject string
	project        string
	dataset        string
	maxBytesBilled int64
}

func registerImportFlags(fs *flag.FlagSet) *importFlags {
	f := &importFlags{}
	fs.StringVar(&f.contract, "import-contract", "", "Import mode: tracked contract (C...) to import history for")
	fs.UintVar(&f.from, "import-from", 0, "Import mode: first ledger to import (inclusive)")
	fs.UintVar(&f.to, "import-to", 0, "Import mode: last ledger to import (inclusive); clipped below the RPC backfill")
	fs.UintVar(&f.batch, "import-batch", uint(history.DefaultBatchLedgers), "Import mode: ledgers per BigQuery query")
	fs.StringVar(&f.billingProject, "hubble-billing-project", envString("HUBBLE_BILLING_PROJECT", ""), "Import mode: Google Cloud project BigQuery bills queries to")
	fs.StringVar(&f.project, "hubble-project", envString("HUBBLE_PROJECT", ""), "Import mode: project of the events dataset (default "+history.DefaultHubbleProject+")")
	fs.StringVar(&f.dataset, "hubble-dataset", envString("HUBBLE_DATASET", ""), "Import mode: events dataset (default "+history.DefaultHubbleDataset+", mainnet only)")
	fs.Int64Var(&f.maxBytesBilled, "hubble-max-bytes-billed", int64(envInt("HUBBLE_MAX_BYTES_BILLED", int(history.DefaultMaxBytesBilled))), "Import mode: largest scan one query may bill; BigQuery refuses a query over it")
	return f
}

func (f *importFlags) request() (history.Request, error) {
	if f.contract == "" || f.from == 0 || f.to == 0 {
		return history.Request{}, errors.New("import mode needs -import-contract, -import-from and -import-to")
	}
	if f.from > uint(^uint32(0)) || f.to > uint(^uint32(0)) {
		return history.Request{}, errors.New("import ledgers must fit in 32 bits")
	}
	return history.Request{ContractID: f.contract, FromLedger: uint32(f.from), ToLedger: uint32(f.to)}, nil
}

func (f *importFlags) hubbleConfig() history.HubbleConfig {
	return history.HubbleConfig{
		BillingProject: f.billingProject,
		Project:        f.project,
		Dataset:        f.dataset,
		MaxBytesBilled: f.maxBytesBilled,
	}
}

// importStore adapts pgstore to the history package.
type importStore struct{ st *pgstore.Store }

func (s importStore) ImportTarget(ctx context.Context, contractID string) (history.Target, error) {
	t, err := s.st.ImportTarget(ctx, contractID)
	if err != nil {
		return history.Target{}, err
	}
	return history.Target{Network: t.Network, RPCFloorLedger: t.RPCFloorLedger}, nil
}

func (s importStore) CommitHistoryBatch(ctx context.Context, events []poller.Event, invocations []poller.Invocation) ([]poller.FailedEvent, error) {
	return s.st.CommitHistoryBatch(ctx, events, invocations)
}

// bigQueryScope is the OAuth2 scope the import's queries need.
const bigQueryScope = "https://www.googleapis.com/auth/bigquery"

// adcTokens adapts Application Default Credentials to history.TokenSource.
type adcTokens struct{ ts oauth2.TokenSource }

func (a adcTokens) Token(context.Context) (string, error) {
	tok, err := a.ts.Token()
	if err != nil {
		return "", err
	}
	return tok.AccessToken, nil
}

// newBigQueryReader builds the REST reader with Application Default
// Credentials: GOOGLE_APPLICATION_CREDENTIALS, workload identity, or
// `gcloud auth application-default login`.
func newBigQueryReader(ctx context.Context, cfg history.HubbleConfig) (history.RowReader, error) {
	creds, err := google.FindDefaultCredentials(ctx, bigQueryScope)
	if err != nil {
		return nil, fmt.Errorf("hubble: no Google Cloud credentials: %w", err)
	}
	return &history.BigQueryReader{
		BillingProject: cfg.BillingProject,
		MaxBytesBilled: cfg.MaxBytesBilled,
		Tokens:         adcTokens{ts: creds.TokenSource},
	}, nil
}

// runImport imports one contract's history and prints a JSON summary.
//
// The newest ledger the RPC reports anchors the close-time estimate the
// BigQuery queries prune partitions with.
func runImport(ctx context.Context, f *importFlags, store history.Store, rpcClients map[string]poller.RPCClient,
	newReader func(context.Context, history.HubbleConfig) (history.RowReader, error),
	out io.Writer, log *slog.Logger) error {
	req, err := f.request()
	if err != nil {
		return err
	}
	target, err := store.ImportTarget(ctx, req.ContractID)
	if err != nil {
		return err
	}
	cfg := f.hubbleConfig()
	if err := cfg.Validate(target.Network); err != nil {
		return err
	}
	rpc, ok := rpcClients[target.Network]
	if !ok {
		// SOROBAN_RPC_URL serves every network without its own endpoint.
		rpc, ok = rpcClients[""]
	}
	if !ok {
		return fmt.Errorf("no Soroban RPC configured for %s, which the import needs to anchor ledger times", networkLabel(target.Network))
	}
	latest, err := rpc.GetLatestLedger(ctx)
	if err != nil {
		return fmt.Errorf("read the latest ledger: %w", err)
	}

	reader, err := newReader(ctx, cfg)
	if err != nil {
		return err
	}

	im := &history.Importer{
		Source:       history.NewHubbleSource(cfg, reader, latest.Sequence, time.Now()),
		Store:        store,
		BatchLedgers: uint32(f.batch),
		Log:          log,
	}
	res, err := im.Import(ctx, req)
	if encodeErr := json.NewEncoder(out).Encode(res); encodeErr != nil && err == nil {
		err = encodeErr
	}
	return err
}
