## sorolens

Sorolens - Soroban smart contract observability CLI

### Synopsis

sorolens is the command-line interface for the Sorolens observability
platform. It lets you track, inspect, and monitor Soroban smart contracts
on the Stellar network.

### Options

```
      --api-key string     API key for authenticating with the Sorolens API
      --api-url string     Sorolens API base URL
  -h, --help               help for sorolens
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

* [sorolens alerts](sorolens_alerts.md)	 - View and manage watchdog alerts
* [sorolens compare](sorolens_compare.md)	 - Compare 2 to 5 contracts side by side
* [sorolens completion](sorolens_completion.md)	 - Generate shell completion scripts
* [sorolens config](sorolens_config.md)	 - Show and manage CLI configuration and profiles
* [sorolens contracts](sorolens_contracts.md)	 - List and inspect tracked contracts
* [sorolens doctor](sorolens_doctor.md)	 - Diagnose configuration, connectivity, authentication, version and clock problems
* [sorolens events](sorolens_events.md)	 - List events emitted by a tracked contract
* [sorolens inspect](sorolens_inspect.md)	 - Show detailed information about a tracked contract or a transaction
* [sorolens invocations](sorolens_invocations.md)	 - List invocations of a tracked contract
* [sorolens label](sorolens_label.md)	 - Register a human-readable account or contract label
* [sorolens plugin](sorolens_plugin.md)	 - Inspect external subcommand plugins
* [sorolens report](sorolens_report.md)	 - Get monthly SLA and uptime report for a contract
* [sorolens snapshot](sorolens_snapshot.md)	 - Show a storage and state snapshot for a contract
* [sorolens track](sorolens_track.md)	 - Register a contract for tracking
* [sorolens ttl](sorolens_ttl.md)	 - Show storage entry TTL health for a tracked contract
* [sorolens tui](sorolens_tui.md)	 - Launch an interactive terminal dashboard for tracked contracts
* [sorolens version](sorolens_version.md)	 - Print the sorolens CLI version
* [sorolens watchdog](sorolens_watchdog.md)	 - Interact with the Sorolens watchdog
