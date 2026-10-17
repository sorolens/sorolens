import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import SLAReportsPage from "./page";

const getContractReportHistory = vi.fn();
const listMonitoredContracts = vi.fn();

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    getContractReportHistory: (...args: unknown[]) =>
      getContractReportHistory(...args),
    listMonitoredContracts: (...args: unknown[]) =>
      listMonitoredContracts(...args),
  };
});

const contractId = "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA";

afterEach(() => {
  cleanup();
  getContractReportHistory.mockReset();
  listMonitoredContracts.mockReset();
  window.history.replaceState(null, "", "/watchdog/reports");
});

describe("SLAReportsPage", () => {
  it("loads a contract/month deep link and offers CSV, PDF, and JSON exports", async () => {
    window.history.replaceState(
      null,
      "",
      `/watchdog/reports?contract=${contractId}&month=2026-09`
    );
    listMonitoredContracts.mockResolvedValue({ contracts: [] });
    getContractReportHistory.mockResolvedValue({
      contract_id: contractId,
      months: [],
    });

    render(<SLAReportsPage />);

    await waitFor(() =>
      expect(getContractReportHistory).toHaveBeenCalledWith(contractId, 12)
    );
    expect(screen.getByLabelText("Month")).toHaveValue("2026-09");
    for (const format of ["csv", "pdf", "json"] as const) {
      expect(
        screen.getByRole("link", { name: `Export ${format.toUpperCase()}` })
      ).toHaveAttribute(
        "href",
        expect.stringContaining(`month=2026-09&format=${format}`)
      );
    }
  });

  it("ignores an invalid month in the query string", async () => {
    window.history.replaceState(
      null,
      "",
      `/watchdog/reports?contract=${contractId}&month=2026-13`
    );
    listMonitoredContracts.mockResolvedValue({ contracts: [] });
    getContractReportHistory.mockResolvedValue({
      contract_id: contractId,
      months: [],
    });

    render(<SLAReportsPage />);

    await waitFor(() =>
      expect(window.location.search).not.toContain("2026-13")
    );
    expect(screen.getByLabelText("Month")).not.toHaveValue("2026-13");
  });
});
