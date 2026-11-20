## sorolens events tail

Stream a contract's events live as they are indexed

### Synopsis

Tail a contract's events using the Sorolens server-sent events endpoint.

Unlike "sorolens events <contract-id>", which returns a page of already-indexed
events, tail keeps the connection open and pretty-prints each event as it
arrives. Press Ctrl-C to close the stream.

```
sorolens events tail --contract <contract-id> [flags]
```

### Options

```
      --contract string   Contract ID to stream events for (required)
  -h, --help              help for tail
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

* [sorolens events](sorolens_events.md)	 - List events emitted by a tracked contract
