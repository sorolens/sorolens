/**
 * Discord embed builders for the Sorolens query commands.
 *
 * Kept free of the API and database layers so the exact rendered output can
 * be asserted in tests (`EmbedBuilder#toJSON()`), and so colours/titles stay
 * consistent across `/status`, `/alerts` and the watchlist commands.
 */

import { EmbedBuilder } from "discord.js";
import type { ContractAlert, ContractStatus } from "./api.js";

/** Sorolens brand cyan, used when a status has no dedicated colour. */
export const BRAND_COLOR = 0x06b6d4;

const STATUS_COLORS: Record<string, number> = {
  active: 0x22c55e, // green
  pending: 0xf59e0b, // amber
  backfilling: 0xf59e0b,
  paused: 0x64748b, // slate
  error: 0xef4444, // red
  unknown: 0x94a3b8, // grey
};

const SEVERITY_COLORS: Record<string, number> = {
  Critical: 0xef4444,
  Warning: 0xf59e0b,
  Info: 0x38bdf8,
};

export function colorForStatus(status: string): number {
  return STATUS_COLORS[status.toLowerCase()] ?? BRAND_COLOR;
}

export function colorForSeverity(severity: string): number {
  return SEVERITY_COLORS[severity] ?? BRAND_COLOR;
}

/** Human-facing contract reference: prefer the label, fall back to the id. */
export function contractTitle(c: Pick<ContractStatus, "id" | "label">): string {
  return c.label && c.label.trim() !== "" ? c.label.trim() : shortId(c.id);
}

/** `CABC…WXYZ` — contracts ids are 56 chars and unreadable in full. */
export function shortId(id: string): string {
  if (id.length <= 16) return id;
  return `${id.slice(0, 6)}…${id.slice(-6)}`;
}

/** Embed for `/status <contract>`. */
export function statusEmbed(status: ContractStatus): EmbedBuilder {
  const embed = new EmbedBuilder()
    .setColor(colorForStatus(status.status))
    .setTitle(`Contract ${contractTitle(status)}`)
    .addFields(
      { name: "Status", value: `\`${status.status || "unknown"}\``, inline: true },
      { name: "Network", value: status.network || "unknown", inline: true },
      { name: "Contract", value: `\`${status.id}\`` },
    )
    .setFooter({ text: "Sorolens • /status" })
    .setTimestamp(new Date());

  if (status.wasmHash) {
    embed.addFields({ name: "Wasm hash", value: `\`${shortId(status.wasmHash)}\``, inline: true });
  }
  return embed;
}

/** Embed for `/alerts <contract> [limit]`. */
export function alertsEmbed(contractId: string, alerts: ContractAlert[]): EmbedBuilder {
  const embed = new EmbedBuilder()
    .setTitle(`Alerts for ${shortId(contractId)}`)
    .setFooter({ text: "Sorolens • /alerts" })
    .setTimestamp(new Date());

  if (alerts.length === 0) {
    return embed
      .setColor(BRAND_COLOR)
      .setDescription("No alerts recorded for this contract. All quiet. ✅");
  }

  const top = alerts[0];
  embed
    .setColor(colorForSeverity(top.severity))
    .setDescription(
      alerts
        .map((a) => `**${a.severity}** — ${a.message}\n\u200b\u200bledger \`${a.ledger}\``)
        .join("\n\n"),
    );
  return embed;
}

/** Embed for `/watch` and `/unwatch`. */
export function watchlistEmbed(
  contractId: string,
  watched: boolean,
  action: "watch" | "unwatch",
  count?: number,
): EmbedBuilder {
  const embed = new EmbedBuilder()
    .setTitle(action === "watch" ? "Watchlist updated" : "Removed from watchlist")
    .setDescription(
      action === "watch"
        ? `\`${contractId}\` is now on your Sorolens watchlist. You will see it on the dashboard.`
        : `\`${contractId}\` is no longer on your Sorolens watchlist.`,
    )
    .setColor(watched ? 0x22c55e : 0x64748b)
    .setFooter({ text: "Sorolens • /watch" })
    .setTimestamp(new Date());

  if (typeof count === "number") {
    embed.addFields({ name: "Watching", value: `${count} contract${count === 1 ? "" : "s"}`, inline: true });
  }
  // `watched` reflects the API's post-condition; surface it so the user can
  // tell a no-op apart from a change.
  embed.addFields({ name: "In watchlist", value: watched ? "yes" : "no", inline: true });
  return embed;
}
