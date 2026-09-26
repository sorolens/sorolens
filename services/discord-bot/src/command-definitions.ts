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
  new SlashCommandBuilder()
    .setName("members")
    .setDescription("(Admin) List linked members and their roles")
    .setDefaultMemberPermissions("0")
    .addStringOption((o) =>
      o
        .setName("filter")
        .setDescription("Filter by tier")
        .setRequired(false)
        .addChoices(
          { name: "All linked", value: "all" },
          { name: "Contributor", value: "contributor" },
          { name: "Core Contributor", value: "core" },
          { name: "No role yet", value: "none" },
        ),
    ),
  new SlashCommandBuilder()
    .setName("status")
    .setDescription("Show the current Sorolens status of a tracked contract")
    .addStringOption((o) =>
      o
        .setName("contract")
        .setDescription("Stellar contract id (56 characters, starts with C)")
        .setRequired(true)
        .setMinLength(1)
        .setMaxLength(64),
    ),
  new SlashCommandBuilder()
    .setName("alerts")
    .setDescription("Show the most recent watchdog alerts for a contract")
    .addStringOption((o) =>
      o
        .setName("contract")
        .setDescription("Stellar contract id (56 characters, starts with C)")
        .setRequired(true)
        .setMinLength(1)
        .setMaxLength(64),
    )
    .addIntegerOption((o) =>
      o
        .setName("limit")
        .setDescription("How many alerts to show (1-10, default 5)")
        .setMinValue(1)
        .setMaxValue(10)
        .setRequired(false),
    ),
  new SlashCommandBuilder()
    .setName("watch")
    .setDescription("Add a contract to your Sorolens watchlist")
    .addStringOption((o) =>
      o
        .setName("contract")
        .setDescription("Stellar contract id (56 characters, starts with C)")
        .setRequired(true)
        .setMinLength(1)
        .setMaxLength(64),
    ),
  new SlashCommandBuilder()
    .setName("unwatch")
    .setDescription("Remove a contract from your Sorolens watchlist")
    .addStringOption((o) =>
      o
        .setName("contract")
        .setDescription("Stellar contract id (56 characters, starts with C)")
        .setRequired(true)
        .setMinLength(1)
        .setMaxLength(64),
    ),
].map((c) => c.toJSON());
