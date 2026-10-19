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
| `sorolens label <name> <id>`         | Attach a public label to a contract or account |
| `sorolens tui`                       | Interactive terminal UI                        |
| `sorolens version`                   | Print the CLI version                          |

Add `--json` to any command for machine-readable output.

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
