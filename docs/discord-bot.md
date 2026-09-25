# Discord bot

The Sorolens Discord bot (`services/discord-bot/`) has two jobs: it links a
Discord member to their GitHub account so contributor roles can be granted
automatically, and it exposes Sorolens contract data as slash commands so
people can query a contract without leaving Discord.

This page documents the slash commands and how they authenticate.

## Commands

### Contributor linking

| Command | Purpose |
|---|---|
| `/connect` | One-click GitHub OAuth linking. Replies with a personal, HMAC-signed link that expires after 10 minutes. |
| `/link github <username>` | Manual fallback that skips OAuth. |
| `/unlink` | Removes the GitHub link **and** revokes the member's Sorolens API key. |
| `/whoami` | Shows the GitHub account linked to your Discord id. |
| `/mypr` | Shows the merged-PR count in `sorolens/sorolens` and the current role tier. |

### Contract queries

These commands call the Sorolens HTTP API (`apps/api`). All four require a
linked GitHub account so the bot can attribute the request to a real person.

| Command | API call | Reply |
|---|---|---|
| `/status <contract>` | `GET /api/v1/contracts/{id}` | Embed with the contract's status, network, label and wasm hash. |
| `/alerts <contract> [limit]` | `GET /api/v1/watchdog/contracts/{id}/alerts` | Embed listing the most recent watchdog alerts (1–10, default 5). |
| `/watch <contract>` | `POST /api/v1/watchlist` | Confirms the contract is on your watchlist. |
| `/unwatch <contract>` | `DELETE /api/v1/watchlist/{contractId}` | Confirms removal. |

`<contract>` must be a Stellar contract id: 56 characters, starting with `C`.
The bot validates the shape before spending an API call and replies
ephemerally with an explanation when it does not match.

Every reply is **ephemeral** (`flags: 64`), so contract queries never clutter
the channel.

## Identity: the per-user API key

The issue this feature was built against asks for calls to run "under the
user's identity". That is implemented with a per-user Sorolens API key:

1. When a member completes `/connect` (or `/link`), the bot calls
   `POST /api/v1/api-keys` with its own admin-scoped key and a name like
   `discord:<discordId>:<githubLogin>`, requesting the scopes
   `read:contracts`, `read:watchdog` and `write:contracts`.
2. The plaintext key is returned exactly once. The bot stores it in its SQLite
   database (`user_api_keys` table) against the Discord id. It is never logged
   and never sent back to Discord.
3. Query commands read that key and send it as
   `Authorization: Bearer sl_...`, plus `X-User-ID: <discordId>` which is how
   the watchlist endpoints key their rows.
4. `/unlink` calls `DELETE /api/v1/api-keys/{id}` to revoke the key and drops
   the local row, so a re-linked account can never inherit the previous
   owner's credential.

Provisioning is **best effort**. If `SOROLENS_API_KEY` is unset, or the API is
briefly unavailable when the member links, the link still succeeds and the
commands fall back to the bot's shared (or no) key — the public read surface
is still reachable. Members can re-run `/connect` to pick up a personal key
later.

## Rate limiting

Each Discord user gets a sliding 60-second window, configured by
`COMMAND_RATE_LIMIT_PER_MINUTE` (default 10). Exceeding it produces an
ephemeral "try again in N seconds" reply instead of an API call, so one member
cannot exhaust the upstream quota for everyone.

## Configuration

Add these to the bot's environment (see `.env.example`):

| Variable | Required | Meaning |
|---|---|---|
| `SOROLENS_API_BASE_URL` | no (default `http://localhost:8080`) | Base URL of the Sorolens API. |
| `SOROLENS_API_KEY` | no | Admin-scoped (`admin:*`) key used to mint per-user keys. Without it, commands run on the public read surface. |
| `COMMAND_RATE_LIMIT_PER_MINUTE` | no (default `10`) | Per-user command budget per minute. |

After deploying, re-run the registration script so Discord learns the new
commands:

```bash
cd services/discord-bot
npm run register
```

## Failure modes

| Situation | Behaviour |
|---|---|
| Contract not tracked by Sorolens | Ephemeral "not tracked" message (API `404`). |
| API key rejected or missing scope | Ephemeral prompt to re-run `/connect` (API `401`/`403`). |
| Sorolens API unreachable | Ephemeral "try again shortly"; the interaction is still acknowledged so Discord does not show a failed command. |
| Member over the rate limit | Ephemeral retry message; no API call is made. |
| Command run without a linked account | Ephemeral prompt to run `/connect` first. |

## Tests

```bash
cd services/discord-bot
npm test
```

`api.test.ts` covers request building, credential precedence and error
mapping against a fake `fetch`; `apikey.test.ts` covers provisioning and
revocation; `ratelimit.test.ts` covers the window arithmetic; `embeds.test.ts`
asserts the rendered embed JSON; `db.test.ts` covers the key store and the
`/unlink` cleanup.
