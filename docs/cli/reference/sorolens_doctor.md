## sorolens doctor

Diagnose configuration, connectivity, authentication, version and clock problems

### Synopsis

Run a set of checks against the active configuration and the Sorolens API and
print pass, warn or fail for each, with a hint on how to fix anything that is
not passing:

  config        config file, active profile, API URL and where the API key comes from
  reachability  the API answers at --api-url
  tls           HTTPS is used and the server certificate is valid
  auth          the API key is accepted and has the read scopes the CLI needs
  version       the server version is in the range this CLI supports
  clock         local clock skew against the server's Date header

API keys and tokens never appear in the output, so the report is safe to
paste into an issue. Use --json for a stable machine-readable report. The
command exits with status 1 when any check fails.

```
sorolens doctor [flags]
```

### Examples

```
  sorolens doctor
  sorolens doctor --profile prod
  sorolens doctor --json
```

### Options

```
  -h, --help   help for doctor
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
