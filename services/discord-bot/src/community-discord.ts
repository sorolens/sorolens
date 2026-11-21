/**
 * Discord glue for community and rewards: `/leaderboard`, `/community-setup`,
 * `/milestones`, merge congratulations (from the GitHub webhook) and the
 * weekly contributor-of-the-week post. Text comes from `community.ts`.
 */

import { EmbedBuilder, type ChatInputCommandInteraction, type Client, type Guild } from "discord.js";
import {
  allLinks,
  getCommunitySettings,
  listMilestones,
  removeMilestone,
  setMilestone,
  updateCommunitySettings,
} from "./db.js";
import { listMergedPRs, type MergedPR } from "./github.js";
import {
  SUGGESTED_MILESTONES,
  dueCotwWeek,
  formatCongrats,
  formatCotw,
  formatLeaderboard,
  milestonesReached,
  rankContributors,
  weekStart,
  type LeaderboardRow,
} from "./community.js";
import type { Config } from "./config.js";

const EPHEMERAL = 64;
const LEADERBOARD_SIZE = 10;
const CACHE_TTL_MS = 5 * 60_000;
const COTW_CHECK_INTERVAL_MS = 10 * 60_000;

export const COMMUNITY_COMMANDS = new Set([
  "leaderboard",
  "community-setup",
  "milestones",
]);

/** GitHub search wants second precision, without milliseconds. */
function isoSeconds(d: Date): string {
  return d.toISOString().replace(/\.\d{3}Z$/, "Z");
}

const prCache = new Map<string, { at: number; prs: MergedPR[] }>();
async function mergedPRsCached(
  repo: string,
  since?: Date,
  until?: Date
): Promise<MergedPR[]> {
  const key = `${since?.toISOString() ?? ""}|${until?.toISOString() ?? ""}`;
  const hit = prCache.get(key);
  if (hit && Date.now() - hit.at < CACHE_TTL_MS) return hit.prs;
  const prs = await listMergedPRs(repo, {
    since: since && isoSeconds(since),
    until: until && isoSeconds(until),
  });
  prCache.set(key, { at: Date.now(), prs });
  return prs;
}

/** Ranked linked members for a window. */
async function linkedLeaderboard(
  repo: string,
  since?: Date,
  until?: Date
): Promise<LeaderboardRow[]> {
  const links = allLinks();
  const byLogin = new Map(links.map((l) => [l.githubLogin.toLowerCase(), l.discordId]));
  const prs = await mergedPRsCached(repo, since, until);
  return rankContributors(prs, new Set(byLogin.keys())).map((r) => ({
    login: r.login,
    count: r.count,
    discordId: byLogin.get(r.login.toLowerCase())!,
  }));
}

export async function handleCommunityCommand(
  interaction: ChatInputCommandInteraction,
  config: Config
): Promise<void> {
  switch (interaction.commandName) {
    case "leaderboard":
      return handleLeaderboard(interaction, config);
    case "community-setup":
      return handleSetup(interaction);
    case "milestones":
      return handleMilestones(interaction);
  }
}

async function handleLeaderboard(
  interaction: ChatInputCommandInteraction,
  config: Config
) {
  const period = interaction.options.getString("period") ?? "all";
  // Public reply: everyone in the channel sees the leaderboard.
  await interaction.deferReply();
  const now = new Date();
  let since: Date | undefined;
  let title = "🏆 All-time leaderboard";
  if (period === "week") {
    since = weekStart(now);
    title = "🏆 This week's leaderboard (since Sunday 00:00 UTC)";
  } else if (period === "month") {
    since = new Date(now.getTime() - 30 * 86_400_000);
    title = "🏆 Leaderboard: last 30 days";
  }
  const rows = await linkedLeaderboard(config.githubRepo, since);
  await interaction.editReply({
    content: formatLeaderboard(title, rows.slice(0, LEADERBOARD_SIZE)),
    allowedMentions: { parse: [] },
  });
}

function describeSettings(): string {
  const s = getCommunitySettings();
  const ms = listMilestones();
  return [
    `**Merge congrats:** ${s.congratsEnabled ? "on" : "off"} in ${s.congratsChannelId ? `<#${s.congratsChannelId}>` : "(no channel set)"}`,
    `**Weekly post channel:** ${s.channelId ? `<#${s.channelId}>` : "(not set)"}`,
    `**Contributor of the week:** ${s.cotwEnabled ? "on" : "off"} (Sundays 08:00 UTC)` +
      (s.cotwRoleId ? `, role <@&${s.cotwRoleId}>` : ", no role"),
    `**Milestones:** ${ms.length ? ms.map((m) => `${m.count} → <@&${m.roleId}>`).join(", ") : "(none)"}`,
  ].join("\n");
}

async function handleSetup(interaction: ChatInputCommandInteraction) {
  const channel = interaction.options.getChannel("weekly_channel");
  const congratsChannel = interaction.options.getChannel("congrats_channel");
  const congrats = interaction.options.getBoolean("congrats");
  const cotw = interaction.options.getBoolean("contributor_of_week");
  const role = interaction.options.getRole("cotw_role");
  const clearRole = interaction.options.getBoolean("clear_cotw_role") ?? false;
  const patch: Parameters<typeof updateCommunitySettings>[0] = {};
  if (channel) patch.channelId = channel.id;
  if (congratsChannel) patch.congratsChannelId = congratsChannel.id;
  if (congrats !== null) patch.congratsEnabled = congrats;
  if (cotw !== null) patch.cotwEnabled = cotw;
  if (role) patch.cotwRoleId = role.id;
  if (clearRole) patch.cotwRoleId = null;
  updateCommunitySettings(patch);
  await interaction.reply({
    content: `Saved.\n${describeSettings()}`,
    flags: EPHEMERAL,
    allowedMentions: { parse: [] },
  });
}

async function handleMilestones(interaction: ChatInputCommandInteraction) {
  const sub = interaction.options.getSubcommand();
  let note = "";
  if (sub === "add") {
    const count = interaction.options.getInteger("count", true);
    const role = interaction.options.getRole("role", true);
    setMilestone(count, role.id);
    note = `Saved: ${count} merged PRs → <@&${role.id}>.`;
  } else if (sub === "remove") {
    const count = interaction.options.getInteger("count", true);
    note = removeMilestone(count) ? `Removed the ${count}-PR milestone.` : `No milestone at ${count}.`;
  }
  const ms = listMilestones();
  const suggestion = SUGGESTED_MILESTONES.map((m) => `${m.count} ${m.name}`).join(", ");
  await interaction.reply({
    content:
      `${note}\n**Milestones:** ${ms.length ? ms.map((m) => `${m.count} → <@&${m.roleId}>`).join(", ") : "(none)"}` +
      `\n_Suggested: ${suggestion}. Create the roles in Server Settings > Roles, keep them below the bot's role, then \`/milestones add\`._`,
    flags: EPHEMERAL,
    allowedMentions: { parse: [] },
  });
}

/**
 * Called from the webhook for every merged PR. Linked members get any
 * milestone roles they reached and a congratulation that tags them with
 * their Discord avatar; unlinked authors get one with their GitHub avatar
 * and a nudge to `/connect`.
 */
export async function onMerge(
  guild: Guild,
  config: Config,
  merge: {
    discordId: string | null;
    login: string;
    githubAvatarUrl: string | null;
    prNumber: number;
    prTitle: string;
    prUrl: string;
  }
): Promise<void> {
  // Count from the author's merged PRs; the search index can lag a few
  // seconds behind the webhook, so add this PR if it is not listed yet.
  const prs = await listMergedPRs(config.githubRepo, { author: merge.login });
  const total = prs.length + (prs.some((p) => p.number === merge.prNumber) ? 0 : 1);

  const member = merge.discordId
    ? await guild.members.fetch(merge.discordId).catch(() => null)
    : null;
  let newMilestone: { count: number; roleId: string } | null = null;
  if (member) {
    for (const m of milestonesReached(total, listMilestones())) {
      if (member.roles.cache.has(m.roleId)) continue;
      try {
        await member.roles.add(m.roleId, `sorolens-bot: ${total} merged PRs`);
        newMilestone = m;
      } catch (err) {
        console.warn(`community: could not grant milestone role ${m.roleId}`, err);
      }
    }
  }

  const s = getCommunitySettings();
  if (!s.congratsEnabled || !s.congratsChannelId) return;
  const channel = await guild.channels.fetch(s.congratsChannelId).catch(() => null);
  if (!channel?.isSendable()) return;

  const msg = formatCongrats({
    discordId: merge.discordId,
    login: merge.login,
    avatarUrl: member ? member.displayAvatarURL({ size: 256 }) : merge.githubAvatarUrl,
    prNumber: merge.prNumber,
    prTitle: merge.prTitle,
    prUrl: merge.prUrl,
    total,
    milestone: newMilestone,
  });
  const embed = new EmbedBuilder()
    .setTitle(msg.embed.title)
    .setURL(msg.embed.url)
    .setDescription(msg.embed.description)
    .setFooter({ text: msg.embed.footer })
    .setColor(msg.embed.color);
  if (msg.embed.thumbnail) embed.setThumbnail(msg.embed.thumbnail);
  await channel.send({
    content: msg.content,
    embeds: [embed],
    allowedMentions: { users: merge.discordId ? [merge.discordId] : [], roles: [] },
  });
}

/** Post the contributor of the week if it is due. Exported for tests/manual runs. */
export async function runCotwIfDue(
  client: Client,
  config: Config,
  now = new Date()
): Promise<boolean> {
  const s = getCommunitySettings();
  if (!s.cotwEnabled || !s.channelId) return false;
  const week = dueCotwWeek(now, s.cotwLastWeek);
  if (!week) return false;

  const guild = await client.guilds.fetch(config.discordGuildId);
  const rows = await linkedLeaderboard(config.githubRepo, week.start, week.end);
  const winner = rows[0];

  if (s.cotwRoleId) {
    if (s.cotwHolderId && s.cotwHolderId !== winner?.discordId) {
      const prev = await guild.members.fetch(s.cotwHolderId).catch(() => null);
      await prev?.roles.remove(s.cotwRoleId, "sorolens-bot: contributor of the week ended").catch(() => undefined);
    }
    if (winner) {
      const m = await guild.members.fetch(winner.discordId).catch(() => null);
      await m?.roles.add(s.cotwRoleId, "sorolens-bot: contributor of the week").catch((err) => {
        console.warn("community: could not grant contributor-of-the-week role", err);
      });
    }
  }

  const channel = await guild.channels.fetch(s.channelId).catch(() => null);
  if (channel?.isSendable()) {
    await channel.send({
      content: formatCotw({ weekStart: week.start, weekEnd: week.end, rows, roleId: s.cotwRoleId }),
      allowedMentions: { users: winner ? [winner.discordId] : [], roles: [] },
    });
  }
  updateCommunitySettings({
    cotwLastWeek: week.key,
    cotwHolderId: winner?.discordId ?? null,
  });
  return true;
}

/** Check every 10 minutes whether the weekly post is due. */
export function startCommunityScheduler(client: Client, config: Config): void {
  const tick = () =>
    runCotwIfDue(client, config).catch((err) =>
      console.error("community: contributor of the week failed", err)
    );
  setTimeout(tick, 30_000);
  setInterval(tick, COTW_CHECK_INTERVAL_MS).unref();
}
