/**
 * Discord slash command definitions and handlers.
 *
 * All commands are guild-scoped so they only appear inside the Sorolens
 * server. The mapping between Discord user and GitHub login is written
 * by the contributor themselves via `/link github <username>`; the bot
 * verifies the GitHub username actually exists before storing.
 */

import type { ChatInputCommandInteraction, Client, Interaction, Guild } from "discord.js";
import { SlashCommandBuilder } from "discord.js";
import { getByDiscord, unlink, upsertLink } from "./db.js";
import { countMergedPRs, getUser } from "./github.js";
import { syncRoles, type Tiers } from "./roles.js";
import type { Config } from "./config.js";

export const commandDefinitions = [
  new SlashCommandBuilder()
    .setName("link")
    .setDescription("Link your GitHub account so the bot can grant you contributor roles")
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


// ---- runtime handlers ------------------------------------------------------

interface CommandContext {
  config: Config;
  tiers: Tiers;
}

export function registerHandlers(client: Client, ctx: CommandContext) {
  client.on("interactionCreate", async (interaction: Interaction) => {
    if (!interaction.isChatInputCommand()) return;
    try {
      await handle(interaction, ctx);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      console.error("command failed:", interaction.commandName, msg);
      if (interaction.deferred || interaction.replied) {
        await interaction.editReply({ content: `Something went wrong: ${msg}` });
      } else {
        await interaction.reply({ content: `Something went wrong: ${msg}`, ephemeral: true });
      }
    }
  });
}


async function handle(interaction: ChatInputCommandInteraction, ctx: CommandContext) {
  switch (interaction.commandName) {
    case "link":
      return handleLink(interaction, ctx);
    case "unlink":
      return handleUnlink(interaction);
    case "whoami":
      return handleWhoAmI(interaction);
    case "mypr":
      return handleMyPR(interaction, ctx);
    default:
      await interaction.reply({ content: "Unknown command.", ephemeral: true });
  }
}


async function handleLink(interaction: ChatInputCommandInteraction, ctx: CommandContext) {
  const gh = interaction.options.getString("github", true).trim();
  await interaction.deferReply({ ephemeral: true });

  const user = await getUser(gh);
  if (!user) {
    await interaction.editReply({
      content: `GitHub user \`${gh}\` was not found. Please double-check the spelling.`,
    });
    return;
  }

  try {
    upsertLink(interaction.user.id, user.login);
  } catch (err) {
    await interaction.editReply({
      content: err instanceof Error ? err.message : "Could not save the link.",
    });
    return;
  }

  // Immediate role sync so contributors do not have to wait for their next merge.
  const guild = interaction.guild;
  let syncMsg = "";
  if (guild) {
    const count = await countMergedPRs(user.login, ctx.config.githubRepo);
    const result = await syncRoles(guild, interaction.user.id, count, ctx.tiers);
    syncMsg = describeSync(count, result.targetTier);
  }

  await interaction.editReply({
    content:
      `Linked Discord \`${interaction.user.tag}\` to GitHub \`${user.login}\`.\n` +
      (syncMsg
        ? syncMsg
        : "Once your PRs get merged into sorolens/sorolens the bot will grant your role automatically."),
  });
}


async function handleUnlink(interaction: ChatInputCommandInteraction) {
  const existed = unlink(interaction.user.id);
  await interaction.reply({
    content: existed
      ? "Your GitHub link has been removed. Your Discord roles were left as-is; a maintainer can adjust them if needed."
      : "You did not have a GitHub link on file.",
    ephemeral: true,
  });
}


async function handleWhoAmI(interaction: ChatInputCommandInteraction) {
  const link = getByDiscord(interaction.user.id);
  await interaction.reply({
    content: link
      ? `Discord \`${interaction.user.tag}\` -> GitHub \`${link.githubLogin}\` (linked ${link.linkedAt} UTC).`
      : "No GitHub account linked. Use `/link github <your-username>`.",
    ephemeral: true,
  });
}


async function handleMyPR(interaction: ChatInputCommandInteraction, ctx: CommandContext) {
  const link = getByDiscord(interaction.user.id);
  if (!link) {
    await interaction.reply({
      content: "You have not linked a GitHub account yet. Run `/link github <your-username>` first.",
      ephemeral: true,
    });
    return;
  }
  await interaction.deferReply({ ephemeral: true });
  const count = await countMergedPRs(link.githubLogin, ctx.config.githubRepo);
  const tier = tierFor(count, ctx.tiers);
  await interaction.editReply({
    content: `\`${link.githubLogin}\` has **${count}** merged PR${count === 1 ? "" : "s"} in ${ctx.config.githubRepo}. Current tier: **${tier}**.`,
  });
}


// ---- helpers ---------------------------------------------------------------

function tierFor(count: number, tiers: Tiers): string {
  if (count >= tiers.coreContributorThreshold) return "Core Contributor";
  if (count >= tiers.contributorThreshold) return "Contributor";
  return "Verified";
}

function describeSync(count: number, target: "core" | "contributor" | "none"): string {
  const label = target === "core" ? "Core Contributor" : target === "contributor" ? "Contributor" : "Verified";
  return `You have **${count}** merged PR${count === 1 ? "" : "s"} and are now tier **${label}**.`;
}

// runtime export for the entry file
export async function syncOnMerge(
  guild: Guild,
  discordId: string,
  githubLogin: string,
  ctx: CommandContext,
) {
  const count = await countMergedPRs(githubLogin, ctx.config.githubRepo);
  return syncRoles(guild, discordId, count, ctx.tiers);
}
