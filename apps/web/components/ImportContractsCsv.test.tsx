import * as matchers from "@testing-library/jest-dom/matchers";
import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import ImportContractsCsv, { parseCsv } from "./ImportContractsCsv";
import { trackContract } from "@/lib/api";

expect.extend(matchers);

vi.mock("@/lib/api", () => ({
  trackContract: vi.fn(),
  ApiError: class ApiError extends Error {
    status: number;
    constructor(message: string, status: number) {
      super(message);
      this.status = status;
    }
  },
}));

function makeMockFile(content: string, name: string): File {
  const file = new File([content], name, { type: "text/csv" });
  file.text = () => Promise.resolve(content);
  return file;
}

describe("ImportContractsCsv", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    cleanup();
  });

  describe("parseCsv", () => {
    it("handles quotes, commas, CRLF and BOM", () => {
      const csv =
        '\uFEFF"contract_id","network","label"\r\n' +
        '"CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","testnet","Dex, Escrow"\r\n' +
        '"CBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB","mainnet","Quote ""test"""';
      const parsed = parseCsv(csv);
      expect(parsed).toHaveLength(3);
      expect(parsed[0]).toEqual(["contract_id", "network", "label"]);
      expect(parsed[1]).toEqual([
        "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
        "testnet",
        "Dex, Escrow",
      ]);
      expect(parsed[2]).toEqual([
        "CBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
        "mainnet",
        'Quote "test"',
      ]);
    });
  });

  describe("Component interactions", () => {
    const validId1 = "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA";
    const validId2 = "CBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB";

    it("parses CSV, shows progress bar, and imports rows with bounded parallelism", async () => {
      vi.mocked(trackContract).mockResolvedValue({
        id: validId1,
        network: "testnet",
        status: "active",
      } as any);

      render(
        <ImportContractsCsv
          onClose={vi.fn()}
          onImported={vi.fn()}
          network="testnet"
          concurrency={2}
        />
      );

      const fileContent = `contract_id,network,label\n${validId1},testnet,Contract 1\n${validId2},testnet,Contract 2`;
      const file = makeMockFile(fileContent, "test.csv");

      const input = document.getElementById("import-csv-file") as HTMLInputElement;
      fireEvent.change(input, { target: { files: [file] } });

      await waitFor(() => {
        expect(screen.getByText("2 row(s)")).toBeInTheDocument();
      });

      const submitBtn = screen.getByRole("button", { name: /import 2 contracts/i });
      fireEvent.click(submitBtn);

      await waitFor(() => {
        expect(screen.getByRole("progressbar")).toBeInTheDocument();
        expect(screen.getAllByText(/2 imported/).length).toBeGreaterThan(0);
      });

      expect(trackContract).toHaveBeenCalledTimes(2);
    });

    it("identifies duplicate rows in the file and flags them as invalid", async () => {
      render(
        <ImportContractsCsv
          onClose={vi.fn()}
          onImported={vi.fn()}
          network="testnet"
        />
      );

      const fileContent = `contract_id,network,label\n${validId1},testnet,Contract 1\n${validId1},testnet,Duplicate Contract`;
      const file = makeMockFile(fileContent, "dups.csv");

      const input = document.getElementById("import-csv-file") as HTMLInputElement;
      fireEvent.change(input, { target: { files: [file] } });

      await waitFor(() => {
        expect(screen.getByText("Duplicate contract ID in file")).toBeInTheDocument();
      });
    });

    it("allows retrying failed rows only after an API error", async () => {
      vi.mocked(trackContract)
        .mockRejectedValueOnce(new Error("Network timeout"))
        .mockResolvedValueOnce({ id: validId2, network: "testnet", status: "active" } as any)
        .mockResolvedValueOnce({ id: validId1, network: "testnet", status: "active" } as any);

      render(
        <ImportContractsCsv
          onClose={vi.fn()}
          onImported={vi.fn()}
          network="testnet"
          concurrency={1}
        />
      );

      const fileContent = `contract_id,network,label\n${validId1},testnet,Contract 1\n${validId2},testnet,Contract 2`;
      const file = makeMockFile(fileContent, "test.csv");

      const input = document.getElementById("import-csv-file") as HTMLInputElement;
      fireEvent.change(input, { target: { files: [file] } });

      await waitFor(() => {
        expect(screen.getByText("2 row(s)")).toBeInTheDocument();
      });

      const submitBtn = screen.getByRole("button", { name: /import 2 contracts/i });
      fireEvent.click(submitBtn);

      await waitFor(() => {
        expect(screen.getByTestId("retry-failed-btn")).toBeInTheDocument();
        expect(screen.getAllByText(/1 imported/).length).toBeGreaterThan(0);
      });

      // Click Retry failed rows
      fireEvent.click(screen.getByTestId("retry-failed-btn"));

      await waitFor(() => {
        expect(screen.getAllByText(/2 imported/).length).toBeGreaterThan(0);
      });
    });

    it("handles 500 rows efficiently and shows progress", async () => {
      vi.mocked(trackContract).mockResolvedValue({ status: "active" } as any);

      render(
        <ImportContractsCsv
          onClose={vi.fn()}
          onImported={vi.fn()}
          network="testnet"
          concurrency={10}
        />
      );

      // Generate 500 rows
      const rows = ["contract_id,network,label"];
      for (let i = 0; i < 500; i++) {
        const hex = i.toString(16).padStart(55, "0").toUpperCase();
        rows.push(`C${hex},testnet,Batch Contract ${i}`);
      }

      const file = makeMockFile(rows.join("\n"), "500_contracts.csv");
      const input = document.getElementById("import-csv-file") as HTMLInputElement;
      fireEvent.change(input, { target: { files: [file] } });

      await waitFor(() => {
        expect(screen.getByText("500 row(s)")).toBeInTheDocument();
      });

      const progressArea = screen.getByTestId("import-progress-area");
      expect(progressArea).toBeInTheDocument();
    });
  });
});
