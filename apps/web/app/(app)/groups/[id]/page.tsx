"use client";

import { use, useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { getGroup, getGroupStats, removeContractFromGroup } from "@/lib/api";
import type { GroupContract, GroupDetail, GroupStats } from "@/lib/types";
import { getUserId } from "@/lib/user";
import { StatCard } from "@/components/StatCard";
import { CardSkeleton, TableSkeleton } from "@/components/Skeleton";

interface Props {
  params: Promise<{ id: string }>;
}

function healthTone(score: number): "default" | "warning" | "danger" | "safe" {
  if (score >= 80) return "safe";
  if (score >= 50) return "warning";
  return "danger";
}

function formatHealth(score: number): string {
  return score > 0 ? score.toFixed(1) : "--";
}

function formatDate(iso: string | null): string {
  if (!iso) return "--";
  return new Date(iso).toLocaleString();
}

export default function GroupDetailPage({ params }: Props) {
  const { id } = use(params);
  return <GroupDetailContent id={id} />;
}

function GroupDetailContent({ id }: { id: string }) {
  const [userId] = useState<string>(getUserId);
  const [group, setGroup] = useState<GroupDetail | null>(null);
  const [stats, setStats] = useState<GroupStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [detail, aggregate] = await Promise.all([
        getGroup(id, userId),
        getGroupStats(id, userId),
      ]);
      setGroup(detail);
      setStats(aggregate);
      setError(null);
    } catch {
      setGroup(null);
      setError("Group not found");
    } finally {
      setLoading(false);
    }
  }, [id, userId]);

  useEffect(() => {
    load();
  }, [load]);

  const handleRemove = async (contractID: string) => {
    try {
      await removeContractFromGroup(id, contractID, userId);
      await load();
    } catch {
      setError("Could not remove the contract from this group.");
    }
  };

  if (loading) {
    return (
      <div>
        <div className="mb-8 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <CardSkeleton />
          <CardSkeleton />
          <CardSkeleton />
          <CardSkeleton />
        </div>
        <TableSkeleton />
      </div>
    );
  }

  if (error && !group) {
    return (
      <div className="mx-auto max-w-xl rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] px-8 py-16 text-center">
        <p className="text-lg font-medium text-[var(--color-text-primary)]">
          {error}
        </p>
        <p className="mt-2 text-sm text-[var(--color-text-secondary)]">
          Return to the{" "}
          <Link
            href="/groups"
            className="text-[var(--color-accent)] underline underline-offset-2 hover:opacity-80"
          >
            groups list
          </Link>
          .
        </p>
      </div>
    );
  }

  const contracts: GroupContract[] = group?.contracts ?? [];

  return (
    <div>
      <header className="mb-8">
        <div className="mb-1 text-xs font-medium text-[var(--color-text-secondary)]">
          <Link href="/groups" className="hover:text-[var(--color-text-primary)]">
            Groups
          </Link>{" "}
          /{" "}
        </div>
        <h1 className="text-2xl font-semibold text-[var(--color-text-primary)]">
          {group?.name}
        </h1>
      </header>

      {error && (
        <p className="mb-4 text-sm text-[var(--color-danger)]" role="alert">
          {error}
        </p>
      )}

      <section className="mb-8 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-5">
        <StatCard
          label="Contracts"
          value={stats?.contract_count?.toLocaleString() ?? "--"}
          subtext="In this group"
        />
        <StatCard
          label="Events"
          value={stats?.event_count?.toLocaleString() ?? "--"}
          subtext="Total indexed"
        />
        <StatCard
          label="Invocations"
          value={stats?.invocation_count?.toLocaleString() ?? "--"}
          subtext="Total transactions"
        />
        <StatCard
          label="Storage Entries"
          value={stats?.storage_entry_count?.toLocaleString() ?? "--"}
          subtext="Currently tracked"
        />
        <StatCard
          label="Avg Health"
          value={formatHealth(stats?.average_health_score ?? 0)}
          subtext="Across scored contracts"
          tone={
            (stats?.average_health_score ?? 0) > 0
              ? healthTone(stats?.average_health_score ?? 0)
              : "default"
          }
        />
      </section>

      <section className="mb-8">
        <h2 className="mb-4 text-xl font-semibold text-[var(--color-text-primary)]">
          Contracts
        </h2>

        {contracts.length === 0 ? (
          <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] px-8 py-12 text-center">
            <p className="text-sm text-[var(--color-text-secondary)]">
              No contracts in this group yet. Open a contract and use{" "}
              <span className="text-[var(--color-text-primary)]">Add to group</span>.
            </p>
          </div>
        ) : (
          <div className="overflow-x-auto rounded-lg border border-[var(--color-border)]">
            <table className="min-w-full text-left text-sm">
              <thead className="bg-[var(--color-bg-card)] text-[var(--color-text-secondary)]">
                <tr>
                  <th className="px-4 py-2 font-medium">Contract</th>
                  <th className="px-4 py-2 font-medium">Network</th>
                  <th className="px-4 py-2 font-medium">Status</th>
                  <th className="px-4 py-2 font-medium">Health</th>
                  <th className="px-4 py-2 font-medium">Last activity</th>
                  <th className="px-4 py-2 font-medium"></th>
                </tr>
              </thead>
              <tbody>
                {contracts.map((c) => (
                  <tr
                    key={c.contract_id}
                    className="border-t border-[var(--color-border)] transition-colors hover:bg-[var(--color-bg-card)]"
                  >
                    <td className="px-4 py-3 font-mono text-xs">
                      <Link
                        href={`/contracts/${c.contract_id}`}
                        className="text-[var(--color-text-primary)] hover:text-[var(--color-accent)]"
                      >
                        {c.label || c.contract_id}
                      </Link>
                    </td>
                    <td className="px-4 py-3 text-xs text-[var(--color-text-secondary)]">
                      {c.network}
                    </td>
                    <td className="px-4 py-3 text-xs capitalize text-[var(--color-text-secondary)]">
                      {c.status}
                    </td>
                    <td className="px-4 py-3 tabular-nums text-[var(--color-text-primary)]">
                      {c.health_score ?? "--"}
                    </td>
                    <td className="px-4 py-3 text-xs text-[var(--color-text-secondary)]">
                      {formatDate(c.last_activity_at)}
                    </td>
                    <td className="px-4 py-3 text-right">
                      <button
                        type="button"
                        onClick={() => handleRemove(c.contract_id)}
                        aria-label={`Remove ${c.contract_id} from group`}
                        className="rounded-md border border-[var(--color-border)] px-2.5 py-1 text-xs font-medium text-[var(--color-text-secondary)] transition-colors hover:border-[var(--color-danger)] hover:text-[var(--color-danger)]"
                      >
                        Remove
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}
