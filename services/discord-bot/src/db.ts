/**
 * SQLite-backed mapping between Discord user ids and GitHub usernames.
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
  `);
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
    .prepare("SELECT discord_id FROM links WHERE github_login = ? COLLATE NOCASE")
    .get(githubLogin) as { discord_id: string } | undefined;
  if (existing && existing.discord_id !== discordId) {
    throw new Error(
      `github login "${githubLogin}" is already linked to a different Discord user`,
    );
  }
  conn
    .prepare(
      `INSERT INTO links (discord_id, github_login) VALUES (?, ?)
       ON CONFLICT(discord_id) DO UPDATE SET
         github_login = excluded.github_login,
         linked_at    = datetime('now')`,
    )
    .run(discordId, githubLogin);
  return getByDiscord(discordId)!;
}

export function unlink(discordId: string): boolean {
  const conn = requireDb();
  const r = conn.prepare("DELETE FROM links WHERE discord_id = ?").run(discordId);
  return r.changes > 0;
}

export function getByDiscord(discordId: string): Link | null {
  const conn = requireDb();
  const row = conn
    .prepare(
      "SELECT discord_id AS discordId, github_login AS githubLogin, linked_at AS linkedAt FROM links WHERE discord_id = ?",
    )
    .get(discordId) as Link | undefined;
  return row ?? null;
}

export function getByGithub(githubLogin: string): Link | null {
  const conn = requireDb();
  const row = conn
    .prepare(
      "SELECT discord_id AS discordId, github_login AS githubLogin, linked_at AS linkedAt FROM links WHERE github_login = ? COLLATE NOCASE",
    )
    .get(githubLogin) as Link | undefined;
  return row ?? null;
}
