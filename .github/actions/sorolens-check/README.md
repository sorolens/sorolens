# sorolens-check GitHub Action

Assert Soroban smart contract storage TTL and health thresholds in CI/CD pipelines.

`sorolens-check` integrates with the Sorolens API to monitor contracts and fail pipeline runs when:
- Any storage entry's remaining TTL is below a minimum ledger threshold.
- The contract composite health score is below a minimum threshold.

## Usage

```yaml
- name: Check Soroban Contract Health & TTL
  uses: sorolens/sorolens/.github/actions/sorolens-check@main
  with:
    contract-id: 'CAVRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5'
    api-url: 'https://api.sorolens.com'
    api-key: ${{ secrets.SOROLENS_API_KEY }}
    min-ttl-ledgers: 50000
    min-health-score: 80
```

## Inputs

| Input | Required | Default | Description |
|---|---|---|---|
| `contract-id` | **Yes** | - | Soroban contract address (`C...`) |
| `api-url` | No | `https://api.sorolens.com` | Sorolens API endpoint |
| `api-key` | No | - | API key for authenticated API instances |
| `min-ttl-ledgers` | No | `0` | Minimum ledgers remaining before storage expiry; fails if any entry is below |
| `min-health-score` | No | `0` | Minimum 0-100 composite health score; fails if below |
| `current-ledger` | No | - | Reference ledger sequence (defaults to API last synced ledger) |

## Outputs

| Output | Description |
|---|---|
| `status` | `passed` or `failed` |
| `health-score` | Contract composite health score (0-100) |
| `min-ttl` | Lowest remaining TTL in ledgers across all storage entries |
| `storage-count` | Number of inspected storage entries |
