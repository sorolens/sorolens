/**
 * Alert subscriptions routing, severity filtering, and rate limiting.
 *
 * Implements channel-level routing for Sorolens contract watchdog alerts:
 * - Persists subscriptions per Discord channel (in SQLite via db.ts)
 * - Evaluates contract and severity filters
 * - Enforces per-channel rate limits (sliding window)
 * - Handles deleted channels (Discord 10003) and missing permissions (50013) gracefully
 * - Supports receiving alerts via webhook receiver or API polling
 */

import crypto from "node:crypto";
import type { Client, TextBasedChannel } from "discord.js";
import type { ContractAlert, SorolensApi } from "./api.js";
import {
  clearSubscriptionsForChannel,
  listAllSubscriptions,
  listSubscriptionsForContract,
  type Subscription,
} from "./db.js";
import { alertNotificationEmbed, type AlertNotificationOptions } from "./embeds.js";
import { SlidingWindowRateLimiter } from "./ratelimit.js";

export const VALID_SEVERITIES = ["all", "critical", "warning", "info"] as const;
export type SeverityChoice = (typeof VALID_SEVERITIES)[number];

const SEVERITY_LEVELS: Record<string, number> = {
  info: 1,
  warning: 2,
  critical: 3,
};

/**
 * Checks if an alert's severity matches the subscription's severity filter.
 *
 * Filtering rules:
 * - "all" matches any severity (Info, Warning, Critical)
 * - "warning" matches Warning and Critical (minimum severity threshold)
 * - "critical" matches only Critical
 * - "info" matches all (Info, Warning, Critical)
 * - Comma-separated severities (e.g. "warning,critical") match if the alert's
 *   severity is explicitly present.
 * - Exact case-insensitive match always matches.
 */
export function matchesSeverity(filter: string, alertSeverity: string): boolean {
  const normFilter = filter.trim().toLowerCase();
  const normAlert = alertSeverity.trim().toLowerCase();

  if (!normAlert) return false;
  if (normFilter === "all" || normFilter === "*" || normFilter === "") return true;

  // Comma-separated list check
  if (normFilter.includes(",")) {
    const list = normFilter.split(",").map((s) => s.trim());
    return list.includes(normAlert);
  }

  // Exact match
  if (normFilter === normAlert) return true;

  // Threshold check: "warning" includes "critical"
  const filterLevel = SEVERITY_LEVELS[normFilter];
  const alertLevel = SEVERITY_LEVELS[normAlert];
  if (filterLevel !== undefined && alertLevel !== undefined) {
    return alertLevel >= filterLevel;
  }

  return false;
}

/** Per-channel sliding-window rate limiter for outgoing alert posts. */
export class ChannelRateLimiter {
  private readonly limiter: SlidingWindowRateLimiter;

  constructor(limitPerMinute = 5, now?: () => number) {
    this.limiter = new SlidingWindowRateLimiter(limitPerMinute, 60_000, now);
  }

  check(channelId: string) {
    return this.limiter.check(channelId);
  }

  reset(channelId?: string) {
    this.limiter.reset(channelId);
  }
}

export interface DispatchAlertResult {
  delivered: string[];
  rateLimited: string[];
  deleted: string[];
  forbidden: string[];
  failed: string[];
}

export interface RouteAlertOptions {
  getSubscriptions?: (contractId: string) => Subscription[];
  clearChannelSubscriptions?: (channelId: string) => number;
  rateLimiter?: ChannelRateLimiter;
  contractLabel?: string | null;
}

const defaultLimiter = new ChannelRateLimiter(5);

/**
 * Routes an incoming contract watchdog alert to all subscribed Discord channels
 * whose severity filter matches.
 */
export async function routeAlert(
  client: Client,
  alert: ContractAlert & { explorerUrl?: string },
  opts: RouteAlertOptions = {}
): Promise<DispatchAlertResult> {
  const getSubs = opts.getSubscriptions ?? listSubscriptionsForContract;
  const clearSubs = opts.clearChannelSubscriptions ?? clearSubscriptionsForChannel;
  const limiter = opts.rateLimiter ?? defaultLimiter;

  const result: DispatchAlertResult = {
    delivered: [],
    rateLimited: [],
    deleted: [],
    forbidden: [],
    failed: [],
  };

  const subs = getSubs(alert.contractId);
  if (subs.length === 0) return result;

  const embed = alertNotificationEmbed(alert, {
    contractLabel: opts.contractLabel,
    explorerUrl: alert.explorerUrl,
  });

  for (const sub of subs) {
    if (!matchesSeverity(sub.severity, alert.severity)) {
      continue;
    }

    const rateResult = limiter.check(sub.channelId);
    if (!rateResult.allowed) {
      console.warn(
        `routeAlert: rate limited alert for channel ${sub.channelId} (contract ${alert.contractId})`
      );
      result.rateLimited.push(sub.channelId);
      continue;
    }

    try {
      const channel = await client.channels.fetch(sub.channelId).catch((err: unknown) => {
        const code = (err as { code?: number })?.code;
        const status = (err as { status?: number })?.status;
        if (code === 10003 || status === 404) {
          // Channel was deleted on Discord
          clearSubs(sub.channelId);
          result.deleted.push(sub.channelId);
          return null;
        }
        if (code === 50013 || code === 50001) {
          result.forbidden.push(sub.channelId);
          return null;
        }
        result.failed.push(sub.channelId);
        return null;
      });

      if (!channel) continue;

      if (!("send" in channel) || typeof (channel as { send?: unknown }).send !== "function") {
        result.failed.push(sub.channelId);
        continue;
      }

      await (channel as { send: (options: unknown) => Promise<unknown> }).send({
        embeds: [embed],
      });
      result.delivered.push(sub.channelId);
    } catch (err: unknown) {
      const code = (err as { code?: number })?.code;
      if (code === 10003) {
        clearSubs(sub.channelId);
        result.deleted.push(sub.channelId);
      } else if (code === 50013 || code === 50001) {
        console.warn(`routeAlert: missing permissions in channel ${sub.channelId}`);
        result.forbidden.push(sub.channelId);
      } else {
        console.error(`routeAlert: error sending alert to channel ${sub.channelId}:`, err);
        result.failed.push(sub.channelId);
      }
    }
  }

  return result;
}

/** Verify webhook delivery signature according to docs/webhooks.md. */
export function verifyWebhookSignature(
  secret: string,
  signatureHeader: string | undefined,
  timestampHeader: string | undefined,
  rawBody: Buffer,
  nowMs = Date.now()
): boolean {
  if (!secret || !signatureHeader || !timestampHeader) return false;

  const parts = signatureHeader.split(",");
  let headerTimestamp = "";
  const signatures: string[] = [];

  for (const part of parts) {
    const [k, v] = part.split("=");
    if (!k || !v) continue;
    if (k.trim() === "t") headerTimestamp = v.trim();
    if (k.trim() === "v1") signatures.push(v.trim());
  }

  if (!headerTimestamp || headerTimestamp !== timestampHeader.trim()) {
    return false;
  }

  const tsSec = parseInt(headerTimestamp, 10);
  if (Number.isNaN(tsSec)) return false;

  const nowSec = Math.floor(nowMs / 1000);
  if (Math.abs(nowSec - tsSec) > 300) {
    return false;
  }

  const signedPayload = Buffer.concat([
    Buffer.from(`${headerTimestamp}.`, "utf8"),
    rawBody,
  ]);

  const hmac = crypto.createHmac("sha256", secret);
  hmac.update(signedPayload);
  const expectedSignature = hmac.digest("hex");

  for (const sig of signatures) {
    if (sig.length === expectedSignature.length) {
      if (
        crypto.timingSafeEqual(
          Buffer.from(sig, "utf8"),
          Buffer.from(expectedSignature, "utf8")
        )
      ) {
        return true;
      }
    }
  }

  return false;
}

export interface PollerState {
  lastSeenByContract: Map<string, { lastLedger: number; lastTimestamp: string }>;
}

/**
 * Polls the Sorolens API for contracts that have active subscriptions and
 * dispatches any new alerts found.
 */
export async function pollContractAlerts(
  client: Client,
  api: Pick<SorolensApi, "listAlerts">,
  state: PollerState = { lastSeenByContract: new Map() },
  options: RouteAlertOptions = {}
): Promise<number> {
  const allSubs = options.getSubscriptions
    ? options.getSubscriptions("")
    : listAllSubscriptions();
  const uniqueContracts = [...new Set(allSubs.map((s) => s.contractId))];
  let dispatchedCount = 0;

  for (const contractId of uniqueContracts) {
    try {
      const alerts = await api.listAlerts(contractId, 5);
      if (alerts.length === 0) continue;

      const seen = state.lastSeenByContract.get(contractId);
      if (!seen) {
        // Initializing watermark on first poll to prevent replay of historic alerts
        const newest = alerts[0];
        state.lastSeenByContract.set(contractId, {
          lastLedger: newest.ledger,
          lastTimestamp: newest.timestamp,
        });
        continue;
      }

      const newAlerts = alerts.filter(
        (a) =>
          a.ledger > seen.lastLedger ||
          (a.ledger === seen.lastLedger && a.timestamp > seen.lastTimestamp)
      );

      // Deliver chronologically (oldest new alert first)
      for (const alert of newAlerts.reverse()) {
        const res = await routeAlert(client, alert, options);
        if (res.delivered.length > 0) dispatchedCount += res.delivered.length;
      }

      const newest = alerts[0];
      state.lastSeenByContract.set(contractId, {
        lastLedger: newest.ledger,
        lastTimestamp: newest.timestamp,
      });
    } catch (err) {
      console.warn(`polling alerts failed for contract ${contractId}:`, err);
    }
  }

  return dispatchedCount;
}

/** Start a periodic background poller. */
export function startAlertPoller(
  client: Client,
  api: Pick<SorolensApi, "listAlerts">,
  intervalMs = 60_000,
  options: RouteAlertOptions = {}
): NodeJS.Timeout {
  const state: PollerState = { lastSeenByContract: new Map() };
  return setInterval(() => {
    pollContractAlerts(client, api, state, options).catch((err) => {
      console.warn("alert poller error:", err);
    });
  }, intervalMs);
}
