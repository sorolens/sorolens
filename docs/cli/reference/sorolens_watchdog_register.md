## sorolens watchdog register

Register a contract for monitoring via the watchdog contract

### Synopsis

Register a contract for monitoring by invoking the watchdog contract's
register_contract entry point on-chain.

<monitored-contract-id> is the 'C...' address of the contract to monitor.
--name is stored as a Soroban Symbol (letters, digits and underscores, at
most 9 characters) and --interval is the expected health-check cadence
(e.g. 300s, 5m, 1h).

Credentials, in decreasing precedence:
  1. --secret-key or --source-account flag
  2. SOROLENS_WATCHDOG_SECRET environment variable
  3. a named account configured in the soroban CLI (--soroban-account)

The watchdog contract id comes from --watchdog-contract-id, the
SOROLENS_WATCHDOG_CONTRACT_ID environment variable, or the repository's
WATCHDOG_CONTRACT_ID.

Use --dry-run to print the soroban CLI invocation without executing it.

```
sorolens watchdog register <monitored-contract-id> [flags]
```

### Examples

```
  # Preview the invocation without sending it
  sorolens watchdog register CACX... --name my_app --interval 300s \
    --watchdog-contract-id CAC2... --dry-run

  # Register on testnet using a secret key
  sorolens watchdog register CACX... --name my_app --interval 300s \
    --watchdog-contract-id CAC2... --source-account SABC...

  # Use a soroban CLI key alias instead of a raw secret
  sorolens watchdog register CACX... --name my_app --interval 300s \
    --watchdog-contract-id CAC2... --soroban-account deployer
```

### Options

```
      --dry-run                       Print the soroban CLI invocation without executing it
  -h, --help                          help for register
      --interval duration             Expected health-check interval (e.g. 300s, 5m, 1h; must be > 0)
      --name string                   Registration name stored on-chain as a Soroban Symbol (letters, digits, underscores; max 9 chars)
      --network string                Stellar network (testnet, mainnet, futurenet) (default "testnet")
      --network-passphrase string     Network passphrase (default "Test SDF Network ; September 2015")
      --rpc-url string                Soroban RPC endpoint (default "https://soroban-testnet.stellar.org:443")
      --secret-key string             Stellar secret seed (S...); falls back to $SOROLENS_WATCHDOG_SECRET
      --simulate                      Simulate the invocation instead of submitting it (no transaction is applied)
      --soroban-account string        Named source account already configured in the soroban CLI (soroban keys add)
      --soroban-bin string            Path to the soroban CLI binary (default "soroban")
      --source-account string         Stellar secret seed or account address used to sign (alias of --secret-key)
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

* [sorolens watchdog](sorolens_watchdog.md)	 - Interact with the Sorolens watchdog
