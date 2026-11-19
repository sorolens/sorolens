# Importing history from Hubble

Soroban RPC keeps about 7 days of events, which is all the
[new-contract backfill](indexer.md#new-contract-backfill) can reach. For audits
and long-term trends, the indexer can import a contract's older events from
[Hubble](https://developers.stellar.org/docs/data/analytics/hubble), SDF's
public copy of the Stellar ledger in Google BigQuery.

The importer is **disabled by default**: it runs only when you start the
indexer with `-mode=import`, as a one-off job, and the running poller never
touches BigQuery.

## What it does

1. Reads the contract's network and the oldest ledger its RPC backfill
   covers.
2. Stops the requested range just below that ledger. Those ledgers are
   already indexed from the RPC, so they are never imported twice.
3. Queries Hubble's `history_contract_events` table one batch of ledgers at a
   time (30,000 by default, about two days of mainnet), oldest first.
4. Decodes each row's full `ContractEvent` XDR into the same row live
   indexing stores: base64 XDR topics and value, the event type, the
   transaction hash and the close time. Event IDs use the
   getEvents format (`<operation TOID>-<event index>`), and only events from
   successful transactions are imported, as getEvents returns.
5. Writes each batch with the live indexer's idempotent inserts, so
   **re-running an import is safe**: rows already stored are skipped. A
   failed run can be re-run with the same arguments.

Invocations are not imported; Hubble's events are what the dashboards and
alerts read for history.

## Requirements

- A Google Cloud project with the BigQuery API enabled. Queries against the
  public dataset are billed to **your** project.
- Credentials with the *BigQuery Job User* role on that project, found
  through Application Default Credentials: `GOOGLE_APPLICATION_CREDENTIALS`
  pointing at a service account key, workload identity on GKE, or
  `gcloud auth application-default login` on a workstation.
- The contract must already be tracked by Sorolens, and the indexer needs its
  usual `DATABASE_URL` and a Soroban RPC URL for the contract's network (the
  newest ledger anchors the close-time estimates below).
- Hubble publishes **mainnet** history only. For another network, point
  `-hubble-project` and `-hubble-dataset` at a dataset with the same table.

## Running an import

```bash
export DATABASE_URL=postgres://...
export SOROBAN_RPC_URL_MAINNET=https://your-rpc-provider.example
export HUBBLE_BILLING_PROJECT=my-gcp-project
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account.json

sorolens-indexer -mode=import \
  -import-contract CCLV77FYLRJMBGTILTKVIM76SI6JY56H7KTT47HE26UF4F265UYRDSR4 \
  -import-from 51000000 \
  -import-to 59000000
```

From the container image, pass the same flags to
`ghcr.io/sorolens/sorolens-indexer` and mount the credentials file. In
Kubernetes, run it as a one-off Job with the indexer's environment.

It logs each batch and prints a summary when it finishes:

```json
{"contract_id":"CCLV…","network":"mainnet","from_ledger":51000000,"to_ledger":58912345,"clipped":true,"batches":264,"events":18734,"parked":0,"skipped":false}
```

`clipped` means the range reached the ledgers the RPC backfill covers and
stopped below them; `skipped` means nothing was left to import. `parked`
counts events the store could not insert and moved to the dead-letter queue,
as live indexing does.

| Flag | Environment | Default | Description |
| --- | --- | --- | --- |
| `-import-contract` | | | Tracked contract to import. Required. |
| `-import-from`, `-import-to` | | | Inclusive ledger range. Required. |
| `-import-batch` | | `30000` | Ledgers per BigQuery query. |
| `-hubble-billing-project` | `HUBBLE_BILLING_PROJECT` | | Project queries are billed to. Required. |
| `-hubble-project` | `HUBBLE_PROJECT` | `crypto-stellar` | Project holding the events dataset. |
| `-hubble-dataset` | `HUBBLE_DATASET` | `crypto_stellar` | Dataset holding `history_contract_events`. |
| `-hubble-max-bytes-billed` | `HUBBLE_MAX_BYTES_BILLED` | `10737418240` (10 GiB) | Most one query may scan. BigQuery refuses a larger query instead of running it. |

## Cost and limits

BigQuery bills by bytes scanned (see
[pricing](https://cloud.google.com/bigquery/pricing); the first TiB a month is
free). `history_contract_events` is partitioned by close time, and each batch
only scans the days its ledgers closed in:

- The importer estimates a batch's close times from a known ledger, assuming
  3 to 8 seconds per ledger, and widens the window by an hour each side. The
  ledger filter, not the time window, decides which rows match.
- The first batch is anchored on the RPC's newest ledger, so its window is
  the widest. Every later batch is anchored on the newest event found so far
  and scans a few days. Importing a contract with no events for a long stretch
  keeps the window wide until it finds one; lower `-import-batch` if a query
  hits the byte limit.
- `-hubble-max-bytes-billed` caps every query. A query over it fails with
  BigQuery's "exceeded limit for bytes billed" error and the import stops;
  nothing is billed for it.

Before a large import, try a short range first and check the bytes billed in
the BigQuery console's job history.

## Troubleshooting

| Error | Meaning |
| --- | --- |
| `contract is not tracked` | Add the contract in Sorolens first. |
| `a billing project is required` | Set `HUBBLE_BILLING_PROJECT`. |
| `no Google Cloud credentials` | Application Default Credentials found nothing; see Requirements. |
| `the public dataset only holds mainnet history` | Set `-hubble-dataset` for this network. |
| `exceeded limit for bytes billed` | Lower `-import-batch`, or raise the cap knowingly. |
| `range is already covered by the RPC backfill` (log) | The whole range is newer than the backfill's oldest ledger. |
