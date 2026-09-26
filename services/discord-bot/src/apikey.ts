/**
 * Per-user Sorolens API keys.
 *
 * Acceptance criterion: commands must run under the caller's own identity,
 * using a key stored by the existing OAuth linking flow (see `oauth.ts`).
 *
 * Flow: when a contributor completes `/connect`, the bot mints a key scoped
 * to the bot's read surfaces via the API's own `POST /api/v1/api-keys`
 * endpoint and stores the plaintext key against the Discord id. `/unlink`
 * revokes it. Keys are never logged or echoed back to Discord.
 *
 * The store and the API client are injected so the provisioning logic can be
 * unit tested without a real database or network.
 */

import crypto from "node:crypto";

/** A key that has been minted for a Discord user and persisted locally. */
export interface StoredApiKey {
  discordId: string;
  key: string;
  keyId: string | null;
  createdAt: string;
}

/** The subset of the key store this module needs. */
export interface ApiKeyStore {
  get(discordId: string): StoredApiKey | null;
  set(discordId: string, key: string, keyId: string | null): StoredApiKey;
  clear(discordId: string): boolean;
}

/** The subset of the Sorolens API this module needs. */
export interface ApiKeyMinter {
  provisionApiKey(name: string, scopes?: string[]): Promise<{ id: string; key: string }>;
  revokeApiKey(id: string): Promise<void>;
}

/** Generate a fresh `sl_`-prefixed key, matching the API's token format. */
export function mintApiKey(): string {
  return "sl_" + crypto.randomBytes(32).toString("base64url");
}

/** Stable, human-readable name for a provisioned key. */
export function keyNameFor(discordId: string, githubLogin: string): string {
  return `discord:${discordId}:${githubLogin}`;
}

/** Return the stored key or null. Never provisions. */
export function getUserApiKey(store: ApiKeyStore, discordId: string): StoredApiKey | null {
  return store.get(discordId);
}

/**
 * Ensure the Discord user has a Sorolens API key on file.
 *
 * Idempotent: an existing key is returned untouched. Provisioning failures
 * are swallowed and reported as `null` so a transient API outage can never
 * break `/connect` — the contributor stays linked and simply inherits the
 * bot's shared key until they run the command again.
 */
export async function ensureUserApiKey(
  store: ApiKeyStore,
  minter: ApiKeyMinter,
  discordId: string,
  githubLogin: string,
): Promise<StoredApiKey | null> {
  const existing = store.get(discordId);
  if (existing) return existing;

  try {
    const grant = await minter.provisionApiKey(keyNameFor(discordId, githubLogin));
    return store.set(discordId, grant.key, grant.id ?? null);
  } catch {
    return null;
  }
}

/**
 * Revoke the user's key (on `/unlink`). Best effort: the local row is always
 * cleared, and an API failure is reported via the return value so callers can
 * log it.
 */
export async function revokeUserApiKey(
  store: ApiKeyStore,
  minter: ApiKeyMinter,
  discordId: string,
): Promise<{ revoked: boolean; cleared: boolean }> {
  const existing = store.get(discordId);
  const cleared = store.clear(discordId);
  if (!existing?.keyId) return { revoked: false, cleared };

  try {
    await minter.revokeApiKey(existing.keyId);
    return { revoked: true, cleared };
  } catch {
    return { revoked: false, cleared };
  }
}
