# CLI

`sorolens` inspects tracked contracts, follows live events, checks TTL and
watchdog status, and manages labels from the terminal.

## Install

```sh
brew install sorolens/tap/sorolens
# or, with Go 1.26+
go install github.com/sorolens/sorolens/cli@latest
```

Prebuilt binaries for Linux, macOS and Windows are attached to every
[release](https://github.com/sorolens/sorolens/releases), with
`checksums.txt` and an SBOM per archive. See
[Verifying releases](../verifying-releases.md) to check them.

## Configure

Settings are read from the environment, then `~/.sorolens.yml`, then the defaults below:

| Setting | Flag | Environment variable | Default |
| --- | --- | --- | --- |
| API URL | `--api-url` | `SOROLENS_API_URL` | `http://localhost:8080` |
| Network | `--network` | `SOROLENS_NETWORK` | `testnet` |
| Output | `--output`, `--json` | `SOROLENS_OUTPUT` | `table` |

## Commands

| Command | Description |
| --- | --- |
| `sorolens track <contract-id>` | Start tracking a contract |
| `sorolens inspect <contract-id>` | Contract metadata, stats and recent activity |
| `sorolens events <contract-id>` | List contract events |
| `sorolens tail --contract <id>` | Follow events as they stream in |
| `sorolens status <contract-id>` | Watchdog status for a contract |
| `sorolens ttl <contract-id>` | Entry TTL report |

Add `--json` to any command for machine-readable output. Every command and flag
is documented in the [generated command reference](reference/sorolens.md). The full command list,
profiles and API keys are in the [CLI README](https://github.com/sorolens/sorolens/blob/main/cli/README.md); extending the
CLI is covered in [Plugins](../cli-plugins.md).
