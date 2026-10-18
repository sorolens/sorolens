import { afterEach, describe, expect, it, vi } from "vitest";
import { getInitialContracts, getLandingStats } from "./serverData";

describe("server-rendered page data", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("requests a revalidated first contracts page", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ contracts: [], cursor: null, has_more: false }),
    });
    vi.stubGlobal("fetch", fetchMock);

    await getInitialContracts();

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringMatching(/\/api\/v1\/contracts\?limit=20/),
      expect.objectContaining({ next: { revalidate: 60 } })
    );
  });

  it("requests the selected contracts sort from the server", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ contracts: [], cursor: null, has_more: false }),
    });
    vi.stubGlobal("fetch", fetchMock);

    await getInitialContracts("status", "asc");

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringMatching(
        /\/api\/v1\/contracts\?limit=20&sort=status&dir=asc/
      ),
      expect.objectContaining({ next: { revalidate: 60 } })
    );
  });

  it("returns partial landing stats when the API is unavailable", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ tracked_contracts: 4 }),
      })
      .mockResolvedValueOnce({ ok: false });
    vi.stubGlobal("fetch", fetchMock);

    await expect(getLandingStats()).resolves.toEqual({
      global: { tracked_contracts: 4 },
      watchdog: null,
    });
  });
});
