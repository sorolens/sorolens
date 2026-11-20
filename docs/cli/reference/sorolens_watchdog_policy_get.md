## sorolens watchdog policy get

Read the monitoring policy stored on-chain for a contract

### Synopsis

Read the monitoring policy of a monitored contract by simulating the
watchdog contract's read-only get_policy entry point through the soroban CLI.
Nothing is signed or submitted.

Policies are written by the watchdog admin with set_policy (see
docs/watchdog-policy.md); this command is read-only.

```
sorolens watchdog policy get <monitored-contract-id> [flags]
```

### Examples

```
  sorolens watchdog policy get CACX... --watchdog-contract-id CAC2... \
    --source-account GABC...
```

### Options

```
      --dry-run                       Print the soroban CLI invocation without executing it
  -h, --help                          help for get
      --network string                Stellar network (testnet, mainnet, futurenet) (default "testnet")
      --network-passphrase string     Network passphrase (default "Test SDF Network ; September 2015")
      --rpc-url string                Soroban RPC endpoint (default "https://soroban-testnet.stellar.org:443")
      --soroban-bin string            Path to the soroban CLI binary (default "soroban")
      --source-account string         Public account address (G...) or soroban CLI key alias used to simulate the read
      --watchdog-contract-id string   Watchdog contract id (C...); defaults to $SOROLENS_WATCHDOG_CONTRACT_ID, then $WATCHDOG_CONTRACT_ID
```

### Options inherited from parent commands

```
      --api-key string     API key for authenticating with the Sorolens API
      --api-url string     Sorolens API base URL
      --json               Output machine-readable JSON
      --no-color           Disable color output
      --output string      Output format (table, json, yaml or jsonl)
      --plugin-api-key     When dispatching to a plugin, pass the active profile's API key as SOROLENS_API_KEY (off by default)
      --profile string     Configuration profile name (default: $SOROLENS_PROFILE, then the default profile)
      --query string       jq-style path filter applied to structured output (e.g. '.items[].name')
      --timeout duration   Request timeout (default 10s)
```

### SEE ALSO

* [sorolens watchdog policy](sorolens_watchdog_policy.md)	 - Read on-chain monitoring policies
