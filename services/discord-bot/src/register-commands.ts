/**
 * One-shot script to register the bot's slash commands with Discord.
 *
 * Run `pnpm --filter @sorolens/discord-bot register` after adding or
 * changing a command in commands.ts. Commands are guild-scoped so they
 * apply instantly (global commands can take up to an hour to propagate).
 *
 * This script deliberately does not import the full runtime config so it
 * only requires the three Discord identifiers (token, client id, guild
 * id) to run. Handy for CI, and for local shells that do not have every
 * production secret populated.
 */

import { REST, Routes } from "discord.js";
import { commandDefinitions } from "./command-definitions.js";

function required(name: string): string {
  const v = process.env[name];
  if (!v || v.trim() === "") {
    throw new Error(`Missing required env var: ${name}`);
  }
  return v.trim();
}

const discordToken = required("DISCORD_BOT_TOKEN");
const discordClientId = required("DISCORD_CLIENT_ID");
const discordGuildId = required("DISCORD_GUILD_ID");

const rest = new REST({ version: "10" }).setToken(discordToken);

async function main() {
  console.log(`Registering ${commandDefinitions.length} commands on guild ${discordGuildId}...`);
  await rest.put(
    Routes.applicationGuildCommands(discordClientId, discordGuildId),
    { body: commandDefinitions },
  );
  console.log("Commands registered.");
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
