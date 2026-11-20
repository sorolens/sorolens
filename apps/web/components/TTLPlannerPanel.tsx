"use client";

import { useState } from "react";
import type { StorageEntry } from "@/lib/types";

interface TTLPlannerPanelProps {
  entries: StorageEntry[];
  currentLedger: number;
  network: string;
}

const WARNING_THRESHOLD_LEDGERS = 34560; // ~2 days at 5s/ledger

export function TTLPlannerPanel({
  entries,
  currentLedger,
  network,
}: TTLPlannerPanelProps) {
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [extensionLedgers, setExtensionLedgers] = useState(120960); // ~7 days
  const [result, setResult] = useState<{
    cli_command: string;
    entry_count: number;
  } | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const nearExpiry = entries.filter(
    (e) =>
      e.ledgers_until_expiry != null &&
      e.ledgers_until_expiry < WARNING_THRESHOLD_LEDGERS,
  );

  if (nearExpiry.length === 0) return null;

  const toggleEntry = (keyXdr: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(keyXdr)) next.delete(keyXdr);
      else next.add(keyXdr);
      return next;
    });
  };

  const toggleAll = () => {
    if (selected.size === nearExpiry.length) {
      setSelected(new Set());
    } else {
      setSelected(new Set(nearExpiry.map((e) => e.key_xdr)));
    }
  };

  const plan = async () => {
    setLoading(true);
    setError(null);
    setResult(null);
    try {
      const keys = Array.from(selected);
      const newLiveUntilLedger = currentLedger + extensionLedgers;
      const res = await fetch("/api/ttl-planner", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ keys, newLiveUntilLedger, network }),
      });
      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || `HTTP ${res.status}`);
      }
      setResult(await res.json());
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  };

  return (
    <section className="rounded-lg border border-[var(--color-border)] p-4">
      <h3 className="mb-2 text-sm font-semibold">TTL Extension Planner</h3>
      <p className="mb-3 text-xs text-[var(--color-text-muted)]">
        {nearExpiry.length} entr{nearExpiry.length === 1 ? "y" : "ies"} expiring
        within 2 days. Select entries and generate an unsigned extension
        command.
      </p>

      <ul className="mb-3 max-h-40 space-y-1 overflow-y-auto text-xs">
        {nearExpiry.map((e) => (
          <li key={e.key_xdr} className="flex items-center gap-2">
            <input
              type="checkbox"
              checked={selected.has(e.key_xdr)}
              onChange={() => toggleEntry(e.key_xdr)}
              aria-label={`Select entry ${e.key_xdr.slice(0, 12)}`}
            />
            <span className="font-mono">
              {e.key_xdr.length > 24 ? e.key_xdr.slice(0, 24) + "…" : e.key_xdr}
            </span>
            <span className="ml-auto text-[var(--color-danger)]">
              {e.ledgers_until_expiry} ledgers
            </span>
          </li>
        ))}
      </ul>

      <div className="mb-3 flex flex-wrap items-center gap-3">
        <button
          onClick={toggleAll}
          className="text-xs underline"
          type="button"
        >
          {selected.size === nearExpiry.length ? "Deselect all" : "Select all expiring"}
        </button>
        <label className="flex items-center gap-1 text-xs">
          Extend by
          <input
            type="number"
            value={extensionLedgers}
            min={1}
            onChange={(e) => setExtensionLedgers(Number(e.target.value))}
            className="w-20 rounded border px-1 text-xs"
            aria-label="Extension ledgers"
          />
          ledgers (~{Math.round((extensionLedgers * 5) / 86400)}d)
        </label>
      </div>

      <button
        onClick={plan}
        disabled={selected.size === 0 || loading}
        type="button"
        className="rounded bg-[var(--color-primary)] px-3 py-1 text-xs text-white disabled:opacity-50"
      >
        {loading ? "Planning…" : "Generate CLI Command"}
      </button>

      {error && (
        <p className="mt-2 text-xs text-[var(--color-danger)]">{error}</p>
      )}

      {result && (
        <div className="mt-3">
          <p className="mb-1 text-xs text-[var(--color-text-muted)]">
            {result.entry_count} entr{result.entry_count === 1 ? "y" : "ies"} ·
            no private key needed
          </p>
          <pre className="overflow-x-auto rounded bg-[var(--color-surface-elevated)] p-2 text-xs">
            {result.cli_command}
          </pre>
          <button
            onClick={() =>
              navigator.clipboard.writeText(result.cli_command)
            }
            type="button"
            className="mt-1 text-xs underline"
          >
            Copy to clipboard
          </button>
        </div>
      )}
    </section>
  );
}
