/**
 * GitHub OAuth flow that lets a Discord member connect their GitHub
 * account in one click, no typing.
 *
 * Flow:
 *   1. Bot generates a personalised `/oauth/start` URL for the Discord user
 *      when they run the `/connect` slash command. The URL carries the
 *      Discord id + an HMAC signature so a stranger cannot forge Discord
 *      ids.
 *   2. User clicks the link. `/oauth/start` verifies the HMAC and redirects
 *      them to GitHub's OAuth consent page (scope `read:user`).
 *   3. GitHub redirects back to `/oauth/callback` with a code + our state.
 *   4. `/oauth/callback` verifies the state, exchanges the code for a
 *      short-lived access token, reads the user's GitHub login, stores
 *      the mapping in SQLite via `upsertLink`, and immediately syncs the
 *      member's Discord role.
 *
 * The signed state is short-lived (10 min). The GitHub access token is
 * only used to read the authenticated user's login; it is not stored.
 */

import crypto from "node:crypto";
import type { Client } from "discord.js";
import type { Application, Request, Response } from "express";
import { apiKeyStore, upsertLink } from "./db.js";
import { syncOnMerge } from "./commands.js";
import { SorolensApi } from "./api.js";
import { ensureUserApiKey } from "./apikey.js";
import type { Tiers } from "./roles.js";
import type { Config } from "./config.js";

const STATE_TTL_MS = 10 * 60 * 1000;

export interface OauthDeps {
  client: Client;
  config: Config;
  tiers: Tiers;
}

/** HMAC of a Discord id under the state secret. Used to authenticate
 *  the `discord_id` query parameter on `/oauth/start` so an attacker
 *  cannot start a flow for someone else's Discord id. */
export function signDiscordId(discordId: string, secret: string): string {
  return crypto.createHmac("sha256", secret).update(discordId).digest("hex");
}

function timingSafeHexEqual(a: string, b: string): boolean {
  if (a.length !== b.length) return false;
  try {
    return crypto.timingSafeEqual(Buffer.from(a, "hex"), Buffer.from(b, "hex"));
  } catch {
    return false;
  }
}

function verifySig(discordId: string, sig: string, secret: string): boolean {
  const expected = signDiscordId(discordId, secret);
  return timingSafeHexEqual(sig, expected);
}

function signState(discordId: string, secret: string): string {
  const ts = Date.now();
  const payload = `${discordId}.${ts}`;
  const mac = crypto.createHmac("sha256", secret).update(payload).digest("hex");
  return Buffer.from(`${payload}.${mac}`).toString("base64url");
}

function verifyState(
  state: string,
  secret: string,
): { discordId: string; ts: number } | null {
  try {
    const decoded = Buffer.from(state, "base64url").toString("utf8");
    const parts = decoded.split(".");
    if (parts.length !== 3) return null;
    const [discordId, tsStr, mac] = parts;
    const expected = crypto
      .createHmac("sha256", secret)
      .update(`${discordId}.${tsStr}`)
      .digest("hex");
    if (!timingSafeHexEqual(mac, expected)) return null;
    const ts = parseInt(tsStr, 10);
    if (!Number.isFinite(ts) || Date.now() - ts > STATE_TTL_MS) return null;
    return { discordId, ts };
  } catch {
    return null;
  }
}

/** Build the `/oauth/start` URL for a given Discord user. Include this
 *  URL in DMs or slash-command replies. */
export function buildConnectUrl(discordId: string, config: Config): string {
  const sig = signDiscordId(discordId, config.oauthStateSecret);
  return `${config.publicBaseUrl}/oauth/start?discord_id=${discordId}&sig=${sig}`;
}

function githubAuthorizeUrl(discordId: string, config: Config): string {
  const state = signState(discordId, config.oauthStateSecret);
  const params = new URLSearchParams({
    client_id: config.githubOauthClientId,
    redirect_uri: `${config.publicBaseUrl}/oauth/callback`,
    scope: "read:user",
    state,
    allow_signup: "true",
  });
  return `https://github.com/login/oauth/authorize?${params.toString()}`;
}

async function exchangeCodeForToken(
  code: string,
  config: Config,
): Promise<string> {
  const r = await fetch("https://github.com/login/oauth/access_token", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json",
    },
    body: JSON.stringify({
      client_id: config.githubOauthClientId,
      client_secret: config.githubOauthClientSecret,
      code,
    }),
  });
  if (!r.ok) throw new Error(`github token exchange returned ${r.status}`);
  const body = (await r.json()) as { access_token?: string; error?: string };
  if (!body.access_token) {
    throw new Error(`github token exchange: ${body.error ?? "no access_token"}`);
  }
  return body.access_token;
}

async function fetchGithubLogin(token: string): Promise<string> {
  const r = await fetch("https://api.github.com/user", {
    headers: {
      Authorization: `Bearer ${token}`,
      "User-Agent": "sorolens-discord-bot",
      Accept: "application/vnd.github+json",
    },
  });
  if (!r.ok) throw new Error(`github /user returned ${r.status}`);
  const body = (await r.json()) as { login?: string };
  if (!body.login) throw new Error("github /user returned no login");
  return body.login;
}

export function mountOauthRoutes(app: Application, deps: OauthDeps): void {
  app.get("/oauth/start", (req: Request, res: Response) => {
    const discordId = String(req.query.discord_id ?? "").trim();
    const sig = String(req.query.sig ?? "").trim();
    if (!discordId || !sig) {
      return res.status(400).send(errorPage("Missing parameters."));
    }
    if (!verifySig(discordId, sig, deps.config.oauthStateSecret)) {
      return res.status(400).send(errorPage("Invalid link signature."));
    }
    res.redirect(githubAuthorizeUrl(discordId, deps.config));
  });

  app.get("/oauth/callback", async (req: Request, res: Response) => {
    const code = String(req.query.code ?? "").trim();
    const state = String(req.query.state ?? "").trim();
    if (!code || !state) {
      return res.status(400).send(errorPage("Missing parameters."));
    }
    const parsed = verifyState(state, deps.config.oauthStateSecret);
    if (!parsed) {
      return res
        .status(400)
        .send(errorPage("Invalid or expired link. Run /connect in Discord to get a fresh one."));
    }

    let login: string;
    try {
      const token = await exchangeCodeForToken(code, deps.config);
      login = await fetchGithubLogin(token);
    } catch (err) {
      console.error("oauth callback: token/user fetch failed", err);
      return res
        .status(502)
        .send(errorPage("GitHub did not return your identity. Try again in a minute."));
    }

    try {
      upsertLink(parsed.discordId, login);
    } catch (err) {
      const msg =
        err instanceof Error
          ? err.message
          : "This GitHub account is already linked to another Discord user.";
      return res.status(409).send(errorPage(msg));
    }

    // Mint (or reuse) the contributor's own Sorolens API key so the
    // /status, /alerts, /watch and /unwatch commands run under their
    // identity. Best effort - the link is what gates access.
    try {
      const api = new SorolensApi({
        baseUrl: deps.config.sorolensApiBaseUrl,
        apiKey: deps.config.sorolensAdminApiKey,
      });
      const granted = await ensureUserApiKey(apiKeyStore, api, parsed.discordId, login);
      if (!granted) {
        console.warn("oauth: per-user API key not provisioned for", parsed.discordId);
      }
    } catch (err) {
      console.warn("oauth: API key provisioning failed", err);
    }

    try {
      const guild = await deps.client.guilds.fetch(deps.config.discordGuildId);

      // Grant the Verified marker role if configured. This is the gate
      // that unlocks community channels for members who have proven
      // ownership of a GitHub account via OAuth. Never revoked.
      if (deps.config.roleVerified) {
        const member = await guild.members
          .fetch(parsed.discordId)
          .catch(() => null);
        if (member && !member.roles.cache.has(deps.config.roleVerified)) {
          await member.roles
            .add(deps.config.roleVerified, "sorolens-bot: OAuth linked")
            .catch((e) => console.warn("oauth: could not grant Verified", e));
        }
      }

      const result = await syncOnMerge(guild, parsed.discordId, login, {
        config: deps.config,
        tiers: deps.tiers,
      });
      console.log("oauth: sync result", result);
    } catch (err) {
      console.warn("oauth: role sync failed", err);
    }

    res.send(successPage(login));
  });
}

function successPage(login: string): string {
  return htmlPage(
    "Linked",
    `<h1>You're linked ✓</h1>
     <p>Discord is now connected to GitHub <strong>@${escapeHtml(login)}</strong>.</p>
     <p>Your <strong>Contributor</strong> or <strong>Core Contributor</strong> role updates automatically as PRs get merged in <code>sorolens/sorolens</code>.</p>
     <p>You can close this tab and head back to Discord.</p>`,
  );
}

function errorPage(message: string): string {
  return htmlPage(
    "Link failed",
    `<h1>Link failed</h1>
     <p>${escapeHtml(message)}</p>
     <p>Run <code>/connect</code> in the Sorolens Discord to get a fresh link.</p>`,
  );
}

function htmlPage(title: string, inner: string): string {
  return `<!doctype html>
<html lang="en"><head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>${escapeHtml(title)} | Sorolens</title>
<style>
  :root { color-scheme: dark; }
  body {
    margin: 0; padding: 48px 24px;
    font: 16px/1.55 system-ui, -apple-system, "Segoe UI", sans-serif;
    background: #0f172a; color: #e2e8f0;
  }
  main { max-width: 480px; margin: 0 auto; }
  h1 { color: #06b6d4; margin-top: 0; font-weight: 700; }
  p { margin: 16px 0; }
  a { color: #06b6d4; }
  code { background: #1e293b; padding: 2px 6px; border-radius: 4px; font-size: 0.9em; }
  strong { color: #f8fafc; }
</style>
</head><body><main>${inner}</main></body></html>`;
}

function escapeHtml(s: string): string {
  return s.replace(/[&<>"']/g, (c) =>
    c === "&"
      ? "&amp;"
      : c === "<"
        ? "&lt;"
        : c === ">"
          ? "&gt;"
          : c === '"'
            ? "&quot;"
            : "&#39;",
  );
}
