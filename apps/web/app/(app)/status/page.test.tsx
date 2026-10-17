import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import StatusPage from "./page";

const api = vi.hoisted(() => ({
  getAPIHealth: vi.fn(),
  getAPIReadiness: vi.fn(),
  getAPIVersion: vi.fn(),
  getIndexerStatus: vi.fn(),
}));

vi.mock("@/lib/api", () => api);

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("StatusPage", () => {
  it("keeps successful sections visible when other status endpoints fail", async () => {
    api.getAPIHealth.mockResolvedValue({
      status: "degraded",
      db: "connected",
      redis: "unreachable",
      timestamp: "2026-10-17T12:00:00Z",
    });
    api.getAPIReadiness.mockRejectedValue(new Error("not ready"));
    api.getAPIVersion.mockResolvedValue({
      version: "1.2.3",
      git_sha: "abc123",
      built_at: "2026-10-17T12:00:00Z",
    });
    api.getIndexerStatus.mockRejectedValue(new Error("metrics offline"));

    render(<StatusPage />);

    expect(
      await screen.findByText(
        /partial status data: readiness, indexer metrics/i
      )
    ).toBeInTheDocument();
    expect(screen.getByText("1.2.3")).toBeInTheDocument();
    expect(screen.getByText("connected")).toBeInTheDocument();
    expect(screen.getByText("unreachable")).toBeInTheDocument();
    expect(
      screen.getByText("Indexer metrics are unavailable.")
    ).toBeInTheDocument();
  });
});
