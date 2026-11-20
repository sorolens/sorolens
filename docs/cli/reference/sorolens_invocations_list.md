## sorolens invocations list

List invocations of a tracked contract (same as "invocations <contract-id>")

```
sorolens invocations list <contract-id> [flags]
```

### Options

```
      --fn string       Filter by function name
  -h, --help            help for list
      --limit int       Maximum number of invocations to list (default 50)
      --status string   Filter by status (SUCCESS, FAILED, NOT_FOUND)
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

* [sorolens invocations](sorolens_invocations.md)	 - List invocations of a tracked contract
