## sorolens compare

Compare 2 to 5 contracts side by side

### Synopsis

Render events, invocations, failure rate, average CPU and fees, health
score and hourly event-volume sparklines for 2 to 5 contracts side by side.

A contract with no indexed data yet still renders a column: its cells show an
em dash instead of failing the whole comparison. Only request errors (a bad
contract ID, an unreachable API) exit non-zero.

```
sorolens compare <contract-id>... [flags]
```

### Examples

```
  # Compare two contracts over the last 24 hours
  sorolens compare CAAA... CBBB...

  # Compare five contracts over 7 days
  sorolens compare CAAA... CBBB... CCCC... CDDD... CEEE... --window 7d

  # Machine-readable output for scripts
  sorolens compare CAAA... CBBB... --json
```

### Options

```
  -h, --help            help for compare
      --width int       Table width in columns (0 detects the terminal, minimum 40)
      --window string   Window to compare over (24h, 7d or 30d) (default "24h")
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
