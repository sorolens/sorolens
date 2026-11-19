/**
 * Issue self-assign.
 *
 * The owner opens a pool of issues and switches applications on. Members
 * then drop issue numbers (`#812 #840`) in the configured channel and the
 * bot assigns them on GitHub with the bot's GITHUB_TOKEN, provided that:
 *
 *   - applications are on and the message is in the configured channel;
 *   - the member holds every "required" role and at least one "any of" role;
 *   - the member has linked their GitHub account;
 *   - the request would not take them past the per-person maximum (counting
 *     only issues the bot gave them that are still open and theirs). Asking
 *     for too many declines the whole request and assigns nothing;
 *   - each issue is in the pool, open, unassigned, and the member has applied
 *     to it through Drips Wave.
 *
 * Drips Wave records an application as an issue comment posted from the
 * applicant's own GitHub account carrying a hidden
 * `<!-- wave:application-id:... -->` marker, so that comment is what we look
 * for. Assigning on GitHub is what Drips treats as acceptance.
 *
 * The decision logic lives in `processApplication`, which takes its
 * dependencies as arguments so it can be tested without Discord or GitHub.
 */

import type { AssignGrant, AssignSettings, Link } from "./db.js";
import type { IssueComment, IssueInfo } from "./github.js";

/** Hard ceiling on issue numbers read from one message, to bound API calls. */
export const MAX_ISSUES_PER_MESSAGE = 10;

const WAVE_MARKER = "<!-- wave:application-id:";

/** Pull unique `#123` references out of a message, in order. */
export function parseIssueRefs(content: string): number[] {
  const seen = new Set<number>();
  const out: number[] = [];
  for (const m of content.matchAll(/(?<![\w&])#(\d{1,7})\b/g)) {
    const n = Number(m[1]);
    if (n > 0 && !seen.has(n)) {
      seen.add(n);
      out.push(n);
    }
  }
  return out;
}

/**
 * Parse an owner-supplied list such as `836-850, 812 #900`. Throws on
 * malformed input or ranges wider than `maxSpan` so a typo can not open
 * thousands of issues at once.
 */
export function parseIssueList(input: string, maxSpan = 500): number[] {
  const out = new Set<number>();
  const tokens = input
    .split(/[\s,]+/)
    .map((t) => t.replace(/#/g, "").trim())
    .filter(Boolean);
  if (tokens.length === 0) throw new Error("No issue numbers given.");
  for (const t of tokens) {
    const range = /^(\d+)\s*-\s*(\d+)$/.exec(t);
    if (range) {
      let a = Number(range[1]);
      let b = Number(range[2]);
      if (a > b) [a, b] = [b, a];
      if (a < 1) throw new Error(`Invalid range "${t}".`);
      if (b - a + 1 > maxSpan) {
        throw new Error(`Range "${t}" is wider than ${maxSpan} issues.`);
      }
      for (let n = a; n <= b; n++) out.add(n);
    } else if (/^\d+$/.test(t) && Number(t) > 0) {
      out.add(Number(t));
    } else {
      throw new Error(`"${t}" is not an issue number or range.`);
    }
  }
  return [...out].sort((x, y) => x - y);
}

/** Compact a sorted list of numbers back into `836-850, 900` form. */
export function formatIssueList(nums: number[]): string {
  if (nums.length === 0) return "(none)";
  const sorted = [...nums].sort((a, b) => a - b);
  const parts: string[] = [];
  let start = sorted[0];
  let prev = sorted[0];
  for (const n of sorted.slice(1).concat(Number.NaN)) {
    if (n === prev + 1) {
      prev = n;
      continue;
    }
    parts.push(start === prev ? `#${start}` : `#${start}-#${prev}`);
    start = n;
    prev = n;
  }
  return parts.join(", ");
}

/** True when `login` posted a Drips Wave application comment. */
export function hasWaveApplication(
  comments: readonly IssueComment[],
  login: string
): boolean {
  const who = login.toLowerCase();
  return comments.some(
    (c) => c.author.toLowerCase() === who && c.body.includes(WAVE_MARKER)
  );
}

/** Role gate: every required role, plus one of the any-of roles if set. */
export function meetsRoleRules(
  memberRoles: ReadonlySet<string>,
  settings: Pick<AssignSettings, "requiredRoles" | "anyOfRoles">
): boolean {
  if (!settings.requiredRoles.every((r) => memberRoles.has(r))) return false;
  if (settings.anyOfRoles.length === 0) return true;
  return settings.anyOfRoles.some((r) => memberRoles.has(r));
}

export interface ApplicationInput {
  discordId: string;
  channelId: string;
  memberRoles: ReadonlySet<string>;
  content: string;
}

export interface AssignDeps {
  repo: string;
  getSettings(): AssignSettings;
  getLink(discordId: string): Link | null;
  isInPool(issue: number): boolean;
  getGrant(issue: number): AssignGrant | null;
  listGrantsFor(discordId: string): AssignGrant[];
  claimIssue(issue: number, discordId: string, login: string): boolean;
  releaseIssue(issue: number): boolean;
  getIssue(repo: string, issue: number): Promise<IssueInfo | null>;
  listIssueComments(repo: string, issue: number): Promise<IssueComment[]>;
  addAssignee(repo: string, issue: number, login: string): Promise<string[]>;
}

export type IssueOutcome =
  | { issue: number; ok: true; url: string }
  | { issue: number; ok: false; reason: string };

export type ApplicationResult =
  /** Not an application (wrong channel, no issue numbers, feature unset). */
  | { kind: "ignore" }
  /** The whole request was turned down; nothing was assigned. */
  | { kind: "declined"; reason: string }
  /** Per-issue results; some may have succeeded. */
  | { kind: "processed"; outcomes: IssueOutcome[] };

/**
 * Count the issues the bot gave this member that are still open and still
 * assigned to them. Grants that are no longer live (issue closed, or the
 * member was unassigned) are pruned so they stop counting toward the cap.
 */
export async function activeGrantCount(
  discordId: string,
  deps: AssignDeps
): Promise<number> {
  let active = 0;
  for (const g of deps.listGrantsFor(discordId)) {
    const issue = await deps.getIssue(deps.repo, g.issueNumber);
    const stillTheirs =
      issue !== null &&
      issue.state === "open" &&
      issue.assignees.some(
        (a) => a.toLowerCase() === g.githubLogin.toLowerCase()
      );
    if (stillTheirs) active++;
    else deps.releaseIssue(g.issueNumber);
  }
  return active;
}

export async function processApplication(
  input: ApplicationInput,
  deps: AssignDeps
): Promise<ApplicationResult> {
  const settings = deps.getSettings();
  if (!settings.channelId || input.channelId !== settings.channelId) {
    return { kind: "ignore" };
  }
  const requested = parseIssueRefs(input.content);
  if (requested.length === 0) return { kind: "ignore" };

  if (!settings.enabled) {
    return {
      kind: "declined",
      reason: "Applications are closed right now.",
    };
  }
  if (!meetsRoleRules(input.memberRoles, settings)) {
    return {
      kind: "declined",
      reason: "You don't have the roles needed to apply for issues.",
    };
  }
  const link = deps.getLink(input.discordId);
  if (!link) {
    return {
      kind: "declined",
      reason:
        "Your GitHub isn't linked yet. Run `/connect` first, then try again.",
    };
  }
  if (requested.length > MAX_ISSUES_PER_MESSAGE) {
    return {
      kind: "declined",
      reason: `That's more than ${MAX_ISSUES_PER_MESSAGE} issues in one message.`,
    };
  }

  const active = await activeGrantCount(input.discordId, deps);
  if (active + requested.length > settings.maxPerUser) {
    const left = Math.max(0, settings.maxPerUser - active);
    return {
      kind: "declined",
      reason:
        `You asked for ${requested.length} issue${requested.length === 1 ? "" : "s"} ` +
        `but the limit is ${settings.maxPerUser} per person and you already have ${active}. ` +
        `You can ask for ${left} more. Nothing was assigned.`,
    };
  }

  const outcomes: IssueOutcome[] = [];
  for (const n of requested) {
    outcomes.push(await assignOne(n, input.discordId, link.githubLogin, deps));
  }
  return { kind: "processed", outcomes };
}

async function assignOne(
  n: number,
  discordId: string,
  login: string,
  deps: AssignDeps
): Promise<IssueOutcome> {
  const fail = (reason: string): IssueOutcome => ({ issue: n, ok: false, reason });

  if (!deps.isInPool(n)) return fail("isn't open for applications");

  const issue = await deps.getIssue(deps.repo, n);
  if (!issue || issue.isPullRequest) return fail("isn't an issue in this repo");
  if (issue.state !== "open") return fail("is closed");

  const mine = issue.assignees.some(
    (a) => a.toLowerCase() === login.toLowerCase()
  );
  if (mine) return fail("is already assigned to you");
  if (issue.assignees.length > 0) return fail("is already taken");

  const grant = deps.getGrant(n);
  if (grant && grant.discordId !== discordId) return fail("is already taken");
  // A stale grant for this same member (e.g. they were unassigned) is reused.
  if (grant) deps.releaseIssue(n);

  const comments = await deps.listIssueComments(deps.repo, n);
  if (!hasWaveApplication(comments, login)) {
    return fail(
      `has no Drips Wave application from \`${login}\`. Apply on Drips Wave first`
    );
  }

  if (!deps.claimIssue(n, discordId, login)) return fail("is already taken");
  try {
    const assignees = await deps.addAssignee(deps.repo, n, login);
    if (!assignees.some((a) => a.toLowerCase() === login.toLowerCase())) {
      deps.releaseIssue(n);
      return fail("couldn't be assigned on GitHub");
    }
  } catch (err) {
    deps.releaseIssue(n);
    console.warn(`assign: GitHub assignment of #${n} to ${login} failed`, err);
    return fail("couldn't be assigned on GitHub (try again later)");
  }
  return { issue: n, ok: true, url: issue.url };
}

/** Render a result as a short Discord reply. */
export function formatApplicationReply(
  result: Exclude<ApplicationResult, { kind: "ignore" }>
): string {
  if (result.kind === "declined") return `❌ ${result.reason}`;
  return result.outcomes
    .map((o) =>
      o.ok
        ? `✅ #${o.issue} is now assigned to you: <${o.url}>`
        : `❌ #${o.issue} ${o.reason}.`
    )
    .join("\n");
}
