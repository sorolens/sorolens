"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { MonoId } from "@sorolens/ui";
import { listContracts } from "@/lib/api";
import type { ContractSummary } from "@/lib/types";
import { Skeleton } from "@/components/Skeleton";

export default function ContractsPage() {
  const [contracts, setContracts] = useState<ContractSummary[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;

    async function load() {
      try {
        const data = await listContracts();
        if (!cancelled) setContracts(data.contracts);
      } catch {
        // handled by empty state
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    load();
    return () => {
      cancelled = true;
    };
  }, []);

  if (loading) {
    return (
      <div>
        <h1 className="mb-6 text-2xl font-semibold">Contracts</h1>
        <div className="space-y-2">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-16 w-full" />
          ))}
        </div>
      </div>
    );
  }

  if (contracts.length === 0) {
    return (
      <div>
        <h1 className="mb-6 text-2xl font-semibold">Contracts</h1>
        <p className="text-[var(--color-text-secondary)]">
          No tracked contracts yet.
        </p>
      </div>
    );
  }

  return (
    <div>
      <h1 className="mb-6 text-2xl font-semibold">Contracts</h1>
      <div className="space-y-2">
        {contracts.map((c) => (
          <Link
            key={c.id}
            href={`/contracts/${c.id}`}
            className="flex items-center justify-between rounded-lg bg-[var(--color-bg-card)] px-5 py-4 transition-colors hover:bg-white/5"
          >
            <div className="flex items-center gap-3">
              <div className="font-mono text-sm">
                <MonoId value={c.id} headChars={8} tailChars={8} />
              </div>
              {c.label && (
                <span className="text-sm text-[var(--color-text-secondary)]">
                  {c.label}
                </span>
              )}
            </div>
            <div className="flex items-center gap-3">
              <span className="text-xs text-[var(--color-text-secondary)]">
                {c.network}
              </span>
              <span
                className={`inline-block rounded-full px-2.5 py-0.5 text-xs font-medium capitalize ${
                  c.status === "active"
                    ? "bg-green-900/40 text-green-400"
                    : c.status === "backfilling"
                      ? "bg-blue-900/40 text-blue-400"
                      : c.status === "error"
                        ? "bg-red-900/40 text-red-400"
                        : "bg-yellow-900/40 text-yellow-400"
                }`}
              >
                {c.status}
              </span>
            </div>
          </Link>
        ))}
      </div>
    </div>
  );
}
