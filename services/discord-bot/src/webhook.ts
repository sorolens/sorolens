/**
 * GitHub webhook receiver.
 *
 * Registered on the sorolens/sorolens repo for the `pull_request` event.
 * On a merge, the bot:
 *   1. Verifies the HMAC signature (rejects unsigned payloads).
 *   2. Looks up the PR author's Discord id via /link mapping.
 *   3. Recounts merged PRs and converges their role tier.
 * A missing mapping is not an error - contributors who have not linked
 * simply do not get an auto-role until they do.
 */

import express, {
  type Application,
  type Request,
  type Response,
} from "express";
import { Webhooks } from "@octokit/webhooks";
import type { Client } from "discord.js";
import { getByGithub } from "./db.js";
import { syncOnMerge } from "./commands.js";
import { onMerge } from "./community-discord.js";
import { type Tiers } from "./roles.js";
import type { Config } from "./config.js";

interface Deps {
  client: Client;
  config: Config;
  tiers: Tiers;
}

/** Mount `/healthz` and `/webhook` onto an existing Express app. */
export function mountWebhookRoutes(app: Application, deps: Deps): void {
  const webhooks = new Webhooks({ secret: deps.config.githubWebhookSecret });

  webhooks.on("pull_request.closed", async ({ payload }) => {
    if (!payload.pull_request.merged) return;

    const login = payload.pull_request.user?.login;
    if (!login) return;

    // Bot accounts (dependabot, renovate, ...) get no congratulations.
    if (payload.pull_request.user?.type === "Bot") return;

    const link = getByGithub(login);
    const guild = await deps.client.guilds.fetch(deps.config.discordGuildId);
    if (link) {
      const result = await syncOnMerge(guild, link.discordId, login, {
        config: deps.config,
        tiers: deps.tiers,
      });
      console.log("webhook: sync result", result);
    } else {
      console.log(`webhook: merge by unlinked user ${login}; no role sync`);
    }

    // Community: milestone roles and a congratulation post for every merge.
    // Failures here must not affect the role sync above.
    try {
      await onMerge(guild, deps.config, {
        discordId: link?.discordId ?? null,
        login,
        githubAvatarUrl: payload.pull_request.user?.avatar_url ?? null,
        prNumber: payload.pull_request.number,
        prTitle: payload.pull_request.title,
        prUrl: payload.pull_request.html_url,
      });
    } catch (err) {
      console.error("webhook: community post failed", err);
    }
  });

  webhooks.onError((err) => {
    console.error("webhook error", err);
  });

  app.get("/healthz", (_req, res) => res.json({ ok: true }));

  app.post(
    "/webhook",
    express.raw({ type: "application/json" }),
    async (req: Request, res: Response) => {
      const id = String(req.header("x-github-delivery") ?? "");
      const signature = String(req.header("x-hub-signature-256") ?? "");
      const name = String(req.header("x-github-event") ?? "");
      const body = req.body as Buffer;
      if (!id || !signature || !name) {
        res.status(400).json({ error: "missing github headers" });
        return;
      }
      try {
        await webhooks.verifyAndReceive({
          id,
          name: name as Parameters<typeof webhooks.verifyAndReceive>[0]["name"],
          signature,
          payload: body.toString("utf8"),
        });
        res.status(202).json({ ok: true });
      } catch (err) {
        console.warn(
          "webhook rejected:",
          err instanceof Error ? err.message : err
        );
        res.status(400).json({ error: "invalid signature or payload" });
      }
    }
  );
}
