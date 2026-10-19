import * as matchers from "@testing-library/jest-dom/matchers";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ContractVersionEntry } from "@sorolens/ui";

expect.extend(matchers);

const getContractChangelog = vi.fn();

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    getContractChangelog: (id: string) => getContractChangelog(id),
  };
});

const CONTRACT_ID = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC";

function entry(
  overrides: Partial<ContractVersionEntry> = {}
): ContractVersionEntry {
  return {
    id: 1,
    contract_id: CONTRACT_ID,
    wasm_hash: "a".repeat(64),
    first_seen_ledger: 100,
    recorded_at: "2026-09-01T00:00:00Z",
    ...overrides,
  };
}

async function renderPanel() {
  const { ContractChangelogPanel } = await import("./ContractChangelogPanel");
  return render(<ContractChangelogPanel contractId={CONTRACT_ID} />);
}

describe("ContractChangelogPanel", () => {
  beforeEach(() => {
    getContractChangelog.mockReset();
  });
  afterEach(cleanup);

  it("renders the changelog returned by the API", async () => {
    getContractChangelog.mockResolvedValue([
      entry({ id: 1, wasm_hash: "a".repeat(64), first_seen_ledger: 100 }),
      entry({ id: 2, wasm_hash: "b".repeat(64), first_seen_ledger: 200 }),
    ]);

    await renderPanel();

    expect(await screen.findAllByTestId("changelog-entry")).toHaveLength(2);
    expect(getContractChangelog).toHaveBeenCalledWith(CONTRACT_ID);
    // Newest first, flagged as latest.
    expect(screen.getByText("latest")).toBeInTheDocument();
    expect(screen.getAllByTestId("changelog-entry")[0]).toHaveTextContent(
      "b".repeat(64)
    );
  });

  it("shows the feed and badge links", async () => {
    getContractChangelog.mockResolvedValue([entry()]);

    await renderPanel();
    await screen.findAllByTestId("changelog-entry");

    const feed = screen.getByRole("link", { name: "Subscribe (RSS / Atom)" });
    expect(feed.getAttribute("href")).toMatch(
      new RegExp(`/api/v1/contracts/${CONTRACT_ID}/changelog/feed$`)
    );
    const badge = screen.getByRole("img", { name: "Wasm hash badge" });
    expect(badge.getAttribute("src")).toMatch(
      new RegExp(`/api/v1/contracts/${CONTRACT_ID}/changelog/badge$`)
    );
    expect(
      screen.getByRole("button", { name: /copy badge markdown/i })
    ).toBeInTheDocument();
  });

  it("keeps feed and badge links visible when there are no upgrades yet", async () => {
    getContractChangelog.mockResolvedValue([]);

    await renderPanel();

    expect(
      await screen.findByText(/no wasm hash transitions/i)
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Subscribe (RSS / Atom)" })
    ).toBeInTheDocument();
  });

  it("explains a 404 instead of failing", async () => {
    const { ApiError } = await import("@/lib/api");
    getContractChangelog.mockRejectedValue(new ApiError(404, "not found"));

    await renderPanel();

    expect(
      await screen.findByText(/no changelog is available/i)
    ).toBeInTheDocument();
    expect(screen.queryByTestId("changelog-entry")).not.toBeInTheDocument();
  });

  it("treats a non-array response as unavailable instead of crashing", async () => {
    getContractChangelog.mockResolvedValue({ id: CONTRACT_ID });

    await renderPanel();

    expect(
      await screen.findByText("Changelog unavailable.")
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Subscribe (RSS / Atom)" })
    ).toBeInTheDocument();
  });

  it("reports other errors as unavailable", async () => {
    getContractChangelog.mockRejectedValue(new Error("boom"));

    await renderPanel();

    expect(
      await screen.findByText("Changelog unavailable.")
    ).toBeInTheDocument();
  });
});
