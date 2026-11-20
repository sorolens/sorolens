# sorolens CLI

Command-line client for the Sorolens Soroban smart contract observability
platform: inspect tracked contracts, follow live events, check TTL and
watchdog status, and manage labels without leaving the terminal.

## Install

Homebrew (macOS/Linux):

```sh
brew install sorolens/tap/sorolens
```

Prebuilt binaries for Linux, macOS and Windows are attached to every
[release](https://github.com/sorolens/sorolens/releases), together with
`checksums.txt` and a Software Bill of Materials (`.sbom.json`) per archive.

With Go 1.26 or newer:

```sh
go install github.com/sorolens/sorolens/cli@latest
```

From a checkout:

```sh
cd cli
make build      # writes ./sorolens
```

## Configuration

Configuration is resolved from the environment, then `~/.sorolens.yml`, then
the defaults:

| Setting    | Flag                | Environment variable     | Default                 |
| ---------- | ------------------- | ------------------------ | ----------------------- |
| API URL    | `--api-url`         | `SOROLENS_API_URL`       | `http://localhost:8080` |
| Network    | `--network`         | `SOROLENS_NETWORK`       | `testnet`               |
| Output     | `--output`, `--json`| `SOROLENS_OUTPUT`        | `table`                 |
| Timeout    | `--timeout`         | -                        | `10s`                   |

`SOROLENS_OUTPUT_FORMAT` is accepted as an alias of `SOROLENS_OUTPUT`.

### Profiles and API keys

Named profiles hold an API URL, a network and an API key for different
deployments. Selection order: `--profile`, then `SOROLENS_PROFILE`, then the
default profile in `~/.sorolens.yml`. Within a profile, flags and
`SOROLENS_API_URL` / `SOROLENS_NETWORK` still win over stored values.

```sh
printf '%s' "$KEY" | sorolens config profile add prod \
  --api-url https://api.example.com --network mainnet --api-key-stdin --use
sorolens config profile list
sorolens config profile use prod
sorolens --profile staging contracts list
sorolens config profile remove staging
sorolens config show        # keys are always masked
```

API keys are stored in the OS keyring (macOS Keychain, Windows Credential
Manager, Secret Service) through `github.com/zalando/go-keyring`. When no
keyring is available the key is written to `~/.sorolens-credentials.yml` with
mode `0600` and a warning is printed. Keys are never written to
`~/.sorolens.yml` and never printed. The active profile's key is sent as the
`X-API-Key` / bearer header on API requests.

Existing config files keep working. The first time a profile is written, the
top-level `api_url` and `network` are migrated into a `default` profile.

```sh
sorolens --api-url https://api.sorolens.xyz --network testnet inspect CDLZ...
```

## Commands

| Command                              | Description                                   |
| ------------------------------------ | --------------------------------------------- |
| `sorolens track <contract-id>`       | Start tracking a contract                     |
| `sorolens inspect <contract-id>`     | Contract metadata, stats and recent activity  |
| `sorolens events <contract-id>`      | List contract events                          |
| `sorolens tail --contract <id>`      | Follow events as they stream in                |
| `sorolens status <contract-id>`      | Watchdog status for a contract                 |
| `sorolens watchdog`                  | Watchdog overview and alerts                   |
| `sorolens ttl <contract-id>`         | Entry TTL report                               |
| `sorolens report <contract-id>`      | Monthly SLA and uptime report                  |
| `sorolens compare <id> <id>...`      | Compare 2–5 contracts side by side             |
| `sorolens label <name> <id>`         | Attach a public label to a contract or account |
| `sorolens tui`                       | Interactive terminal UI                        |
| `sorolens version`                   | Print the CLI version                          |

Add `--json` to any command for machine-readable output.

### Output formats and `--query`

`--output` (or `SOROLENS_OUTPUT`) accepts `table` (default), `json`, `yaml` and
`jsonl`; `--json` is shorthand for `--output json`. `jsonl` writes one compact
JSON document per line, element by element for list results.

`--query` applies a jq-style path filter to structured output. Supported
syntax: `.key`, `."quoted key"`, `[N]` (negative counts from the end), `[]`
(iterate) and `|` to chain, for example:

```bash
sorolens contracts list --output jsonl --query '.[] | .id'
sorolens watchdog status CAAA... --output yaml --query '.status'
```

A query implies `--output json` unless a structured format is chosen, and is
rejected with `--output table`. Syntax and runtime errors report the position
in the expression. The filter is implemented in Go; no external program runs.

`compare` renders events, invocations, failure rate, CPU and fee averages,
health scores and a unicode event-volume sparkline for each contract. Pick a
window with `--window 24h|7d|30d`; missing data becomes an em dash instead of
failing the command:

```sh
sorolens compare CAAA... CBBB... --window 7d
sorolens compare CAAA... CBBB... CCCC... --json
```

## Plugins

Executables named `sorolens-<name>` on `PATH` become subcommands
(`sorolens <name>`); `sorolens plugin list` shows them. Built-in commands cannot
be shadowed and API keys are not passed to plugins unless `--plugin-api-key` is
set. See [docs/cli-plugins.md](../docs/cli-plugins.md).

## Releasing

`.goreleaser.yml` defines the release: three GOOS targets, `tar.gz` archives
(`zip` on Windows), `checksums.txt`, one SBOM per archive, and a Homebrew cask
in `sorolens/homebrew-tap`.

The pipeline is [`.github/workflows/cli-release.yml`](../.github/workflows/cli-release.yml).

```sh
git tag cli-v0.1.0
git push origin cli-v0.1.0
```

Pushing a `cli-v*` tag runs GoReleaser and publishes the GitHub release. Pull
requests touching `cli/` run the same configuration in dry-run mode
(`goreleaser check` plus `goreleaser release --snapshot --clean`), which builds,
checksums and writes SBOMs without publishing anything.

Locally, the equivalent dry run is:

```sh
cd cli
make release-dry-run      # goreleaser release --snapshot --clean
```

### Required secrets

| Secret                       | Used for                                                                 |
| ---------------------------- | ------------------------------------------------------------------------ |
| `GITHUB_TOKEN`               | Publishing the GitHub release; supplied by GitHub, no setup needed.      |
| `HOMEBREW_TAP_GITHUB_TOKEN`  | Pushing the generated cask to `sorolens/homebrew-tap`; a personal access token with `contents: write` on that repository. |

Without `HOMEBREW_TAP_GITHUB_TOKEN` the tap update fails, so the secret must be
configured before the first tagged release.

## License

Apache-2.0. See [LICENSE](../LICENSE).
