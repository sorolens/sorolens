import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import crypto from "node:crypto";
import type { AddressInfo } from "node:net";
import express from "express";
import {
  PermissionFlagsBits,
  type ChatInputCommandInteraction,
  type Client,
} from "discord.js";
import {
  openDb,
  upsertSubscription,
  listSubscriptionsForChannel,
  getSubscription,
} from "./db.js";
import {
  matchesSeverity,
  ChannelRateLimiter,
  routeAlert,
  verifyWebhookSignature,
  pollContractAlerts,
  type PollerState,
} from "./subscriptions.js";
import { alertNotificationEmbed } from "./embeds.js";
import { handleSubscriptionCommand } from "./subscriptions-discord.js";
import { mountWebhookRoutes } from "./webhook.js";
import type { ContractAlert } from "./api.js";
import type { Config } from "./config.js";

const CONTRACT_A = "C" + "A".repeat(55);
const CONTRACT_B = "C" + "B".repeat(55);

const mockConfig: Config = {
  discordToken: "mock",
  discordClientId: "mock",
  discordGuildId: "guild_123",
  roleContributor: "role_c",
  roleCoreContributor: "role_cc",
  roleVerified: "",
  contributorThreshold: 1,
  coreContributorThreshold: 3,
  githubToken: "mock",
  githubRepo: "sorolens/sorolens",
  githubWebhookSecret: "mock",
  githubOauthClientId: "mock",
  githubOauthClientSecret: "mock",
  oauthStateSecret: "mock-secret",
  publicBaseUrl: "https://example.com",
  sorolensApiBaseUrl: "https://api.example.com",
  sorolensAdminApiKey: "",
  commandRateLimitPerMinute: 10,
  alertRateLimitPerMinute: 3,
  alertPollIntervalSeconds: 0,
  alertWebhookSecret: "",
  port: 8080,
  dbPath: ":memory:",
};

describe("matchesSeverity", () => {
  it("matches all alerts when filter is 'all' or empty", () => {
    expect(matchesSeverity("all", "Info")).toBe(true);
    expect(matchesSeverity("all", "Warning")).toBe(true);
    expect(matchesSeverity("all", "Critical")).toBe(true);
    expect(matchesSeverity("", "Critical")).toBe(true);
    expect(matchesSeverity("*", "Warning")).toBe(true);
  });

  it("filters critical-only subscriptions", () => {
    expect(matchesSeverity("critical", "Critical")).toBe(true);
    expect(matchesSeverity("critical", "Warning")).toBe(false);
    expect(matchesSeverity("critical", "Info")).toBe(false);
  });

  it("filters warning-and-above subscriptions", () => {
    expect(matchesSeverity("warning", "Critical")).toBe(true);
    expect(matchesSeverity("warning", "Warning")).toBe(true);
    expect(matchesSeverity("warning", "Info")).toBe(false);
  });

  it("matches comma-separated severity lists", () => {
    expect(matchesSeverity("warning,critical", "Critical")).toBe(true);
    expect(matchesSeverity("warning,critical", "Warning")).toBe(true);
    expect(matchesSeverity("warning,critical", "Info")).toBe(false);
  });

  it("is case-insensitive", () => {
    expect(matchesSeverity("CRITICAL", "critical")).toBe(true);
    expect(matchesSeverity("Warning", "WARNING")).toBe(true);
  });
});

describe("verifyWebhookSignature", () => {
  const secret = "whsec_example_0123456789abcdef0123456789abcdef";
  const body = Buffer.from(
    '{"event":"watchdog.alert","severity":"Critical"}',
    "utf8"
  );
  const ts = 1700000000;
  // Published vector in docs/webhooks.md
  const expectedSig =
    "97dc0f80c8e0c9be056f34e48e360411160395a1b3e3a7978092d869315c4615";

  it("accepts conforming deliveries matching the test vector", () => {
    const valid = verifyWebhookSignature(
      secret,
      `t=${ts},v1=${expectedSig}`,
      String(ts),
      body,
      ts * 1000
    );
    expect(valid).toBe(true);
  });

  it("rejects mismatched signature", () => {
    const valid = verifyWebhookSignature(
      secret,
      `t=${ts},v1=badbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadb`,
      String(ts),
      body,
      ts * 1000
    );
    expect(valid).toBe(false);
  });

  it("rejects timestamps outside the 5-minute replay window", () => {
    const valid = verifyWebhookSignature(
      secret,
      `t=${ts},v1=${expectedSig}`,
      String(ts),
      body,
      (ts + 301) * 1000
    );
    expect(valid).toBe(false);
  });

  it("rejects when timestamp header and signature timestamp disagree", () => {
    const valid = verifyWebhookSignature(
      secret,
      `t=${ts},v1=${expectedSig}`,
      String(ts + 1),
      body,
      ts * 1000
    );
    expect(valid).toBe(false);
  });
});

describe("alertNotificationEmbed", () => {
  const alert: ContractAlert = {
    contractId: CONTRACT_A,
    severity: "Critical",
    message: "Contract memory threshold exceeded",
    ledger: 123456,
    txHash: "a".repeat(64),
    timestamp: "2026-11-20T10:00:00Z",
  };

  it("sets severity color, title, and transaction link", () => {
    const embed = alertNotificationEmbed(alert, {
      contractLabel: "Counter Contract",
    });
    const json = embed.toJSON();

    expect(json.color).toBe(0xef4444); // Critical red
    expect(json.title).toContain("Critical Alert: Counter Contract");
    expect(json.url).toBe(
      `https://sorobanexplorer.com/transaction/${alert.txHash}`
    );
    expect(json.description).toBe("Contract memory threshold exceeded");
    expect(json.fields).toEqual(
      expect.arrayContaining([
        { name: "Contract", value: `\`${CONTRACT_A}\``, inline: undefined },
        { name: "Severity", value: "`Critical`", inline: true },
        { name: "Ledger", value: "`123456`", inline: true },
      ])
    );
  });

  it("falls back to amber for Warning and cyan for Info", () => {
    const warnEmbed = alertNotificationEmbed({ ...alert, severity: "Warning" });
    expect(warnEmbed.toJSON().color).toBe(0xf59e0b);

    const infoEmbed = alertNotificationEmbed({ ...alert, severity: "Info" });
    expect(infoEmbed.toJSON().color).toBe(0x38bdf8);
  });
});

describe("routeAlert", () => {
  let dir: string;
  beforeEach(() => {
    dir = mkdtempSync(join(tmpdir(), "sorolens-subs-"));
    openDb(join(dir, "test.db"));
  });

  const criticalAlert: ContractAlert = {
    contractId: CONTRACT_A,
    severity: "Critical",
    message: "Out of gas",
    ledger: 100,
    txHash: "t1",
    timestamp: "2026-11-20T10:00:00Z",
  };

  const infoAlert: ContractAlert = {
    contractId: CONTRACT_A,
    severity: "Info",
    message: "Routine status",
    ledger: 101,
    txHash: "t2",
    timestamp: "2026-11-20T10:01:00Z",
  };

  it("routes alert to subscribed channels matching severity filter", async () => {
    upsertSubscription("c1", CONTRACT_A, "all");
    upsertSubscription("c2", CONTRACT_A, "critical");
    upsertSubscription("c3", CONTRACT_A, "warning");
    upsertSubscription("c4", CONTRACT_B, "all"); // different contract

    const sendC1 = vi.fn().mockResolvedValue({});
    const sendC2 = vi.fn().mockResolvedValue({});
    const sendC3 = vi.fn().mockResolvedValue({});

    const client = {
      channels: {
        fetch: vi.fn((id: string) => {
          if (id === "c1") return Promise.resolve({ isTextBased: () => true, send: sendC1 });
          if (id === "c2") return Promise.resolve({ isTextBased: () => true, send: sendC2 });
          if (id === "c3") return Promise.resolve({ isTextBased: () => true, send: sendC3 });
          return Promise.resolve(null);
        }),
      },
    } as unknown as Client;

    // Dispatch Info alert: only c1 should receive it
    const infoResult = await routeAlert(client, infoAlert);
    expect(infoResult.delivered).toEqual(["c1"]);
    expect(sendC1).toHaveBeenCalledTimes(1);
    expect(sendC2).not.toHaveBeenCalled();
    expect(sendC3).not.toHaveBeenCalled();

    // Dispatch Critical alert: c1, c2, c3 should receive it
    const critResult = await routeAlert(client, criticalAlert);
    expect(critResult.delivered).toEqual(["c1", "c2", "c3"]);
    expect(sendC1).toHaveBeenCalledTimes(2);
    expect(sendC2).toHaveBeenCalledTimes(1);
    expect(sendC3).toHaveBeenCalledTimes(1);
  });

  it("enforces rate limits per channel", async () => {
    upsertSubscription("chan_rate", CONTRACT_A, "all");

    const sendFn = vi.fn().mockResolvedValue({});
    const client = {
      channels: {
        fetch: vi.fn().mockResolvedValue({ isTextBased: () => true, send: sendFn }),
      },
    } as unknown as Client;

    const rateLimiter = new ChannelRateLimiter(2); // max 2 per minute

    const res1 = await routeAlert(client, criticalAlert, { rateLimiter });
    expect(res1.delivered).toEqual(["chan_rate"]);

    const res2 = await routeAlert(client, criticalAlert, { rateLimiter });
    expect(res2.delivered).toEqual(["chan_rate"]);

    // Third call within the window should be rate-limited
    const res3 = await routeAlert(client, criticalAlert, { rateLimiter });
    expect(res3.delivered).toEqual([]);
    expect(res3.rateLimited).toEqual(["chan_rate"]);
    expect(sendFn).toHaveBeenCalledTimes(2);
  });

  it("handles deleted channels gracefully (error 10003) and removes subscriptions", async () => {
    upsertSubscription("deleted_chan", CONTRACT_A, "all");

    const client = {
      channels: {
        fetch: vi.fn().mockRejectedValue(Object.assign(new Error("Unknown Channel"), { code: 10003 })),
      },
    } as unknown as Client;

    const result = await routeAlert(client, criticalAlert);
    expect(result.deleted).toEqual(["deleted_chan"]);
    expect(result.delivered).toEqual([]);

    // Subscriptions for deleted channel must be cleaned up
    expect(listSubscriptionsForChannel("deleted_chan")).toEqual([]);
  });

  it("handles missing permissions gracefully (error 50013) without crashing", async () => {
    upsertSubscription("no_perm_chan", CONTRACT_A, "all");

    const client = {
      channels: {
        fetch: vi.fn().mockResolvedValue({
          isTextBased: () => true,
          send: vi.fn().mockRejectedValue(Object.assign(new Error("Missing Permissions"), { code: 50013 })),
        }),
      },
    } as unknown as Client;

    const result = await routeAlert(client, criticalAlert);
    expect(result.forbidden).toEqual(["no_perm_chan"]);
    expect(result.delivered).toEqual([]);
    // Subscription is preserved so it works once permissions are fixed
    expect(listSubscriptionsForChannel("no_perm_chan")).toHaveLength(1);
  });
});

describe("pollContractAlerts", () => {
  let dir: string;
  beforeEach(() => {
    dir = mkdtempSync(join(tmpdir(), "sorolens-poll-"));
    openDb(join(dir, "test.db"));
  });

  it("initializes baseline on first run and dispatches new alerts on second run", async () => {
    upsertSubscription("c1", CONTRACT_A, "all");

    const sendFn = vi.fn().mockResolvedValue({});
    const client = {
      channels: {
        fetch: vi.fn().mockResolvedValue({ isTextBased: () => true, send: sendFn }),
      },
    } as unknown as Client;

    const mockAlerts: ContractAlert[] = [
      {
        contractId: CONTRACT_A,
        severity: "Critical",
        message: "Old alert",
        ledger: 100,
        txHash: "t100",
        timestamp: "2026-11-20T09:00:00Z",
      },
    ];

    const api = {
      listAlerts: vi.fn().mockImplementation(() => Promise.resolve([...mockAlerts])),
    };

    const state: PollerState = { lastSeenByContract: new Map() };

    // First poll: sets baseline, dispatches 0 alerts
    const initialDispatched = await pollContractAlerts(client, api, state);
    expect(initialDispatched).toBe(0);
    expect(sendFn).not.toHaveBeenCalled();

    // New alert arrives
    mockAlerts.unshift({
      contractId: CONTRACT_A,
      severity: "Warning",
      message: "New alert",
      ledger: 105,
      txHash: "t105",
      timestamp: "2026-11-20T09:05:00Z",
    });

    // Second poll: finds and dispatches the new alert
    const secondDispatched = await pollContractAlerts(client, api, state);
    expect(secondDispatched).toBe(1);
    expect(sendFn).toHaveBeenCalledTimes(1);
  });
});

describe("slash commands: /subscribe, /unsubscribe, /subscriptions", () => {
  let dir: string;
  beforeEach(() => {
    dir = mkdtempSync(join(tmpdir(), "sorolens-cmd-"));
    openDb(join(dir, "test.db"));
  });

  interface MakeInteractionOptions {
    commandName: string;
    hasPermission?: boolean | null; // default channel permission, null = cannot determine
    contract?: string;
    severity?: string;
    channelId?: string;
    targetOption?: {
      id: string;
      name?: string;
      guildId?: string;
      hasPermission?: boolean | null;
    } | null;
    guildId?: string | null;
  }

  function makeInteraction(opts: MakeInteractionOptions) {
    const reply = vi.fn().mockResolvedValue(undefined);
    const channelId = opts.channelId ?? "default_channel";
    const guildId = opts.guildId === undefined ? "guild_123" : opts.guildId;

    const user = { id: "user_123" };
    const member = { id: "user_123" };

    const defaultChannel = {
      id: channelId,
      name: "alerts-channel",
      guildId: "guild_123",
      permissionsFor: vi.fn((_m) => {
        if (opts.hasPermission === null) return null;
        const allowed = opts.hasPermission ?? true;
        return {
          has: vi.fn((perm: bigint) => {
            if (
              perm === PermissionFlagsBits.ManageChannels ||
              perm === PermissionFlagsBits.Administrator
            ) {
              return allowed;
            }
            return false;
          }),
        };
      }),
    };

    const channelsMap = new Map<string, any>();
    channelsMap.set(channelId, defaultChannel);

    let targetChannelObj: any = null;
    if (opts.targetOption) {
      targetChannelObj = {
        id: opts.targetOption.id,
        name: opts.targetOption.name ?? "target-channel",
        guildId: opts.targetOption.guildId ?? "guild_123",
        permissionsFor: vi.fn((_m) => {
          if (opts.targetOption?.hasPermission === null) return null;
          const allowed = opts.targetOption?.hasPermission ?? false;
          return {
            has: vi.fn((perm: bigint) => {
              if (
                perm === PermissionFlagsBits.ManageChannels ||
                perm === PermissionFlagsBits.Administrator
              ) {
                return allowed;
              }
              return false;
            }),
          };
        }),
      };
      channelsMap.set(opts.targetOption.id, targetChannelObj);
    }

    const guild = guildId
      ? {
          id: guildId,
          channels: {
            cache: channelsMap,
            fetch: vi.fn((id: string) =>
              Promise.resolve(channelsMap.get(id) ?? null)
            ),
          },
          members: {
            fetch: vi.fn((_id: string) => Promise.resolve(member)),
          },
        }
      : null;

    return {
      guildId,
      guild,
      user,
      member,
      channelId,
      channel: defaultChannel,
      commandName: opts.commandName,
      options: {
        getString: vi.fn((name: string) => {
          if (name === "contract") return opts.contract ?? null;
          if (name === "severity") return opts.severity ?? null;
          return null;
        }),
        getChannel: vi.fn((name: string) => {
          if (name === "channel") return targetChannelObj;
          return null;
        }),
      },
      reply,
    } as unknown as ChatInputCommandInteraction;
  }

  it("/subscribe rejects users without Manage Channels permission on invocation channel", async () => {
    const interaction = makeInteraction({
      commandName: "subscribe",
      hasPermission: false,
      contract: CONTRACT_A,
    });

    await handleSubscriptionCommand(interaction, mockConfig);
    expect(interaction.reply).toHaveBeenCalledWith(
      expect.objectContaining({
        content: expect.stringContaining("Manage Channels"),
        flags: 64,
      })
    );
    expect(getSubscription("default_channel", CONTRACT_A)).toBeNull();
  });

  it("/subscribe denies cross-channel operation when user lacks Manage Channels on target channel", async () => {
    const interaction = makeInteraction({
      commandName: "subscribe",
      hasPermission: true, // Caller has Manage Channels in invocation channel
      targetOption: {
        id: "chan_announcements",
        name: "announcements",
        guildId: "guild_123",
        hasPermission: false, // But lacks Manage Channels in target channel
      },
      contract: CONTRACT_A,
    });

    await handleSubscriptionCommand(interaction, mockConfig);
    expect(interaction.reply).toHaveBeenCalledWith(
      expect.objectContaining({
        content: expect.stringContaining("chan_announcements"),
        flags: 64,
      })
    );
    expect(interaction.reply).toHaveBeenCalledWith(
      expect.objectContaining({
        content: expect.stringContaining("Manage Channels"),
      })
    );
    expect(getSubscription("chan_announcements", CONTRACT_A)).toBeNull();
  });

  it("/unsubscribe denies cross-channel operation when user lacks Manage Channels on target channel", async () => {
    upsertSubscription("chan_announcements", CONTRACT_A, "all");

    const interaction = makeInteraction({
      commandName: "unsubscribe",
      hasPermission: true,
      targetOption: {
        id: "chan_announcements",
        hasPermission: false,
      },
      contract: CONTRACT_A,
    });

    await handleSubscriptionCommand(interaction, mockConfig);
    expect(interaction.reply).toHaveBeenCalledWith(
      expect.objectContaining({
        content: expect.stringContaining("chan_announcements"),
        flags: 64,
      })
    );
    // Subscription should remain untouched
    expect(getSubscription("chan_announcements", CONTRACT_A)).not.toBeNull();
  });

  it("/subscribe permits cross-channel operation when user has Manage Channels on target channel", async () => {
    const interaction = makeInteraction({
      commandName: "subscribe",
      hasPermission: false, // Caller lacks permission in invocation channel
      targetOption: {
        id: "chan_announcements",
        hasPermission: true, // But has Manage Channels on target channel
      },
      contract: CONTRACT_A,
      severity: "critical",
    });

    await handleSubscriptionCommand(interaction, mockConfig);
    expect(interaction.reply).toHaveBeenCalledWith(
      expect.objectContaining({
        content: expect.stringContaining("Subscribed <#chan_announcements>"),
        flags: 64,
      })
    );
    const sub = getSubscription("chan_announcements", CONTRACT_A);
    expect(sub?.severity).toBe("critical");
  });

  it("/subscribe rejects target channels belonging to a different Discord server", async () => {
    const interaction = makeInteraction({
      commandName: "subscribe",
      hasPermission: true,
      targetOption: {
        id: "foreign_channel",
        guildId: "other_guild_999",
        hasPermission: true,
      },
      contract: CONTRACT_A,
    });

    await handleSubscriptionCommand(interaction, mockConfig);
    expect(interaction.reply).toHaveBeenCalledWith(
      expect.objectContaining({
        content: expect.stringContaining(
          "The selected channel does not belong to this Discord server"
        ),
        flags: 64,
      })
    );
    expect(getSubscription("foreign_channel", CONTRACT_A)).toBeNull();
  });

  it("/subscribe denies operation when caller permissions cannot be determined", async () => {
    const interaction = makeInteraction({
      commandName: "subscribe",
      hasPermission: null,
      contract: CONTRACT_A,
    });

    await handleSubscriptionCommand(interaction, mockConfig);
    expect(interaction.reply).toHaveBeenCalledWith(
      expect.objectContaining({
        content: expect.stringContaining(
          "Could not determine your permissions"
        ),
        flags: 64,
      })
    );
    expect(getSubscription("default_channel", CONTRACT_A)).toBeNull();
  });

  it("/subscribe rejects invalid contract IDs", async () => {
    const interaction = makeInteraction({
      commandName: "subscribe",
      hasPermission: true,
      contract: "INVALID_CONTRACT",
    });

    await handleSubscriptionCommand(interaction, mockConfig);
    expect(interaction.reply).toHaveBeenCalledWith(
      expect.objectContaining({
        content: expect.stringContaining("not a Stellar contract id"),
        flags: 64,
      })
    );
    expect(listSubscriptionsForChannel("default_channel")).toHaveLength(0);
  });

  it("/subscribe saves subscription with chosen severity", async () => {
    const interaction = makeInteraction({
      commandName: "subscribe",
      hasPermission: true,
      contract: CONTRACT_A,
      severity: "critical",
    });

    await handleSubscriptionCommand(interaction, mockConfig);
    expect(interaction.reply).toHaveBeenCalledWith(
      expect.objectContaining({
        content: expect.stringContaining("Subscribed"),
      })
    );

    const sub = getSubscription("default_channel", CONTRACT_A);
    expect(sub?.severity).toBe("critical");
  });

  it("/unsubscribe removes subscription", async () => {
    upsertSubscription("default_channel", CONTRACT_A, "all");

    const interaction = makeInteraction({
      commandName: "unsubscribe",
      hasPermission: true,
      contract: CONTRACT_A,
    });

    await handleSubscriptionCommand(interaction, mockConfig);
    expect(interaction.reply).toHaveBeenCalledWith(
      expect.objectContaining({
        content: expect.stringContaining("Unsubscribed"),
      })
    );
    expect(getSubscription("default_channel", CONTRACT_A)).toBeNull();
  });

  it("/unsubscribe informs when channel was not subscribed", async () => {
    const interaction = makeInteraction({
      commandName: "unsubscribe",
      hasPermission: true,
      contract: CONTRACT_A,
    });

    await handleSubscriptionCommand(interaction, mockConfig);
    expect(interaction.reply).toHaveBeenCalledWith(
      expect.objectContaining({
        content: expect.stringContaining("is not subscribed"),
      })
    );
  });

  it("/subscriptions lists active subscriptions", async () => {
    upsertSubscription("default_channel", CONTRACT_A, "critical");
    upsertSubscription("default_channel", CONTRACT_B, "all");

    const interaction = makeInteraction({
      commandName: "subscriptions",
      hasPermission: true,
    });

    await handleSubscriptionCommand(interaction, mockConfig);
    expect(interaction.reply).toHaveBeenCalledWith(
      expect.objectContaining({
        embeds: [
          expect.objectContaining({
            data: expect.objectContaining({
              title: expect.stringContaining("Alert Subscriptions"),
              description: expect.stringContaining(CONTRACT_A),
            }),
          }),
        ],
      })
    );
  });

  it("/subscriptions informs when no subscriptions exist", async () => {
    const interaction = makeInteraction({
      commandName: "subscriptions",
      hasPermission: true,
    });

    await handleSubscriptionCommand(interaction, mockConfig);
    expect(interaction.reply).toHaveBeenCalledWith(
      expect.objectContaining({
        content: expect.stringContaining("No active alert subscriptions"),
      })
    );
  });
});

describe("webhook routes: /webhook/alerts and /alerts", () => {
  let server: ReturnType<express.Application["listen"]> | null = null;
  let baseUrl: string;
  let deliveredAlerts: any[];

  function signPayload(secret: string, timestamp: number, body: string) {
    const buf = Buffer.from(body, "utf8");
    const signedPayload = Buffer.concat([
      Buffer.from(`${timestamp}.`, "utf8"),
      buf,
    ]);
    const sig = crypto
      .createHmac("sha256", secret)
      .update(signedPayload)
      .digest("hex");
    return {
      timestamp: String(timestamp),
      signature: `t=${timestamp},v1=${sig}`,
    };
  }

  function startServer(config: Config): Promise<string> {
    const app = express();
    deliveredAlerts = [];
    const mockClient = {
      channels: {
        fetch: vi.fn().mockResolvedValue({
          isTextBased: () => true,
          send: vi.fn().mockImplementation((payload) => {
            deliveredAlerts.push(payload);
            return Promise.resolve({});
          }),
        }),
      },
    } as unknown as Client;

    mountWebhookRoutes(app, {
      client: mockClient,
      config,
      tiers: {
        contributor: "c",
        coreContributor: "cc",
        contributorThreshold: 1,
        coreContributorThreshold: 3,
      },
    });

    return new Promise<string>((resolve) => {
      server = app.listen(0, "127.0.0.1", () => {
        const port = (server?.address() as AddressInfo).port;
        baseUrl = `http://127.0.0.1:${port}`;
        resolve(baseUrl);
      });
    });
  }

  afterEach(async () => {
    if (server) {
      await new Promise<void>((resolve) => server!.close(() => resolve()));
      server = null;
    }
  });

  let dir: string;
  beforeEach(() => {
    dir = mkdtempSync(join(tmpdir(), "sorolens-webhook-"));
    openDb(join(dir, "test.db"));
  });

  it.each(["/webhook/alerts", "/alerts"])(
    "%s fails closed (503) when ALERT_WEBHOOK_SECRET is empty or missing",
    async (path) => {
      await startServer({ ...mockConfig, alertWebhookSecret: "" });
      const body = JSON.stringify({
        contract_id: CONTRACT_A,
        severity: "Critical",
        message: "Out of memory",
      });

      const res = await fetch(`${baseUrl}${path}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body,
      });

      expect(res.status).toBe(503);
      const json = (await res.json()) as any;
      expect(json.error).toContain("alert webhook delivery is disabled");
    }
  );

  it.each(["/webhook/alerts", "/alerts"])(
    "%s returns 401 when signature/timestamp headers are missing",
    async (path) => {
      await startServer({
        ...mockConfig,
        alertWebhookSecret: "test-secret-whsec-123",
      });
      const body = JSON.stringify({
        contract_id: CONTRACT_A,
        severity: "Critical",
      });

      const res = await fetch(`${baseUrl}${path}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body,
      });

      expect(res.status).toBe(401);
      const json = (await res.json()) as any;
      expect(json.error).toContain("missing signature or timestamp headers");
    }
  );

  it.each(["/webhook/alerts", "/alerts"])(
    "%s returns 401 when signature is invalid",
    async (path) => {
      const secret = "test-secret-whsec-123";
      await startServer({ ...mockConfig, alertWebhookSecret: secret });
      const body = JSON.stringify({
        contract_id: CONTRACT_A,
        severity: "Critical",
      });
      const nowSec = Math.floor(Date.now() / 1000);

      const res = await fetch(`${baseUrl}${path}`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "x-sorolens-timestamp": String(nowSec),
          "x-sorolens-signature": `t=${nowSec},v1=bad0000000000000000000000000000000000000000000000000000000000000`,
        },
        body,
      });

      expect(res.status).toBe(401);
      const json = (await res.json()) as any;
      expect(json.error).toContain("invalid signature");
    }
  );

  it.each(["/webhook/alerts", "/alerts"])(
    "%s accepts valid signed request (202) and dispatches alert",
    async (path) => {
      const secret = "test-secret-whsec-123";
      await startServer({ ...mockConfig, alertWebhookSecret: secret });
      // Subscribe a channel in db so routing delivers
      upsertSubscription("target_chan", CONTRACT_A, "all");

      const body = JSON.stringify({
        contract_id: CONTRACT_A,
        severity: "Critical",
        message: "Watchdog memory threshold exceeded",
        ledger: 12345,
        tx_hash: "abcd".repeat(16),
      });

      const nowSec = Math.floor(Date.now() / 1000);
      const sig = signPayload(secret, nowSec, body);

      const res = await fetch(`${baseUrl}${path}`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "x-sorolens-timestamp": sig.timestamp,
          "x-sorolens-signature": sig.signature,
        },
        body,
      });

      expect(res.status).toBe(202);
      const json = (await res.json()) as any;
      expect(json.ok).toBe(true);
      expect(json.delivered).toBe(1);
      expect(deliveredAlerts).toHaveLength(1);
    }
  );
});
