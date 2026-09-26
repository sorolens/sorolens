"use client";

import { useState } from "react";
import { ApiError, getContractSnapshot } from "@/lib/api";
import type { ContractSnapshot } from "@/lib/types";

interface SnapshotPanelProps {
  contractId: string;
  /** Latest indexed ledger, used to prefill the scrubber. */
  currentLedger: number;
}

/**
 * Ledger scrubber for time-travel debugging: pick a ledger, see the storage
 * state and last known event as they were at that point.
 */
export function SnapshotPanel({ contractId, currentLedger }: SnapshotPanelProps) {
  const [ledger, setLedger] = useState("");
  const [snapshot, setSnapshot] = useState<ContractSnapshot | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const handleLoad = async () => {
    const n = Number(ledger);
    if (!Number.isInteger(n) || n <= 0) {
      setError("Enter a positive ledger number");
      return;
    }
    setLoading(true);
    setError(null);
    try {
      setSnapshot(await getContractSnapshot(contractId, n));
    } catch (err) {
      setSnapshot(null);
      setError(
        err instanceof ApiError
          ? err.message
          : "Failed to load snapshot for that ledger",
      );
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="rounded-lg bg-[var(--color-bg-card)]">
      <div className="flex flex-wrap items-end gap-3 border-b border-[var(--color-border)] p-4">
        <div>
          <label
            htmlFor="snapshot-ledger"
            className="mb-1.5 block text-xs font-medium text-[var(--color-text-secondary)]"
          >
            Replay state at ledger
          </label>
          <input
            id="snapshot-ledger"
            type="number"
            min={1}
            value={ledger}
            onChange={(e) => {
              setLedger(e.target.value);
              setError(null);
            }}
            placeholder={currentLedger > 0 ? String(currentLedger) : "e.g. 12345"}
            className="w-48 rounded-lg border border-[var(--color-border)] bg-black/30 px-3 py-2 font-mono text-sm text-[var(--color-text-primary)] placeholder-[var(--color-text-secondary)] focus:border-[var(--color-accent)] focus:outline-none"
          />
        </div>
        <button
          id="snapshot-load-btn"
          type="button"
          onClick={handleLoad}
          disabled={loading || !ledger}
          className="rounded-lg bg-[var(--color-accent)] px-4 py-2 text-sm font-semibold text-[var(--color-bg-page)] transition-opacity hover:opacity-90 disabled:opacity-50"
        >
          {loading ? "Loading…" : "View snapshot"}
        </button>
        {snapshot && (
          <span className="text-xs text-[var(--color-text-secondary)]">
            First tracked ledger: {snapshot.first_tracked_ledger.toLocaleString()}
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

      {snapshot && (
        <div className="p-4">
          {snapshot.last_event ? (
            <div className="mb-4 rounded-md border border-[var(--color-border)] bg-black/20 p-3 text-xs">
              <div className="mb-1 font-medium text-[var(--color-text-secondary)]">
                Last known event at or before ledger {snapshot.ledger.toLocaleString()}
              </div>
              <div className="font-mono">
                {snapshot.last_event.id} · ledger{" "}
                {snapshot.last_event.ledger.toLocaleString()} ·{" "}
                {snapshot.last_event.tx_hash}
              </div>
            </div>
          ) : (
            <p className="mb-4 text-xs text-[var(--color-text-secondary)]">
              No event was recorded at or before ledger{" "}
              {snapshot.ledger.toLocaleString()}.
            </p>
          )}

          {snapshot.storage.length === 0 ? (
            <p className="text-sm text-[var(--color-text-secondary)]">
              No storage entries were live at ledger{" "}
              {snapshot.ledger.toLocaleString()}.
            </p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-sm">
                <thead>
                  <tr className="border-b border-[var(--color-border)] text-xs font-medium text-[var(--color-text-secondary)]">
                    <th className="px-3 py-2">Key</th>
                    <th className="px-3 py-2">Value</th>
                    <th className="px-3 py-2">Durability</th>
                    <th className="px-3 py-2 text-right">Last modified</th>
                  </tr>
                </thead>
                <tbody>
                  {snapshot.storage.map((entry) => (
                    <tr
                      key={entry.key_xdr}
                      className="border-b border-[var(--color-border)]"
                    >
                      <td className="max-w-[220px] truncate px-3 py-2 font-mono text-xs">
                        {entry.key_decoded || entry.key_xdr}
                      </td>
                      <td className="max-w-[220px] truncate px-3 py-2 font-mono text-xs">
                        {entry.value_xdr ?? "-"}
                      </td>
                      <td className="px-3 py-2 text-xs capitalize">
                        {entry.durability}
                      </td>
                      <td className="px-3 py-2 text-right font-mono text-xs text-[var(--color-text-secondary)]">
                        {entry.last_modified_ledger ?? "-"}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}

      {!snapshot && !error && (
        <p className="px-4 py-4 text-sm text-[var(--color-text-secondary)]">
          Enter a ledger to see what this contract&apos;s storage looked like at
          that point in time.
        </p>
      )}
    </div>
  );
}
