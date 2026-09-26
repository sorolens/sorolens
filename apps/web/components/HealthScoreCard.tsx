"use client";

import { useEffect, useState } from "react";
import { getContractHealthScore } from "@/lib/api";
import type { HealthScoreResponse } from "@/lib/types";

interface HealthScoreCardProps {
  contractId: string;
}

export function HealthScoreCard({ contractId }: HealthScoreCardProps) {
  const [state, setState] = useState<{
    data: HealthScoreResponse | null;
    error: string | null;
  }>({ data: null, error: null });
  const [open, setOpen] = useState(false);

  useEffect(() => {
    let cancelled = false;
    getContractHealthScore(contractId)
      .then((data) => {
        if (!cancelled) setState({ data, error: null });
      })
      .catch((e: unknown) => {
        if (cancelled) return;
        setState({
          data: null,
          error:
            e instanceof Error && /not (found|computed|yet)/i.test(e.message)
              ? "Not yet computed"
              : "Unavailable",
        });
      });
    return () => {
      cancelled = true;
    };
  }, [contractId]);

  const data = state.data;
  const error = state.error;

  if (data && !data.components) {
    return (
      <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-4 text-sm text-[var(--color-text-secondary)]">
        Health score: Unavailable
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-4 text-sm text-[var(--color-text-secondary)]">
        Health score: {error}
      </div>
    );
  }

  if (!data) {
    return null;
  }

  const c = data.components;
  const grade =
    data.score >= 80 ? "safe" : data.score >= 50 ? "warning" : "danger";
  const border =
    grade === "safe"
      ? "var(--color-safe)"
      : grade === "warning"
        ? "var(--color-warning)"
        : "var(--color-danger)";

  return (
    <section className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-4">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        className="w-full text-left"
      >
        <div
          className="flex items-baseline justify-between border-b-4 pb-2"
          style={{ borderColor: border }}
        >
          <span className="text-sm font-medium text-[var(--color-text-secondary)]">
            Health Score
          </span>
          <span className="text-2xl font-bold text-[var(--color-text-primary)]">
            {data.score}
            <span className="ml-1 text-sm font-normal text-[var(--color-text-secondary)]">
              /100
            </span>
          </span>
        </div>
        <div className="mt-3 grid grid-cols-2 gap-x-6 gap-y-1 text-sm">
          {[
            ["Uptime", c.uptime],
            ["Error rate", c.error_rate],
            ["Performance", c.performance],
            ["Storage TTL", c.storage_ttl],
          ].map(([label, value]) => (
            <div
              key={label}
              className="flex justify-between text-[var(--color-text-secondary)]"
            >
              <span>{label}</span>
              <span className="font-medium text-[var(--color-text-primary)]">
                {value}
              </span>
            </div>
          ))}
        </div>
        <div className="mt-2 text-xs text-[var(--color-text-secondary)]">
          {open ? "Hide" : "Show"} component breakdown
        </div>
      </button>
    </section>
  );
}
