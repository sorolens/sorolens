# @sorolens/discord-bot

Contributor bot for the [Sorolens Discord](https://discord.gg/D9jATUezYX). Two responsibilities:

1. Lets contributors link their Discord identity to their GitHub username via a slash command.
2. Watches merged PRs on `sorolens/sorolens` and auto-grants **Contributor** or **Core Contributor** roles based on merge count.

Runs as a single Node.js process. Deploys to Railway (Docker), Fly, or any container platform.

## Slash commands

| Command | What it does |
|---|---|
| `/link github <username>` | Link your GitHub account. The bot verifies the username exists, saves the mapping, and immediately syncs your Discord role. |
| `/unlink` | Remove the GitHub link. Roles are left in place; a maintainer can adjust. |
| `/whoami` | Show your current linked GitHub account, if any. |
| `/mypr` | Show your merged PR count and current tier. |

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

## Production deploy (Railway)

1. Create a new Railway project. Point it at `services/discord-bot` in the monorepo.
2. Attach a **Volume** at `/data` so the SQLite link mappings survive redeploys.
3. Set every variable from `.env.example` in Railway's environment tab.
4. Deploy. The health check hits `/healthz` and the bot logs in on startup.
5. Copy the deployed public URL (e.g. `https://sorolens-discord-bot.up.railway.app`).
6. In GitHub: **Repo Settings > Webhooks > Add webhook**.
   - Payload URL: `https://<railway-url>/webhook`
   - Content type: `application/json`
   - Secret: same value as `GITHUB_WEBHOOK_SECRET`.
   - Events: **Pull requests** only.
7. Save. GitHub sends a `ping` immediately - the delivery should show 202 in Recent Deliveries.

## How it verifies signatures

The webhook receiver uses `@octokit/webhooks` which validates the HMAC-SHA256 signature GitHub attaches on every delivery. An unsigned or mis-signed request returns 400 without touching the database.

## Failure modes and what happens

| Scenario | Behaviour |
|---|---|
| PR merged by contributor who has not run `/link` | Bot logs and skips. Contributor can `/link` any time; roles sync immediately. |
| Contributor tries to `/link` a GitHub name already linked to another Discord user | Command replies with an error; no state changes. |
| GitHub API rate limit | Command replies with the error message. Retry manually or wait ~1 min. |
| Bot crashes / redeploys | Slash commands stay registered; mappings persist in the volume; no data loss. |

## Security notes

- GitHub PAT needs only read access to `sorolens/sorolens` (metadata + pull_requests). No write scopes.
- Bot Discord permissions: only Manage Roles (scoped below the highest role it will manage). Never grant Administrator.
- Webhook secret should be 32+ random characters. Rotate if leaked.

## Tests

```bash
npm test
```
