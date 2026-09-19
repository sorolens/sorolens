/**
 * Discord slash command definitions and handlers.
 *
 * All commands are guild-scoped so they only appear inside the Sorolens
 * server. The mapping between Discord user and GitHub login is written
 * by the contributor themselves via `/link github <username>`; the bot
 * verifies the GitHub username actually exists before storing.
 */

import type { ChatInputCommandInteraction, Client, Interaction, Guild } from "discord.js";
import { getByDiscord, unlink, upsertLink } from "./db.js";
import { countMergedPRs, getUser } from "./github.js";
import { syncRoles, type Tiers } from "./roles.js";
import { buildConnectUrl } from "./oauth.js";
import type { Config } from "./config.js";

// Re-exported from command-definitions.ts so callers that only import
// commands.ts still get the definitions; the split lets the register
// script skip loading the runtime config.
export { commandDefinitions } from "./command-definitions.js";


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
      // Swallow secondary errors: the interaction may already be
      // acknowledged, or have expired past the 3s window.
      try {
        if (interaction.deferred || interaction.replied) {
          await interaction.editReply({ content: `Something went wrong: ${msg}` });
        } else {
          // MessageFlags.Ephemeral === 1 << 6 (64).
          await interaction.reply({ content: `Something went wrong: ${msg}`, flags: 64 });
        }
      } catch (reportErr) {
        console.warn("command failed to report error:", reportErr);
      }
    }
  });
}


async function handle(interaction: ChatInputCommandInteraction, ctx: CommandContext) {
  switch (interaction.commandName) {
    case "connect":
      return handleConnect(interaction, ctx);
    case "link":
      return handleLink(interaction, ctx);
    case "unlink":
      return handleUnlink(interaction);
    case "whoami":
      return handleWhoAmI(interaction);
    case "mypr":
      return handleMyPR(interaction, ctx);
    default:
      await interaction.reply({ content: "Unknown command.", flags: 64 });
  }
}


async function handleConnect(interaction: ChatInputCommandInteraction, ctx: CommandContext) {
  const url = buildConnectUrl(interaction.user.id, ctx.config);
  await interaction.reply({
    content:
      `Click here to link your GitHub in one step (link is personal to you, do not share):\n${url}\n\n` +
      `Expires in 10 minutes. Run \`/connect\` again if it expires. Prefer manual typing? Use \`/link github <username>\`.`,
    flags: 64,
  });
}


async function handleLink(interaction: ChatInputCommandInteraction, ctx: CommandContext) {
  const gh = interaction.options.getString("github", true).trim();
  await interaction.deferReply({ flags: 64 });

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
    flags: 64,
  });
}


async function handleWhoAmI(interaction: ChatInputCommandInteraction) {
  const link = getByDiscord(interaction.user.id);
  await interaction.reply({
    content: link
      ? `Discord \`${interaction.user.tag}\` -> GitHub \`${link.githubLogin}\` (linked ${link.linkedAt} UTC).`
      : "No GitHub account linked. Use `/link github <your-username>`.",
    flags: 64,
  });
}


async function handleMyPR(interaction: ChatInputCommandInteraction, ctx: CommandContext) {
  const link = getByDiscord(interaction.user.id);
  if (!link) {
    await interaction.reply({
      content: "You have not linked a GitHub account yet. Run `/link github <your-username>` first.",
      flags: 64,
    });
    return;
  }
  await interaction.deferReply({ flags: 64 });
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
