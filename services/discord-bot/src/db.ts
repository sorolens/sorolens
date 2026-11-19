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
