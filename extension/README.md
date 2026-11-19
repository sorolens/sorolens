# Sorolens browser extension

A Manifest V3 browser extension that puts a **Track on Sorolens** button next to
every Soroban contract ID on Stellar Expert, Stellar Lab and stellar.org, and
tracks contracts from the toolbar popup without copy-pasting into the dashboard.

One codebase, two targets: Chrome (MV3 service worker) and Firefox (MV3 event
page).

```
extension/
├── manifest.json                 Chrome MV3 manifest (load-unpacked entry point)
├── manifest.firefox.json         Firefox MV3 manifest (same code, gecko settings)
├── package.json                  scripts + metadata only, zero dependencies
├── scripts/
│   ├── build.mjs                 copies src/ + the right manifest into dist/<target>
│   ├── package.mjs               builds both targets and emits store-ready zips
│   └── check.mjs                 node --check every module + validate both manifests
├── tools/
│   └── generate-icons.py         renders src/icons/*.png from the web app's logo geometry
├── src/
│   ├── shared/                   modules shared by every context (no chrome.* access)
│   │   ├── api.js                Sorolens REST client (fetch, no SDK runtime)
│   │   ├── browser.js            thin browser.*/chrome.* promise shim
│   │   ├── constants.js          API base URL, networks, storage keys, tuning
│   │   ├── contract-id.js        StrKey detection/validation
│   │   ├── messages.js           message-type contract between contexts
│   │   └── storage.js            settings + tracking-status cache
│   ├── background/
│   │   ├── alerts.js             alert polling, de-duplication and backoff (pure)
│   │   └── service-worker.js     the only context that calls the API
│   ├── content/                  loader.js (classic) -> content.js (ES module) + content.css
│   ├── popup/                    popup.html / popup.js / popup.css
│   ├── options/                  options.html / options.js / options.css
│   └── icons/                    generated PNG icons
└── tests/                        node:test suites (fixtures.mjs + 4 test files)
```

## Install (no build step)

The extension loads straight out of `extension/`; there is no bundler and no
`npm install` step.

**Chrome / Chromium (110+)**

1. `chrome://extensions` → enable **Developer mode**.
2. **Load unpacked** → select the `extension/` directory.

**Firefox (142+)**

1. `about:debugging#/runtime/this-firefox` → **Load Temporary Add-on…**
2. Select `extension/manifest.firefox.json`.

Firefox needs the Firefox manifest because MV3 event pages
(`background.scripts`) are still the portable option there; the only difference
between the two files is the `background` block and
`browser_specific_settings.gecko`. The 142 minimum is what AMO requires for the
`optional_host_permissions` and `data_collection_permissions` keys the manifest
uses.

## Configure

Open the popup → **Settings** (or the extension's Options page):

| Setting          | Where it is stored | Notes                                                                                                                                                 |
| ---------------- | ------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| API key (#131)   | `storage.local`    | Scoped key; `write:contracts` lets the extension register contracts that are not indexed yet. Never synced, never sent to a page script.              |
| User ID          | `storage.sync`     | Sent as `X-User-ID`, the identity the `/watchlist` routes key on. Copy `sorolens_user_id` from the dashboard's `localStorage` to share one watchlist. |
| API base URL     | `storage.sync`     | Defaults to `https://api.sorolens.xyz`. Host access is requested at save time, so self-hosted APIs work.                                              |
| Default network  | `storage.sync`     | `mainnet`, `testnet`, `futurenet` or `standalone`.                                                                                                    |
| Watchlist toggle | `storage.sync`     | Off means "register for indexing only".                                                                                                               |
| Inline buttons   | `storage.sync`     | Off keeps detection (badge + popup) but injects no page UI.                                                                                           |
| Notifications    | `storage.sync`     | On by default. Polls watched contracts every 5 minutes, shows an unread badge and raises one notification per new alert. Choose Critical/Warning/Info. |

## What the button does

1. `GET /api/v1/contracts/{id}` — already indexed?
2. If not, `POST /api/v1/contracts` with `{ id, network }` (needs
   `write:contracts` + contributor role; a failure is surfaced as a warning and
   does not block step 3).
3. `POST /api/v1/watchlist` with `{ contract_id }` — adds it to
   `X-User-ID`'s watchlist.
4. `GET /api/v1/watchlist/{id}/status` re-reads the resulting state.

Clicking a **Tracked** button reverses step 3 with
`DELETE /api/v1/watchlist/{id}`. All endpoints come from `docs/openapi.yaml` and
`apps/api/internal/router/router.go`.

## Develop

```bash
cd extension

node scripts/check.mjs      # node --check on every module + manifest validation
node --test "tests/*.test.mjs"   # unit tests (node:test, no dependencies)
node scripts/build.mjs chrome    # dist/chrome/  (load unpacked from there)
node scripts/build.mjs firefox   # dist/firefox/
node scripts/package.mjs         # both zips (needs the `zip` command)
python3 tools/generate-icons.py  # regenerate src/icons/*.png
```

`npm run check`, `npm test`, `npm run build` and `npm run package` wrap the same
commands. `npm run package:chrome` / `npm run package:firefox` build one target.

### Testing both browsers

1. Run `node scripts/build.mjs chrome` and load `dist/chrome/` as an unpacked
extension in `chrome://extensions`.
2. Run `node scripts/build.mjs firefox` and load `dist/firefox/manifest.json`
as a temporary add-on in `about:debugging#/runtime/this-firefox`.
3. Open the options page in both, save an API key and watchlist identity, then
add a contract to the watchlist. The next alarm tick (up to five minutes) shows
the badge and, for a new alert, a notification that opens the contract page.
4. `npx web-ext lint --source-dir dist/firefox` reports no errors. One warning
is expected: the content script dynamically imports its module through
`runtime.getURL`, which is the documented way to load a web-accessible module
from a classic content script.

## Package for the stores

`node scripts/package.mjs` writes `dist/sorolens-<version>-chrome.zip` and
`dist/sorolens-<version>-firefox.zip`, each with `manifest.json` at the archive
root. Versions come from the two manifests, and packaging refuses to run if
they drifted apart. The `Extension Release` workflow does the same on an
`extension-v*` tag (or a manual run), lints the Firefox build, uploads the zips
as artifacts and attaches them to the GitHub release. It does not run on pull
requests.

Store submission stays manual; no credentials are stored in the repository:

- **Chrome Web Store:** upload the `-chrome.zip`, fill in the listing and
  privacy disclosures, and submit for review.
- **Firefox AMO:** upload the `-firefox.zip` (the gecko id is
  `extension@sorolens.xyz`), complete the data-collection and privacy forms, and
  submit for review.

Notes on the design:

- The **content script does no network I/O**. It detects IDs, injects buttons
  and forwards clicks to the service worker, so the API key stays out of the
  page context and the same client code serves the popup.
- Buttons are inserted _after_ the matched text node or link instead of
  rewriting text nodes, so React-based explorers can re-render freely. The
  rescan MutationObserver ignores its own mutations via the
  `data-sorolens-ui` attribute.
- `src/shared/browser.js` is the only module that touches `chrome`/`browser`,
  and it accepts both the callback (Chrome) and promise (Firefox) shapes.
- The service worker is registered as a module, so `src/shared/*.js` are
  imported natively; the content script dynamic-imports `content.js` through
  `web_accessible_resources`.

## Not done here

The extension is not published to the Chrome Web Store or AMO yet. Packaging is
reproducible (`scripts/package.mjs`) and the release workflow attaches the
versioned zips, but uploading them remains a manual, credential-holding step.
