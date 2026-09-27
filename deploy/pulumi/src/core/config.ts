import * as pulumi from "@pulumi/pulumi";

/**
 * Cloud-agnostic Sorolens settings. Mirrors deploy/terraform/modules/core:
 * validates the app configuration and turns it into the environment each
 * service reads. Creates no resources.
 */

export const STELLAR_NETWORKS = ["testnet", "mainnet", "futurenet"] as const;
export type StellarNetwork = (typeof STELLAR_NETWORKS)[number];

export const LOG_LEVELS = ["debug", "info", "warn", "error"] as const;
export type LogLevel = (typeof LOG_LEVELS)[number];

export interface AppConfigArgs {
  /** Name prefix for resources: ^[a-z][a-z0-9-]{0,19}$, no trailing hyphen. Default "sorolens". */
  name?: string;
  /** Default Stellar network for the API. Default "testnet". */
  stellarNetwork?: StellarNetwork;
  /**
   * Soroban RPC endpoint per network. testnet and futurenet fall back to the
   * public SDF endpoints; mainnet has no default. The indexer polls every
   * network listed here plus stellarNetwork.
   */
  sorobanRpcUrls?: Partial<Record<StellarNetwork, string>>;
  /** Deployed sorolens-watchdog contract ID; enables watchdog processing. */
  watchdogContractId?: string;
  /** API log level. Default "info". */
  logLevel?: LogLevel;
  /** Extra browser origins allowed to call the API (ALLOWED_ORIGINS). */
  allowedOrigins?: string[];
  /** Sleep between indexer passes, as a Go duration. Default "5m". */
  indexerPollInterval?: string;
}

export type Env = Record<string, pulumi.Input<string>>;

export interface AppConfig {
  name: string;
  apiPort: number;
  dashboardPort: number;
  indexerMetricsPort: number;
  apiEnvironment: Env;
  indexerEnvironment: Env;
  dashboardEnvironment: Env;
  indexerCommand: string[];
}

export const API_PORT = 8080;
export const DASHBOARD_PORT = 3000;
export const INDEXER_METRICS_PORT = 9100;

const DEFAULT_RPC_URLS: Partial<Record<StellarNetwork, string>> = {
  testnet: "https://soroban-testnet.stellar.org:443",
  futurenet: "https://soroban-futurenet.stellar.org:443",
};

const NAME_RE = /^[a-z][a-z0-9-]{0,19}$/;
const CONTRACT_ID_RE = /^C[A-Z2-7]{55}$/;
const ORIGIN_RE = /^https?:\/\/[^/]+$/;
const GO_DURATION_RE = /^([0-9]+(ns|us|ms|s|m|h))+$/;

/** Thrown for invalid configuration, before any resource is registered. */
export class SorolensConfigError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "SorolensConfigError";
  }
}

/**
 * Validates the settings and builds each service's environment.
 * @param publicUrl origin the dashboard and API are served from (may be an
 *   Output, e.g. the load balancer DNS name).
 */
export function buildAppConfig(args: AppConfigArgs, publicUrl: pulumi.Input<string>): AppConfig {
  const name = args.name ?? "sorolens";
  if (!NAME_RE.test(name) || name.endsWith("-")) {
    throw new SorolensConfigError(`name must match ${NAME_RE} and must not end with a hyphen (got "${name}")`);
  }

  const network = args.stellarNetwork ?? "testnet";
  if (!STELLAR_NETWORKS.includes(network)) {
    throw new SorolensConfigError(`stellarNetwork must be one of ${STELLAR_NETWORKS.join(", ")} (got "${network}")`);
  }

  const given = args.sorobanRpcUrls ?? {};
  for (const [net, url] of Object.entries(given)) {
    if (!STELLAR_NETWORKS.includes(net as StellarNetwork)) {
      throw new SorolensConfigError(`sorobanRpcUrls keys must be ${STELLAR_NETWORKS.join(", ")} (got "${net}")`);
    }
    if (typeof url !== "string" || !url.startsWith("https://")) {
      throw new SorolensConfigError(`sorobanRpcUrls.${net} must be an https:// URL`);
    }
  }
  if (network === "mainnet" && !given.mainnet) {
    throw new SorolensConfigError("stellarNetwork is mainnet, so sorobanRpcUrls.mainnet is required (there is no public default)");
  }

  if (args.watchdogContractId !== undefined && !CONTRACT_ID_RE.test(args.watchdogContractId)) {
    throw new SorolensConfigError("watchdogContractId must be a 56-character contract strkey starting with C");
  }

  const logLevel = args.logLevel ?? "info";
  if (!LOG_LEVELS.includes(logLevel)) {
    throw new SorolensConfigError(`logLevel must be one of ${LOG_LEVELS.join(", ")}`);
  }

  const origins = args.allowedOrigins ?? [];
  for (const o of origins) {
    if (!ORIGIN_RE.test(o)) {
      throw new SorolensConfigError(`allowedOrigins entries must be origins without a path (got "${o}")`);
    }
  }

  const pollInterval = args.indexerPollInterval ?? "5m";
  if (!GO_DURATION_RE.test(pollInterval)) {
    throw new SorolensConfigError(`indexerPollInterval must be a Go duration such as 30s or 5m (got "${pollInterval}")`);
  }

  const rpcUrls = { ...DEFAULT_RPC_URLS, ...given };
  const indexedNetworks = [...new Set<StellarNetwork>([network, ...(Object.keys(given) as StellarNetwork[]).sort()])];
  const perNetworkRpcEnv: Env = Object.fromEntries(
    indexedNetworks.map((n) => [`SOROBAN_RPC_URL_${n.toUpperCase()}`, rpcUrls[n] as string]),
  );

  const apiEnvironment: Env = {
    PORT: String(API_PORT),
    STELLAR_NETWORK: network,
    LOG_LEVEL: logLevel,
    SOROBAN_RPC_URL: rpcUrls[network] as string,
    ...perNetworkRpcEnv,
    ...(origins.length > 0 ? { ALLOWED_ORIGINS: origins.join(",") } : {}),
  };

  const indexerEnvironment: Env = {
    ...perNetworkRpcEnv,
    // Single process owning every shard: the only shard store on main is
    // in-memory, so the indexer must run as exactly one instance.
    INDEXER_ROLE: "all",
    INDEXER_METRICS_ADDR: `:${INDEXER_METRICS_PORT}`,
    WATCHDOG_ENABLED: args.watchdogContractId ? "true" : "false",
    ...(args.watchdogContractId ? { WATCHDOG_CONTRACT_ID: args.watchdogContractId } : {}),
  };

  const dashboardEnvironment: Env = {
    PORT: String(DASHBOARD_PORT),
    HOSTNAME: "0.0.0.0",
    // Next.js inlines NEXT_PUBLIC_* at build time; this only takes effect if
    // the image was built with the same value.
    NEXT_PUBLIC_API_URL: publicUrl,
  };

  return {
    name,
    apiPort: API_PORT,
    dashboardPort: DASHBOARD_PORT,
    indexerMetricsPort: INDEXER_METRICS_PORT,
    apiEnvironment,
    indexerEnvironment,
    dashboardEnvironment,
    indexerCommand: ["-mode=continuous", `-poll-interval=${pollInterval}`],
  };
}
