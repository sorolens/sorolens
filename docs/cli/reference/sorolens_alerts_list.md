## sorolens alerts list

List watchdog alerts

### Synopsis

List watchdog alerts across monitored contracts, optionally filtered by severity.

```
sorolens alerts list [flags]
```

### Examples

```
  # List recent alerts
  sorolens alerts list

  # Filter by severity
  sorolens alerts list --severity critical

  # Limit number of alerts returned
  sorolens alerts list --limit 20

  # Output in machine-readable JSON format
  sorolens alerts list --json
```

### Options

```
  -h, --help              help for list
  -l, --limit int         Maximum number of alerts to fetch (default 50)
  -s, --severity string   Filter alerts by severity (e.g. critical, warning, info)
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

* [sorolens alerts](sorolens_alerts.md)	 - View and manage watchdog alerts
