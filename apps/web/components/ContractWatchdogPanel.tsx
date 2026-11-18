"use client";

import Link from "next/link";
import { HealthScoreCard } from "@/components/HealthScoreCard";

interface Props {
  contractId: string;
}

export function ContractWatchdogPanel({ contractId }: Props) {
  return (
    <div data-testid="contract-watchdog-panel" className="space-y-6">
      <div className="flex flex-col gap-4 rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-6 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h3 className="text-base font-semibold text-[var(--color-text-primary)]">
            Watchdog Monitoring & Alerts
          </h3>
          <p className="mt-1 text-sm text-[var(--color-text-secondary)]">
            Configure automated health probes, anomaly detection, and alert
            rules for this contract.
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Link
            data-testid="watchdog-dashboard-link"
            href={`/watchdog?contract=${encodeURIComponent(contractId)}`}
            className="rounded-lg border border-[var(--color-border)] px-4 py-2 text-sm font-medium text-[var(--color-text-primary)] transition-colors hover:border-[var(--color-accent)]"
          >
            View Watchdog
          </Link>
          <Link
            data-testid="watchdog-new-rule-link"
            href={`/rules/new?contract=${encodeURIComponent(contractId)}`}
            className="rounded-lg bg-[var(--color-accent)] px-4 py-2 text-sm font-semibold text-[var(--color-bg-page)] transition-opacity hover:opacity-90"
          >
            + Create Rule
          </Link>
        </div>
      </div>

      <section>
        <HealthScoreCard contractId={contractId} />
      </section>
    </div>
  );
}
