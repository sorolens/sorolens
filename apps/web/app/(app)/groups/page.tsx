"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { createGroup, deleteGroup, listGroups } from "@/lib/api";
import type { GroupSummary } from "@/lib/types";
import { getUserId } from "@/lib/user";
import { StatCard } from "@/components/StatCard";
import { CardSkeleton } from "@/components/Skeleton";

function healthTone(score: number): "default" | "warning" | "danger" | "safe" {
  if (score >= 80) return "safe";
  if (score >= 50) return "warning";
  return "danger";
}

function formatHealth(score: number): string {
  return score > 0 ? score.toFixed(1) : "--";
}

/**
 * Portfolio list: every group the caller owns with the aggregate statistics
 * across its contracts, so the whole portfolio fits on one screen.
 */
export default function GroupsPage() {
  const [userId] = useState<string>(getUserId);
  const [groups, setGroups] = useState<GroupSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [name, setName] = useState("");
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await listGroups(userId);
      setGroups(data.groups ?? []);
    } catch {
      // Backend not reachable yet: fall through to the empty state rather
      // than showing an error the user cannot act on.
      setGroups([]);
    } finally {
      setLoading(false);
    }
  }, [userId]);

  useEffect(() => {
    load();
  }, [load]);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = name.trim();
    if (!trimmed || creating) return;
    setCreating(true);
    setError(null);
    try {
      await createGroup(trimmed, userId);
      setName("");
      await load();
    } catch {
      setError("Could not create the group. Check the name and try again.");
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (groupID: string) => {
    try {
      await deleteGroup(groupID, userId);
      setGroups((prev) => prev.filter((g) => g.id !== groupID));
    } catch {
      setError("Could not delete the group.");
    }
  };

  return (
    <div>
      <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <h1 className="text-2xl font-semibold text-[var(--color-text-primary)]">
          Groups
        </h1>

        <form onSubmit={handleCreate} className="flex gap-2">
          <input
            id="group-name"
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="New group name"
            aria-label="New group name"
            maxLength={100}
            className="w-56 rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] px-4 py-2 text-sm text-[var(--color-text-primary)] placeholder-[var(--color-text-secondary)] focus:border-[var(--color-accent)] focus:outline-none"
          />
          <button
            id="create-group-btn"
            type="submit"
            disabled={!name.trim() || creating}
            className="rounded-lg bg-[var(--color-accent)] px-4 py-2 text-sm font-semibold text-[var(--color-bg-page)] transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {creating ? "Creating…" : "New group"}
          </button>
        </form>
      </div>

      {error && (
        <p className="mb-4 text-sm text-[var(--color-danger)]" role="alert">
          {error}
        </p>
      )}

      {loading && (
        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          {Array.from({ length: 2 }).map((_, i) => (
            <div key={i} className="rounded-lg border border-[var(--color-border)] p-4">
              <div className="mb-4 grid grid-cols-2 gap-4 sm:grid-cols-4">
                <CardSkeleton />
                <CardSkeleton />
                <CardSkeleton />
                <CardSkeleton />
              </div>
            </div>
          ))}
        </div>
      )}

      {!loading && groups.length === 0 && (
        <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] px-8 py-16 text-center">
          <p className="text-lg font-medium text-[var(--color-text-primary)]">
            No groups yet
          </p>
          <p className="mt-2 text-sm text-[var(--color-text-secondary)]">
            Create a group above, then add contracts from the{" "}
            <Link
              href="/contracts"
              className="text-[var(--color-accent)] underline underline-offset-2 hover:opacity-80"
            >
              contracts page
            </Link>
            .
          </p>
        </div>
      )}

      {!loading && groups.length > 0 && (
        <ul className="grid grid-cols-1 gap-6 lg:grid-cols-2">
          {groups.map((group) => (
            <li
              key={group.id}
              className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)]/40 p-4"
            >
              <div className="mb-4 flex items-center justify-between gap-3">
                <Link
                  href={`/groups/${group.id}`}
                  className="text-lg font-semibold text-[var(--color-text-primary)] hover:text-[var(--color-accent)]"
                >
                  {group.name}
                </Link>
                <div className="flex items-center gap-3">
                  <span className="text-xs text-[var(--color-text-secondary)]">
                    {group.stats.contract_count}{" "}
                    {group.stats.contract_count === 1 ? "contract" : "contracts"}
                  </span>
                  <button
                    type="button"
                    onClick={() => handleDelete(group.id)}
                    aria-label={`Delete group ${group.name}`}
                    className="rounded-md border border-[var(--color-border)] px-2.5 py-1 text-xs font-medium text-[var(--color-text-secondary)] transition-colors hover:border-[var(--color-danger)] hover:text-[var(--color-danger)]"
                  >
                    Delete
                  </button>
                </div>
              </div>

              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
                <StatCard
                  label="Events"
                  value={group.stats.event_count.toLocaleString()}
                  subtext="Total indexed"
                />
                <StatCard
                  label="Invocations"
                  value={group.stats.invocation_count.toLocaleString()}
                  subtext="Total transactions"
                />
                <StatCard
                  label="Storage Entries"
                  value={group.stats.storage_entry_count.toLocaleString()}
                  subtext="Currently tracked"
                />
                <StatCard
                  label="Avg Health"
                  value={formatHealth(group.stats.average_health_score)}
                  subtext="Across scored contracts"
                  tone={
                    group.stats.average_health_score > 0
                      ? healthTone(group.stats.average_health_score)
                      : "default"
                  }
                />
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
