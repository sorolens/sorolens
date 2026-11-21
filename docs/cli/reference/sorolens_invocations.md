## sorolens invocations

List invocations of a tracked contract

### Synopsis

List the most recent invocations of a contract: transaction hash, ledger,
status, function and resource usage.

Use --status and --fn to filter, --limit to cap the rows, and --json for
machine-readable output. Pass a transaction hash to "sorolens inspect --tx" to
see one invocation in detail.

```
sorolens invocations <contract-id> [flags]
```

### Examples

```
  sorolens invocations CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
  sorolens invocations <contract-id> --status FAILED --fn transfer
  sorolens invocations list <contract-id> --limit 10 --json
```

### Options

```
      --fn string       Filter by function name
  -h, --help            help for invocations
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

* [sorolens](sorolens.md)	 - Sorolens - Soroban smart contract observability CLI
* [sorolens invocations list](sorolens_invocations_list.md)	 - List invocations of a tracked contract (same as "invocations <contract-id>")
