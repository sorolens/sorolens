/**
 * Community and rewards: leaderboard, merge congratulations, contributor of
 * the week and milestone roles. Everything here is plain text built from
 * GitHub data; nothing calls an AI service.
 *
 * Pure helpers live here so they can be tested without Discord or GitHub.
 */

import type { MergedPR } from "./github.js";

export interface RankedContributor {
  login: string;
  count: number;
  /** When they reached `count` (merge time of their count-th PR). */
  reachedAt: string;
}

/**
 * Rank authors by merged PRs. Ties go to whoever reached the count first.
 * When `onlyLogins` is given, other authors are left out (case-insensitive).
 */
export function rankContributors(
  prs: readonly MergedPR[],
  onlyLogins?: ReadonlySet<string>
): RankedContributor[] {
  const byAuthor = new Map<string, { login: string; times: string[] }>();
  for (const pr of prs) {
    const key = pr.author.toLowerCase();
    if (onlyLogins && !onlyLogins.has(key)) continue;
    const entry = byAuthor.get(key) ?? { login: pr.author, times: [] };
    entry.times.push(pr.mergedAt);
    byAuthor.set(key, entry);
  }
  return [...byAuthor.values()]
    .map(({ login, times }) => {
      const sorted = [...times].sort();
      return { login, count: sorted.length, reachedAt: sorted[sorted.length - 1] };
    })
    .sort((a, b) =>
      b.count !== a.count
        ? b.count - a.count
        : a.reachedAt < b.reachedAt
          ? -1
          : a.reachedAt > b.reachedAt
            ? 1
            : a.login.localeCompare(b.login)
    );
}

const DAY_MS = 86_400_000;

/** Start (Sunday 00:00 UTC) of the week containing `now`. */
export function weekStart(now: Date): Date {
  const d = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()));
  return new Date(d.getTime() - d.getUTCDay() * DAY_MS);
}

/** The full Sunday-Saturday UTC week before the one containing `now`. */
export function previousWeek(now: Date): { start: Date; end: Date; key: string } {
  const end = weekStart(now);
  const start = new Date(end.getTime() - 7 * DAY_MS);
  return { start, end, key: start.toISOString().slice(0, 10) };
}

/** Hour (UTC) on Sunday when the contributor of the week is announced. */
export const COTW_HOUR_UTC = 8;

/**
 * The week to announce now, or null. It is due from Sunday 08:00 UTC
 * onwards (so a bot that was asleep at 08:00 still posts when it wakes),
 * once per week.
 */
export function dueCotwWeek(
  now: Date,
  lastPostedKey: string | null
): { start: Date; end: Date; key: string } | null {
  const announceAt = weekStart(now).getTime() + COTW_HOUR_UTC * 3_600_000;
  if (now.getTime() < announceAt) return null;
  const week = previousWeek(now);
  return week.key === lastPostedKey ? null : week;
}

export interface Milestone {
  count: number;
  roleId: string;
}

/** Suggested milestones after the built-in 1 (Contributor) and 3 (Core). */
export const SUGGESTED_MILESTONES: { count: number; name: string }[] = [
  { count: 5, name: "Rising Star" },
  { count: 10, name: "Veteran" },
  { count: 20, name: "Elite" },
  { count: 40, name: "Legend" },
  { count: 80, name: "Mythic" },
];

/** Milestones at or below `count` (roles the member should hold). */
export function milestonesReached(count: number, milestones: readonly Milestone[]): Milestone[] {
  return milestones.filter((m) => count >= m.count).sort((a, b) => a.count - b.count);
}

export function ordinal(n: number): string {
  const mod100 = n % 100;
  if (mod100 >= 11 && mod100 <= 13) return `${n}th`;
  return `${n}${["th", "st", "nd", "rd"][n % 10] ?? "th"}`;
}

const MEDALS = ["🥇", "🥈", "🥉"];

export interface LeaderboardRow {
  login: string;
  count: number;
  discordId: string;
}

export function formatLeaderboard(title: string, rows: readonly LeaderboardRow[]): string {
  if (rows.length === 0) return `**${title}**\nNo merged PRs from linked members yet.`;
  const lines = rows.map(
    (r, i) =>
      `${MEDALS[i] ?? `**${i + 1}.**`} <@${r.discordId}> (\`${r.login}\`): **${r.count}** merged PR${r.count === 1 ? "" : "s"}`
  );
  return [`**${title}**`, ...lines, "", "_Link your GitHub with `/connect` to appear here._"].join("\n");
}

export interface CongratsInput {
  /** Linked Discord member, or null when the author has not linked. */
  discordId: string | null;
  login: string;
  /** Discord avatar for linked members, GitHub avatar otherwise. */
  avatarUrl: string | null;
  prNumber: number;
  prTitle: string;
  prUrl: string;
  total: number;
  milestone?: { count: number; roleId: string } | null;
}

export interface CongratsMessage {
  /** Plain message text; carries the @mention so the member is pinged. */
  content: string;
  embed: {
    title: string;
    url: string;
    description: string;
    thumbnail: string | null;
    footer: string;
    color: number;
  };
}

/** Merge congratulation: a short pinging line plus an embed with the avatar. */
export function formatCongrats(opts: CongratsInput): CongratsMessage {
  const title = opts.prTitle.length > 200 ? `${opts.prTitle.slice(0, 197)}…` : opts.prTitle;
  const who = opts.discordId ? `<@${opts.discordId}>` : `**${opts.login}**`;
  const description = [
    `That's ${opts.discordId ? "their" : `\`${opts.login}\`'s`} **${ordinal(opts.total)}** merged PR in Sorolens!`,
  ];
  if (opts.milestone) {
    description.push(
      `🏅 Milestone unlocked: **${opts.milestone.count} merged PRs** and the <@&${opts.milestone.roleId}> role!`
    );
  }
  if (!opts.discordId) {
    description.push(
      `Is this you, \`${opts.login}\`? Run \`/connect\` to link your GitHub and earn contributor roles.`
    );
  }
  return {
    content: `🎉 ${who} just got PR #${opts.prNumber} merged!`,
    embed: {
      title: `#${opts.prNumber} ${title}`.slice(0, 256),
      url: opts.prUrl,
      description: description.join("\n"),
      thumbnail: opts.avatarUrl,
      footer: `${opts.login} · sorolens/sorolens`,
      color: 0x2ecc71,
    },
  };
}

export function formatCotw(opts: {
  weekStart: Date;
  weekEnd: Date;
  rows: readonly LeaderboardRow[];
  roleId?: string | null;
}): string {
  const fmt = (d: Date) => d.toISOString().slice(0, 10);
  const lastDay = new Date(opts.weekEnd.getTime() - DAY_MS);
  const range = `${fmt(opts.weekStart)} to ${fmt(lastDay)}`;
  if (opts.rows.length === 0) {
    return `🗓️ **Contributor of the Week** (${range})\nNo merged PRs from linked members last week. This week's spot is open!`;
  }
  const [winner, ...rest] = opts.rows;
  const lines = [
    `🏆 **Contributor of the Week** (${range})`,
    `Congratulations <@${winner.discordId}> (\`${winner.login}\`) with **${winner.count}** merged PR${winner.count === 1 ? "" : "s"}!` +
      (opts.roleId ? ` You hold <@&${opts.roleId}> until next Sunday.` : ""),
  ];
  if (rest.length) {
    lines.push(
      "Runners-up: " +
        rest
          .slice(0, 2)
          .map((r) => `<@${r.discordId}> (${r.count})`)
          .join(", ")
    );
  }
  return lines.join("\n");
}
