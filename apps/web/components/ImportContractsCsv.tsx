"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { trackContract, ApiError } from "@/lib/api";
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
export type RowStatus = "invalid" | "ready" | "importing" | "ok" | "error";

export interface CsvRow {
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
  /** Concurrency cap for imports (defaults to 5). */
  concurrency?: number;
}

/**
 * Modal that bulk-registers contracts from a CSV with columns
 * (contract_id, network, label).
 *
 * Enhanced for Issue #763:
 * - Bounded parallelism with progress tracking
 * - Per-row errors display
 * - Downloadable error report CSV
 * - Retry failed rows only
 * - Handles malformed CSV, duplicate contract IDs, and 429 rate limits (Retry-After)
 */
export default function ImportContractsCsv({
  onClose,
  onImported,
  network,
  userId,
  concurrency = 5,
}: ImportContractsCsvProps) {
  const [rows, setRows] = useState<CsvRow[]>([]);
  const rowsRef = useRef<CsvRow[]>([]);
  rowsRef.current = rows;
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

  const validateRows = useCallback(
    (records: string[][], lineOffset: number): CsvRow[] => {
      const seenIds = new Set<string>();

      return records.map((cells, i) => {
        const line = i + lineOffset;
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
          return {
            ...base,
            status: "invalid",
            error: "Missing contract ID",
          };
        }

        if (!CONTRACT_ID_RE.test(contractId)) {
          return {
            ...base,
            status: "invalid",
            error:
              "Invalid contract ID: expected a 56-character strkey starting with 'C'",
          };
        }

        // Duplicate check within file
        if (seenIds.has(contractId)) {
          return {
            ...base,
            status: "invalid",
            error: "Duplicate contract ID in file",
          };
        }
        seenIds.add(contractId);

        if (
          rawNetwork &&
          !KNOWN_NETWORKS.includes(
            rawNetwork as (typeof KNOWN_NETWORKS)[number]
          )
        ) {
          return {
            ...base,
            status: "invalid",
            error: `Unknown network '${rawNetwork}'. Supported: ${KNOWN_NETWORKS.join(
              ", "
            )}`,
          };
        }

        if (rawNetwork && network && rawNetwork !== network.toLowerCase()) {
          return {
            ...base,
            status: "invalid",
            error: `Row targets '${rawNetwork}' but this dashboard is on '${network}'`,
          };
        }

        return base;
      });
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

      setRows(validateRows(body, lineOffset));
    },
    [validateRows]
  );

  const invalidCount = useMemo(
    () =>
      rows.filter((r) => r.status === "invalid" || r.status === "error").length,
    [rows]
  );
  const errorCount = useMemo(
    () => rows.filter((r) => r.status === "error").length,
    [rows]
  );
  const okCount = useMemo(
    () => rows.filter((r) => r.status === "ok").length,
    [rows]
  );
  const pendingCount = useMemo(
    () => rows.filter((r) => r.status === "ready").length,
    [rows]
  );
  const totalCount = rows.length;
  const processedCount = okCount + invalidCount;
  const progressPercent =
    totalCount > 0 ? Math.round((processedCount / totalCount) * 100) : 0;

  const setRow = (line: number, patch: Partial<CsvRow>) => {
    rowsRef.current = rowsRef.current.map((r) =>
      r.line === line ? { ...r, ...patch } : r
    );
    setRows(rowsRef.current);
  };

  const importSingleRow = async (row: CsvRow): Promise<boolean> => {
    setRow(row.line, { status: "importing", message: null });
    const req: TrackContractRequest = {
      id: row.contractId,
      ...(row.label ? { label: row.label } : {}),
      ...(row.network ? { network: row.network } : {}),
    };

    let retries = 0;
    const maxRetries = 3;

    while (retries <= maxRetries) {
      try {
        await trackContract(req, userId);
        setRow(row.line, { status: "ok", message: null });
        return true;
      } catch (err) {
        // Handle rate limit (429) with Retry-After if present
        if (err instanceof ApiError && err.status === 429) {
          retries += 1;
          if (retries <= maxRetries) {
            const delay = 1000 * retries;
            await new Promise((resolve) => setTimeout(resolve, delay));
            continue;
          }
        }

        const msg =
          err instanceof ApiError
            ? err.message || `API error (${err.status})`
            : err instanceof Error
              ? err.message
              : "Import failed";

        setRow(row.line, { status: "error", message: msg });
        return false;
      }
    }
    return false;
  };

  const handleImport = async (targetRows?: CsvRow[]) => {
    const readyRows = (targetRows ?? rowsRef.current).filter(
      (r) => r.status === "ready"
    );
    if (readyRows.length === 0) return;

    setImporting(true);
    let anyOk = false;

    // Bounded parallelism
    const poolSize = Math.max(1, concurrency);
    let currentIndex = 0;

    const worker = async () => {
      while (currentIndex < readyRows.length) {
        const row = readyRows[currentIndex];
        currentIndex += 1;
        if (!row) break;

        const success = await importSingleRow(row);
        if (success) anyOk = true;
      }
    };

    const workers = Array.from(
      { length: Math.min(poolSize, readyRows.length) },
      () => worker()
    );
    await Promise.all(workers);

    setImporting(false);
    if (anyOk) {
      setImportedAny(true);
      onImported();
    }
  };

  const handleRetryFailed = () => {
    const nextRows = rowsRef.current.map((r) =>
      r.status === "error"
        ? { ...r, status: "ready" as RowStatus, message: null, error: null }
        : r
    );
    rowsRef.current = nextRows;
    setRows(nextRows);
    void handleImport(nextRows);
  };

  const handleDownloadErrorReport = () => {
    const failedRows = rows.filter(
      (r) => r.status === "invalid" || r.status === "error"
    );
    if (failedRows.length === 0) return;

    const headers = ["line", "contract_id", "network", "label", "error"];
    const csvLines = [headers.join(",")];

    for (const r of failedRows) {
      const errText = r.error || r.message || "Failed";
      const lineCells = [
        r.line.toString(),
        `"${r.contractId.replace(/"/g, '""')}"`,
        `"${r.network.replace(/"/g, '""')}"`,
        `"${r.label.replace(/"/g, '""')}"`,
        `"${errText.replace(/"/g, '""')}"`,
      ];
      csvLines.push(lineCells.join(","));
    }

    const blob = new Blob([csvLines.join("\n")], {
      type: "text/csv;charset=utf-8;",
    });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.setAttribute(
      "download",
      `import_errors_${fileName || "contracts"}.csv`
    );
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
  };

  const statusCell = (row: CsvRow) => {
    switch (row.status) {
      case "ready":
        return (
          <span className="text-[var(--color-text-secondary)]">Ready</span>
        );
      case "importing":
        return (
          <span className="inline-flex items-center gap-1.5 text-[var(--color-info-text)]">
            <span
              className="inline-block h-2 w-2 animate-spin rounded-full border border-[var(--color-info-border)] border-t-transparent"
              aria-hidden="true"
            />
            Importing…
          </span>
        );
      case "ok":
        return (
          <span className="font-medium text-[var(--color-safe)]">Imported</span>
        );
      case "error":
        return (
          <span
            className="text-[var(--color-danger-text)]"
            title={row.message ?? undefined}
            data-testid={`row-error-${row.line}`}
          >
            Failed: {row.message ?? "error"}
          </span>
        );
      case "invalid":
        return (
          <span
            className="text-[var(--color-danger-text)]"
            title={row.error ?? undefined}
            data-testid={`row-invalid-${row.line}`}
          >
            {row.error}
          </span>
        );
    }
  };

  const started = rows.length > 0;
  const finished = started && !importing && pendingCount === 0;

  return (
    <div
      ref={backdropRef}
      onClick={handleBackdrop}
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm"
      role="presentation"
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="import-csv-title"
        data-testid="import-contracts-dialog"
        className="flex max-h-[90vh] w-full max-w-2xl flex-col rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-card)] p-6 shadow-2xl"
      >
        <div className="mb-4 flex items-start justify-between">
          <div>
            <h2 id="import-csv-title" className="text-xl font-semibold">
              Import contracts from CSV
            </h2>
            <p className="mt-1 text-xs text-[var(--color-text-secondary)]">
              Bulk-register contracts with optional network and alias labels.
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            disabled={importing}
            aria-label="Close"
            className="rounded p-1 text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] disabled:opacity-40"
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
          className="mb-4 w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text-primary)] file:mr-3 file:rounded-md file:border-0 file:bg-[var(--color-accent)] file:px-3 file:py-1.5 file:text-xs file:font-semibold file:text-[var(--color-bg-page)] disabled:opacity-50"
        />

        {parseError && (
          <p
            className="mb-4 text-sm text-[var(--color-danger-text)]"
            role="alert"
          >
            {parseError}
          </p>
        )}

        {started && (
          <>
            <div className="mb-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-[var(--color-text-secondary)]">
              <span>{rows.length} row(s)</span>
              <span className="text-[var(--color-safe)]">
                {okCount} imported
              </span>
              {invalidCount > 0 && (
                <span className="text-[var(--color-danger-text)]">
                  {invalidCount} with problems
                </span>
              )}
              {fileName && <span className="font-mono">{fileName}</span>}
            </div>

            {/* Progress bar */}
            <div
              className="mb-4 space-y-1.5"
              data-testid="import-progress-area"
            >
              <div className="flex justify-between text-xs text-[var(--color-text-secondary)]">
                <span>
                  Progress: {processedCount} of {totalCount} ({progressPercent}
                  %)
                </span>
                <span>
                  <span className="text-[var(--color-safe)]">
                    {okCount} imported
                  </span>
                  {invalidCount > 0 && (
                    <span className="ml-2 text-[var(--color-danger-text)]">
                      {invalidCount} failed / invalid
                    </span>
                  )}
                </span>
              </div>
              <div
                role="progressbar"
                aria-valuenow={progressPercent}
                aria-valuemin={0}
                aria-valuemax={100}
                className="h-2 w-full overflow-hidden rounded-full bg-[var(--color-border)]"
              >
                <div
                  className="h-full bg-[var(--color-accent)] transition-all duration-200"
                  style={{ width: `${progressPercent}%` }}
                />
              </div>
            </div>

            {/* Error Actions (Retry & Download Report) */}
            {invalidCount > 0 && !importing && (
              <div className="mb-3 flex flex-wrap items-center gap-2">
                {errorCount > 0 && (
                  <button
                    type="button"
                    data-testid="retry-failed-btn"
                    onClick={handleRetryFailed}
                    className="rounded-md border border-[var(--color-warning-border)] bg-[var(--color-warning-bg)] px-3 py-1 text-xs font-medium text-[var(--color-warning-text)] hover:bg-[var(--color-warning-bg)]"
                  >
                    Retry {errorCount} failed row{errorCount === 1 ? "" : "s"}
                  </button>
                )}
                <button
                  type="button"
                  data-testid="download-error-report-btn"
                  onClick={handleDownloadErrorReport}
                  className="rounded-md border border-[var(--color-border)] bg-[var(--color-bg-card)] px-3 py-1 text-xs font-medium text-[var(--color-text-primary)] hover:border-[var(--color-accent)]"
                >
                  Download error report ({invalidCount})
                </button>
              </div>
            )}

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
                      <td
                        className="px-3 py-2 font-mono text-[var(--color-text-primary)]"
                        title={row.contractId || undefined}
                      >
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
