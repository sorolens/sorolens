/**
 * One-shot script to register the bot's slash commands with Discord.
 *
 * Run `pnpm --filter @sorolens/discord-bot register` after adding or
 * changing a command in commands.ts. Commands are guild-scoped so they
 * apply instantly (global commands can take up to an hour to propagate).
 */

import { REST, Routes } from "discord.js";
import { commandDefinitions } from "./commands.js";
import { config } from "./config.js";

const rest = new REST({ version: "10" }).setToken(config.discordToken);

async function main() {
  console.log(`Registering ${commandDefinitions.length} commands on guild ${config.discordGuildId}...`);
  await rest.put(
    Routes.applicationGuildCommands(config.discordClientId, config.discordGuildId),
    { body: commandDefinitions },
  );
  console.log("Commands registered.");
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
