# @sorolens/discord-bot

Contributor bot for the [Sorolens Discord](https://discord.gg/D9jATUezYX). It:

1. Links a contributor's Discord account to their GitHub account (`/connect`).
2. Watches merged PRs on `sorolens/sorolens` and grants **Contributor** or **Core Contributor** roles based on merge count.
3. Lets members self-assign issues they applied for on **Drips Wave** by posting issue numbers in one channel ([Issue self-assign](#issue-self-assign-drips-wave)).
4. Answers Sorolens contract queries (`/status`, `/alerts`, `/watch`, `/unwatch`), see [`docs/discord-bot.md`](../../docs/discord-bot.md).

Runs as a single Node.js process with a SQLite database. Deploys to Render's free tier (Docker), with Litestream backing the database up to Cloudflare R2 and an UptimeRobot ping to prevent sleep. Any other container platform works too; only the deploy steps differ.

## Slash commands

### Everyone

| Command                      | What it does                                                            |
| ---------------------------- | ----------------------------------------------------------------------- |
| `/connect`                   | **Recommended.** Links your GitHub via one-click OAuth. No typing.      |
| `/link github <username>`    | Manual fallback if you cannot use OAuth.                                |
| `/unlink`                    | Remove the GitHub link on your Discord account.                         |
| `/whoami`                    | Show your linked GitHub account, if any.                                |
| `/mypr`                      | Show your merged PR count and current tier.                             |
| `/leaderboard [period]`      | Public top 10 linked members by merged PRs (this week, last 30 days, all time). |
| `/status <contract>`         | Show the current Sorolens status of a tracked contract.                 |
| `/alerts <contract> [limit]` | Show the latest watchdog alerts for a contract.                         |
| `/watch <contract>`          | Add a contract to your Sorolens watchlist.                              |
| `/unwatch <contract>`        | Remove a contract from your watchlist.                                  |

### Admins

Admin commands are registered with no default permissions, so only server administrators see them. Access can be widened per command under Server Settings > Integrations.

| Command                                         | What it does                                                                                       |
| ----------------------------------------------- | -------------------------------------------------------------------------------------------------- |
| `/contributions user:@x` or `github:name`       | Show someone's merged PR count and tier.                                                           |
| `/members [filter]`                             | List linked members and their roles.                                                               |
| `/admin-unlink user:@x` or `github:name`        | Remove another member's GitHub link. Roles and assigned issues are left as they are.              |
| `/community-setup`                              | Set `congrats_channel`, `weekly_channel`, turn merge congrats / contributor of the week on or off, set the rotating `cotw_role`. |
| `/milestones add\|remove\|list`                 | Roles granted at merged-PR counts, e.g. `count:5 role:@Rising Star`.                               |
| `/assign-setup`                                 | Set the apply `channel`, `required_role`, `any_role_1..3` and `max` issues per person.             |
| `/assign-available [from] [to] [label] [applicants]` | List open, unassigned issues with a count, range, and which are already released.             |
| `/assign-issues add\|remove issues:836-850, 812` | Choose which issues members can pick. `clear` and `list` too.                                      |
| `/assign-toggle on\|off`                         | Open or close applications.                                                                        |
| `/assign-status`                                | Show the settings, released issues and who the bot gave which issue.                               |
| `/assign-release issue:<n>`                     | Stop an issue counting toward someone's max (does not unassign it on GitHub).                      |

## Issue self-assign (Drips Wave)

Members post issue numbers in one channel and the bot assigns them on GitHub
with `GITHUB_TOKEN`, so the token's owner shows as the assigner. Drips Wave
treats that GitHub assignment as accepting the application.

### Running a round (owner)

1. `/assign-setup channel:#apply required_role:@real any_role_1:@contributor any_role_2:@core-contributor max:2` (once; change any option later).
2. `/assign-available` to see how many open, unassigned issues there are and their numbers. Add `label:Stellar Wave` to narrow it, or `applicants:true` to count Wave applicants per issue.
3. `/assign-issues add issues:836-850` to release issues.
4. `/assign-toggle on`. The bot posts and pins a "how to apply" message in the channel. Only messages sent from this moment on count.
5. The round ends on its own when every released issue has been given out: the bot switches applications off and announces it. `/assign-toggle off` ends it early.

### How members apply

1. Link GitHub with `/connect` (once).
2. Apply to the issue on **Drips Wave**.
3. Post the issue number(s) in the apply channel, e.g. `#812` or `#812 #840`.

The bot reacts ✅ when it assigned at least one issue and ❌ when it assigned
none. It only replies to explain what was skipped or declined.

### Rules the bot enforces

- Applications are on and the message is in the apply channel, sent after `/assign-toggle on`.
- The member holds every required role and at least one of the `any_role` roles.
- The member's GitHub is linked.
- **Max per person:** counts only issues the bot gave them that are still open and assigned to them. Asking for more than they have left declines the whole message and assigns nothing.
- Each issue must be released, open and unassigned, and have a Drips Wave application comment (`<!-- wave:application-id:... -->`) posted by the member's linked GitHub account.
- **First come, first served:** messages are processed one at a time. An issue someone already has is skipped (never reassigned) and the member still gets the other issues they picked.
- Closed issues, and issues assigned outside the bot, drop out of the released list automatically.

## Community and rewards

- **Leaderboard:** `/leaderboard` posts publicly. Only linked members are ranked; ties go to whoever reached the count first. Data comes from GitHub search, cached for 5 minutes.
- **Merge congrats:** for every merged PR (bot accounts excluded) the bot posts in `congrats_channel` (e.g. #live-updates): a line that tags linked members, plus an embed with the PR, the author's running total and their avatar (Discord avatar if linked, GitHub avatar otherwise). Unlinked authors are named by GitHub username with a nudge to run `/connect`. This is separate from the "Issue Resolved" card the `discord-live-updates` workflow posts.
- **Contributor of the week:** the week runs Sunday to Saturday (UTC). From Sunday 08:00 UTC the bot posts the winner and runners-up in `weekly_channel` (e.g. #activity) and moves `cotw_role` to the new winner. If the bot was asleep at 08:00 it posts when it wakes.
- **Milestones:** after the built-in Contributor (1 PR) and Core Contributor (3 PRs) tiers, suggested milestones are 5 Rising Star, 10 Veteran, 20 Elite, 40 Legend, 80 Mythic. Create the roles yourself (the bot never creates roles), keep them below the bot's role, and map them with `/milestones add`. Roles are granted on the member's next merge, and the congrats post mentions a newly unlocked milestone.

## Role tiers

| Tier             | Requirement                                                  | Default threshold |
| ---------------- | ------------------------------------------------------------ | ----------------- |
| Verified         | Completed `/connect` (when `DISCORD_ROLE_VERIFIED_ID` is set) | n/a              |
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

## Production setup

Render's free tier sleeps after 15 minutes of inactivity and has no persistent disk: the database in `/tmp` is wiped on every redeploy **and** every sleep. Steps 4 and 5 below are what keep the bot connected and its data intact.

### 1. Discord application

In the [Discord developer portal](https://discord.com/developers/applications) > your app:

1. **Bot** > **Privileged Gateway Intents**: turn on **Server Members Intent** and **Message Content Intent**. Without Message Content the bot fails to log in (it reads `#numbers` in the apply channel).
2. Invite the bot with the **Manage Roles** permission, and keep its role above the Contributor and Core Contributor roles.
3. In the apply channel, give the bot **View Channel**, **Read Message History**, **Send Messages**, **Add Reactions** and **Manage Messages** (only used to pin the "how to apply" message).

Copy the application id (`DISCORD_CLIENT_ID`), bot token (`DISCORD_BOT_TOKEN`), server id (`DISCORD_GUILD_ID`) and the role ids (right-click > Copy ID with Developer Mode on).

### 2. GitHub access

1. **Token** (`GITHUB_TOKEN`): a fine-grained token at https://github.com/settings/personal-access-tokens on the account that should appear as the assigner. Resource owner `sorolens`, repository `sorolens/sorolens`, permissions **Issues: Read and write** and **Pull requests: Read** (Metadata: Read is added automatically). Editing permissions later keeps the same token value.
2. **OAuth App** (for `/connect`): https://github.com/settings/developers > **New OAuth App**:
   - **Application name:** `Sorolens Contributor Bot`
   - **Homepage URL:** your `PUBLIC_BASE_URL`
   - **Authorization callback URL:** `${PUBLIC_BASE_URL}/oauth/callback` (**must match exactly**)

   Copy the **Client ID** and generate a **Client Secret**.

### 3. Render web service

1. Sign in at https://render.com with GitHub and grant access to `sorolens/sorolens`.
2. **New +** > **Web Service** > `sorolens/sorolens`:

   | Field              | Value                  |
   | ------------------ | ---------------------- |
   | Name               | `sorolens-discord-bot` |
   | Branch             | `main`                 |
   | **Root Directory** | `services/discord-bot` |
   | Runtime            | **Docker**             |
   | Dockerfile Path    | `./Dockerfile`         |
   | Instance Type      | **Free**               |

3. Under **Environment**, add every entry from [`.env.example`](.env.example), with `DATABASE_PATH=/tmp/mappings.db`, plus the backup variables from step 4.
4. **Create Web Service** and wait for **Live**. Copy the public URL.

Merging to `main` redeploys automatically if Render's auto-deploy is on; otherwise use **Manual Deploy** > **Deploy latest commit**.

### 4. Database backup (Litestream + Cloudflare R2)

The bot keeps links, issue self-assign settings, released issues and grants in SQLite. Litestream restores the file from R2 on start and streams every change back.

1. Cloudflare dashboard > **R2** > **Create bucket** (e.g. `sorolens-bot-backup`).
2. R2 > **Manage API tokens** > **Create API token** with **Object Read & Write** on that bucket. Copy the Access Key ID and Secret Access Key (shown once) and the S3 endpoint `https://<account-id>.r2.cloudflarestorage.com` (shown under the keys, in the bucket's Settings > S3 API without the bucket name, or built from the Account ID on the R2 overview).
3. In Render > **Environment**, add:

   | Variable                       | Value                                              |
   | ------------------------------ | -------------------------------------------------- |
   | `LITESTREAM_BUCKET`            | the bucket name                                    |
   | `LITESTREAM_ACCESS_KEY_ID`     | the Access Key ID                                  |
   | `LITESTREAM_SECRET_ACCESS_KEY` | the Secret Access Key                              |
   | `LITESTREAM_ENDPOINT`          | `https://<account-id>.r2.cloudflarestorage.com`    |
   | `LITESTREAM_REGION`            | `auto`                                             |

4. Redeploy. The log shows `restoring /tmp/mappings.db from replica` on start, and the bucket gains a `sorolens-discord-bot/` folder after the first change. If the log says `links are NOT backed up`, a variable is missing.

### 5. Register slash commands (GitHub Actions)

The [`Discord register commands`](../../.github/workflows/discord-register-commands.yml) workflow registers the commands on the server whenever `src/command-definitions.ts` changes on `main`, and can be run by hand from the Actions tab. It needs three repository secrets under **Settings > Secrets and variables > Actions**:

- `DISCORD_BOT_TOKEN`
- `DISCORD_CLIENT_ID`
- `DISCORD_GUILD_ID`

Commands appear in Discord within seconds; press Ctrl+R if your client does not show them yet. (Render's free tier has no Shell, so use this workflow rather than `npm run register` there.)

### 6. Keep the service warm (UptimeRobot)

1. Sign up at https://uptimerobot.com (free).
2. **Add New Monitor**: HTTP(s), URL `https://<your-render-url>/healthz`, interval 5 minutes.

UptimeRobot also emails you if the bot goes down.

### 7. GitHub webhook

`sorolens/sorolens` > Settings > Webhooks > **Add webhook**:

- Payload URL: `https://<render-url>/webhook`
- Content type: `application/json`
- Secret: same value as `GITHUB_WEBHOOK_SECRET`
- Events: **Pull requests** only

GitHub sends a `ping`; the delivery should show **202 Accepted**.

## How it verifies signatures

The webhook receiver uses `@octokit/webhooks`, which validates the HMAC-SHA256 signature GitHub attaches to every delivery. An unsigned or mis-signed request returns 400 without touching the database.

## Failure modes and what happens

| Scenario                                                          | Behaviour                                                                                                                         |
| ----------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| PR merged by a contributor who has not linked                     | Bot logs and skips. Roles sync as soon as they link.                                                                              |
| `/link` with a GitHub name already linked to another member       | Error reply; no state changes.                                                                                                    |
| GitHub API rate limit or bad token                                | The command replies with the error (`Bad credentials` means `GITHUB_TOKEN` is wrong or expired).                                  |
| Bot redeploys or wakes from sleep                                 | Commands stay registered. With Litestream configured the database is restored; without it, links and self-assign state are lost. |
| Command registration workflow fails with `Missing required env var` | The Actions secrets in step 5 are missing.                                                                                      |
| Bot does not react to `#numbers`                                  | Check applications are on (`/assign-status`), the channel is right, Message Content Intent is on and the bot can react there.    |
| GitHub silently refuses an assignment                             | The bot reports "couldn't be assigned" and frees the issue for someone else.                                                     |

## Security notes

- `GITHUB_TOKEN` is scoped to `sorolens/sorolens` only: Issues read/write (to assign), Pull requests read. No other write scopes.
- Bot Discord permissions: Manage Roles (below the highest role it manages) plus the per-channel permissions in step 1. Never grant Administrator.
- Never paste tokens or secrets in chat or commit them; set them only in Render and GitHub Actions secrets. Rotate anything that leaks.
- Webhook and OAuth state secrets should be 32+ random characters.

## Tests

```bash
npm test
```
