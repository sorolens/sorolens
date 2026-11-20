## sorolens alerts

View and manage watchdog alerts

### Synopsis

alerts commands allow you to query and inspect alerts emitted by the
Sorolens watchdog system for monitored contracts.

### Examples

```
  # List all recent alerts
  sorolens alerts list

  # Filter alerts by severity
  sorolens alerts list --severity critical

  # Output in machine-readable JSON format
  sorolens alerts list --json
```

### Options

```
  -h, --help   help for alerts
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
* [sorolens alerts list](sorolens_alerts_list.md)	 - List watchdog alerts
