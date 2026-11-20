# Editing these docs

The site is built with [MkDocs](https://www.mkdocs.org/) and the
[Material](https://squidfunk.github.io/mkdocs-material/) theme from the Markdown
in `docs/`, and published on Read the Docs.

## Why this tooling

- The repository already published the Python SDK with MkDocs, Material and
  `mkdocstrings` on Read the Docs. The site grows from that setup, so nothing
  new has to be hosted and the SDK pages keep working.
- Material provides navigation, built-in client-side search and an
  "edit this page" link without custom code.
- Read the Docs builds one copy per version (see [Versions](#versions)),
  so no extra versioning tool or `gh-pages` branch is needed.
- [`neoteroi-mkdocs`](https://www.neoteroi.dev/mkdocs-plugins/web/oad/) renders
  `docs/openapi.yaml` into ordinary Markdown at build time. The API reference is
  therefore generated, never copied, and its endpoints show up in site search
  (a JavaScript viewer such as Redoc would not be indexed).
- [`mkdocs-monorepo-plugin`](https://github.com/backstage/mkdocs-monorepo-plugin)
  includes `packages/python-sdk/mkdocs.yml` as the Python SDK section, so the
  SDK's pages live in one place.

## Preview locally

You need Python 3.11+.

```bash
make docs-install   # creates .venv-docs with the toolchain and the Python SDK
make docs-serve     # live preview on http://127.0.0.1:8000
make docs-build     # strict build into site/, the same check CI runs
```

Without `make`:

```bash
python -m venv .venv-docs
.venv-docs/bin/pip install -r docs/requirements.txt -e packages/python-sdk
.venv-docs/bin/mkdocs serve
```

## Add or change a page

1. Edit or add a Markdown file under `docs/`.
2. Add new pages to `nav` in `mkdocs.yml`, in the section they belong to
   (Getting started, Self-hosting, API, SDKs, CLI, Operations, Contributing).
   A page that is not in `nav` fails the strict build.
3. Link to other pages with relative paths to the `.md` file, for example
   `[Runbook](../runbook.md#1-indexer-lag)`. MkDocs checks these.
4. Link to code and other files outside `docs/` with a full GitHub URL
   (`https://github.com/sorolens/sorolens/blob/main/...`), because the site
   only contains `docs/`.
5. Run `make docs-build` before you push.

Files kept in `docs/` that are not pages (for example `DESIGN.md` and
`dashboards/`) are listed in `exclude_docs` in `mkdocs.yml`.

### Generated pages

Do not edit these by hand:

| Page | Source | Regenerate |
| --- | --- | --- |
| API reference | `docs/openapi.yaml` | automatic on every build; `make openapi` validates the spec |
| Configuration reference (`docs/configuration.md`) | `apps/api/cmd/configdoc` | `make config-docs` |
| Python SDK API | docstrings in `packages/python-sdk/src` | automatic on every build |

## Link checks

- **Internal links and anchors** are checked by `mkdocs build --strict`, which
  turns every broken link, missing anchor and page left out of `nav` into an
  error. It runs in the existing *SDKs and contracts* CI job when a pull
  request touches the docs, and on every Read the Docs build.
- **External links** are checked weekly by the scheduled
  [`docs-links.yml`](https://github.com/sorolens/sorolens/blob/main/.github/workflows/docs-links.yml)
  workflow with [lychee](https://github.com/lycheeverse/lychee). It does not
  run on pull requests, and can be started by hand from the Actions tab.

## Versions

Read the Docs builds `mkdocs.yml` for each active version:

- **latest**: the `main` branch, rebuilt on every push.
- **stable**: the newest release tag.
- **Release tags** (for example `v1.4.0`): activated by a maintainer in the
  Read the Docs project settings, usually by an automation rule that activates
  new `v*` tags.

Readers switch versions from the Read the Docs menu on each page. Each build
sets `site_url` from `READTHEDOCS_CANONICAL_URL`, so canonical links point at
the right version.
