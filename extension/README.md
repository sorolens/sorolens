# Sorolens browser extension

> **Install from the web app:** open
> [Settings → Browser extension](https://sorolens-web-iota.vercel.app/settings)
> or the landing page's extension section for one-click install links.
>
> **Hand-off:** copy the user ID from Settings and paste it into the extension's
> Options page, or use the `sorolens://configure?user_id=<id>` deep link.

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
│   ├── background/service-worker.js   the only context that calls the API
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

**Firefox (121+)**

1. `about:debugging#/runtime/this-firefox` → **Load Temporary Add-on…**
2. Select `extension/manifest.firefox.json`.

Firefox needs the Firefox manifest because MV3 event pages
(`background.scripts`) are still the portable option there; the only difference
between the two files is the `background` block and
`browser_specific_settings.gecko`.

## Configure

Open the popup → **Settings** (or the extension's Options page):

| Setting          | Where it is stored | Notes                                                                                                                                                 |
| ---------------- | ------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| API key (#131)   | `storage.local`    | Scoped key; `write:contracts` lets the extension register contracts that are not indexed yet. Never synced, never sent to a page script.              |
| User ID          | `storage.sync`     | Sent as `X-User-ID`, the identity the `/watchlist` routes key on. Use the **Copy user ID** button in the web app's Settings, or open the **deep link** `sorolens://configure?user_id=<id>` to hand it off. |
| API base URL     | `storage.sync`     | Defaults to `https://api.sorolens.xyz`. Host access is requested at save time, so self-hosted APIs work.                                              |
| Default network  | `storage.sync`     | `mainnet`, `testnet`, `futurenet` or `standalone`.                                                                                                    |
| Watchlist toggle | `storage.sync`     | Off means "register for indexing only".                                                                                                               |
| Inline buttons   | `storage.sync`     | Off keeps detection (badge + popup) but injects no page UI.                                                                                           |

## User ID hand-off

The extension and the web app share one watchlist by agreeing on the same
`X-User-ID`. The web app exposes this ID in **Settings → Browser extension**:

1. Open [Settings](https://sorolens-web-iota.vercel.app/settings) and find the
   **Browser extension** card.
2. Click **Copy user ID** — this copies the value of `sorolens_user_id` from
   the web app's `localStorage`.
3. Paste it into the extension's Options page under **User ID**, or click the
   **Open extension settings** deep link, which pre-fills the field via the
   `sorolens://configure?user_id=<id>` URL scheme.

The copy button and deep link are covered by `tests/handoff.test.mjs`, so the
web app's Settings page and the extension stay in sync.

The user ID is the only credential the extension needs to share a watchlist
with the web app; it is stored in `storage.sync` and sent as the `X-User-ID`
header on every `/watchlist` request.

The landing page also links to the extension install pages (Chrome Web Store /
AMO once published, or the unpacked `extension/` directory for local builds)
so new users can adopt the extension without leaving the web app.

If you prefer to copy the value manually, open DevTools on the web app and run
`localStorage.getItem('sorolens_user_id')`.

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
node --test "tests/*.test.mjs"   # 50 unit tests (node:test, no dependencies)
node scripts/build.mjs chrome    # dist/chrome/  (load unpacked from there)
node scripts/build.mjs firefox   # dist/firefox/
python3 tools/generate-icons.py  # regenerate src/icons/*.png
```

The web app's Settings page and landing page link to the same install steps
above, and the user ID hand-off is covered by tests in `tests/` (see
`tests/handoff.test.mjs`) so the copy/deep-link flow stays in sync.

Install links live on the landing page and in Settings; the user ID is shown
there with a copy button and a `sorolens://configure?user_id=<id>` deep link.

`npm run check`, `npm test`, `npm run build` wrap the same commands.

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

Store publication is out of scope for this change: the extension is not on the
Chrome Web Store or AMO yet. `scripts/build.mjs` produces the two loadable,
unpacked bundles a store submission would start from.

Install links and the user ID hand-off (copy button + deep link) are surfaced
from the web app's landing page and Settings page.