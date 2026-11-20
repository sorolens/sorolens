/**
 * SQLite-backed mapping between Discord user ids and GitHub usernames, plus
 * the per-user Sorolens API keys minted when a contributor links.
 *
 * The database is a single file so operators can copy it between
 * deployments without a heavier data layer. Two contributors can not
 * claim the same GitHub username: the UNIQUE constraint means the
 * second /link attempt fails cleanly.
 */

import Database from "better-sqlite3";
import { mkdirSync } from "node:fs";
import { dirname } from "node:path";

export interface Link {
  discordId: string;
  githubLogin: string;
  linkedAt: string;
}

/** Per-user Sorolens API key persisted for a linked contributor. */
export interface UserApiKey {
  discordId: string;
  key: string;
  keyId: string | null;
  createdAt: string;
}

let db: Database.Database | null = null;

export function openDb(path: string): Database.Database {
  mkdirSync(dirname(path), { recursive: true });
  const conn = new Database(path);
  conn.pragma("journal_mode = WAL");
  conn.exec(`
    CREATE TABLE IF NOT EXISTS links (
      discord_id   TEXT PRIMARY KEY,
      github_login TEXT NOT NULL UNIQUE COLLATE NOCASE,
      linked_at    TEXT NOT NULL DEFAULT (datetime('now'))
    );

    CREATE TABLE IF NOT EXISTS user_api_keys (
      discord_id TEXT PRIMARY KEY,
      api_key    TEXT NOT NULL,
      key_id     TEXT,
      created_at TEXT NOT NULL DEFAULT (datetime('now'))
    );

    -- Issue self-assign: owner-controlled settings (single row, id = 1).
    CREATE TABLE IF NOT EXISTS assign_settings (
      id              INTEGER PRIMARY KEY CHECK (id = 1),
      enabled         INTEGER NOT NULL DEFAULT 0,
      channel_id      TEXT,
      required_roles  TEXT NOT NULL DEFAULT '[]',
      any_of_roles    TEXT NOT NULL DEFAULT '[]',
      max_per_user    INTEGER NOT NULL DEFAULT 1
    );

    -- Issues the owner has opened up for self-assignment.
    CREATE TABLE IF NOT EXISTS assign_pool (
      issue_number INTEGER PRIMARY KEY,
      added_at     TEXT NOT NULL DEFAULT (datetime('now'))
    );

    -- Issues the bot has handed out. The PRIMARY KEY doubles as the lock
    -- that stops two members claiming the same issue at the same time.
    CREATE TABLE IF NOT EXISTS assign_grants (
      issue_number INTEGER PRIMARY KEY,
      discord_id   TEXT NOT NULL,
      github_login TEXT NOT NULL,
      assigned_at  TEXT NOT NULL DEFAULT (datetime('now'))
    );
    CREATE INDEX IF NOT EXISTS assign_grants_by_user ON assign_grants (discord_id);

    -- Community and rewards settings (single row, id = 1).
    CREATE TABLE IF NOT EXISTS community_settings (
      id                INTEGER PRIMARY KEY CHECK (id = 1),
      channel_id        TEXT,
      congrats_channel_id TEXT,
      congrats_enabled  INTEGER NOT NULL DEFAULT 1,
      cotw_enabled      INTEGER NOT NULL DEFAULT 1,
      cotw_role_id      TEXT,
      cotw_holder_id    TEXT,
      cotw_last_week    TEXT
    );

    -- Merged-PR milestones and the role each one grants.
    CREATE TABLE IF NOT EXISTS milestones (
      pr_count INTEGER PRIMARY KEY,
      role_id  TEXT NOT NULL
    );

    -- Channel alert subscriptions: channels subscribed to watchdog alerts for contracts.
    CREATE TABLE IF NOT EXISTS alert_subscriptions (
      channel_id  TEXT NOT NULL,
      contract_id TEXT NOT NULL,
      severity    TEXT NOT NULL DEFAULT 'all',
      created_at  TEXT NOT NULL DEFAULT (datetime('now')),
      updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
      PRIMARY KEY (channel_id, contract_id)
    );
    CREATE INDEX IF NOT EXISTS alert_subs_contract ON alert_subscriptions (contract_id);
  `);
  // Added after the first release: when applications were last switched on
  // (ms since epoch). Messages sent before it are ignored.
  const cols = conn.prepare("PRAGMA table_info(assign_settings)").all() as {
    name: string;
  }[];
  if (!cols.some((c) => c.name === "opened_at")) {
    conn.exec("ALTER TABLE assign_settings ADD COLUMN opened_at INTEGER");
  }
  db = conn;
  return conn;
}

export function requireDb(): Database.Database {
  if (!db) throw new Error("db not opened; call openDb() first");
  return db;
}

/** Create or replace a link. Throws if the GitHub login is already claimed by someone else. */
export function upsertLink(discordId: string, githubLogin: string): Link {
  const conn = requireDb();
  const existing = conn
    .prepare(
      "SELECT discord_id FROM links WHERE github_login = ? COLLATE NOCASE"
    )
    .get(githubLogin) as { discord_id: string } | undefined;
  if (existing && existing.discord_id !== discordId) {
    throw new Error(
      `github login "${githubLogin}" is already linked to a different Discord user`
    );
  }
  conn
    .prepare(
      `INSERT INTO links (discord_id, github_login) VALUES (?, ?)
       ON CONFLICT(discord_id) DO UPDATE SET
         github_login = excluded.github_login,
         linked_at    = datetime('now')`
    )
    .run(discordId, githubLogin);
  return getByDiscord(discordId)!;
}

export function unlink(discordId: string): boolean {
  const conn = requireDb();
  const r = conn
    .prepare("DELETE FROM links WHERE discord_id = ?")
    .run(discordId);
  // The API key is useless without the link, and leaving it around would let
  // a re-linked account keep the previous owner's credential.
  conn.prepare("DELETE FROM user_api_keys WHERE discord_id = ?").run(discordId);
  return r.changes > 0;
}

export function getByDiscord(discordId: string): Link | null {
  const conn = requireDb();
  const row = conn
    .prepare(
      "SELECT discord_id AS discordId, github_login AS githubLogin, linked_at AS linkedAt FROM links WHERE discord_id = ?"
    )
    .get(discordId) as Link | undefined;
  return row ?? null;
}

export function getByGithub(githubLogin: string): Link | null {
  const conn = requireDb();
  const row = conn
    .prepare(
      "SELECT discord_id AS discordId, github_login AS githubLogin, linked_at AS linkedAt FROM links WHERE github_login = ? COLLATE NOCASE"
    )
    .get(githubLogin) as Link | undefined;
  return row ?? null;
}

export function listLinks(limit = 100, offset = 0): Link[] {
  const conn = requireDb();
  return conn
    .prepare(
      "SELECT discord_id AS discordId, github_login AS githubLogin, linked_at AS linkedAt FROM links ORDER BY linked_at DESC LIMIT ? OFFSET ?"
    )
    .all(limit, offset) as Link[];
}

// ---- per-user Sorolens API keys -------------------------------------------

/** Create or replace the Sorolens API key stored for a Discord user. */
export function setApiKey(
  discordId: string,
  key: string,
  keyId: string | null = null
): UserApiKey {
  const conn = requireDb();
  conn
    .prepare(
      `INSERT INTO user_api_keys (discord_id, api_key, key_id) VALUES (?, ?, ?)
       ON CONFLICT(discord_id) DO UPDATE SET
         api_key    = excluded.api_key,
         key_id     = excluded.key_id,
         created_at = datetime('now')`
    )
    .run(discordId, key, keyId);
  return getApiKey(discordId)!;
}

export function getApiKey(discordId: string): UserApiKey | null {
  const conn = requireDb();
  const row = conn
    .prepare(
      "SELECT discord_id AS discordId, api_key AS key, key_id AS keyId, created_at AS createdAt FROM user_api_keys WHERE discord_id = ?"
    )
    .get(discordId) as UserApiKey | undefined;
  return row ?? null;
}

export function clearApiKey(discordId: string): boolean {
  const conn = requireDb();
  const r = conn
    .prepare("DELETE FROM user_api_keys WHERE discord_id = ?")
    .run(discordId);
  return r.changes > 0;
}

/**
 * Adapter surface consumed by `apikey.ts`, so provisioning code can be tested
 * against an in-memory implementation.
 */
export const apiKeyStore = {
  get: getApiKey,
  set: (discordId: string, key: string, keyId: string | null) =>
    setApiKey(discordId, key, keyId),
  clear: clearApiKey,
};

// ---- issue self-assign ------------------------------------------------------

export interface AssignSettings {
  enabled: boolean;
  channelId: string | null;
  /** Member must hold every one of these roles. */
  requiredRoles: string[];
  /** Member must hold at least one of these roles (ignored when empty). */
  anyOfRoles: string[];
  maxPerUser: number;
  /** When applications were last switched on (ms since epoch). */
  openedAt: number | null;
}

export interface AssignGrant {
  issueNumber: number;
  discordId: string;
  githubLogin: string;
  assignedAt: string;
}

export function getAssignSettings(): AssignSettings {
  const conn = requireDb();
  const row = conn
    .prepare(
      "SELECT enabled, channel_id, required_roles, any_of_roles, max_per_user, opened_at FROM assign_settings WHERE id = 1"
    )
    .get() as
    | {
        enabled: number;
        channel_id: string | null;
        required_roles: string;
        any_of_roles: string;
        max_per_user: number;
        opened_at: number | null;
      }
    | undefined;
  if (!row) {
    return {
      enabled: false,
      channelId: null,
      requiredRoles: [],
      anyOfRoles: [],
      maxPerUser: 1,
      openedAt: null,
    };
  }
  return {
    enabled: row.enabled === 1,
    channelId: row.channel_id,
    requiredRoles: JSON.parse(row.required_roles) as string[],
    anyOfRoles: JSON.parse(row.any_of_roles) as string[],
    maxPerUser: row.max_per_user,
    openedAt: row.opened_at,
  };
}

export function updateAssignSettings(
  patch: Partial<AssignSettings>
): AssignSettings {
  const next = { ...getAssignSettings(), ...patch };
  requireDb()
    .prepare(
      `INSERT INTO assign_settings (id, enabled, channel_id, required_roles, any_of_roles, max_per_user, opened_at)
       VALUES (1, ?, ?, ?, ?, ?, ?)
       ON CONFLICT(id) DO UPDATE SET
         enabled        = excluded.enabled,
         channel_id     = excluded.channel_id,
         required_roles = excluded.required_roles,
         any_of_roles   = excluded.any_of_roles,
         max_per_user   = excluded.max_per_user,
         opened_at      = excluded.opened_at`
    )
    .run(
      next.enabled ? 1 : 0,
      next.channelId,
      JSON.stringify(next.requiredRoles),
      JSON.stringify(next.anyOfRoles),
      next.maxPerUser,
      next.openedAt
    );
  return next;
}

/** Add issues to the pool. Returns how many were new. */
export function addToPool(issues: number[]): number {
  const conn = requireDb();
  const stmt = conn.prepare(
    "INSERT OR IGNORE INTO assign_pool (issue_number) VALUES (?)"
  );
  let added = 0;
  conn.transaction(() => {
    for (const n of issues) added += stmt.run(n).changes;
  })();
  return added;
}

/** Remove issues from the pool. Returns how many were removed. */
export function removeFromPool(issues: number[]): number {
  const conn = requireDb();
  const stmt = conn.prepare("DELETE FROM assign_pool WHERE issue_number = ?");
  let removed = 0;
  conn.transaction(() => {
    for (const n of issues) removed += stmt.run(n).changes;
  })();
  return removed;
}

export function clearPool(): number {
  return requireDb().prepare("DELETE FROM assign_pool").run().changes;
}

export function listPool(): number[] {
  return (
    requireDb()
      .prepare("SELECT issue_number AS n FROM assign_pool ORDER BY issue_number")
      .all() as { n: number }[]
  ).map((r) => r.n);
}

export function isInPool(issue: number): boolean {
  return (
    requireDb()
      .prepare("SELECT 1 FROM assign_pool WHERE issue_number = ?")
      .get(issue) !== undefined
  );
}

/**
 * Atomically reserve an issue for a member. Returns false when the issue
 * is already reserved (by anyone), so concurrent requests cannot both win.
 */
export function claimIssue(
  issue: number,
  discordId: string,
  githubLogin: string
): boolean {
  const r = requireDb()
    .prepare(
      "INSERT OR IGNORE INTO assign_grants (issue_number, discord_id, github_login) VALUES (?, ?, ?)"
    )
    .run(issue, discordId, githubLogin);
  return r.changes > 0;
}

/** Drop a reservation (used when the GitHub assignment fails, or is released). */
export function releaseIssue(issue: number): boolean {
  return (
    requireDb()
      .prepare("DELETE FROM assign_grants WHERE issue_number = ?")
      .run(issue).changes > 0
  );
}

export function getGrant(issue: number): AssignGrant | null {
  const row = requireDb()
    .prepare(
      "SELECT issue_number AS issueNumber, discord_id AS discordId, github_login AS githubLogin, assigned_at AS assignedAt FROM assign_grants WHERE issue_number = ?"
    )
    .get(issue) as AssignGrant | undefined;
  return row ?? null;
}

export function listGrantsFor(discordId: string): AssignGrant[] {
  return requireDb()
    .prepare(
      "SELECT issue_number AS issueNumber, discord_id AS discordId, github_login AS githubLogin, assigned_at AS assignedAt FROM assign_grants WHERE discord_id = ? ORDER BY issue_number"
    )
    .all(discordId) as AssignGrant[];
}

export function listAllGrants(): AssignGrant[] {
  return requireDb()
    .prepare(
      "SELECT issue_number AS issueNumber, discord_id AS discordId, github_login AS githubLogin, assigned_at AS assignedAt FROM assign_grants ORDER BY issue_number"
    )
    .all() as AssignGrant[];
}

// ---- community and rewards --------------------------------------------------

export interface CommunitySettings {
  /** Where the contributor of the week is posted. */
  channelId: string | null;
  /** Where merge congratulations are posted. */
  congratsChannelId: string | null;
  congratsEnabled: boolean;
  cotwEnabled: boolean;
  cotwRoleId: string | null;
  /** Discord id currently holding the contributor-of-the-week role. */
  cotwHolderId: string | null;
  /** Week key (Sunday date) last announced. */
  cotwLastWeek: string | null;
}

export function getCommunitySettings(): CommunitySettings {
  const row = requireDb()
    .prepare(
      "SELECT channel_id, congrats_channel_id, congrats_enabled, cotw_enabled, cotw_role_id, cotw_holder_id, cotw_last_week FROM community_settings WHERE id = 1"
    )
    .get() as
    | {
        channel_id: string | null;
        congrats_channel_id: string | null;
        congrats_enabled: number;
        cotw_enabled: number;
        cotw_role_id: string | null;
        cotw_holder_id: string | null;
        cotw_last_week: string | null;
      }
    | undefined;
  return {
    channelId: row?.channel_id ?? null,
    congratsChannelId: row?.congrats_channel_id ?? null,
    congratsEnabled: row ? row.congrats_enabled === 1 : true,
    cotwEnabled: row ? row.cotw_enabled === 1 : true,
    cotwRoleId: row?.cotw_role_id ?? null,
    cotwHolderId: row?.cotw_holder_id ?? null,
    cotwLastWeek: row?.cotw_last_week ?? null,
  };
}

export function updateCommunitySettings(
  patch: Partial<CommunitySettings>
): CommunitySettings {
  const n = { ...getCommunitySettings(), ...patch };
  requireDb()
    .prepare(
      `INSERT INTO community_settings (id, channel_id, congrats_channel_id, congrats_enabled, cotw_enabled, cotw_role_id, cotw_holder_id, cotw_last_week)
       VALUES (1, ?, ?, ?, ?, ?, ?, ?)
       ON CONFLICT(id) DO UPDATE SET
         channel_id       = excluded.channel_id,
         congrats_channel_id = excluded.congrats_channel_id,
         congrats_enabled = excluded.congrats_enabled,
         cotw_enabled     = excluded.cotw_enabled,
         cotw_role_id     = excluded.cotw_role_id,
         cotw_holder_id   = excluded.cotw_holder_id,
         cotw_last_week   = excluded.cotw_last_week`
    )
    .run(
      n.channelId,
      n.congratsChannelId,
      n.congratsEnabled ? 1 : 0,
      n.cotwEnabled ? 1 : 0,
      n.cotwRoleId,
      n.cotwHolderId,
      n.cotwLastWeek
    );
  return n;
}

export function setMilestone(count: number, roleId: string): void {
  requireDb()
    .prepare(
      `INSERT INTO milestones (pr_count, role_id) VALUES (?, ?)
       ON CONFLICT(pr_count) DO UPDATE SET role_id = excluded.role_id`
    )
    .run(count, roleId);
}

export function removeMilestone(count: number): boolean {
  return (
    requireDb().prepare("DELETE FROM milestones WHERE pr_count = ?").run(count)
      .changes > 0
  );
}

export function listMilestones(): { count: number; roleId: string }[] {
  return requireDb()
    .prepare("SELECT pr_count AS count, role_id AS roleId FROM milestones ORDER BY pr_count")
    .all() as { count: number; roleId: string }[];
}

/** Every linked member (no paging), for leaderboards. */
export function allLinks(): Link[] {
  return requireDb()
    .prepare(
      "SELECT discord_id AS discordId, github_login AS githubLogin, linked_at AS linkedAt FROM links"
    )
    .all() as Link[];
}

// ---- alert subscriptions ----------------------------------------------------

export interface Subscription {
  channelId: string;
  contractId: string;
  severity: string;
  createdAt: string;
  updatedAt: string;
}

export function upsertSubscription(
  channelId: string,
  contractId: string,
  severity = "all"
): Subscription {
  const conn = requireDb();
  const normalizedContract = contractId.trim().toUpperCase();
  const normalizedSeverity = severity.trim().toLowerCase();
  conn
    .prepare(
      `INSERT INTO alert_subscriptions (channel_id, contract_id, severity, created_at, updated_at)
       VALUES (?, ?, ?, datetime('now'), datetime('now'))
       ON CONFLICT(channel_id, contract_id) DO UPDATE SET
         severity   = excluded.severity,
         updated_at = datetime('now')`
    )
    .run(channelId, normalizedContract, normalizedSeverity);
  return getSubscription(channelId, normalizedContract)!;
}

export function removeSubscription(
  channelId: string,
  contractId: string
): boolean {
  const conn = requireDb();
  const normalizedContract = contractId.trim().toUpperCase();
  const r = conn
    .prepare(
      "DELETE FROM alert_subscriptions WHERE channel_id = ? AND contract_id = ?"
    )
    .run(channelId, normalizedContract);
  return r.changes > 0;
}

export function getSubscription(
  channelId: string,
  contractId: string
): Subscription | null {
  const conn = requireDb();
  const normalizedContract = contractId.trim().toUpperCase();
  const row = conn
    .prepare(
      `SELECT channel_id AS channelId, contract_id AS contractId, severity, created_at AS createdAt, updated_at AS updatedAt
       FROM alert_subscriptions WHERE channel_id = ? AND contract_id = ?`
    )
    .get(channelId, normalizedContract) as Subscription | undefined;
  return row ?? null;
}

export function listSubscriptionsForChannel(channelId: string): Subscription[] {
  const conn = requireDb();
  return conn
    .prepare(
      `SELECT channel_id AS channelId, contract_id AS contractId, severity, created_at AS createdAt, updated_at AS updatedAt
       FROM alert_subscriptions WHERE channel_id = ? ORDER BY contract_id ASC`
    )
    .all(channelId) as Subscription[];
}

export function listSubscriptionsForContract(contractId: string): Subscription[] {
  const conn = requireDb();
  const normalizedContract = contractId.trim().toUpperCase();
  return conn
    .prepare(
      `SELECT channel_id AS channelId, contract_id AS contractId, severity, created_at AS createdAt, updated_at AS updatedAt
       FROM alert_subscriptions WHERE contract_id = ? ORDER BY channel_id ASC`
    )
    .all(normalizedContract) as Subscription[];
}

export function listAllSubscriptions(): Subscription[] {
  const conn = requireDb();
  return conn
    .prepare(
      `SELECT channel_id AS channelId, contract_id AS contractId, severity, created_at AS createdAt, updated_at AS updatedAt
       FROM alert_subscriptions ORDER BY channel_id ASC, contract_id ASC`
    )
    .all() as Subscription[];
}

export function clearSubscriptionsForChannel(channelId: string): number {
  const conn = requireDb();
  const r = conn
    .prepare("DELETE FROM alert_subscriptions WHERE channel_id = ?")
    .run(channelId);
  return r.changes;
}
