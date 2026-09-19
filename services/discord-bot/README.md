# @sorolens/discord-bot

Contributor bot for the [Sorolens Discord](https://discord.gg/D9jATUezYX). Two responsibilities:

1. Lets contributors link their Discord identity to their GitHub username via a slash command.
2. Watches merged PRs on `sorolens/sorolens` and auto-grants **Contributor** or **Core Contributor** roles based on merge count.

Runs as a single Node.js process. Deploys to Render's free tier (Docker) with an UptimeRobot ping to prevent sleep. Any other container platform works too - only the deploy instructions differ.

## Slash commands

| Command | What it does |
|---|---|
| `/connect` | **Recommended.** Auto-links your GitHub via one-click OAuth. No typing. |
| `/link github <username>` | Manual fallback if you cannot use OAuth. |
| `/unlink` | Remove the GitHub link on your Discord account. |
| `/whoami` | Show your current linked GitHub account, if any. |
| `/mypr` | Show your merged PR count and current tier. |

New members are also DM'd a personal `/connect` link automatically when they join, so most contributors never need to type anything.

## Role tiers

| Tier | Requirement | Default threshold |
|---|---|---|
| Verified | Member of the server (assigned via Discord Onboarding) | n/a |
| Contributor | Merged PRs in `sorolens/sorolens` >= `CONTRIBUTOR_THRESHOLD` | 1 |
| Core Contributor | Merged PRs >= `CORE_CONTRIBUTOR_THRESHOLD` | 3 |

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

   | Field | Value |
   |---|---|
   | Name | `sorolens-discord-bot` |
   | Region | Oregon or Ohio |
   | Branch | `main` |
   | **Root Directory** | `services/discord-bot` |
   | Runtime | **Docker** |
   | Dockerfile Path | `./Dockerfile` |
   | Instance Type | **Free** |

4. Under **Environment Variables**, add every entry from `.env.example`. Set `DATABASE_PATH=/tmp/mappings.db` (Render's free tier has no persistent disk; contributors can `/link` again after each redeploy).
5. Click **Create Web Service** and wait for the green **Live** badge (~4-6 minutes).
6. Copy the public URL (e.g. `https://sorolens-discord-bot.onrender.com`).

### 2. Register slash commands

Render Dashboard > your service > **Shell** tab:

```bash
npm run register
```

Slash commands appear in Discord instantly.

### 3. Keep the service warm with UptimeRobot

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

| Scenario | Behaviour |
|---|---|
| PR merged by contributor who has not run `/link` | Bot logs and skips. Contributor can `/link` any time; roles sync immediately. |
| Contributor tries to `/link` a GitHub name already linked to another Discord user | Command replies with an error; no state changes. |
| GitHub API rate limit | Command replies with the error message. Retry manually or wait ~1 min. |
| Bot crashes / redeploys | Slash commands stay registered. On Render's free tier `DATABASE_PATH` points at `/tmp` and mappings reset on redeploy - contributors just run `/link` again. Move to a paid disk if that gets annoying. |

## Security notes

- GitHub PAT needs only read access to `sorolens/sorolens` (metadata + pull_requests). No write scopes.
- Bot Discord permissions: only Manage Roles (scoped below the highest role it will manage). Never grant Administrator.
- Webhook secret should be 32+ random characters. Rotate if leaked.

## Tests

```bash
npm test
```
