/**
 * Discord slash command handlers for alert subscriptions:
 * - /subscribe <contract> [severity] [channel]
 * - /unsubscribe <contract> [channel]
 * - /subscriptions [channel]
 */

import {
  EmbedBuilder,
  PermissionFlagsBits,
  type ChatInputCommandInteraction,
} from "discord.js";
import { isValidContractId } from "./api.js";
import {
  listSubscriptionsForChannel,
  removeSubscription,
  upsertSubscription,
} from "./db.js";
import { shortId } from "./embeds.js";
import type { Config } from "./config.js";

const EPHEMERAL = 64;

export const SUBSCRIPTION_COMMANDS = new Set([
  "subscribe",
  "unsubscribe",
  "subscriptions",
]);

export interface TargetPermissionResult {
  allowed: boolean;
  channelId: string;
  error?: string;
}

export async function verifyTargetChannelManagePermission(
  interaction: ChatInputCommandInteraction
): Promise<TargetPermissionResult> {
  if (!interaction.guildId || !interaction.guild) {
    return {
      allowed: false,
      channelId: "",
      error: "Subscription commands can only be used inside a Discord server.",
    };
  }

  const targetOption = interaction.options.getChannel("channel");
  const targetChannelId = targetOption ? targetOption.id : interaction.channelId;

  if (!targetChannelId) {
    return {
      allowed: false,
      channelId: "",
      error: "Could not resolve the target channel.",
    };
  }

  if (
    targetOption &&
    "guildId" in targetOption &&
    targetOption.guildId &&
    targetOption.guildId !== interaction.guildId
  ) {
    return {
      allowed: false,
      channelId: targetChannelId,
      error: "The selected channel does not belong to this Discord server.",
    };
  }

  let channel =
    (interaction.guild.channels?.cache?.get?.(targetChannelId) as any) ??
    (await interaction.guild.channels?.fetch?.(targetChannelId).catch(() => null));

  if (!channel && targetOption && ("permissionsFor" in targetOption || "guildId" in targetOption)) {
    channel = targetOption as any;
  }

  if (!channel && targetChannelId === interaction.channelId && interaction.channel) {
    channel = interaction.channel as any;
  }

  if (!channel) {
    return {
      allowed: false,
      channelId: targetChannelId,
      error: "The target channel could not be found in this Discord server.",
    };
  }

  const channelGuildId =
    ("guildId" in channel && channel.guildId) ||
    ("guild" in channel && (channel as any).guild?.id);
  if (channelGuildId && channelGuildId !== interaction.guildId) {
    return {
      allowed: false,
      channelId: targetChannelId,
      error: "The selected channel does not belong to this Discord server.",
    };
  }

  const member =
    interaction.member ??
    (await interaction.guild.members?.fetch?.(interaction.user.id).catch(() => null));

  if (!member) {
    return {
      allowed: false,
      channelId: targetChannelId,
      error: "Could not determine your permissions for the target channel.",
    };
  }

  let perms: { has: (perm: bigint) => boolean } | null = null;
  if ("permissionsFor" in channel && typeof channel.permissionsFor === "function") {
    perms = channel.permissionsFor(member);
  } else if ("permissionsIn" in member && typeof (member as any).permissionsIn === "function") {
    perms = (member as any).permissionsIn(channel);
  }

  if (!perms || typeof perms.has !== "function") {
    return {
      allowed: false,
      channelId: targetChannelId,
      error: "Could not determine your permissions for the target channel.",
    };
  }

  const hasManage =
    perms.has(PermissionFlagsBits.ManageChannels) ||
    perms.has(PermissionFlagsBits.Administrator);

  if (!hasManage) {
    return {
      allowed: false,
      channelId: targetChannelId,
      error: `You need the **Manage Channels** permission in <#${targetChannelId}> to change its alert subscriptions.`,
    };
  }

  return {
    allowed: true,
    channelId: targetChannelId,
  };
}

export async function handleSubscriptionCommand(
  interaction: ChatInputCommandInteraction,
  _config: Config
): Promise<void> {
  if (!interaction.guildId) {
    await interaction.reply({
      content: "Subscription commands can only be used inside a Discord server.",
      flags: EPHEMERAL,
    });
    return;
  }

  switch (interaction.commandName) {
    case "subscribe":
      return handleSubscribe(interaction);
    case "unsubscribe":
      return handleUnsubscribe(interaction);
    case "subscriptions":
      return handleSubscriptions(interaction);
  }
}

async function handleSubscribe(interaction: ChatInputCommandInteraction) {
  const permCheck = await verifyTargetChannelManagePermission(interaction);
  if (!permCheck.allowed) {
    await interaction.reply({
      content: permCheck.error ?? "You do not have permission to manage this channel.",
      flags: EPHEMERAL,
    });
    return;
  }

  const targetChannelId = permCheck.channelId;
  const rawContract = interaction.options
    .getString("contract", true)
    .trim()
    .toUpperCase();

  if (!isValidContractId(rawContract)) {
    await interaction.reply({
      content: `\`${rawContract}\` is not a Stellar contract id. Expected 56 characters starting with \`C\`.`,
      flags: EPHEMERAL,
    });
    return;
  }

  const severity = (
    interaction.options.getString("severity") ?? "all"
  ).toLowerCase();

  upsertSubscription(targetChannelId, rawContract, severity);

  await interaction.reply({
    content: `Subscribed <#${targetChannelId}> to **${severity}** alerts for \`${rawContract}\`.`,
    flags: EPHEMERAL,
  });
}

async function handleUnsubscribe(interaction: ChatInputCommandInteraction) {
  const permCheck = await verifyTargetChannelManagePermission(interaction);
  if (!permCheck.allowed) {
    await interaction.reply({
      content: permCheck.error ?? "You do not have permission to manage this channel.",
      flags: EPHEMERAL,
    });
    return;
  }

  const targetChannelId = permCheck.channelId;
  const rawContract = interaction.options
    .getString("contract", true)
    .trim()
    .toUpperCase();

  if (!isValidContractId(rawContract)) {
    await interaction.reply({
      content: `\`${rawContract}\` is not a Stellar contract id. Expected 56 characters starting with \`C\`.`,
      flags: EPHEMERAL,
    });
    return;
  }

  const removed = removeSubscription(targetChannelId, rawContract);

  if (removed) {
    await interaction.reply({
      content: `Unsubscribed <#${targetChannelId}> from alerts for \`${rawContract}\`.`,
      flags: EPHEMERAL,
    });
  } else {
    await interaction.reply({
      content: `<#${targetChannelId}> is not subscribed to alerts for \`${rawContract}\`.`,
      flags: EPHEMERAL,
    });
  }
}

async function handleSubscriptions(interaction: ChatInputCommandInteraction) {
  if (!interaction.guildId) {
    await interaction.reply({
      content: "Subscription commands can only be used inside a Discord server.",
      flags: EPHEMERAL,
    });
    return;
  }

  const targetOption = interaction.options.getChannel("channel");
  const targetChannelId = targetOption ? targetOption.id : interaction.channelId;
  if (!targetChannelId) {
    await interaction.reply({
      content: "Could not resolve the target channel.",
      flags: EPHEMERAL,
    });
    return;
  }

  if (
    targetOption &&
    "guildId" in targetOption &&
    targetOption.guildId &&
    targetOption.guildId !== interaction.guildId
  ) {
    await interaction.reply({
      content: "The selected channel does not belong to this Discord server.",
      flags: EPHEMERAL,
    });
    return;
  }

  const subs = listSubscriptionsForChannel(targetChannelId);

  if (subs.length === 0) {
    await interaction.reply({
      content: `No active alert subscriptions for <#${targetChannelId}>. Use \`/subscribe\` to add one.`,
      flags: EPHEMERAL,
    });
    return;
  }

  const channelName =
    targetOption?.name ||
    (interaction.channel && "name" in interaction.channel
      ? interaction.channel.name
      : targetChannelId);

  const embed = new EmbedBuilder()
    .setTitle(`Alert Subscriptions for #${channelName}`)
    .setDescription(
      subs
        .map(
          (s) =>
            `• \`${s.contractId}\` (${shortId(s.contractId)}) — Severity: **${s.severity}**`
        )
        .join("\n")
    )
    .setFooter({ text: "Sorolens • /subscriptions" })
    .setTimestamp(new Date())
    .setColor(0x06b6d4);

  await interaction.reply({
    embeds: [embed],
    flags: EPHEMERAL,
  });
}
