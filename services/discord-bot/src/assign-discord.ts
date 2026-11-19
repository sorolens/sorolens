/**
 * Discord glue for issue self-assign: the owner's `/assign-*` commands and
 * the channel listener that reads `#123` messages. Decision logic lives in
 * `assign.ts`.
 */

import type {
  ChatInputCommandInteraction,
  Client,
  Message,
} from "discord.js";
import {
  addToPool,
  clearPool,
  claimIssue,
  getAssignSettings,
  getByDiscord,
  getGrant,
  isInPool,
  listAllGrants,
  listGrantsFor,
  listPool,
  releaseIssue,
  removeFromPool,
  updateAssignSettings,
} from "./db.js";
import { addAssignee, getIssue, listIssueComments } from "./github.js";
import {
  formatApplicationReply,
  formatIssueList,
  parseIssueList,
  processApplication,
  type AssignDeps,
} from "./assign.js";
import type { Config } from "./config.js";

const EPHEMERAL = 64;

export const ASSIGN_COMMANDS = new Set([
  "assign-setup",
  "assign-issues",
  "assign-toggle",
  "assign-status",
  "assign-release",
]);

function deps(config: Config): AssignDeps {
  return {
    repo: config.githubRepo,
    getSettings: getAssignSettings,
    getLink: getByDiscord,
    isInPool,
    getGrant,
    listGrantsFor,
    claimIssue,
    releaseIssue,
    getIssue,
    listIssueComments,
    addAssignee,
  };
}

// Applications are handled one at a time so two messages can not race past
// the per-person cap or grab the same issue. Volume is tiny, so a single
// queue is simpler than per-user locks.
let queue: Promise<unknown> = Promise.resolve();
function serial<T>(fn: () => Promise<T>): Promise<T> {
  const run = queue.then(fn, fn);
  queue = run.catch(() => undefined);
  return run;
}

/** Listen for `#123` messages in the configured channel. */
export function registerAssignListener(client: Client, config: Config): void {
  client.on("messageCreate", async (message: Message) => {
    if (message.author.bot || !message.inGuild()) return;
    if (message.guildId !== config.discordGuildId) return;
    // Cheap pre-filter before touching the database.
    if (!message.content.includes("#")) return;
    if (message.channelId !== getAssignSettings().channelId) return;

    try {
      const member =
        message.member ?? (await message.guild.members.fetch(message.author.id));
      const result = await serial(() =>
        processApplication(
          {
            discordId: message.author.id,
            channelId: message.channelId,
            memberRoles: new Set(member.roles.cache.keys()),
            content: message.content,
          },
          deps(config)
        )
      );
      if (result.kind === "ignore") return;
      await message.reply({
        content: formatApplicationReply(result),
        allowedMentions: { repliedUser: true, parse: [] },
      });
    } catch (err) {
      console.error("assign: failed to process message", err);
      try {
        await message.reply({
          content: "❌ Something went wrong while assigning. Please try again later.",
          allowedMentions: { repliedUser: true, parse: [] },
        });
      } catch {
        // Channel may be gone or the bot lacks Send Messages; nothing to do.
      }
    }
  });
}

/** Route an `/assign-*` slash command. */
export async function handleAssignCommand(
  interaction: ChatInputCommandInteraction,
  config: Config
): Promise<void> {
  switch (interaction.commandName) {
    case "assign-setup":
      return handleSetup(interaction);
    case "assign-issues":
      return handleIssues(interaction);
    case "assign-toggle":
      return handleToggle(interaction);
    case "assign-status":
      return handleStatus(interaction, config);
    case "assign-release":
      return handleRelease(interaction);
  }
}

function roleList(ids: string[]): string {
  return ids.length ? ids.map((id) => `<@&${id}>`).join(", ") : "(none)";
}

function describeSettings(): string {
  const s = getAssignSettings();
  const pool = listPool();
  return [
    `**Applications:** ${s.enabled ? "ON" : "OFF"}`,
    `**Channel:** ${s.channelId ? `<#${s.channelId}>` : "(not set)"}`,
    `**Must have all of:** ${roleList(s.requiredRoles)}`,
    `**And at least one of:** ${s.anyOfRoles.length ? roleList(s.anyOfRoles) : "(any)"}`,
    `**Max per person:** ${s.maxPerUser}`,
    `**Issues open (${pool.length}):** ${formatIssueList(pool)}`,
  ].join("\n");
}

async function handleSetup(interaction: ChatInputCommandInteraction) {
  const channel = interaction.options.getChannel("channel");
  const max = interaction.options.getInteger("max");
  const required = interaction.options.getRole("required_role");
  const anyOf = ["any_role_1", "any_role_2", "any_role_3"]
    .map((n) => interaction.options.getRole(n))
    .filter((r): r is NonNullable<typeof r> => r !== null)
    .map((r) => r.id);
  const clearRoles = interaction.options.getBoolean("clear_roles") ?? false;

  const patch: Parameters<typeof updateAssignSettings>[0] = {};
  if (channel) patch.channelId = channel.id;
  if (max !== null) patch.maxPerUser = max;
  if (clearRoles) {
    patch.requiredRoles = [];
    patch.anyOfRoles = [];
  }
  if (required) patch.requiredRoles = [required.id];
  if (anyOf.length) patch.anyOfRoles = [...new Set(anyOf)];
  updateAssignSettings(patch);

  await interaction.reply({
    content: `Saved.\n${describeSettings()}`,
    flags: EPHEMERAL,
    allowedMentions: { parse: [] },
  });
}

async function handleIssues(interaction: ChatInputCommandInteraction) {
  const sub = interaction.options.getSubcommand();
  let note = "";
  if (sub === "add" || sub === "remove") {
    let nums: number[];
    try {
      nums = parseIssueList(interaction.options.getString("issues", true));
    } catch (err) {
      await interaction.reply({
        content: err instanceof Error ? err.message : "Invalid issue list.",
        flags: EPHEMERAL,
      });
      return;
    }
    note =
      sub === "add"
        ? `Added ${addToPool(nums)} issue(s).`
        : `Removed ${removeFromPool(nums)} issue(s).`;
  } else if (sub === "clear") {
    note = `Removed ${clearPool()} issue(s).`;
  }
  const pool = listPool();
  await interaction.reply({
    content: `${note}\n**Issues open (${pool.length}):** ${formatIssueList(pool)}`.trim(),
    flags: EPHEMERAL,
  });
}

async function handleToggle(interaction: ChatInputCommandInteraction) {
  const on = interaction.options.getString("state", true) === "on";
  const s = getAssignSettings();
  if (on && !s.channelId) {
    await interaction.reply({
      content: "Set a channel first with `/assign-setup channel:`.",
      flags: EPHEMERAL,
    });
    return;
  }
  updateAssignSettings({ enabled: on });
  const warn =
    on && listPool().length === 0
      ? "\nNote: no issues are open yet. Add some with `/assign-issues add`."
      : "";
  await interaction.reply({
    content: `Issue applications are now **${on ? "ON" : "OFF"}**.${warn}`,
    flags: EPHEMERAL,
  });
}

async function handleStatus(
  interaction: ChatInputCommandInteraction,
  config: Config
) {
  const grants = listAllGrants();
  const lines = grants
    .slice(0, 40)
    .map((g) => `#${g.issueNumber} → <@${g.discordId}> (\`${g.githubLogin}\`)`);
  if (grants.length > lines.length) {
    lines.push(`…and ${grants.length - lines.length} more`);
  }
  await interaction.reply({
    content:
      `${describeSettings()}\n**Repo:** ${config.githubRepo}\n` +
      `**Given out by the bot (${grants.length}):**\n${lines.join("\n") || "(none)"}`,
    flags: EPHEMERAL,
    allowedMentions: { parse: [] },
  });
}

async function handleRelease(interaction: ChatInputCommandInteraction) {
  const n = interaction.options.getInteger("issue", true);
  const released = releaseIssue(n);
  await interaction.reply({
    content: released
      ? `#${n} no longer counts toward anyone's max. It is still assigned on GitHub; unassign it there if needed.`
      : `#${n} wasn't given out by the bot.`,
    flags: EPHEMERAL,
  });
}
