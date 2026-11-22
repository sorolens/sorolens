## sorolens config profile add

Create or update a profile

### Synopsis

Create or update a named profile. The API key is read from standard input
with --api-key-stdin so it never appears in shell history or the process list,
and is stored in the OS keyring (a 0600 file with a warning when no keyring
is available).

```
sorolens config profile add <name> [flags]
```

### Examples

```
  printf '%s' "$KEY" | sorolens config profile add prod --api-url https://api.example.com --network mainnet --api-key-stdin --use
```

### Options

```
      --api-key-stdin    Read the API key from standard input
      --api-url string   Sorolens API base URL for this profile
  -h, --help             help for add
      --network string   Stellar network for this profile
      --use              Make this the default profile
```

### Options inherited from parent commands

```
      --api-key string     API key for authenticating with the Sorolens API
      --json               Output machine-readable JSON
      --no-color           Disable color output
      --output string      Output format (table, json, yaml or jsonl)
      --plugin-api-key     When dispatching to a plugin, pass the active profile's API key as SOROLENS_API_KEY (off by default)
      --profile string     Configuration profile name (default: $SOROLENS_PROFILE, then the default profile)
      --query string       jq-style path filter applied to structured output (e.g. '.items[].name')
      --timeout duration   Request timeout (default 10s)
```

### SEE ALSO

* [sorolens config profile](sorolens_config_profile.md)	 - Manage named profiles (API URL, network and API key)
