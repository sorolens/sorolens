import { describe, it, expect } from "vitest";
import {
  BRAND_COLOR,
  alertsEmbed,
  colorForSeverity,
  colorForStatus,
  contractTitle,
  shortId,
  statusEmbed,
  watchlistEmbed,
} from "./embeds.js";
import type { ContractAlert, ContractStatus } from "./api.js";

const CONTRACT = "C" + "A".repeat(55);

const status: ContractStatus = {
  id: CONTRACT,
  network: "testnet",
  label: "Watchdog",
  status: "active",
  wasmHash: "abcdef1234567890",
  addedAt: "2026-01-01T00:00:00Z",
};

describe("colorForStatus", () => {
  it("maps known statuses and falls back to the brand colour", () => {
    expect(colorForStatus("active")).toBe(0x22c55e);
    expect(colorForStatus("paused")).toBe(0x64748b);
    expect(colorForStatus("error")).toBe(0xef4444);
    expect(colorForStatus("ACTIVE")).toBe(0x22c55e);
    expect(colorForStatus("something-new")).toBe(BRAND_COLOR);
  });
});

describe("colorForSeverity", () => {
  it("maps the three watchdog severities", () => {
    expect(colorForSeverity("Critical")).toBe(0xef4444);
    expect(colorForSeverity("Warning")).toBe(0xf59e0b);
    expect(colorForSeverity("Info")).toBe(0x38bdf8);
    expect(colorForSeverity("nonsense")).toBe(BRAND_COLOR);
  });
});

describe("contractTitle / shortId", () => {
  it("prefers the label", () => {
    expect(contractTitle({ id: CONTRACT, label: "Watchdog" })).toBe("Watchdog");
  });

  it("falls back to a shortened id", () => {
    expect(contractTitle({ id: CONTRACT, label: null })).toBe(shortId(CONTRACT));
  });

  it("does not shorten short values", () => {
    expect(shortId("abc")).toBe("abc");
  });
});

describe("statusEmbed", () => {
  it("renders the status, network and contract", () => {
    const json = statusEmbed(status).toJSON();
    expect(json.color).toBe(0x22c55e);
    expect(json.title).toBe("Contract Watchdog");
    const fields = json.fields ?? [];
    expect(fields.find((f) => f.name === "Status")?.value).toBe("`active`");
    expect(fields.find((f) => f.name === "Network")?.value).toBe("testnet");
    expect(fields.find((f) => f.name === "Contract")?.value).toBe(`\`${CONTRACT}\``);
    expect(fields.find((f) => f.name === "Wasm hash")).toBeDefined();
  });

  it("degrades gracefully for an unknown status, and omits wasm when absent", () => {
    const json = statusEmbed({ ...status, status: "", wasmHash: null, label: null }).toJSON();
    expect(json.color).toBe(BRAND_COLOR);
    const fields = json.fields ?? [];
    expect(fields.find((f) => f.name === "Status")?.value).toBe("`unknown`");
    expect(fields.find((f) => f.name === "Wasm hash")).toBeUndefined();
  });
});

describe("alertsEmbed", () => {
  it("reports an all-clear when there are no alerts", () => {
    const json = alertsEmbed(CONTRACT, []).toJSON();
    expect(json.color).toBe(BRAND_COLOR);
    expect(json.description).toContain("No alerts");
  });

  it("uses the newest alert's severity and lists them", () => {
    const alerts: ContractAlert[] = [
      {
        contractId: CONTRACT,
        severity: "Critical",
        message: "TTL below threshold",
        ledger: 99,
        txHash: "aa",
        timestamp: "",
      },
      {
        contractId: CONTRACT,
        severity: "Info",
        message: "registered",
        ledger: 1,
        txHash: "bb",
        timestamp: "",
      },
    ];
    const json = alertsEmbed(CONTRACT, alerts).toJSON();
    expect(json.color).toBe(0xef4444);
    expect(json.description).toContain("TTL below threshold");
    expect(json.description).toContain("registered");
    expect(json.title).toContain("Alerts for");
  });
});

describe("watchlistEmbed", () => {
  it("confirms an add, including the watchlist size", () => {
    const json = watchlistEmbed(CONTRACT, true, "watch", 3).toJSON();
    expect(json.title).toBe("Watchlist updated");
    expect(json.color).toBe(0x22c55e);
    const fields = json.fields ?? [];
    expect(fields.find((f) => f.name === "In watchlist")?.value).toBe("yes");
    expect(fields.find((f) => f.name === "Watching")?.value).toBe("3 contracts");
  });

  it("confirms a removal without a count", () => {
    const json = watchlistEmbed(CONTRACT, false, "unwatch").toJSON();
    expect(json.title).toBe("Removed from watchlist");
    expect(json.color).toBe(0x64748b);
    const fields = json.fields ?? [];
    expect(fields.find((f) => f.name === "In watchlist")?.value).toBe("no");
    expect(fields.find((f) => f.name === "Watching")).toBeUndefined();
  });
});
