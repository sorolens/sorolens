/**
 * Sorolens contributor bot entry point.
 *
 * Boots the Discord client + Express HTTP server in a single process:
 * - `/healthz` and `/webhook` for the GitHub webhook receiver.
 * - `/oauth/start` and `/oauth/callback` for the one-click GitHub link.
 * Deployment targets are single-instance (Render, Fly, a small VPS), so
 * we do not need horizontal-scaling considerations.
 */

import express from "express";
import { Client, GatewayIntentBits, Partials } from "discord.js";
import { config } from "./config.js";
import { openDb } from "./db.js";
import { initGithub } from "./github.js";
import { registerHandlers } from "./commands.js";
import { mountWebhookRoutes } from "./webhook.js";
import { buildConnectUrl, mountOauthRoutes } from "./oauth.js";

async function main() {
  openDb(config.dbPath);
  initGithub(config.githubToken);

  const client = new Client({
    intents: [
      GatewayIntentBits.Guilds,
      GatewayIntentBits.GuildMembers,
      GatewayIntentBits.DirectMessages,
    ],
    partials: [Partials.Channel],
  });

  const tiers = {
    contributor: config.roleContributor,
    coreContributor: config.roleCoreContributor,
    contributorThreshold: config.contributorThreshold,
    coreContributorThreshold: config.coreContributorThreshold,
  };

  registerHandlers(client, { config, tiers });

  // Auto-DM new members with a one-click GitHub link. Users with DMs
  // disabled fall through silently; they can still run `/connect` in
  // the server whenever they are ready.
  client.on("guildMemberAdd", async (member) => {
    if (member.guild.id !== config.discordGuildId) return;
    const url = buildConnectUrl(member.id, config);
    try {
      await member.send({
        content: [
          `Welcome to Sorolens.`,
          ``,
          `One-click link to auto-claim your Contributor role when your PRs merge:`,
          url,
          ``,
          `The link is personal - do not share it. It expires in 10 minutes; run \`/connect\` in the server to get a fresh one.`,
          ``,
          `Repo: https://github.com/sorolens/sorolens`,
          `Read #rules for the workflow.`,
        ].join("\n"),
      });
    } catch (err) {
      console.log(
        `guildMemberAdd: DM failed for ${member.id} (likely DMs disabled): ` +
          (err instanceof Error ? err.message : String(err)),
      );
    }
  });

  client.once("clientReady", () => {
    console.log(`Discord client ready as ${client.user?.tag}`);
  });

  await client.login(config.discordToken);

  const app = express();
  mountWebhookRoutes(app, { client, config, tiers });
  mountOauthRoutes(app, { client, config, tiers });
  app.listen(config.port, () => {
    console.log(`http listening on :${config.port}`);
  });

  const stop = (signal: string) => {
    console.log(`Received ${signal}, shutting down...`);
    client.destroy().finally(() => process.exit(0));
  };
  process.on("SIGINT", () => stop("SIGINT"));
  process.on("SIGTERM", () => stop("SIGTERM"));
}

main().catch((e) => {
  console.error("fatal:", e);
  process.exit(1);
});
