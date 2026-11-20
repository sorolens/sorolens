# CI/CD Contract Health Assertions with `sorolens-check`

`sorolens-check` is an official GitHub Action that asserts Soroban smart contract storage TTL and composite health scores in deploy and test pipelines.

## Motivation

On Stellar Soroban, contract storage entries have finite TTLs. If entries expire, contract state becomes archived or restored at high cost, potentially breaking dApps or dependent smart contracts. Deploy pipelines should verify that contracts maintain sufficient live ledgers and acceptable health scores before executing new deployments or routine tasks.

## Quickstart

Add this step to your GitHub Actions workflow:

```yaml
name: Contract Health Verification
on:
  schedule:
    - cron: '0 0 * * *' # Daily check
  workflow_dispatch:

jobs:
  assert-contract-health:
    runs-on: ubuntu-latest
    steps:
      - name: Verify Contract TTL and Health
        uses: sorolens/sorolens/.github/actions/sorolens-check@main
        with:
          contract-id: ${{ vars.CONTRACT_ID }}
          api-url: 'https://api.sorolens.xyz'
          api-key: ${{ secrets.SOROLENS_API_KEY }}
          min-ttl-ledgers: 50000 # ~3.5 days of ledgers
          min-health-score: 80
```

## Action Inputs and Outputs

See [.github/actions/sorolens-check/README.md](../.github/actions/sorolens-check/README.md) for full configuration options.
