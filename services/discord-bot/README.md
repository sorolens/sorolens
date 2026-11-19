# @sorolens/discord-bot

Contributor bot for the [Sorolens Discord](https://discord.gg/D9jATUezYX). Two responsibilities:

1. Lets contributors link their Discord identity to their GitHub username via a slash command.
2. Watches merged PRs on `sorolens/sorolens` and auto-grants **Contributor** or **Core Contributor** roles based on merge count.

Runs as a single Node.js process. Deploys to Render's free tier (Docker) with an UptimeRobot ping to prevent sleep. Any other container platform works too - only the deploy instructions differ.

## Slash commands

| Command                      | What it does                                                            |
| ---------------------------- | ----------------------------------------------------------------------- |
| `/connect`                   | **Recommended.** Auto-links your GitHub via one-click OAuth. No typing. |
| `/link github <username>`    | Manual fallback if you cannot use OAuth.                                |
| `/unlink`                    | Remove the GitHub link on your Discord account.                         |
| `/whoami`                    | Show your current linked GitHub account, if any.                        |
| `/mypr`                      | Show your merged PR count and current tier.                             |
| `/status <contract>`         | Show the current Sorolens status of a tracked contract.                 |
| `/alerts <contract> [limit]` | Show the latest watchdog alerts for a contract.                         |
| `/watch <contract>`          | Add a contract to your Sorolens watchlist.                              |
| `/unwatch <contract>`        | Remove a contract from your watchlist.                                  |

See [`docs/discord-bot.md`](../../docs/discord-bot.md) for the API-backed query commands in detail.

New members are also DM'd a personal `/connect` link automatically when they join, so most contributors never need to type anything.

## Issue self-assign (Drips Wave)

Members drop issue numbers like `#812 #840` in one channel and the bot
assigns them on GitHub using `GITHUB_TOKEN` (so the token's account shows
as the assigner, which Drips Wave treats as accepting the application).

Owner commands (hidden from non-admins):

| Command | What it does |
| --- | --- |
| `/assign-setup` | Set the channel, `required_role` (must have), `any_role_1..3` (must have one of), and `max` issues per person |
| `/assign-issues add\|remove issues:836-850, 812` | Choose which issues can be picked (`clear`, `list` too) |
| `/assign-toggle on\|off` | Open or close applications |
| `/assign-status` | Show settings and who the bot gave which issue |
| `/assign-release issue:` | Stop an issue counting toward someone's max |
| `/assign-available` | List open, unassigned issues (optional `from`, `to`, `label`, `applicants`), with a count and which are already in your list |

A message is accepted only when applications are on, it is in the set
channel, the member has the roles, has linked GitHub (`/connect`), and the
request keeps them within `max`. The max counts only issues the bot gave
them that are still open and assigned to them; asking for too many declines
the whole request. Each issue must be in the list, open, unassigned, and
carry a Drips Wave application comment (`<!-- wave:application-id:... -->`)
posted by the member's linked GitHub account.

The bot reacts ✅ to a message when it assigned at least one issue and ❌
when it assigned none, replying only to explain anything skipped or declined.
`/assign-toggle on` posts (and pins, if the bot may manage messages) a
"how to apply" message in the channel; declines point members to it. Only
messages sent after `/assign-toggle on` count. Closed issues and issues
assigned outside the bot drop out of the list, and when every released issue
has been given out the bot switches applications off and says so in the
channel.

Setup: enable **Message Content Intent** under Bot > Privileged Gateway
Intents in the Discord developer portal (the bot fails to log in without
it), give the bot View Channel, Read Message History, Send Messages and Add Reactions in the channel, and give
`GITHUB_TOKEN` `issues: read and write` on the repo.

## Role tiers

| Tier             | Requirement                                                  | Default threshold |
| ---------------- | ------------------------------------------------------------ | ----------------- |
| Verified         | Member of the server (assigned via Discord Onboarding)       | n/a               |
| Contributor      | Merged PRs in `sorolens/sorolens` >= `CONTRIBUTOR_THRESHOLD` | 1                 |
| Core Contributor | Merged PRs >= `CORE_CONTRIBUTOR_THRESHOLD`                   | 3                 |

Only the highest tier a contributor qualifies for is held at any time (Core Contributors do not also hold Contributor).

## Local development

```bash
cd services/discord-bot
cp .env.example .env    # fill in the values
npm install
npm run register        # register slash commands with Discord (one-shot)
npm run dev             # tsx watch mode
```

The bot serves the webhook receiver on `http://localhost:8080/webhook`. Use `smee.io` or `ngrok` to tunnel that publicly and point a test GitHub webhook at it.

## Production deploy (Render, free tier)

Render's free tier sleeps after 15 minutes of inactivity, which breaks the Discord gateway connection. We paper over that with an external ping (see the UptimeRobot step below).

### 0. Create a GitHub OAuth App (for `/connect`)

1. https://github.com/settings/developers > **New OAuth App**.
2. Fill in:
   - **Application name:** `Sorolens Contributor Bot`
   - **Homepage URL:** `https://sorolens.onrender.com` (or whatever your PUBLIC_BASE_URL will be)
   - **Authorization callback URL:** `${PUBLIC_BASE_URL}/oauth/callback` - **must match exactly**
3. Register. Copy the **Client ID** and generate a **Client Secret**; both go into env vars.

### 1. Create the web service

1. Sign in at https://render.com with GitHub. Grant access to `sorolens/sorolens`.
2. Dashboard: **New +** > **Web Service** > pick `sorolens/sorolens`.
3. Fields:

   | Field              | Value                  |
   | ------------------ | ---------------------- |
   | Name               | `sorolens-discord-bot` |
   | Region             | Oregon or Ohio         |
   | Branch             | `main`                 |
   | **Root Directory** | `services/discord-bot` |
   | Runtime            | **Docker**             |
   | Dockerfile Path    | `./Dockerfile`         |
   | Instance Type      | **Free**               |

4. Under **Environment Variables**, add every entry from `.env.example`. Set `DATABASE_PATH=/tmp/mappings.db`. Render's free tier has no persistent disk, so also set up the backup in [Keep links across redeploys](#keep-links-across-redeploys) or contributors must `/link` again after every deploy.
5. Click **Create Web Service** and wait for the green **Live** badge (~4-6 minutes).
6. Copy the public URL (e.g. `https://sorolens-discord-bot.onrender.com`).

### 2. Register slash commands

Render Dashboard > your service > **Shell** tab:

```bash
npm run register
```

Slash commands appear in Discord instantly.

### 3. Keep links across redeploys

The bot stores GitHub links in SQLite. Litestream copies the file to S3-compatible storage and restores it on every start.

1. Create a free bucket, for example Cloudflare R2 (dashboard > R2 > **Create bucket**) and an API token with read and write access to it.
2. In Render > your service > **Environment**, add:

   | Variable | Value |
   | --- | --- |
   | `LITESTREAM_BUCKET` | your bucket name |
   | `LITESTREAM_ACCESS_KEY_ID` | the token's access key id |
   | `LITESTREAM_SECRET_ACCESS_KEY` | the token's secret |
   | `LITESTREAM_ENDPOINT` | R2: `https://<account-id>.r2.cloudflarestorage.com` (leave empty for AWS S3) |
   | `LITESTREAM_REGION` | R2: `auto`; AWS: your region |

3. Redeploy. The log shows `restoring ... from replica` on start. Without `LITESTREAM_BUCKET` the bot warns that links are not backed up.

### 4. Keep the service warm with UptimeRobot

1. Sign up at https://uptimerobot.com (free).
2. **Add New Monitor**:
   - Monitor Type: HTTP(s)
   - Friendly Name: `Sorolens bot keepalive`
   - URL: `https://<your-render-url>/healthz`
   - Monitoring Interval: 5 minutes
3. Create monitor.

UptimeRobot doubles as an alerting layer - it will email you if the bot goes down.

### 4. Point the GitHub webhook

`sorolens/sorolens` > Settings > Webhooks > **Add webhook**:

- Payload URL: `https://<render-url>/webhook`
- Content type: `application/json`
- Secret: same value as `GITHUB_WEBHOOK_SECRET`
- Events: **Pull requests** only
- Active: on

Save. GitHub fires a `ping` immediately; the delivery should show **202 Accepted** in Recent Deliveries.

## How it verifies signatures

The webhook receiver uses `@octokit/webhooks` which validates the HMAC-SHA256 signature GitHub attaches on every delivery. An unsigned or mis-signed request returns 400 without touching the database.

## Failure modes and what happens

| Scenario                                                                          | Behaviour                                                                                                                                                                                               |
| --------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| PR merged by contributor who has not run `/link`                                  | Bot logs and skips. Contributor can `/link` any time; roles sync immediately.                                                                                                                           |
| Contributor tries to `/link` a GitHub name already linked to another Discord user | Command replies with an error; no state changes.                                                                                                                                                        |
| GitHub API rate limit                                                             | Command replies with the error message. Retry manually or wait ~1 min.                                                                                                                                  |
| Bot crashes / redeploys                                                           | Slash commands stay registered. On Render's free tier `DATABASE_PATH` points at `/tmp` and mappings reset on redeploy unless the Litestream backup below is configured; then the database is restored automatically on start. |

## Security notes

- GitHub PAT needs only read access to `sorolens/sorolens` (metadata + pull_requests). No write scopes.
- Bot Discord permissions: only Manage Roles (scoped below the highest role it will manage). Never grant Administrator.
- Webhook secret should be 32+ random characters. Rotate if leaked.

## Tests

```bash
npm test
```
