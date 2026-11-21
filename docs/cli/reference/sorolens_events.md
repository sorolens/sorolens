## sorolens events

List events emitted by a tracked contract

```
sorolens events <contract-id> [flags]
```

### Options

```
      --csv           Output RFC 4180 CSV with header row
      --follow        Poll every 3 seconds and append new events
  -h, --help          help for events
      --limit int     Number of events to fetch per request (default 50)
      --type string   Filter by event type
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
* [sorolens events tail](sorolens_events_tail.md)	 - Stream a contract's events live as they are indexed
