import { HealthBadge } from "@/components/WatchdogBadges";
import type { CompareStats, ContractSummary } from "@/lib/types";
import { formatDate } from "@/lib/utils";

interface CompareCardProps {
  contract: ContractSummary;
  stats: CompareStats;
  health_status: string;
  last_check: string | null;
}

export function CompareCard({ contract, stats, health_status, last_check }: CompareCardProps) {
  const label = contract.label ?? contract.id;
  const lastActivity = last_check ?? stats.last_activity;

  return (
    <div className="flex flex-1 flex-col gap-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-card)] p-6">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-lg font-semibold text-[var(--color-text-primary)]">
            {label}
          </h3>
          <p className="mt-0.5 text-xs text-[var(--color-text-secondary)]">
            {contract.id}
          </p>
        </div>
        <HealthBadge status={health_status} />
      </div>

      <div className="grid grid-cols-2 gap-3">
        <StatBlock
          label="Events (24h)"
          value={stats.event_count_24h.toLocaleString()}
        />
        <StatBlock
          label="Events (7d)"
          value={stats.event_count_7d.toLocaleString()}
        />
        <StatBlock
          label="Invocations"
          value={stats.invocation_count.toLocaleString()}
        />
        <StatBlock
          label="Avg CPU"
          value={stats.avg_cpu > 0 ? `${stats.avg_cpu.toLocaleString()}` : "-"}
        />
        <StatBlock
          label="Avg Fee"
          value={stats.avg_fee > 0 ? `${stats.avg_fee.toLocaleString()}` : "-"}
        />
        <StatBlock
          label="Last Activity"
          value={lastActivity ? formatDate(lastActivity) : "-"}
          subtext={lastActivity ?? undefined}
        />
      </div>
    </div>
  );
}

function StatBlock({
  label,
  value,
  subtext,
}: {
  label: string;
  value: string | number;
  subtext?: string;
}) {
  return (
    <div className="rounded-lg bg-[var(--color-bg-page)] p-3">
      <div className="text-xs text-[var(--color-text-secondary)]">{label}</div>
      <div className="mt-1 text-xl font-bold text-[var(--color-text-primary)]">
        {value}
      </div>
      {subtext && (
        <div className="mt-0.5 text-[10px] text-[var(--color-text-secondary)]">
          {subtext}
        </div>
      )}
    </div>
  );
}
