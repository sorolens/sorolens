/**
 * Discord slash command definitions and handlers.
 *
 * All commands are guild-scoped so they only appear inside the Sorolens
 * server. The mapping between Discord user and GitHub login is written
 * by the contributor themselves via `/link github <username>`; the bot
 * verifies the GitHub username actually exists before storing.
 *
 * Query commands (`/status`, `/alerts`, `/watch`, `/unwatch`) answer
 * ephemerally and call the Sorolens API with the caller's own provisioned
 * API key, falling back to the bot's shared key when provisioning is not
 * configured. Every one of them counts against a per-user rate limit.
 */

import type {
  AutocompleteInteraction,
  ChatInputCommandInteraction,
  Client,
  Interaction,
  Guild,
} from "discord.js";
import {
  apiKeyStore,
  getApiKey,
  getByDiscord,
  getByGithub,
  listLinks,
  unlink,
  upsertLink,
  type Link,
} from "./db.js";
import { countMergedPRs, getUser } from "./github.js";
import { syncRoles, type Tiers } from "./roles.js";
import { buildConnectUrl } from "./oauth.js";
import {
  ApiError,
  SorolensApi,
  isValidContractId,
  type ApiCredentials,
  type ContractSearchResult,
} from "./api.js";
import { alertsEmbed, shortId, statusEmbed, watchlistEmbed } from "./embeds.js";
import { describeRetry, SlidingWindowRateLimiter } from "./ratelimit.js";
import { ensureUserApiKey, revokeUserApiKey } from "./apikey.js";
import type { Config } from "./config.js";
import { ASSIGN_COMMANDS, handleAssignCommand } from "./assign-discord.js";
import { COMMUNITY_COMMANDS, handleCommunityCommand } from "./community-discord.js";

// Re-exported from command-definitions.ts so callers that only import
// commands.ts still get the definitions; the split lets the register
// script skip loading the runtime config.
export { commandDefinitions } from "./command-definitions.js";

// ---- runtime handlers ------------------------------------------------------

interface CommandContext {
  config: Config;
  tiers: Tiers;
}

const CONTRACT_AUTOCOMPLETE_COMMANDS = new Set([
  "status",
  "alerts",
  "watch",
  "unwatch",
]);
const AUTOCOMPLETE_LIMIT = 25;
const AUTOCOMPLETE_CACHE_TTL_MS = 5_000;

export interface ContractAutocompleteChoice {
  name: string;
  value: string;
}

/** Convert API search results to Discord choices without exposing full IDs. */
export function contractAutocompleteChoices(
  results: readonly ContractSearchResult[]
): ContractAutocompleteChoice[] {
  const seen = new Set<string>();
  return results
    .filter((result) => {
      if (!result.id || seen.has(result.id)) return false;
      seen.add(result.id);
      return true;
    })
    .slice(0, AUTOCOMPLETE_LIMIT)
    .map((result) => {
      const label = result.label?.trim();
      const id = shortId(result.id);
      const suffix = label ? ` (${id})` : "";
      const maxLabelLength = 100 - suffix.length;
      const displayLabel = label
        ? label.length > maxLabelLength
          ? `${label.slice(0, maxLabelLength - 1)}…`
          : label
        : id;
      return {
        name: `${displayLabel}${suffix}`,
        value: result.id,
      };
    });
}

interface AutocompleteCacheEntry {
  expiresAt: number;
  choices?: ContractAutocompleteChoice[];
  pending?: Promise<ContractAutocompleteChoice[]>;
}

/**
 * Build the autocomplete responder. Discord already debounces typing events;
 * this short cache also coalesces identical in-flight requests and avoids
 * repeatedly searching while a user moves through a query.
 */
export function createContractAutocompleteHandler(
  api: Pick<SorolensApi, "searchContracts">
) {
  const cache = new Map<string, AutocompleteCacheEntry>();

  async function choicesFor(
    query: string
  ): Promise<ContractAutocompleteChoice[]> {
    const key = query.toLowerCase();
    const now = Date.now();
    const cached = cache.get(key);
    if (cached && cached.expiresAt > now) {
      if (cached.choices) return cached.choices;
      if (cached.pending) return cached.pending;
    }

    const pending = Promise.resolve()
      .then(() => api.searchContracts(query, AUTOCOMPLETE_LIMIT))
      .then(contractAutocompleteChoices);
    cache.set(key, { expiresAt: now + AUTOCOMPLETE_CACHE_TTL_MS, pending });

    try {
      const choices = await pending;
      if (cache.get(key)?.pending === pending) {
        cache.set(key, {
          expiresAt: Date.now() + AUTOCOMPLETE_CACHE_TTL_MS,
          choices,
        });
      }
      return choices;
    } catch {
      // Do not cache failures: the next keystroke should be able to recover.
      if (cache.get(key)?.pending === pending) cache.delete(key);
      return [];
    }
  }

  return async function handleAutocomplete(
    interaction: AutocompleteInteraction
  ): Promise<void> {
    const focused = interaction.options.getFocused(true);
    const optionName = typeof focused === "string" ? "" : focused.name;
    if (
      !CONTRACT_AUTOCOMPLETE_COMMANDS.has(interaction.commandName) ||
      optionName !== "contract"
    ) {
      await interaction.respond([]);
      return;
    }

    const rawQuery = interaction.options.getFocused();
    const query = String(rawQuery).trim();
    if (!query) {
      await interaction.respond([]);
      return;
    }

    await interaction.respond(await choicesFor(query));
  };
}

export function registerHandlers(client: Client, ctx: CommandContext) {
  const api = new SorolensApi({
    baseUrl: ctx.config.sorolensApiBaseUrl,
    apiKey: ctx.config.sorolensAdminApiKey,
  });
  const limiter = new SlidingWindowRateLimiter(
    ctx.config.commandRateLimitPerMinute
  );
  const handleAutocomplete = createContractAutocompleteHandler(api);

  client.on("interactionCreate", async (interaction: Interaction) => {
    if (interaction.isAutocomplete()) {
      try {
        await handleAutocomplete(interaction);
      } catch (err) {
        console.warn("autocomplete failed:", err);
        try {
          await interaction.respond([]);
        } catch {
          // The interaction may have expired while the API was unavailable.
        }
      }
      return;
    }
    if (!interaction.isChatInputCommand()) return;
    try {
      await handle(interaction, ctx, api, limiter);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      console.error("command failed:", interaction.commandName, msg);
      // Swallow secondary errors: the interaction may already be
      // acknowledged, or have expired past the 3s window.
      try {
        if (interaction.deferred || interaction.replied) {
          await interaction.editReply({
            content: `Something went wrong: ${msg}`,
          });
        } else {
          // MessageFlags.Ephemeral === 1 << 6 (64).
          await interaction.reply({
            content: `Something went wrong: ${msg}`,
            flags: 64,
          });
        }
      } catch (reportErr) {
        console.warn("command failed to report error:", reportErr);
      }
    }
  });
}

async function handle(
  interaction: ChatInputCommandInteraction,
  ctx: CommandContext,
  api: SorolensApi,
  limiter: SlidingWindowRateLimiter
) {
  if (ASSIGN_COMMANDS.has(interaction.commandName)) {
    return handleAssignCommand(interaction, ctx.config);
  }
  if (COMMUNITY_COMMANDS.has(interaction.commandName)) {
    return handleCommunityCommand(interaction, ctx.config);
  }
  switch (interaction.commandName) {
    case "connect":
      return handleConnect(interaction, ctx);
    case "link":
      return handleLink(interaction, ctx, api);
    case "admin-unlink":
      return handleAdminUnlink(interaction, api);
    case "unlink":
      return handleUnlink(interaction, api);
    case "whoami":
      return handleWhoAmI(interaction);
    case "mypr":
      return handleMyPR(interaction, ctx);
    case "contributions":
      return handleContributions(interaction, ctx);
    case "members":
      return handleMembers(interaction, ctx);
    case "status":
      return handleStatus(interaction, api, limiter);
    case "alerts":
      return handleAlerts(interaction, api, limiter);
    case "watch":
      return handleWatch(interaction, api, limiter);
    case "unwatch":
      return handleUnwatch(interaction, api, limiter);
    default:
      await interaction.reply({ content: "Unknown command.", flags: 64 });
  }
}

async function handleConnect(
  interaction: ChatInputCommandInteraction,
  ctx: CommandContext
) {
  const url = buildConnectUrl(interaction.user.id, ctx.config);
  await interaction.reply({
    content:
      `Click here to link your GitHub in one step (link is personal to you, do not share):\n${url}\n\n` +
      `Expires in 10 minutes. Run \`/connect\` again if it expires. Prefer manual typing? Use \`/link github <username>\`.`,
    flags: 64,
  });
}

async function handleLink(
  interaction: ChatInputCommandInteraction,
  ctx: CommandContext,
  api: SorolensApi
) {
  const gh = interaction.options.getString("github", true).trim();
  await interaction.deferReply({ flags: 64 });

  const user = await getUser(gh);
  if (!user) {
    await interaction.editReply({
      content: `GitHub user \`${gh}\` was not found. Please double-check the spelling.`,
    });
    return;
  }

  try {
    upsertLink(interaction.user.id, user.login);
  } catch (err) {
    await interaction.editReply({
      content: err instanceof Error ? err.message : "Could not save the link.",
    });
    return;
  }

  // Give the contributor their own API key so the query commands run under
  // their identity. Best effort: a failure must not undo the link.
  await ensureUserApiKey(apiKeyStore, api, interaction.user.id, user.login);

  // Immediate role sync so contributors do not have to wait for their next merge.
  const guild = interaction.guild;
  let syncMsg = "";
  if (guild) {
    const count = await countMergedPRs(user.login, ctx.config.githubRepo);
    const result = await syncRoles(
      guild,
      interaction.user.id,
      count,
      ctx.tiers
    );
    syncMsg = describeSync(count, result.targetTier);
  }

  await interaction.editReply({
    content:
      `Linked Discord \`${interaction.user.tag}\` to GitHub \`${user.login}\`.\n` +
      (syncMsg
        ? syncMsg
        : "Once your PRs get merged into sorolens/sorolens the bot will grant your role automatically."),
  });
}

/**
 * Resolve which link `/admin-unlink` targets: exactly one of a Discord user
 * or a GitHub login. Returns an error message instead when that fails.
 */
export function resolveUnlinkTarget(
  userId: string | null,
  github: string | null,
  lookup: {
    byDiscord: (id: string) => Link | null;
    byGithub: (login: string) => Link | null;
  }
): { link: Link } | { error: string } {
  const login = github?.trim() || null;
  if ((userId && login) || (!userId && !login)) {
    return { error: "Provide exactly one of `user` or `github`." };
  }
  const link = userId ? lookup.byDiscord(userId) : lookup.byGithub(login!);
  if (!link) {
    return {
      error: userId
        ? "That member has no GitHub link."
        : `No member is linked to GitHub \`${login}\`.`,
    };
  }
  return { link };
}

async function handleAdminUnlink(
  interaction: ChatInputCommandInteraction,
  api: SorolensApi
) {
  const target = resolveUnlinkTarget(
    interaction.options.getUser("user")?.id ?? null,
    interaction.options.getString("github"),
    { byDiscord: getByDiscord, byGithub: getByGithub }
  );
  if ("error" in target) {
    await interaction.reply({ content: target.error, flags: 64 });
    return;
  }
  const { discordId, githubLogin } = target.link;
  await revokeUserApiKey(apiKeyStore, api, discordId);
  unlink(discordId);
  console.log(
    `admin-unlink: ${interaction.user.id} removed link ${discordId} -> ${githubLogin}`
  );
  await interaction.reply({
    content:
      `Removed the link <@${discordId}> → GitHub \`${githubLogin}\`. ` +
      "Their roles and any assigned issues were left as they are. They can link again with `/connect`.",
    flags: 64,
    allowedMentions: { parse: [] },
  });
}

async function handleUnlink(
  interaction: ChatInputCommandInteraction,
  api: SorolensApi
) {
  const hadLink = getByDiscord(interaction.user.id) !== null;
  const result = await revokeUserApiKey(apiKeyStore, api, interaction.user.id);
  unlink(interaction.user.id);
  if (result.cleared && !result.revoked) {
    console.warn(
      "unlink: cleared local API key but revocation failed for",
      interaction.user.id
    );
  }
  await interaction.reply({
    content: hadLink
      ? "Your GitHub link has been removed. Your Discord roles were left as-is; a maintainer can adjust them if needed."
      : "You did not have a GitHub link on file.",
    flags: 64,
  });
}

async function handleWhoAmI(interaction: ChatInputCommandInteraction) {
  const link = getByDiscord(interaction.user.id);
  await interaction.reply({
    content: link
      ? `Discord \`${interaction.user.tag}\` -> GitHub \`${link.githubLogin}\` (linked ${link.linkedAt} UTC).`
      : "No GitHub account linked. Use `/link github <your-username>`.",
    flags: 64,
  });
}

async function handleMyPR(
  interaction: ChatInputCommandInteraction,
  ctx: CommandContext
) {
  const link = getByDiscord(interaction.user.id);
  if (!link) {
    await interaction.reply({
      content:
        "You have not linked a GitHub account yet. Run `/link github <your-username>` first.",
      flags: 64,
    });
    return;
  }
  await interaction.deferReply({ flags: 64 });
  const count = await countMergedPRs(link.githubLogin, ctx.config.githubRepo);
  const tier = tierFor(count, ctx.tiers);
  await interaction.editReply({
    content: `\`${link.githubLogin}\` has **${count}** merged PR${count === 1 ? "" : "s"} in ${ctx.config.githubRepo}. Current tier: **${tier}**.`,
  });
}

async function handleContributions(
  interaction: ChatInputCommandInteraction,
  ctx: CommandContext
) {
  const user = interaction.options.getUser("user");
  const github = interaction.options.getString("github")?.trim();

  if ((user && github) || (!user && !github)) {
    await interaction.reply({
      content: "Provide exactly one of `user` or `github`.",
      flags: 64,
    });
    return;
  }

  let login: string;
  if (user) {
    const link = getByDiscord(user.id);
    if (!link) {
      await interaction.reply({
        content: `${user.tag} has not linked a GitHub account.`,
        flags: 64,
      });
      return;
    }
    login = link.githubLogin;
  } else {
    // GitHub logins are alphanumeric with single hyphens, max 39 chars.
    // Validating also keeps the value safe to embed in a search query.
    if (!/^[A-Za-z0-9](?:[A-Za-z0-9]|-(?=[A-Za-z0-9])){0,38}$/.test(github!)) {
      await interaction.reply({
        content: "That is not a valid GitHub username.",
        flags: 64,
      });
      return;
    }
    login = github!;
  }

  await interaction.deferReply({ flags: 64 });
  const count = await countMergedPRs(login, ctx.config.githubRepo);
  const tier = tierFor(count, ctx.tiers);
  await interaction.editReply({
    content: `\`${login}\` has **${count}** merged PR${count === 1 ? "" : "s"} in ${ctx.config.githubRepo}. Tier: **${tier}**.`,
  });
}

async function handleMembers(
  interaction: ChatInputCommandInteraction,
  ctx: CommandContext
) {
  await interaction.deferReply({ flags: 64 });

  const filter = (interaction.options.getString("filter") ?? "all") as
    "all" | "contributor" | "core" | "none";

  const links = listLinks(200);
  if (links.length === 0) {
    await interaction.editReply({ content: "No linked members yet." });
    return;
  }

  const guild = interaction.guild;
  const rows: string[] = [];

  for (const link of links) {
    let tier = "none";
    if (guild) {
      const member = await guild.members
        .fetch(link.discordId)
        .catch(() => null);
      if (member) {
        if (member.roles.cache.has(ctx.tiers.coreContributor)) tier = "core";
        else if (member.roles.cache.has(ctx.tiers.contributor))
          tier = "contributor";
      }
    }
    if (filter !== "all" && tier !== filter) continue;
    const tierLabel =
      tier === "core"
        ? "⭐ Core Contributor"
        : tier === "contributor"
          ? "✅ Contributor"
          : "— No role";
    rows.push(`<@${link.discordId}> → \`${link.githubLogin}\` ${tierLabel}`);
  }

  if (rows.length === 0) {
    await interaction.editReply({
      content: `No members match filter **${filter}**.`,
    });
    return;
  }

  // Discord message limit is 2000 chars; split into chunks if needed.
  const header = `**Linked members** (${rows.length}):\n`;
  const chunks: string[] = [];
  let current = header;
  for (const row of rows) {
    if (current.length + row.length + 1 > 1900) {
      chunks.push(current);
      current = "";
    }
    current += row + "\n";
  }
  if (current) chunks.push(current);

  await interaction.editReply({ content: chunks[0] });
  for (const chunk of chunks.slice(1)) {
    await interaction.followUp({ content: chunk, flags: 64 });
  }
}

// ---- Sorolens API query commands -------------------------------------------

async function handleStatus(
  interaction: ChatInputCommandInteraction,
  api: SorolensApi,
  limiter: SlidingWindowRateLimiter
) {
  if (!(await requireLink(interaction))) return;
  if (!(await enforceRateLimit(interaction, limiter))) return;
  const contractId = await contractArg(interaction);
  if (!contractId) return;

  await interaction.deferReply({ flags: 64 });
  try {
    const status = await api.getContractStatus(
      contractId,
      credsFor(interaction.user.id)
    );
    await interaction.editReply({ embeds: [statusEmbed(status)] });
  } catch (err) {
    await interaction.editReply({ content: describeApiError(err, contractId) });
  }
}

async function handleAlerts(
  interaction: ChatInputCommandInteraction,
  api: SorolensApi,
  limiter: SlidingWindowRateLimiter
) {
  if (!(await requireLink(interaction))) return;
  if (!(await enforceRateLimit(interaction, limiter))) return;
  const contractId = await contractArg(interaction);
  if (!contractId) return;
  const limit = interaction.options.getInteger("limit") ?? 5;

  await interaction.deferReply({ flags: 64 });
  try {
    const alerts = await api.listAlerts(
      contractId,
      limit,
      credsFor(interaction.user.id)
    );
    await interaction.editReply({ embeds: [alertsEmbed(contractId, alerts)] });
  } catch (err) {
    await interaction.editReply({ content: describeApiError(err, contractId) });
  }
}

async function handleWatch(
  interaction: ChatInputCommandInteraction,
  api: SorolensApi,
  limiter: SlidingWindowRateLimiter
) {
  if (!(await requireLink(interaction))) return;
  if (!(await enforceRateLimit(interaction, limiter))) return;
  const contractId = await contractArg(interaction);
  if (!contractId) return;

  await interaction.deferReply({ flags: 64 });
  const creds = credsFor(interaction.user.id);
  try {
    const watched = await api.addToWatchlist(contractId, creds);
    const count = await api.listWatchlist(creds).then(
      (ids) => ids.length,
      () => undefined
    );
    await interaction.editReply({
      embeds: [watchlistEmbed(contractId, watched, "watch", count)],
    });
  } catch (err) {
    await interaction.editReply({ content: describeApiError(err, contractId) });
  }
}

async function handleUnwatch(
  interaction: ChatInputCommandInteraction,
  api: SorolensApi,
  limiter: SlidingWindowRateLimiter
) {
  if (!(await requireLink(interaction))) return;
  if (!(await enforceRateLimit(interaction, limiter))) return;
  const contractId = await contractArg(interaction);
  if (!contractId) return;

  await interaction.deferReply({ flags: 64 });
  const creds = credsFor(interaction.user.id);
  try {
    const watched = await api.removeFromWatchlist(contractId, creds);
    const count = await api.listWatchlist(creds).then(
      (ids) => ids.length,
      () => undefined
    );
    await interaction.editReply({
      embeds: [watchlistEmbed(contractId, watched, "unwatch", count)],
    });
  } catch (err) {
    await interaction.editReply({ content: describeApiError(err, contractId) });
  }
}

// ---- helpers ---------------------------------------------------------------

/** Credentials for a command: the user's own key when provisioned, and the
 *  Discord id as the watchlist identity. */
function credsFor(discordId: string): ApiCredentials {
  return { apiKey: getApiKey(discordId)?.key, userId: discordId };
}

/** Reject the command when the caller has no GitHub link on file. */
async function requireLink(
  interaction: ChatInputCommandInteraction
): Promise<Link | null> {
  const link = getByDiscord(interaction.user.id);
  if (!link) {
    await interaction.reply({
      content:
        "Link your GitHub first with `/connect` (or `/link github <your-username>`).",
      flags: 64,
    });
    return null;
  }
  return link;
}

/** Returns false (after replying) when the caller is over their budget. */
async function enforceRateLimit(
  interaction: ChatInputCommandInteraction,
  limiter: SlidingWindowRateLimiter
): Promise<boolean> {
  const result = limiter.check(interaction.user.id);
  if (result.allowed) return true;
  await interaction.reply({
    content: `You are sending commands too quickly. Try again in ${describeRetry(result.retryAfterMs)}.`,
    flags: 64,
  });
  return false;
}

/** Validate + normalise the `contract` option, replying on invalid input. */
async function contractArg(
  interaction: ChatInputCommandInteraction
): Promise<string | null> {
  const raw = interaction.options
    .getString("contract", true)
    .trim()
    .toUpperCase();
  if (!isValidContractId(raw)) {
    await interaction.reply({
      content: `\`${raw}\` is not a Stellar contract id. Expected 56 characters starting with \`C\`.`,
      flags: 64,
    });
    return null;
  }
  return raw;
}

/** Turn an API failure into something a Discord user can act on. */
export function describeApiError(err: unknown, contractId: string): string {
  if (err instanceof ApiError) {
    if (err.status === 404)
      return `Contract \`${contractId}\` is not tracked by Sorolens.`;
    if (err.status === 401 || err.status === 403) {
      return "Your Sorolens API key was rejected or lacks scope. Run `/connect` again to refresh it.";
    }
    if (err.status === 429)
      return "Sorolens API rate limit reached. Try again in a minute.";
    return `Sorolens API error (${err.status}): ${err.message}`;
  }
  return "Could not reach the Sorolens API. Try again shortly.";
}

function tierFor(count: number, tiers: Tiers): string {
  if (count >= tiers.coreContributorThreshold) return "Core Contributor";
  if (count >= tiers.contributorThreshold) return "Contributor";
  return "Verified";
}

function describeSync(
  count: number,
  target: "core" | "contributor" | "none"
): string {
  const label =
    target === "core"
      ? "Core Contributor"
      : target === "contributor"
        ? "Contributor"
        : "Verified";
  return `You have **${count}** merged PR${count === 1 ? "" : "s"} and are now tier **${label}**.`;
}

// runtime export for the entry file
export async function syncOnMerge(
  guild: Guild,
  discordId: string,
  githubLogin: string,
  ctx: CommandContext
) {
  const count = await countMergedPRs(githubLogin, ctx.config.githubRepo);
  return syncRoles(guild, discordId, count, ctx.tiers);
}
