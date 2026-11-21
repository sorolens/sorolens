## sorolens completion

Generate shell completion scripts

### Synopsis

Generate shell completion scripts for sorolens.

To load completions:

Bash:
  $ source <(sorolens completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ sorolens completion bash > /etc/bash_completion.d/sorolens
  # macOS:
  $ sorolens completion bash > $(brew --prefix)/etc/bash_completion.d/sorolens

Zsh:
  # If shell completion is not already enabled in your environment,
  # enable it first:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ sorolens completion zsh > "${fpath[1]}/_sorolens"

  # Then start a new shell for this to take effect.

Fish:
  $ sorolens completion fish | source

  # To load completions for each session, execute once:
  $ sorolens completion fish > ~/.config/fish/completions/sorolens.fish

PowerShell:
  PS> sorolens completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> sorolens completion powershell > sorolens.ps1
  # and source this file from your PowerShell profile.


```
sorolens completion [bash|zsh|fish|powershell]
```

### Options

```
  -h, --help   help for completion
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
