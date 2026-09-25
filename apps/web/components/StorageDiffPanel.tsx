"use client";

import { useEffect, useRef, useState } from "react";
import { ApiError, getContractStorageDiff } from "@/lib/api";
import type {
  DiffStorageEntry,
  StorageChange,
  StorageDiffResponse,
} from "@/lib/types";

interface StorageDiffPanelProps {
  contractId: string;
  /** Latest indexed ledger, used to prefill a sensible default range. */
  currentLedger: number;
}

const KIND_LABELS: Record<string, string> = {
  created: "Created",
  updated: "Updated",
  deleted: "Deleted",
  expired: "Expired",
};

const KIND_STYLES: Record<string, string> = {
  created: "bg-green-900/40 text-green-400",
  updated: "bg-blue-900/40 text-blue-400",
  deleted: "bg-red-900/40 text-red-400",
  expired: "bg-yellow-900/40 text-yellow-400",
};

const DURABILITY_STYLES: Record<string, string> = {
  persistent: "bg-blue-900/40 text-blue-400",
  temporary: "bg-purple-900/40 text-purple-400",
  instance: "bg-yellow-900/40 text-yellow-400",
};

export interface RenderedValue {
  type: string;
  display: string;
  multiline: boolean;
}

/**
 * Render a decoded storage value according to its runtime type: integers get
 * thousands separators, 56-character C.../G... strings are labelled as
 * addresses, and structs/vectors are pretty-printed as JSON.
 */
export function describeStorageValue(
  entry: DiffStorageEntry | null,
): RenderedValue {
  if (!entry) {
    return { type: "none", display: "—", multiline: false };
  }
  const value = entry.value_decoded;
  if (value === null || value === undefined) {
    if (entry.value_xdr) {
      return { type: "xdr", display: entry.value_xdr, multiline: false };
    }
    return { type: "void", display: "—", multiline: false };
  }
  if (typeof value === "number") {
    return { type: "integer", display: value.toLocaleString(), multiline: false };
  }
  if (typeof value === "boolean") {
    return { type: "bool", display: value ? "true" : "false", multiline: false };
  }
  if (typeof value === "string") {
    if (/^[CG][A-Z2-7]{55}$/.test(value)) {
      return { type: "address", display: value, multiline: false };
    }
    return { type: "string", display: value, multiline: false };
  }
  let display: string;
  try {
    display = JSON.stringify(value, null, 2);
  } catch {
    display = entry.value_xdr ?? "—";
  }
  return {
    type: Array.isArray(value) ? "vec/map" : "struct",
    display,
    multiline: true,
  };
}

function ValueCell({ entry }: { entry: DiffStorageEntry | null }) {
  const rendered = describeStorageValue(entry);
  return (
    <div className="min-w-0">
      <div className="mb-1 text-[10px] uppercase tracking-wide text-[var(--color-text-secondary)]">
        {rendered.type}
      </div>
      <pre
        className={`m-0 whitespace-pre-wrap break-all font-mono text-xs ${
          rendered.multiline ? "" : "truncate"
        }`}
      >
        {rendered.display}
      </pre>
    </div>
  );
}

function keyLabel(change: StorageChange): string {
  if (typeof change.key_decoded === "string" && change.key_decoded.length > 0) {
    return change.key_decoded;
  }
  return change.key_xdr;
}

function ChangeRow({ change }: { change: StorageChange }) {
  const kindStyle =
    KIND_STYLES[change.kind] ?? "bg-white/10 text-[var(--color-text-secondary)]";
  const durabilityStyle =
    DURABILITY_STYLES[change.durability] ??
    "bg-white/10 text-[var(--color-text-secondary)]";

  return (
    <div className="border-b border-[var(--color-border)] px-4 py-4">
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <span
          className={`rounded-full px-2 py-0.5 text-xs font-medium ${kindStyle}`}
        >
          {KIND_LABELS[change.kind] ?? change.kind}
        </span>
        <span className="max-w-full truncate font-mono text-xs text-[var(--color-text-primary)]">
          {keyLabel(change)}
        </span>
        <span
          className={`rounded-full px-2 py-0.5 text-xs font-medium capitalize ${durabilityStyle}`}
        >
          {change.durability}
        </span>
        {change.changed_fields && change.changed_fields.length > 0 && (
          <span className="text-xs text-[var(--color-text-secondary)]">
            changed: {change.changed_fields.join(", ")}
          </span>
        )}
      </div>
      <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
        <div className="rounded-md border border-[var(--color-border)] bg-black/20 p-3">
          <div className="mb-2 text-xs font-medium text-[var(--color-text-secondary)]">
            Before
          </div>
          <ValueCell entry={change.before} />
        </div>
        <div className="rounded-md border border-[var(--color-border)] bg-black/20 p-3">
          <div className="mb-2 text-xs font-medium text-[var(--color-text-secondary)]">
            After
          </div>
          <ValueCell entry={change.after} />
        </div>
      </div>
    </div>
  );
}

/**
 * Storage entry diff between two ledgers. A ledger-range picker drives
 * `GET /contracts/{id}/storage/diff` and the result is shown as a two-column
 * before/after view with typed value rendering.
 */
export function StorageDiffPanel({
  contractId,
  currentLedger,
}: StorageDiffPanelProps) {
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [diff, setDiff] = useState<StorageDiffResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const seeded = useRef(false);

  // Prefill a sensible default range (~1000 ledgers) once the latest ledger
  // is known, without clobbering anything the user has already typed.
  useEffect(() => {
    if (seeded.current || currentLedger <= 0) return;
    seeded.current = true;
    setTo(String(currentLedger));
    setFrom(String(Math.max(1, currentLedger - 1000)));
  }, [currentLedger]);

  const handleCompare = async () => {
    const fromN = Number(from);
    const toN = Number(to);
    if (!Number.isInteger(fromN) || fromN <= 0) {
      setError("Enter a positive 'from' ledger");
      return;
    }
    if (!Number.isInteger(toN) || toN <= 0) {
      setError("Enter a positive 'to' ledger");
      return;
    }
    if (fromN > toN) {
      setError("'from' must be less than or equal to 'to'");
      return;
    }
    setLoading(true);
    setError(null);
    try {
      setDiff(await getContractStorageDiff(contractId, fromN, toN));
    } catch (err) {
      setDiff(null);
      setError(
        err instanceof ApiError ? err.message : "Failed to load storage diff",
      );
    } finally {
      setLoading(false);
    }
  };

  const totalChanges = diff ? diff.changes.length : 0;

  return (
    <div className="rounded-lg bg-[var(--color-bg-card)]">
      <div className="flex flex-wrap items-end gap-3 border-b border-[var(--color-border)] p-4">
        <div>
          <label
            htmlFor="storage-diff-from"
            className="mb-1.5 block text-xs font-medium text-[var(--color-text-secondary)]"
          >
            From ledger
          </label>
          <input
            id="storage-diff-from"
            type="number"
            min={1}
            value={from}
            onChange={(e) => {
              setFrom(e.target.value);
              setError(null);
            }}
            placeholder="e.g. 1000"
            className="w-40 rounded-lg border border-[var(--color-border)] bg-black/30 px-3 py-2 font-mono text-sm text-[var(--color-text-primary)] placeholder-[var(--color-text-secondary)] focus:border-[var(--color-accent)] focus:outline-none"
          />
        </div>
        <div>
          <label
            htmlFor="storage-diff-to"
            className="mb-1.5 block text-xs font-medium text-[var(--color-text-secondary)]"
          >
            To ledger
          </label>
          <input
            id="storage-diff-to"
            type="number"
            min={1}
            value={to}
            onChange={(e) => {
              setTo(e.target.value);
              setError(null);
            }}
            placeholder={currentLedger > 0 ? String(currentLedger) : "e.g. 2000"}
            className="w-40 rounded-lg border border-[var(--color-border)] bg-black/30 px-3 py-2 font-mono text-sm text-[var(--color-text-primary)] placeholder-[var(--color-text-secondary)] focus:border-[var(--color-accent)] focus:outline-none"
          />
        </div>
        <button
          id="storage-diff-compare-btn"
          type="button"
          onClick={handleCompare}
          disabled={loading || !from || !to}
          className="rounded-lg bg-[var(--color-accent)] px-4 py-2 text-sm font-semibold text-[var(--color-bg-page)] transition-opacity hover:opacity-90 disabled:opacity-50"
        >
          {loading ? "Comparing…" : "Compare"}
        </button>
        {diff && (
          <span className="text-xs text-[var(--color-text-secondary)]">
            {totalChanges.toLocaleString()} change
            {totalChanges === 1 ? "" : "s"} between ledgers{" "}
            {diff.from_ledger.toLocaleString()}–
            {diff.to_ledger.toLocaleString()}
          </span>
        )}
      </div>

      {error && (
        <p
          role="alert"
          className="border-b border-[var(--color-border)] px-4 py-3 text-sm text-[var(--color-danger)]"
        >
          {error}
        </p>
      )}

      {diff && (
        <>
          <div className="flex flex-wrap gap-3 border-b border-[var(--color-border)] px-4 py-3 text-xs">
            {(["created", "updated", "deleted", "expired"] as const).map(
              (kind) => (
                <span
                  key={kind}
                  className={`rounded-full px-2.5 py-0.5 font-medium ${KIND_STYLES[kind]}`}
                >
                  {KIND_LABELS[kind]}:{" "}
                  {(diff.counts[kind] ?? 0).toLocaleString()}
                </span>
              ),
            )}
          </div>

          {diff.changes.length === 0 ? (
            <p className="px-4 py-6 text-sm text-[var(--color-text-secondary)]">
              No storage entries changed between ledgers{" "}
              {diff.from_ledger.toLocaleString()} and{" "}
              {diff.to_ledger.toLocaleString()}.
            </p>
          ) : (
            <div>
              {diff.changes.map((change) => (
                <ChangeRow key={change.key_xdr} change={change} />
              ))}
            </div>
          )}
        </>
      )}

      {!diff && !error && (
        <p className="px-4 py-4 text-sm text-[var(--color-text-secondary)]">
          Pick two ledgers to see which storage entries were created, updated,
          or expired in between.
        </p>
      )}
    </div>
  );
}
