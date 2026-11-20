## sorolens contracts list

List tracked contracts

### Synopsis

List the contracts tracked by the Sorolens API on the selected network.

Use --network to choose the network (default: testnet), --limit to cap the
number of rows, and --json for machine-readable output.

```
sorolens contracts list [flags]
```

### Examples

```
  sorolens contracts list
  sorolens contracts list --network mainnet --limit 10
  sorolens contracts list --json
```

### Options

```
  -h, --help        help for list
      --limit int   Maximum number of contracts to list (default 50)
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

* [sorolens contracts](sorolens_contracts.md)	 - List and inspect tracked contracts
