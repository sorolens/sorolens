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
import {
  addAssignee,
  getIssue,
  listIssueComments,
  listOpenUnassignedIssues,
} from "./github.js";
import {
  countWaveApplicants,
  formatAvailableSummary,
  describeResponse,
  formatHowToApply,
  remainingIssues,
  formatIssueList,
  parseIssueList,
  processApplication,
  pruneStaleGrants,
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
  "assign-available",
]);

/** Most issues whose comments `/assign-available applicants:true` reads. */
const APPLICANT_CHECK_LIMIT = 60;

function deps(config: Config): AssignDeps {
  return {
    repo: config.githubRepo,
    getSettings: getAssignSettings,
    getLink: getByDiscord,
    isInPool,
    getGrant,
    listPool,
    removeFromPool,
    setEnabled: (enabled: boolean) => {
      updateAssignSettings({ enabled });
    },
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
            createdAt: message.createdTimestamp,
          },
          deps(config)
        )
      );
      if (result.kind === "ignore") return;
      const response = describeResponse(result);
      await message.react(response.reaction).catch((err) => {
        console.warn("assign: could not react", err);
      });
      if (response.reply) {
        await message.reply({
          content: response.reply,
          allowedMentions: { repliedUser: true, parse: [] },
        });
      }
      if (result.kind === "processed" && result.exhausted) {
        await message.channel.send({
          content:
            "🎉 All released issues have been taken. Applications are now closed until the next release.",
          allowedMentions: { parse: [] },
        });
      }
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
      return handleIssues(interaction, config);
    case "assign-toggle":
      return handleToggle(interaction, config);
    case "assign-status":
      return handleStatus(interaction, config);
    case "assign-release":
      return handleRelease(interaction);
    case "assign-available":
      return handleAvailable(interaction, config);
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

/**
 * Check the bot's records for the issues in the list against GitHub and
 * drop the stale ones, so issues people were unassigned from count as free.
 */
async function freeStaleIssues(config: Config): Promise<string> {
  const freed = await pruneStaleGrants(listPool(), {
    repo: config.githubRepo,
    getGrant,
    getIssue,
    releaseIssue,
  });
  return freed.length
    ? `\nFreed ${freed.length} issue(s) no longer assigned on GitHub: ${formatIssueList(freed)}`
    : "";
}

async function handleIssues(
  interaction: ChatInputCommandInteraction,
  config: Config
) {
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
    if (sub === "add") {
      // Checking GitHub can take longer than Discord's 3s reply window.
      await interaction.deferReply({ flags: EPHEMERAL });
      note = `Added ${addToPool(nums)} issue(s).${await freeStaleIssues(config)}`;
    } else {
      note = `Removed ${removeFromPool(nums)} issue(s).`;
    }
  } else if (sub === "clear") {
    note = `Removed ${clearPool()} issue(s).`;
  }
  const pool = listPool();
  const content = `${note}\n**Issues open (${pool.length}):** ${formatIssueList(pool)}`.trim();
  if (interaction.deferred) await interaction.editReply({ content });
  else await interaction.reply({ content, flags: EPHEMERAL });
}

async function handleToggle(
  interaction: ChatInputCommandInteraction,
  config: Config
) {
  const on = interaction.options.getString("state", true) === "on";
  const s = getAssignSettings();
  if (on && !s.channelId) {
    await interaction.reply({
      content: "Set a channel first with `/assign-setup channel:`.",
      flags: EPHEMERAL,
    });
    return;
  }
  await interaction.deferReply({ flags: EPHEMERAL });
  // Before announcing, free issues whose bot record is stale on GitHub.
  let note = on ? await freeStaleIssues(config) : "";
  const settings = updateAssignSettings(
    on ? { enabled: true, openedAt: Date.now() } : { enabled: false }
  );
  const left = remainingIssues({ listPool, getGrant });
  if (on && left.length === 0) {
    note += "\nNote: no issues are left to give out. Add some with `/assign-issues add`.";
  } else if (on) {
    note += `\n${left.length} issue(s) can be picked. Only messages sent from now on count, and applications close by themselves when all are taken.`;
  }

  // Announce in the apply channel so members know how to apply.
  try {
    const channel = await interaction.client.channels.fetch(settings.channelId!);
    if (channel?.isSendable()) {
      const posted = await channel.send({
        content: on
          ? formatHowToApply(settings, left)
          : "🔒 Issue applications are closed for now.",
        allowedMentions: { parse: [] },
      });
      if (on) await posted.pin().catch(() => undefined);
    }
  } catch (err) {
    console.warn("assign: could not announce in channel", err);
    note += "\n(I couldn't post in the channel: check my permissions there.)";
  }

  await interaction.editReply({
    content: `Issue applications are now **${on ? "ON" : "OFF"}**.${note}`,
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
  const released = nums.filter((n) => releaseIssue(n));
  const skipped = nums.filter((n) => !released.includes(n));
  const lines = [
    released.length
      ? `Released ${formatIssueList(released)}: no longer counted toward anyone's max and free to give out again. Anything still assigned on GitHub stays assigned; unassign it there if needed.`
      : "",
    skipped.length ? `Not given out by the bot: ${formatIssueList(skipped)}` : "",
  ];
  await interaction.reply({
    content: lines.filter(Boolean).join("\n"),
    flags: EPHEMERAL,
  });
}

async function handleAvailable(
  interaction: ChatInputCommandInteraction,
  config: Config
) {
  const from = interaction.options.getInteger("from") ?? 1;
  const to = interaction.options.getInteger("to") ?? Number.MAX_SAFE_INTEGER;
  const label = interaction.options.getString("label")?.trim() || undefined;
  const checkApplicants = interaction.options.getBoolean("applicants") ?? false;
  await interaction.deferReply({ flags: EPHEMERAL });

  const open = (await listOpenUnassignedIssues(config.githubRepo, label))
    .map((i) => i.number)
    .filter((n) => n >= Math.min(from, to) && n <= Math.max(from, to))
    .sort((a, b) => a - b);

  let applicants: Map<number, number> | undefined;
  let unchecked: number[] | undefined;
  if (checkApplicants) {
    applicants = new Map();
    const toCheck = open.slice(0, APPLICANT_CHECK_LIMIT);
    unchecked = open.slice(APPLICANT_CHECK_LIMIT);
    for (const n of toCheck) {
      const comments = await listIssueComments(config.githubRepo, n);
      applicants.set(n, countWaveApplicants(comments));
    }
  }

  await interaction.editReply({
    content: formatAvailableSummary({
      open,
      pool: listPool(),
      applicants,
      unchecked,
      label,
    }),
  });
}
