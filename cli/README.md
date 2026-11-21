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

## Shell completion

`sorolens completion` generates a completion script for your shell:

```sh
sorolens completion bash | source /dev/stdin   # current session only
sorolens completion zsh > "${fpath[1]}/_sorolens"
sorolens completion fish > ~/.config/fish/completions/sorolens.fish
sorolens completion powershell > sorolens.ps1   # then source it from your profile
```

To install it permanently:

```sh
# Bash (Linux)
sorolens completion bash > /etc/bash_completion.d/sorolens
# Bash (macOS, Homebrew)
sorolens completion bash > $(brew --prefix)/etc/bash_completion.d/sorolens
# Zsh (enable completion first if needed: autoload -U compinit; compinit)
sorolens completion zsh > "${fpath[1]}/_sorolens"
# Fish
sorolens completion fish > ~/.config/fish/completions/sorolens.fish
# PowerShell
sorolens completion powershell > sorolens.ps1
```

See `sorolens completion --help` for the full instructions per shell.

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
| `sorolens inspect --tx <hash>`       | One transaction: function, resources, events  |
| `sorolens invocations <contract-id>` | Recent invocations (`--status`, `--fn`)       |
| `sorolens events <contract-id>`      | List contract events                          |
| `sorolens tail --contract <id>`      | Follow events as they stream in                |
| `sorolens status <contract-id>`      | Watchdog status for a contract                 |
| `sorolens watchdog`                  | Watchdog overview and alerts                   |
| `sorolens ttl <contract-id>`         | Entry TTL report                               |
| `sorolens report <contract-id>`      | Monthly SLA and uptime report                  |
| `sorolens compare <id> <id>...`      | Compare 2–5 contracts side by side             |
| `sorolens label <name> <id>`         | Attach a public label to a contract or account |
| `sorolens tui`                       | Interactive terminal UI                        |
| `sorolens doctor`                    | Diagnose config, connectivity, auth and clock  |
| `sorolens version`                   | Print the CLI version                          |

Add `--json` to any command for machine-readable output.

### Diagnostics: `sorolens doctor`

`sorolens doctor` checks the active configuration and the API it points at, and
prints `pass`, `warn` or `fail` for each check with a hint for anything that is
not passing. It exits with status 1 when any check fails, so it can gate a
script. API keys, bearer tokens and URL credentials are redacted from every
output format, so the report is safe to paste into an issue.

| Check          | What it verifies                                                                                   |
| -------------- | -------------------------------------------------------------------------------------------------- |
| `config`       | `~/.sorolens.yml` parses, the active profile exists, the API URL is valid, where the key comes from |
| `reachability` | `GET /health` answers at the API URL                                                               |
| `tls`          | HTTPS is used for a remote API and the certificate is trusted and not about to expire              |
| `auth`         | The API key is accepted and has `read:contracts` (fail) and `read:watchdog` (warn)                 |
| `version`      | `GET /api/version` reports a server version this CLI supports (`>=0.1.0 <1.0.0`)                   |
| `clock`        | Local clock skew against the server's `Date` header (warn at 30s, fail at 5m)                       |

When the API URL is invalid or the API cannot be reached, the checks that need
it report `warn` with "Not checked" rather than being left out, so the list of
checks is always the same.

`sorolens doctor --json` prints a stable document. `schema_version` changes
only when a field or a check is removed or renamed; new fields and checks may
be added without a bump.

```json
{
  "schema_version": 1,
  "ok": false,
  "cli_version": "v0.2.0",
  "api_url": "https://api.sorolens.xyz",
  "profile": "prod",
  "summary": { "pass": 5, "warn": 0, "fail": 1 },
  "checks": [
    { "name": "config", "status": "pass", "message": "Using ~/.sorolens.yml, profile \"prod\", API https://api.sorolens.xyz." },
    { "name": "auth", "status": "fail", "message": "The API key was rejected (HTTP 401): it is unknown, revoked or expired.", "hint": "Create a new key and store it with 'sorolens config profile add <name> --api-key-stdin'." }
  ]
}
```

- `checks` always lists `config`, `reachability`, `tls`, `auth`, `version` and `clock`, in that order.
- `status` is one of `pass`, `warn` or `fail`; `hint` is omitted on `pass`.
- `ok` is `false` exactly when `summary.fail` is greater than zero.
- `profile` is `(none)` when no profile is active.

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
