/**
 * Sorolens contributor bot entry point.
 *
 * Boots the Discord client + Express webhook receiver in a single
 * process. Deployment targets are single-instance (Railway free tier,
 * a small VPS) so we do not need horizontal-scaling considerations.
 */

import { Client, GatewayIntentBits } from "discord.js";
import { config } from "./config.js";
import { openDb } from "./db.js";
import { initGithub } from "./github.js";
import { registerHandlers } from "./commands.js";
import { makeWebhookApp } from "./webhook.js";

async function main() {
  openDb(config.dbPath);
  initGithub(config.githubToken);

  const client = new Client({
    intents: [GatewayIntentBits.Guilds, GatewayIntentBits.GuildMembers],
  });

  const tiers = {
    contributor: config.roleContributor,
    coreContributor: config.roleCoreContributor,
    contributorThreshold: config.contributorThreshold,
    coreContributorThreshold: config.coreContributorThreshold,
  };

  registerHandlers(client, { config, tiers });

  client.once("clientReady", () => {
    console.log(`Discord client ready as ${client.user?.tag}`);
  });

  await client.login(config.discordToken);

  const app = makeWebhookApp({ client, config, tiers });
  app.listen(config.port, () => {
    console.log(`webhook listening on :${config.port}`);
  });

  // Graceful shutdown.
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
