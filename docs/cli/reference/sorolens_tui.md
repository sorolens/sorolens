## sorolens tui

Launch an interactive terminal dashboard for tracked contracts

### Synopsis

Launch a full-screen, k9s-style dashboard for the Sorolens API.

The dashboard shows tracked contracts alongside their live event stream,
invocations, watchdog alerts and storage TTL warnings. Data is refreshed on a
timer (default every 5s) and can be refreshed on demand with "r".

Key bindings:
  tab / shift+tab   cycle panes
  1-5               jump to contracts / events / invocations / alerts / storage
  j / k, up / down  move the selection
  enter             drill down into the selected contract (or alert target)
  /                 filter the contract list
  r                 refresh now
  q, ctrl+c          quit

The dashboard talks to the API at --api-url and authenticates with an API key
passed via --api-key or the SOROLENS_API_KEY environment variable.

```
sorolens tui [flags]
```

### Options

```
  -h, --help               help for tui
      --refresh duration   Polling interval for live data (default 5s)
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
