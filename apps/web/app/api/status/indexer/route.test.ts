import { afterEach, describe, expect, it, vi } from "vitest";
import { GET } from "./route";

describe("indexer status route", () => {
  const originalURL = process.env.INDEXER_METRICS_URL;

  afterEach(() => {
    vi.unstubAllGlobals();
    if (originalURL === undefined) delete process.env.INDEXER_METRICS_URL;
    else process.env.INDEXER_METRICS_URL = originalURL;
  });

  it("returns per-network indexed ledger and lag gauges", async () => {
    process.env.INDEXER_METRICS_URL = "http://indexer.test/metrics";
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          new Response(
            [
              "# HELP sorolens_indexer_lag_ledgers lag",
              'sorolens_indexer_last_indexed_ledger{network="testnet"} 12345',
              'sorolens_indexer_lag_ledgers{network="testnet"} 18',
              'sorolens_indexer_lag_ledgers{network="mainnet"} 0',
              "unrelated_metric 9",
            ].join("\n"),
            { status: 200 }
          )
        )
    );

    const response = await GET();
    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({
      networks: [
        { network: "mainnet", last_indexed_ledger: null, lag_ledgers: 0 },
        { network: "testnet", last_indexed_ledger: 12345, lag_ledgers: 18 },
      ],
    });
  });
});
