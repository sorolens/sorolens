## sorolens report

Get monthly SLA and uptime report for a contract

### Synopsis

Display the monthly service-level agreement (SLA) and uptime report for a
tracked contract, derived from watchdog health checks and alerts.

Validates the reporting month format (YYYY-MM). When omitted, the current UTC
month is used.

```
sorolens report [contract-id] [flags]
```

### Examples

```
  # View report for the current month
  sorolens report CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA

  # View report for a specific month
  sorolens report CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA --month 2026-02

  # Output in machine-readable JSON format
  sorolens report CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA --month 2026-02 --json
```

### Options

```
      --contract string   Contract ID (alternative to positional argument)
  -h, --help              help for report
      --month string      Reporting month in YYYY-MM format (UTC)
```

### Options inherited from parent commands

```
      --api-key string     API key for authenticating with the Sorolens API
      --api-url string     Sorolens API base URL
      --json               Output machine-readable JSON
      --network string     Stellar network (testnet, mainnet, futurenet, standalone)
      --no-color           Disable color output
      --output string      Output format (table, json, yaml or jsonl)
      --plugin-api-key     When dispatching to a plugin, pass the active profile's API key as SOROLENS_API_KEY (off by default)
      --profile string     Configuration profile name (default: $SOROLENS_PROFILE, then the default profile)
      --query string       jq-style path filter applied to structured output (e.g. '.items[].name')
      --timeout duration   Request timeout (default 10s)
```

### SEE ALSO

* [sorolens](sorolens.md)	 - Sorolens - Soroban smart contract observability CLI
