## sorolens inspect

Show detailed information about a tracked contract or a transaction

### Synopsis

Show detailed information about a tracked contract, or with --tx about a
single transaction: its function, status, resource usage and emitted events.

With --tx the contract is looked up from the transaction hash. Pass a contract
ID as well to skip that lookup.

```
sorolens inspect <contract-id> | inspect --tx <hash> [contract-id] [flags]
```

### Examples

```
  sorolens inspect CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
  sorolens inspect --tx <64-hex-hash>
  sorolens inspect --tx <64-hex-hash> <contract-id> --json
```

### Options

```
  -h, --help        help for inspect
      --tx string   Inspect a transaction by its hash instead of a contract
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
