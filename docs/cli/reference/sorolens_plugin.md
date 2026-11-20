## sorolens plugin

Inspect external subcommand plugins

### Synopsis

External subcommands are executables named sorolens-<name> on PATH. Running
'sorolens <name> [args...]' dispatches to them when <name> is not a built-in
command. See docs/cli-plugins.md for the environment contract.

### Options

```
  -h, --help   help for plugin
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
* [sorolens plugin list](sorolens_plugin_list.md)	 - List plugins found on PATH (nothing is executed)
