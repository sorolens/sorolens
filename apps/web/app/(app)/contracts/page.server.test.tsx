import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { ContractsListResponse } from "@/lib/types";

const serverDataMocks = vi.hoisted(() => ({
  getInitialContracts: vi.fn(),
}));

vi.mock("@/lib/serverData", () => ({
  getInitialContracts: serverDataMocks.getInitialContracts,
}));

vi.mock("./ContractsClient", async () => {
  const React = await import("react");
  const { useSWRConfig } = await import("swr");

  return {
    // The page no longer passes sort props (the client reads everything from
    // the URL); what must reach the client is the SWR fallback the server
    // seeded, so read it straight from the SWR context.
    default: () => {
      const { fallback } = useSWRConfig();
      const seeded = Object.values(fallback ?? {})[0] as
        ContractsListResponse | undefined;
      return React.createElement(
        "p",
        null,
        seeded?.contracts?.[0]?.label ?? "No server data"
      );
    },
  };
});

import ContractsPage from "./page";

describe("server-rendered contracts page", () => {
  afterEach(() => vi.resetAllMocks());

  it("renders the initial list in HTML and honors URL sort parameters", async () => {
    serverDataMocks.getInitialContracts.mockResolvedValue({
      contracts: [
        {
          id: "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
          network: "mainnet",
          label: "Server-rendered contract",
          status: "active",
          wasm_hash: null,
          added_at: "2026-01-01T00:00:00Z",
          last_activity_at: null,
        },
      ],
      cursor: null,
      has_more: false,
    });

    const page = await ContractsPage({
      searchParams: Promise.resolve({ sort: "status", dir: "asc" }),
    });
    const markup = renderToStaticMarkup(page);

    expect(markup).toContain("Server-rendered contract");
    expect(serverDataMocks.getInitialContracts).toHaveBeenCalledWith(
      "status",
      "asc"
    );
  });
});
