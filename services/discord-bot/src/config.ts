/**
 * Runtime configuration for the Sorolens contributor bot.
 *
 * Every value comes from an environment variable. Fail loudly at startup
 * rather than trying to run with placeholders.
 */

function required(name: string): string {
  const v = process.env[name];
  if (!v || v.trim() === "") {
    throw new Error(`Missing required env var: ${name}`);
  }
  return v.trim();
}

function optional(name: string, fallback: string): string {
  const v = process.env[name];
  return v && v.trim() !== "" ? v.trim() : fallback;
}

function intOpt(name: string, fallback: number): number {
  const v = process.env[name];
  if (!v || v.trim() === "") return fallback;
  const n = parseInt(v, 10);
  if (Number.isNaN(n)) throw new Error(`${name} must be an integer`);
  return n;
}

export const config = {
  // Discord
  discordToken: required("DISCORD_BOT_TOKEN"),
  discordClientId: required("DISCORD_CLIENT_ID"),
  discordGuildId: required("DISCORD_GUILD_ID"),
  roleContributor: required("DISCORD_ROLE_CONTRIBUTOR_ID"),
  roleCoreContributor: required("DISCORD_ROLE_CORE_CONTRIBUTOR_ID"),

  // Optional. If set, the bot grants this role whenever /connect
  // completes successfully. Unlike the tier roles it is never revoked -
  // it is a persistent "linked" marker used to gate channel access
  // behind the OAuth flow.
  roleVerified: optional("DISCORD_ROLE_VERIFIED_ID", ""),

  // Thresholds (proxying "PR merged" via merged PR count from GitHub API)
  contributorThreshold: intOpt("CONTRIBUTOR_THRESHOLD", 1),
  coreContributorThreshold: intOpt("CORE_CONTRIBUTOR_THRESHOLD", 3),

  // GitHub
  githubToken: required("GITHUB_TOKEN"),
  githubRepo: optional("GITHUB_REPO", "sorolens/sorolens"),
  githubWebhookSecret: required("GITHUB_WEBHOOK_SECRET"),

  // GitHub OAuth (for zero-typing account linking)
  githubOauthClientId: required("GITHUB_OAUTH_CLIENT_ID"),
  githubOauthClientSecret: required("GITHUB_OAUTH_CLIENT_SECRET"),
  oauthStateSecret: required("OAUTH_STATE_SECRET"),
  publicBaseUrl: required("PUBLIC_BASE_URL"),

  // HTTP server
  port: intOpt("PORT", 8080),

  // Storage
  dbPath: optional("DATABASE_PATH", "./data/mappings.db"),
};

export type Config = typeof config;
