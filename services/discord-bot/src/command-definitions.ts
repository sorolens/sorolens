/**
 * Slash-command schema definitions, kept in a separate module from the
 * runtime handlers so `register-commands.ts` can register them without
 * loading the full bot runtime (Discord client, OAuth routes, GitHub
 * client, config). That keeps the register script runnable with only
 * `DISCORD_BOT_TOKEN`, `DISCORD_CLIENT_ID`, and `DISCORD_GUILD_ID` set.
 */

import { SlashCommandBuilder } from "discord.js";

export const commandDefinitions = [
  new SlashCommandBuilder()
    .setName("connect")
    .setDescription("Auto-link your GitHub via one-click OAuth (no typing needed)"),
  new SlashCommandBuilder()
    .setName("link")
    .setDescription("Link your GitHub account manually (advanced; most users want /connect)")
    .addStringOption((o) =>
      o
        .setName("github")
        .setDescription("Your GitHub username (case-insensitive)")
        .setRequired(true)
        .setMinLength(1)
        .setMaxLength(39),
    ),
  new SlashCommandBuilder()
    .setName("unlink")
    .setDescription("Remove the GitHub link on your Discord account"),
  new SlashCommandBuilder()
    .setName("whoami")
    .setDescription("Show your current linked GitHub account, if any"),
  new SlashCommandBuilder()
    .setName("mypr")
    .setDescription("Show your merged PR count and current tier for sorolens/sorolens"),
].map((c) => c.toJSON());
