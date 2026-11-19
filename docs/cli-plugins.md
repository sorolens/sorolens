# CLI plugins

`sorolens <name> [args...]` runs the executable `sorolens-<name>` found on
`PATH` when `<name>` is not a built-in command (git/cargo style).

- Names match `[a-z0-9][a-z0-9_-]*` (max 64 chars). Anything else is never
  dispatched, so a subcommand cannot contain a path separator.
- Built-in commands always win; a plugin named like a built-in is never run.
  `sorolens plugin list` flags it with a warning.
- Arguments are passed as an argument vector to the executable. No shell is
  involved, so arguments cannot be used for injection.
- Discovery (`sorolens plugin list`) only reads directory metadata. It skips
  directories and non-executable files (on Windows it accepts `.exe`, `.bat`,
  `.cmd` and `.com`) and never runs a plugin.
- A missing plugin gives `unknown command "<name>": no built-in command or
  plugin "sorolens-<name>" found on PATH`. The plugin's exit code is returned
  as the exit code of `sorolens`.
- Global flags (`--profile`, `--api-url`, ...) must come before the plugin
  name; everything after the name belongs to the plugin.

## Environment contract

| Variable                      | Value                                                         |
| ----------------------------- | ------------------------------------------------------------- |
| `SOROLENS_PLUGIN_API_VERSION` | `1` (bumped on incompatible changes)                          |
| `SOROLENS_API_URL`            | resolved API base URL                                         |
| `SOROLENS_NETWORK`            | resolved network                                              |
| `SOROLENS_OUTPUT`             | resolved output format (`table`, `json`, `yaml`, `jsonl`)     |
| `SOROLENS_PROFILE`            | active profile name (only set when a profile is active)       |
| `SOROLENS_API_KEY`            | **only** with `--plugin-api-key`; otherwise removed from env  |

API keys are not exposed by default: any inherited `SOROLENS_API_KEY` is
removed from the plugin's environment unless you pass `--plugin-api-key`, which
passes the active profile's key.

## Writing a plugin

```sh
#!/bin/sh
# ~/bin/sorolens-hello  (chmod +x)
echo "Hello from $SOROLENS_NETWORK via $SOROLENS_API_URL: $*"
```

```console
$ sorolens --network mainnet hello world
Hello from mainnet via http://localhost:8080: world
```
