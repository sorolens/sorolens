/**
 * Tests for apps/web/components/ImportContractsCsv.tsx (issue #765).
 *
 * The modal parses untrusted user files and writes contracts, so the tests
 * cover both halves: `parseCsv` (quotes, blank lines, BOM, CRLF, duplicates)
 * and the UI (per-row validation, per-row API failure, loading state). The API
 * client is mocked, so nothing here touches the network.
 */

import "@testing-library/jest-dom/vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import ImportContractsCsv, { parseCsv } from "./ImportContractsCsv";

const mockTrackContract = vi.fn();

vi.mock("@/lib/api", () => ({
  trackContract: (...args: unknown[]) => mockTrackContract(...args),
}));

const ID = `C${"A".repeat(55)}`;
const OTHER_ID = `C${"B".repeat(55)}`;

function deferred() {
  let resolve!: (value: unknown) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

/** A File-shaped object: the component only reads `name` and `text()`. */
function csvFile(contents: string, name = "contracts.csv"): File {
  return { name, text: async () => contents } as unknown as File;
}

function renderModal(
  props: Partial<Parameters<typeof ImportContractsCsv>[0]> = {}
) {
  const handlers = { onClose: vi.fn(), onImported: vi.fn() };
  const view = render(
    <ImportContractsCsv
      onClose={handlers.onClose}
      onImported={handlers.onImported}
      network="testnet"
      {...props}
    />
  );
  return { ...handlers, container: view.container };
}

async function upload(container: HTMLElement, file: File) {
  const input =
    container.querySelector<HTMLInputElement>('input[type="file"]')!;
  fireEvent.change(input, { target: { files: [file] } });
}

function rowCount() {
  // The table's header row is the first <tr>.
  return screen.getAllByRole("row").length - 1;
}

beforeEach(() => {
  vi.clearAllMocks();
  mockTrackContract.mockResolvedValue({ id: ID });
});

afterEach(cleanup);

describe("parseCsv", () => {
  it("splits plain rows and columns", () => {
    expect(parseCsv("a,b\nc,d\n")).toEqual([
      ["a", "b"],
      ["c", "d"],
    ]);
  });

  it("handles quoted fields, embedded commas and escaped quotes", () => {
    expect(parseCsv('x,"a,b","say ""hi"""\n')).toEqual([
      ["x", "a,b", 'say "hi"'],
    ]);
  });

  it("keeps a newline that is inside quotes", () => {
    expect(parseCsv('x,"line one\nline two"\n')).toEqual([
      ["x", "line one\nline two"],
    ]);
  });

  it("normalises CRLF line endings", () => {
    expect(parseCsv("a,b\r\nc,d\r\n")).toEqual([
      ["a", "b"],
      ["c", "d"],
    ]);
  });

  it("strips a UTF-8 BOM from the first cell", () => {
    const rows = parseCsv("\uFEFFcontract_id,network\n");
    expect(rows).toEqual([["contract_id", "network"]]);
  });

  it("flushes the last record when the file has no trailing newline", () => {
    expect(parseCsv("a,b")).toEqual([["a", "b"]]);
  });

  it("does not invent an empty record for a trailing newline", () => {
    expect(parseCsv("a,b\n")).toHaveLength(1);
  });
});

describe("ImportContractsCsv", () => {
  it("closes on Escape and on a backdrop click", () => {
    const { onClose } = renderModal();

    fireEvent.keyDown(window, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole("dialog"));
    expect(onClose).toHaveBeenCalledTimes(2);
  });

  it("does not close when the click lands inside the modal", () => {
    const { onClose } = renderModal();

    fireEvent.click(
      screen.getByRole("heading", { name: "Import contracts from CSV" })
    );

    expect(onClose).not.toHaveBeenCalled();
  });

  it("reports an unreadable file instead of an empty table", async () => {
    const { container } = renderModal();
    const file = {
      name: "broken.csv",
      text: () => Promise.reject(new Error("EIO")),
    } as unknown as File;

    await upload(container, file);

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Could not read that file."
    );
    expect(screen.queryByRole("table")).toBeNull();
  });

  it("rejects a file with no rows", async () => {
    const { container } = renderModal();

    await upload(container, csvFile("\n\n\n"));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "That file has no rows."
    );
  });

  it("rejects a file that only has a header row", async () => {
    const { container } = renderModal();

    await upload(container, csvFile("contract_id,network,label\n"));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "That file only has a header row."
    );
  });

  it("accepts a BOM-prefixed header and numbers the data from line 2", async () => {
    const { container } = renderModal();

    await upload(
      container,
      csvFile(`\uFEFFContract ID,Network,Label\n${
        ID
      },testnet,Payments\n`)
    );

    await waitFor(() => expect(rowCount()).toBe(1));
    expect(screen.getByText("2")).toBeVisible(); // line number, header skipped
    expect(screen.getByText("Payments")).toBeVisible();
    expect(
      screen.getByRole("button", { name: "Import 1 contract" })
    ).toBeEnabled();
  });

  it("skips blank lines between records", async () => {
    const { container } = renderModal();

    await upload(
      container,
      csvFile(`${
        ID
      },testnet\n\n${
        OTHER_ID
      },testnet\n\n`)
    );

    await waitFor(() => expect(rowCount()).toBe(2));
    expect(screen.getByText("2 row(s)")).toBeVisible();
  });

  it("lists duplicate rows individually so both are imported", async () => {
    const { container } = renderModal();

    await upload(container, csvFile(`${
      ID
    },testnet\n${
      ID
    },testnet\n`));

    await waitFor(() => expect(rowCount()).toBe(2));
    fireEvent.click(screen.getByRole("button", { name: "Import 2 contracts" }));

    await waitFor(() => expect(mockTrackContract).toHaveBeenCalledTimes(2));
  });

  it("flags a missing contract id", async () => {
    const { container } = renderModal();

    await upload(container, csvFile(",testnet,no id\n"));

    expect(await screen.findByText("Missing contract_id")).toBeVisible();
    expect(screen.getByRole("button", { name: "Import" })).toBeDisabled();
  });

  it("flags a malformed contract id", async () => {
    const { container } = renderModal();

    await upload(container, csvFile("not-a-contract,testnet\n"));

    expect(
      await screen.findByText(
        "contract_id must be 56 chars starting with 'C' (A–Z, 0–9)"
      )
    ).toBeVisible();
  });

  it("flags an unknown network", async () => {
    const { container } = renderModal();

    await upload(container, csvFile(`${
      ID
    },devnet\n`));

    expect(await screen.findByText("Unknown network 'devnet'")).toBeVisible();
  });

  it("flags a row that targets a different network than the dashboard", async () => {
    const { container } = renderModal();

    await upload(container, csvFile(`${
      ID
    },mainnet\n`));

    expect(
      await screen.findByText(
        "Row targets 'mainnet' but this dashboard is on 'testnet'"
      )
    ).toBeVisible();
  });

  it("imports valid rows and reports success once", async () => {
    const { container, onImported } = renderModal();

    await upload(container, csvFile(`${
      ID
    },testnet,Payments\n`));
    await waitFor(() => expect(rowCount()).toBe(1));

    fireEvent.click(screen.getByRole("button", { name: "Import 1 contract" }));

    await waitFor(() => expect(onImported).toHaveBeenCalledTimes(1));
    expect(mockTrackContract).toHaveBeenCalledWith(
      { id: ID, label: "Payments" },
      undefined
    );
    expect(screen.getByText("1 imported")).toBeVisible();
    expect(screen.getByRole("button", { name: "Imported" })).toBeDisabled();
    // Once finished, the secondary action offers to close rather than cancel.
    expect(screen.getByRole("button", { name: "Close" })).toBeVisible();
  });

  it("passes the user id through to the API", async () => {
    const { container } = renderModal({ userId: "user-42" });

    await upload(container, csvFile(`${
      ID
    },testnet\n`));
    await waitFor(() => expect(rowCount()).toBe(1));
    fireEvent.click(screen.getByRole("button", { name: "Import 1 contract" }));

    await waitFor(() =>
      expect(mockTrackContract).toHaveBeenCalledWith({ id: ID }, "user-42")
    );
  });

  it("keeps going after a row fails and surfaces the API error", async () => {
    mockTrackContract
      .mockRejectedValueOnce(new Error("Rate limit exceeded."))
      .mockResolvedValueOnce({ id: OTHER_ID });
    const { container, onImported } = renderModal();

    await upload(
      container,
      csvFile(`${
        ID
      },testnet\n${
        OTHER_ID
      },testnet\n`)
    );
    await waitFor(() => expect(rowCount()).toBe(2));

    fireEvent.click(screen.getByRole("button", { name: "Import 2 contracts" }));

    expect(await screen.findByText("Rate limit exceeded.")).toBeVisible();
    expect(mockTrackContract).toHaveBeenCalledTimes(2);
    expect(screen.getByText("1 imported")).toBeVisible();
    expect(screen.getByText("1 with problems")).toBeVisible();
    // One row succeeded, so the page is still told to refresh.
    expect(onImported).toHaveBeenCalledTimes(1);
  });

  it("does not report success when every row fails", async () => {
    mockTrackContract.mockRejectedValue(new Error("Service unavailable."));
    const { container, onImported } = renderModal();

    await upload(container, csvFile(`${
      ID
    },testnet\n`));
    await waitFor(() => expect(rowCount()).toBe(1));
    fireEvent.click(screen.getByRole("button", { name: "Import 1 contract" }));

    expect(await screen.findByText("Service unavailable.")).toBeVisible();
    expect(onImported).not.toHaveBeenCalled();
    expect(screen.getByText("0 imported")).toBeVisible();
    expect(screen.getByText("1 with problems")).toBeVisible();
  });

  it("skips invalid rows and imports only the valid ones", async () => {
    const { container } = renderModal();

    await upload(
      container,
      csvFile(`bad-id,testnet\n${
        ID
      },testnet\n`)
    );
    await waitFor(() => expect(rowCount()).toBe(2));

    fireEvent.click(screen.getByRole("button", { name: "Import 1 contract" }));

    await waitFor(() => expect(mockTrackContract).toHaveBeenCalledTimes(1));
    expect(mockTrackContract).toHaveBeenCalledWith({ id: ID }, undefined);
  });

  it("shows a loading state and blocks closing while importing", async () => {
    const pending = deferred();
    mockTrackContract.mockReturnValue(pending.promise);
    const { container, onClose, onImported } = renderModal();

    await upload(container, csvFile(`${
      ID
    },testnet\n`));
    await waitFor(() => expect(rowCount()).toBe(1));
    fireEvent.click(screen.getByRole("button", { name: "Import 1 contract" }));

    const submit = screen.getByRole("button", { name: "Importing…" });
    expect(submit).toBeDisabled();
    expect(screen.getByRole("button", { name: "Close modal" })).toBeDisabled();
    expect(
      container.querySelector<HTMLInputElement>('input[type="file"]')
    ).toBeDisabled();

    fireEvent.keyDown(window, { key: "Escape" });
    expect(onClose).not.toHaveBeenCalled();

    pending.resolve({ id: ID });
    await waitFor(() => expect(onImported).toHaveBeenCalledTimes(1));
    expect(screen.queryByRole("button", { name: "Importing…" })).toBeNull();
  });
});
