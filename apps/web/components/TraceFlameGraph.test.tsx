/**
 * Tests for apps/web/components/TraceFlameGraph.tsx and CallTracePanel.tsx
 *
 * We use vitest + @testing-library/react + jsdom. `@sorolens/ui` is mocked so
 * we don't need the built dist, and `@/lib/api` is mocked so we control the
 * trace the panel receives.
 */

import * as matchers from "@testing-library/jest-dom/matchers";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { TraceNode, TraceResponse } from "@/lib/types";
import { TraceFlameGraph } from "./TraceFlameGraph";
import { CallTracePanel } from "./CallTracePanel";

// ── Mock @sorolens/ui ────────────────────────────────────────────────────────
vi.mock("@sorolens/ui", () => ({
  MonoId: ({ value }: { value: string }) => (
    <span data-testid="mono-id">{value}</span>
  ),
}));

// ── Mock @/lib/api ───────────────────────────────────────────────────────────
const mockGetInvocationTrace = vi.fn();

vi.mock("@/lib/api", () => ({
  getInvocationTrace: (...args: unknown[]) => mockGetInvocationTrace(...args),
  ApiError: class ApiError extends Error {
    constructor(
      public status: number,
      message: string,
    ) {
      super(message);
      this.name = "ApiError";
    }
  },
}));

expect.extend(matchers);

// ── Fixtures ─────────────────────────────────────────────────────────────────

const TX_HASH = "a".repeat(64);

function leaf(span: string, depth: number, overrides: Partial<TraceNode> = {}): TraceNode {
  return {
    span_id: span,
    parent_span_id: span.slice(0, span.lastIndexOf(".")),
    contract_id: "CLEAF",
    function_name: "mint",
    cpu: 50,
    mem: 10,
    fee_share: 0,
    depth,
    children: [],
    ...overrides,
  };
}

const ROOT: TraceNode = {
  span_id: "0",
  contract_id: "CROOTCONTRACT",
  function_name: "swap",
  cpu: 1000,
  mem: 500,
  fee_share: 100,
  depth: 0,
  children: [
    {
      span_id: "0.0",
      parent_span_id: "0",
      contract_id: "CMIDDLE",
      function_name: "transfer",
      cpu: 300,
      mem: 100,
      fee_share: 60,
      depth: 1,
      children: [leaf("0.0.0", 2)],
    },
    {
      span_id: "0.1",
      parent_span_id: "0",
      contract_id: "CSIBLING",
      function_name: "burn",
      cpu: 200,
      mem: 80,
      fee_share: 40,
      depth: 1,
      children: [],
    },
  ],
};

const TRACE: TraceResponse = {
  tx_hash: TX_HASH,
  status: "SUCCESS",
  network: "testnet",
  ledger: 1234,
  root: ROOT,
  edge_count: 3,
  has_edges: true,
  truncated: false,
};

// ── TraceFlameGraph ──────────────────────────────────────────────────────────

describe("TraceFlameGraph", () => {
  afterEach(cleanup);

  it("renders one frame per node with its span id and depth", () => {
    render(<TraceFlameGraph root={ROOT} hasEdges truncated={false} />);

    const frames = screen.getAllByTestId("trace-frame");
    expect(frames).toHaveLength(4);

    expect(frames.map((f) => f.getAttribute("data-span-id"))).toEqual([
      "0",
      "0.0",
      "0.0.0",
      "0.1",
    ]);
    expect(frames.map((f) => f.getAttribute("data-depth"))).toEqual([
      "0",
      "1",
      "2",
      "1",
    ]);
  });

  it("labels frames with the function name", () => {
    render(<TraceFlameGraph root={ROOT} hasEdges />);
    expect(screen.getByText("swap")).toBeDefined();
    expect(screen.getByText("transfer")).toBeDefined();
    expect(screen.getByText("mint")).toBeDefined();
    expect(screen.getByText("burn")).toBeDefined();
  });

  it("sizes frames by CPU and says so", () => {
    render(<TraceFlameGraph root={ROOT} hasEdges />);
    expect(screen.getByTestId("flamegraph-metric").textContent).toMatch(
      /cpu instructions/i,
    );
  });

  it("falls back to fee share when no CPU is reported", () => {
    const noCPU: TraceNode = {
      ...ROOT,
      cpu: 0,
      children: ROOT.children.map((c) => ({ ...c, cpu: 0, children: [] })),
    };
    render(<TraceFlameGraph root={noCPU} hasEdges />);
    expect(screen.getByTestId("flamegraph-metric").textContent).toMatch(
      /fee share/i,
    );
  });

  it("explains an empty call graph instead of showing a bare root", () => {
    const bare: TraceNode = { ...ROOT, children: [], cpu: 0, fee_share: 0 };
    render(<TraceFlameGraph root={bare} hasEdges={false} />);
    expect(screen.getByTestId("flamegraph-empty")).toBeDefined();
  });

  it("flags a truncated tree", () => {
    render(<TraceFlameGraph root={ROOT} hasEdges truncated />);
    expect(screen.getByTestId("flamegraph-truncated")).toBeDefined();
  });
});

// ── CallTracePanel ───────────────────────────────────────────────────────────

describe("CallTracePanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(cleanup);

  it("loads and renders the trace for the prefilled transaction", async () => {
    mockGetInvocationTrace.mockResolvedValue(TRACE);

    render(<CallTracePanel initialTxHash={TX_HASH} />);

    await waitFor(() =>
      expect(screen.getByTestId("trace-flamegraph")).toBeDefined(),
    );
    expect(mockGetInvocationTrace).toHaveBeenCalledWith(TX_HASH);
    expect(screen.getByText("3 cross-contract calls")).toBeDefined();
    expect(screen.getByText(/ledger:/i)).toBeDefined();
  });

  it("loads a trace when a hash is typed and submitted", async () => {
    mockGetInvocationTrace.mockResolvedValue(TRACE);
    render(<CallTracePanel />);

    fireEvent.change(screen.getByLabelText(/transaction hash/i), {
      target: { value: TX_HASH.toUpperCase() },
    });
    fireEvent.submit(screen.getByRole("button", { name: /load trace/i }).closest("form")!);

    await waitFor(() =>
      expect(mockGetInvocationTrace).toHaveBeenCalledWith(TX_HASH),
    );
  });

  it("NEGATIVE: rejects a malformed hash without calling the API", async () => {
    render(<CallTracePanel />);

    fireEvent.change(screen.getByLabelText(/transaction hash/i), {
      target: { value: "not-a-hash" },
    });
    fireEvent.submit(screen.getByRole("button", { name: /load trace/i }).closest("form")!);

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toMatch(/64-character transaction hash/i);
    expect(mockGetInvocationTrace).not.toHaveBeenCalled();
    expect(screen.queryByTestId("trace-flamegraph")).toBeNull();
  });

  it("NEGATIVE: explains a 404 as an unknown invocation", async () => {
    const { ApiError } = await import("@/lib/api");
    mockGetInvocationTrace.mockRejectedValue(new ApiError(404, "not found"));

    render(<CallTracePanel initialTxHash={TX_HASH} />);

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toMatch(/no invocation found/i);
  });

  it("NEGATIVE: falls back to a generic message for other errors", async () => {
    mockGetInvocationTrace.mockRejectedValue(new TypeError("Failed to fetch"));

    render(<CallTracePanel initialTxHash={TX_HASH} />);

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toMatch(/could not load the call trace/i);
  });
});
