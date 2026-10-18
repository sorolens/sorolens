/**
 * Tests for apps/web/app/(app)/contracts/[id]/page.tsx failure handling: every
 * request that feeds the page must surface a failure with a retry instead of
 * rendering as "not found" or as an empty section.
 */

import * as matchers from "@testing-library/jest-dom/matchers";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from "@testing-library/react";
import { Suspense } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/link", () => ({
  default: ({
    children,
    href,
  }: {
    children: React.ReactNode;
    href: string;
  }) => <a href={href}>{children}</a>,
}));

const mockNotFound = vi.hoisted(() =>
  vi.fn(() => {
    throw new Error("NEXT_NOT_FOUND");
  })
);
vi.mock("next/navigation", () => ({ notFound: mockNotFound }));

vi.mock("@sorolens/ui", () => ({
  MonoId: ({ value }: { value: string }) => <span>{value}</span>,
}));

const mockGetContract = vi.fn();
const mockGetContractEvents = vi.fn();
const mockGetContractStorage = vi.fn();
const mockGetContractStats = vi.fn();
const mockGetActivitySeries = vi.fn();

vi.mock("@/lib/api", () => ({
  ApiError: class ApiError extends Error {
    constructor(
      public status: number,
      message: string
    ) {
      super(message);
      this.name = "ApiError";
    }
  },
  getContract: (...a: unknown[]) => mockGetContract(...a),
  getContractEvents: (...a: unknown[]) => mockGetContractEvents(...a),
  getContractStorage: (...a: unknown[]) => mockGetContractStorage(...a),
  getContractStats: (...a: unknown[]) => mockGetContractStats(...a),
  getContractSnapshot: vi.fn(),
  addContractTag: vi.fn(),
  removeContractTag: vi.fn(),
}));
vi.mock("@/lib/activitySeries", () => ({
  getActivitySeries: (...a: unknown[]) => mockGetActivitySeries(...a),
}));
vi.mock("@/lib/resourceTrend", () => ({
  getResourceTrend: () => Promise.resolve([]),
}));
vi.mock("@/lib/user", () => ({
  ensureSession: () => Promise.resolve({ user_id: "u" }),
}));
vi.mock("@/hooks/useEventStream", () => ({
  useEventStream: () => ({ isConnected: false }),
}));

// Presentational and unrelated children are stubbed out.
vi.mock("@/components/Skeleton", () => ({
  CardSkeleton: () => <div data-testid="skeleton" />,
  ChartSkeleton: () => <div data-testid="skeleton" />,
  TableSkeleton: () => <div data-testid="skeleton" />,
}));
vi.mock("@/components/StatCard", () => ({
  StatCard: ({ label, value }: { label: string; value: string | number }) => (
    <div data-testid="stat">
      {label}: {value}
    </div>
  ),
}));
vi.mock("@/components/TagInput", () => ({ TagInput: () => <div /> }));
vi.mock("@/components/WindowSelector", () => ({
  WindowSelector: () => <div />,
}));
vi.mock("@/components/EventVolumeChart", () => ({
  EventVolumeChart: () => <div data-testid="event-chart" />,
}));
vi.mock("@/components/InvocationChart", () => ({
  InvocationChart: () => <div data-testid="invocation-chart" />,
}));
vi.mock("@/components/ResourceTrendChart", () => ({
  ResourceTrendChart: () => <div />,
}));
vi.mock("@/components/EventsTable", () => ({
  EventsTable: ({
    events,
    onLoadMore,
    hasMore,
  }: {
    events: { id: string }[];
    onLoadMore: () => void;
    hasMore: boolean;
  }) => (
    <div data-testid="events-table">
      {events.map((e) => (
        <span key={e.id}>{e.id}</span>
      ))}
      {hasMore && (
        <button type="button" onClick={onLoadMore}>
          Load more events
        </button>
      )}
    </div>
  ),
}));
vi.mock("@/components/StoragePanel", () => ({
  StoragePanel: ({
    entries,
    onLoadMore,
    hasMore,
  }: {
    entries: { key: string }[];
    onLoadMore: () => void;
    hasMore: boolean;
  }) => (
    <div data-testid="storage-panel">
      {entries.map((e) => (
        <span key={e.key}>{e.key}</span>
      ))}
      {hasMore && (
        <button type="button" onClick={onLoadMore}>
          Load more storage
        </button>
      )}
    </div>
  ),
}));
vi.mock("@/components/ContractNotesPanel", () => ({
  ContractNotesPanel: () => <div />,
}));
vi.mock("@/components/SnapshotPanel", () => ({ SnapshotPanel: () => <div /> }));
vi.mock("@/components/HealthScoreCard", () => ({
  HealthScoreCard: () => <div />,
}));
vi.mock("@/components/AddToGroup", () => ({ AddToGroup: () => <div /> }));
vi.mock("@/components/VerifiedBadge", () => ({ VerifiedBadge: () => <div /> }));
vi.mock("@/components/CallTracePanel", () => ({
  CallTracePanel: () => <div />,
}));
vi.mock("@/components/ContractChangelogPanel", () => ({
  ContractChangelogPanel: () => <div />,
}));
vi.mock("@/components/ContractForecastChart", () => ({
  ContractForecastChart: () => <div />,
}));

expect.extend(matchers);

const ID = "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA";

const CONTRACT = {
  id: ID,
  network: "testnet",
  label: "My Contract",
  status: "active",
  tags: [],
};
const STATS = {
  event_count: 1234,
  invocation_count: 99,
  storage_count: 5,
  last_synced_ledger: 777,
  window_event_count: 10,
  window_invocation_count: 2,
  window_duration: "7d",
};

// One promise for the whole run (a new one per render would suspend forever),
// pre-marked as fulfilled so React 19's use() reads it without suspending.
const PARAMS = Object.assign(Promise.resolve({ id: ID }), {
  status: "fulfilled",
  value: { id: ID },
}) as Promise<{ id: string }>;

async function renderPage() {
  const { default: Page } = await import("@/app/(app)/contracts/[id]/page");
  return render(
    <Suspense fallback={<div>suspended</div>}>
      <Page params={PARAMS} />
    </Suspense>
  );
}

/** Finds the retry button inside the alert titled `title`. */
async function retryButtonFor(title: string): Promise<HTMLElement> {
  const alert = (await screen.findByText(title)).closest(
    "[role=alert]"
  ) as HTMLElement;
  return within(alert).getByRole("button", { name: "Try again" });
}

describe("ContractDetailPage failure states", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetContract.mockResolvedValue(CONTRACT);
    mockGetContractStats.mockResolvedValue(STATS);
    mockGetActivitySeries.mockResolvedValue({ events: [], invocations: [] });
    mockGetContractEvents.mockResolvedValue({
      events: [{ id: "evt-1" }],
      next_cursor: "",
    });
    mockGetContractStorage.mockResolvedValue({
      storage: [{ key: "storage-1" }],
      next_cursor: "",
    });
  });

  afterEach(() => {
    cleanup();
  });

  it("renders the loaded contract with no error", async () => {
    await renderPage();

    expect(await screen.findByText("My Contract")).toBeInTheDocument();
    expect(await screen.findByText("evt-1")).toBeInTheDocument();
    expect(await screen.findByText("storage-1")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("shows a retryable error, not 'Contract not found', when the contract request fails", async () => {
    mockGetContract.mockRejectedValueOnce(new Error("API down"));
    await renderPage();

    expect(
      await screen.findByText("Couldn't load this contract.")
    ).toBeInTheDocument();
    expect(screen.getByText("API down")).toBeInTheDocument();
    expect(screen.queryByText("Contract not found")).toBeNull();

    fireEvent.click(await retryButtonFor("Couldn't load this contract."));

    expect(await screen.findByText("My Contract")).toBeInTheDocument();
    expect(screen.queryByText("Couldn't load this contract.")).toBeNull();
    expect(mockGetContract).toHaveBeenCalledTimes(2);
  });

  it("still routes a 404 to the not-found page", async () => {
    const { ApiError } = await import("@/lib/api");
    mockGetContract.mockRejectedValueOnce(new ApiError(404, "missing"));
    const errors = vi.spyOn(console, "error").mockImplementation(() => {});
    try {
      await renderPage();
      await vi.waitFor(() => expect(mockNotFound).toHaveBeenCalled());
    } finally {
      errors.mockRestore();
    }
  });

  it("shows an error instead of dashes when the stats request fails, and retries", async () => {
    mockGetContractStats.mockRejectedValueOnce(new Error("stats down"));
    await renderPage();

    expect(
      await screen.findByText("Couldn't load contract stats.")
    ).toBeInTheDocument();
    expect(screen.getByText("stats down")).toBeInTheDocument();

    fireEvent.click(await retryButtonFor("Couldn't load contract stats."));

    expect(await screen.findByText("Events: 1,234")).toBeInTheDocument();
    expect(screen.queryByText("Couldn't load contract stats.")).toBeNull();
  });

  it("shows an error instead of empty charts when the activity request fails, and retries", async () => {
    mockGetActivitySeries.mockRejectedValueOnce(new Error("series down"));
    await renderPage();

    expect(
      await screen.findByText("Couldn't load activity charts.")
    ).toBeInTheDocument();
    expect(screen.queryByTestId("event-chart")).toBeNull();

    fireEvent.click(await retryButtonFor("Couldn't load activity charts."));

    expect(await screen.findByTestId("event-chart")).toBeInTheDocument();
    expect(screen.queryByText("Couldn't load activity charts.")).toBeNull();
  });

  it("shows an error instead of an empty events table when events fail, and retries", async () => {
    mockGetContractEvents.mockRejectedValueOnce(new Error("events down"));
    await renderPage();

    expect(
      await screen.findByText("Couldn't load events.")
    ).toBeInTheDocument();
    expect(screen.queryByTestId("events-table")).toBeNull();

    fireEvent.click(await retryButtonFor("Couldn't load events."));

    expect(await screen.findByText("evt-1")).toBeInTheDocument();
    expect(screen.queryByText("Couldn't load events.")).toBeNull();
  });

  it("shows an error instead of an empty storage panel when storage fails, and retries", async () => {
    mockGetContractStorage.mockRejectedValueOnce(new Error("storage down"));
    await renderPage();

    expect(
      await screen.findByText("Couldn't load storage entries.")
    ).toBeInTheDocument();
    expect(screen.queryByTestId("storage-panel")).toBeNull();

    fireEvent.click(await retryButtonFor("Couldn't load storage entries."));

    expect(await screen.findByText("storage-1")).toBeInTheDocument();
    expect(screen.queryByText("Couldn't load storage entries.")).toBeNull();
  });

  it("keeps loaded events and offers a retry when 'load more' fails", async () => {
    mockGetContractEvents
      .mockResolvedValueOnce({ events: [{ id: "evt-1" }], next_cursor: "c2" })
      .mockRejectedValueOnce(new Error("page 2 down"))
      .mockResolvedValueOnce({ events: [{ id: "evt-2" }], next_cursor: "" });
    await renderPage();

    fireEvent.click(
      await screen.findByRole("button", { name: "Load more events" })
    );

    expect(
      await screen.findByText("Couldn't load more events.")
    ).toBeInTheDocument();
    expect(screen.getByText("evt-1")).toBeInTheDocument();

    fireEvent.click(await retryButtonFor("Couldn't load more events."));

    expect(await screen.findByText("evt-2")).toBeInTheDocument();
    expect(screen.queryByText("Couldn't load more events.")).toBeNull();
  });

  it("keeps loaded storage and offers a retry when 'load more' fails", async () => {
    mockGetContractStorage
      .mockResolvedValueOnce({
        storage: [{ key: "storage-1" }],
        next_cursor: "c2",
      })
      .mockRejectedValueOnce(new Error("page 2 down"))
      .mockResolvedValueOnce({
        storage: [{ key: "storage-2" }],
        next_cursor: "",
      });
    await renderPage();

    fireEvent.click(
      await screen.findByRole("button", { name: "Load more storage" })
    );

    expect(
      await screen.findByText("Couldn't load more storage entries.")
    ).toBeInTheDocument();
    expect(screen.getByText("storage-1")).toBeInTheDocument();

    fireEvent.click(
      await retryButtonFor("Couldn't load more storage entries.")
    );

    expect(await screen.findByText("storage-2")).toBeInTheDocument();
  });
});
