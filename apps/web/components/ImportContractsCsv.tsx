"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { trackContract } from "@/lib/api";
import type { TrackContractRequest } from "@/lib/types";

// Same rule the single-contract form enforces in contracts/page.tsx: Soroban
// contract IDs are 56-character strkeys beginning with 'C'.
const CONTRACT_ID_RE = /^C[A-Z0-9]{55}$/;

const KNOWN_NETWORKS = [
  "testnet",
  "mainnet",
  "futurenet",
  "standalone",
  "local",
] as const;

/** Per-row lifecycle: validated -> importing -> settled. */
type RowStatus = "invalid" | "ready" | "importing" | "ok" | "error";

interface CsvRow {
  /** 1-based line number in the uploaded file, so errors point back at it. */
  line: number;
  contractId: string;
  network: string;
  label: string;
  error: string | null;
  status: RowStatus;
  /** API failure reason, or a note for a skipped row. */
  message: string | null;
}

/**
 * Minimal RFC 4180 CSV reader.
 *
 * Handles the cases spreadsheet exports actually produce: quoted fields,
 * escaped quotes (""), embedded commas and newlines inside quotes, CRLF line
 * endings and a UTF-8 BOM. Returns one array of cells per record.
 */
export function parseCsv(text: string): string[][] {
  const rows: string[][] = [];
  let row: string[] = [];
  let field = "";
  let inQuotes = false;

  // Strip a UTF-8 BOM so the first header cell compares cleanly.
  const src = text.charCodeAt(0) === 0xfeff ? text.slice(1) : text;

  for (let i = 0; i < src.length; i += 1) {
    const ch = src[i];

    if (inQuotes) {
      if (ch === '"') {
        if (src[i + 1] === '"') {
          field += '"';
          i += 1; // consumed the doubled quote
        } else {
          inQuotes = false;
        }
      } else {
        field += ch;
      }
      continue;
    }

    if (ch === '"') {
      inQuotes = true;
    } else if (ch === ",") {
      row.push(field);
      field = "";
    } else if (ch === "\r") {
      // normalised away; the \n that follows ends the record
    } else if (ch === "\n") {
      row.push(field);
      rows.push(row);
      row = [];
      field = "";
    } else {
      field += ch;
    }
  }

  // Flush the trailing field/record when the file has no final newline.
  if (field.length > 0 || row.length > 0) {
    row.push(field);
    rows.push(row);
  }

  return rows;
}

const HEADER_HINTS = [
  "contract_id",
  "contractid",
  "contract",
  "network",
  "label",
  "alias",
];

function looksLikeHeader(cells: string[]): boolean {
  const first = (cells[0] ?? "")
    .trim()
    .toLowerCase()
    .replace(/[\s_-]/g, "");
  if (!first) return false;
  return HEADER_HINTS.some((h) => first === h.replace(/[\s_-]/g, ""));
}

interface ImportContractsCsvProps {
  onClose: () => void;
  /** Called once at least one row imported, so the page can refresh. */
  onImported: () => void;
  /** The dashboard's active network, used to sanity-check the CSV column. */
  network: string;
  userId?: string;
}

/**
 * Modal that bulk-registers contracts from a CSV with columns
 * (contract_id, network, label).
 *
 * Rows are validated before anything is sent, and then imported one at a time
 * so a single bad row cannot abort the batch: every row reports its own
 * success or failure. That is the issue's acceptance criterion — malformed
 * rows surface inline while valid rows still succeed.
 */
export default function ImportContractsCsv({
  onClose,
  onImported,
  network,
  userId,
}: ImportContractsCsvProps) {
  const [rows, setRows] = useState<CsvRow[]>([]);
  const [fileName, setFileName] = useState<string | null>(null);
  const [parseError, setParseError] = useState<string | null>(null);
  const [importing, setImporting] = useState(false);
  const [importedAny, setImportedAny] = useState(false);

  const backdropRef = useRef<HTMLDivElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Close on Escape, unless an import is mid-flight.
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !importing) onClose();
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [onClose, importing]);

  const handleBackdrop = (e: React.MouseEvent) => {
    if (e.target === backdropRef.current && !importing) onClose();
  };

  const validate = useCallback(
    (cells: string[], line: number): CsvRow => {
      const contractId = (cells[0] ?? "").trim();
      const rawNetwork = (cells[1] ?? "").trim().toLowerCase();
      const label = (cells[2] ?? "").trim();

      const base: CsvRow = {
        line,
        contractId,
        network: rawNetwork,
        label,
        error: null,
        status: "ready",
        message: null,
      };

      if (!contractId) {
        return { ...base, status: "invalid", error: "Missing contract_id" };
      }
      if (!CONTRACT_ID_RE.test(contractId)) {
        return {
          ...base,
          status: "invalid",
          error: "contract_id must be 56 chars starting with 'C' (A–Z, 0–9)",
        };
      }
      if (
        rawNetwork &&
        !KNOWN_NETWORKS.includes(rawNetwork as (typeof KNOWN_NETWORKS)[number])
      ) {
        return {
          ...base,
          status: "invalid",
          error: `Unknown network '${rawNetwork}'`,
        };
      }
      // The API registers a contract against its own configured network, so a
      // row targeting a different one would silently land on the wrong chain.
      // Fail it loudly instead and tell the reader how to fix it.
      if (rawNetwork && network && rawNetwork !== network.toLowerCase()) {
        return {
          ...base,
          status: "invalid",
          error: `Row targets '${rawNetwork}' but this dashboard is on '${network}'`,
        };
      }
      return base;
    },
    [network]
  );

  const handleFile = useCallback(
    async (file: File) => {
      setParseError(null);
      setFileName(file.name);
      setImportedAny(false);

      let text: string;
      try {
        text = await file.text();
      } catch {
        setParseError("Could not read that file.");
        setRows([]);
        return;
      }

      const records = parseCsv(text).filter((cells) =>
        cells.some((c) => c.trim() !== "")
      );

      if (records.length === 0) {
        setParseError("That file has no rows.");
        setRows([]);
        return;
      }

      // Drop a header row when the first cell names the columns.
      const hasHeader = looksLikeHeader(records[0]);
      const body = hasHeader ? records.slice(1) : records;
      const lineOffset = hasHeader ? 2 : 1;

      if (body.length === 0) {
        setParseError("That file only has a header row.");
        setRows([]);
        return;
      }

      setRows(body.map((cells, i) => validate(cells, i + lineOffset)));
    },
    [validate]
  );

  const validRows = useMemo(
    () => rows.filter((r) => r.status === "ready" || r.status === "ok"),
    [rows]
  );
  const invalidCount = useMemo(
    () =>
      rows.filter((r) => r.status === "invalid" || r.status === "error").length,
    [rows]
  );
  const pendingCount = useMemo(
    () => rows.filter((r) => r.status === "ready").length,
    [rows]
  );

  const setRow = (line: number, patch: Partial<CsvRow>) =>
    setRows((prev) =>
      prev.map((r) => (r.line === line ? { ...r, ...patch } : r))
    );

  const handleImport = async () => {
    setImporting(true);
    let anyOk = false;

    // Sequential on purpose: the API's rate limiter and the per-row error
    // reporting both stay predictable, and one failure never aborts the batch.
    for (const row of rows) {
      if (row.status !== "ready") continue;

      setRow(row.line, { status: "importing", message: null });
      const req: TrackContractRequest = {
        id: row.contractId,
        ...(row.label ? { label: row.label } : {}),
      };

      try {
        await trackContract(req, userId);
        setRow(row.line, { status: "ok", message: null });
        anyOk = true;
      } catch (e) {
        const message = e instanceof Error ? e.message : "Import failed";
        setRow(row.line, { status: "error", message });
      }
    }

    setImporting(false);
    if (anyOk) {
      setImportedAny(true);
      onImported();
    }
  };

  const statusCell = (row: CsvRow) => {
    switch (row.status) {
      case "ok":
        return <span className="text-green-400">Imported</span>;
      case "error":
        return <span className="text-red-400">{row.message ?? "Failed"}</span>;
      case "importing":
        return (
          <span className="text-[var(--color-text-secondary)]">Importing…</span>
        );
      case "invalid":
        return <span className="text-red-400">{row.error}</span>;
      default:
        return (
          <span className="text-[var(--color-text-secondary)]">Ready</span>
        );
    }
  };

  const okCount = rows.filter((r) => r.status === "ok").length;
  const started = rows.length > 0;
  const finished = started && !importing && pendingCount === 0;

  return (
    <div
      ref={backdropRef}
      onClick={handleBackdrop}
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm"
      aria-modal="true"
      role="dialog"
      aria-labelledby="import-csv-title"
    >
      <div className="flex max-h-[90vh] w-full max-w-2xl flex-col rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-card)] p-6 shadow-2xl">
        <div className="mb-4 flex items-center justify-between">
          <h2
            id="import-csv-title"
            className="text-lg font-semibold text-[var(--color-text-primary)]"
          >
            Import contracts from CSV
          </h2>
          <button
            type="button"
            onClick={onClose}
            disabled={importing}
            className="rounded-md p-1 text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)] disabled:opacity-40"
            aria-label="Close modal"
            title="Close modal"
          >
            ✕
          </button>
        </div>

        <p className="mb-4 text-sm text-[var(--color-text-secondary)]">
          Columns: <code className="font-mono">contract_id</code>,{" "}
          <code className="font-mono">network</code>,{" "}
          <code className="font-mono">label</code>. A header row is optional.
          Invalid rows are reported per row and do not stop the valid ones.
        </p>

        <input
          ref={fileInputRef}
          id="import-csv-file"
          type="file"
          accept=".csv,text/csv"
          disabled={importing}
          onChange={(e) => {
            const file = e.target.files?.[0];
            if (file) void handleFile(file);
          }}
          className="mb-4 w-full rounded-lg border border-[var(--color-border)] bg-black/30 px-3 py-2 text-sm text-[var(--color-text-primary)] file:mr-3 file:rounded-md file:border-0 file:bg-[var(--color-accent)] file:px-3 file:py-1.5 file:text-xs file:font-semibold file:text-[var(--color-bg-page)] disabled:opacity-50"
        />

        {parseError && (
          <p className="mb-4 text-sm text-red-400" role="alert">
            {parseError}
          </p>
        )}

        {started && (
          <>
            <div className="mb-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-[var(--color-text-secondary)]">
              <span>{rows.length} row(s)</span>
              <span className="text-green-400">{okCount} imported</span>
              {invalidCount > 0 && (
                <span className="text-red-400">
                  {invalidCount} with problems
                </span>
              )}
              {fileName && <span className="font-mono">{fileName}</span>}
            </div>

            <div className="mb-4 min-h-0 flex-1 overflow-auto rounded-lg border border-[var(--color-border)]">
              <table className="w-full border-collapse text-left text-xs">
                <thead className="sticky top-0 bg-[var(--color-bg-card)]">
                  <tr className="text-[var(--color-text-secondary)]">
                    <th className="px-3 py-2 font-medium">Line</th>
                    <th className="px-3 py-2 font-medium">Contract ID</th>
                    <th className="px-3 py-2 font-medium">Network</th>
                    <th className="px-3 py-2 font-medium">Alias</th>
                    <th className="px-3 py-2 font-medium">Result</th>
                  </tr>
                </thead>
                <tbody>
                  {rows.map((row) => (
                    <tr
                      key={row.line}
                      className="border-t border-[var(--color-border)]"
                    >
                      <td className="px-3 py-2 text-[var(--color-text-secondary)]">
                        {row.line}
                      </td>
                      <td className="px-3 py-2 font-mono text-[var(--color-text-primary)]">
                        {row.contractId
                          ? `${row.contractId.slice(0, 12)}…`
                          : "--"}
                      </td>
                      <td className="px-3 py-2 text-[var(--color-text-secondary)]">
                        {row.network || "--"}
                      </td>
                      <td className="px-3 py-2 text-[var(--color-text-primary)]">
                        {row.label || "--"}
                      </td>
                      <td className="px-3 py-2">{statusCell(row)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </>
        )}

        <div className="mt-auto flex gap-3 pt-2">
          <button
            type="button"
            onClick={onClose}
            disabled={importing}
            className="flex-1 rounded-lg border border-[var(--color-border)] px-4 py-2.5 text-sm font-medium text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)] disabled:opacity-40"
          >
            {finished ? "Close" : "Cancel"}
          </button>
          <button
            id="import-csv-submit"
            type="button"
            onClick={() => void handleImport()}
            disabled={importing || pendingCount === 0}
            className="flex-1 rounded-lg bg-[var(--color-accent)] px-4 py-2.5 text-sm font-semibold text-[var(--color-bg-page)] transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {importing
              ? "Importing…"
              : pendingCount > 0
                ? `Import ${pendingCount} contract${pendingCount === 1 ? "" : "s"}`
                : importedAny
                  ? "Imported"
                  : "Import"}
          </button>
        </div>
      </div>
    </div>
  );
}
