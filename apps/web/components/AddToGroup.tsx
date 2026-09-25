"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { addContractToGroup, listGroups } from "@/lib/api";
import type { GroupSummary } from "@/lib/types";
import { getUserId } from "@/lib/user";

/**
 * "Add to group" action for the contract detail page. Opens a picker of the
 * caller's groups and adds the contract to every selected group. Membership is
 * many-to-many, so a contract can belong to several portfolios at once.
 */
export function AddToGroup({ contractId }: { contractId: string }) {
  const [userId] = useState<string>(getUserId);
  const [open, setOpen] = useState(false);
  const [groups, setGroups] = useState<GroupSummary[]>([]);
  const [loading, setLoading] = useState(false);
  const [selected, setSelected] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [confirmation, setConfirmation] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await listGroups(userId);
      setGroups(data.groups ?? []);
    } catch {
      setGroups([]);
    } finally {
      setLoading(false);
    }
  }, [userId]);

  useEffect(() => {
    if (open) {
      setError(null);
      load();
    }
  }, [open, load]);

  const toggle = (groupID: string) => {
    setSelected((prev) =>
      prev.includes(groupID)
        ? prev.filter((id) => id !== groupID)
        : [...prev, groupID],
    );
  };

  const handleAdd = async () => {
    if (selected.length === 0 || busy) return;
    setBusy(true);
    setError(null);
    try {
      for (const groupID of selected) {
        await addContractToGroup(groupID, contractId, userId);
      }
      setConfirmation(
        `Added to ${selected.length} group${selected.length === 1 ? "" : "s"}`,
      );
      setSelected([]);
      setOpen(false);
    } catch {
      setError("Could not add the contract to the selected groups.");
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <button
        id="add-to-group-btn"
        type="button"
        onClick={() => setOpen(true)}
        className="rounded-lg border border-[var(--color-border)] px-3 py-1.5 text-xs font-medium text-[var(--color-text-secondary)] transition-colors hover:border-[var(--color-accent)] hover:text-[var(--color-text-primary)]"
      >
        Add to group
      </button>

      {confirmation && (
        <span className="text-xs text-[var(--color-safe)]" role="status">
          {confirmation}
        </span>
      )}

      {open && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
          aria-modal="true"
          role="dialog"
          aria-labelledby="add-to-group-title"
        >
          <div className="w-full max-w-md rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-card)] p-6 shadow-2xl">
            <div className="mb-5 flex items-center justify-between">
              <h2
                id="add-to-group-title"
                className="text-lg font-semibold text-[var(--color-text-primary)]"
              >
                Add to group
              </h2>
              <button
                type="button"
                onClick={() => setOpen(false)}
                aria-label="Close"
                className="rounded-md p-1 text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
              >
                ✕
              </button>
            </div>

            {loading && (
              <p className="text-sm text-[var(--color-text-secondary)]">
                Loading groups…
              </p>
            )}

            {!loading && groups.length === 0 && (
              <p className="text-sm text-[var(--color-text-secondary)]">
                No groups yet.{" "}
                <Link
                  href="/groups"
                  className="text-[var(--color-accent)] underline underline-offset-2 hover:opacity-80"
                >
                  Create a group
                </Link>{" "}
                first.
              </p>
            )}

            {!loading && groups.length > 0 && (
              <ul className="mb-4 max-h-64 space-y-2 overflow-y-auto">
                {groups.map((group) => (
                  <li key={group.id}>
                    <label className="flex cursor-pointer items-center gap-3 rounded-lg border border-[var(--color-border)] px-3 py-2 text-sm text-[var(--color-text-primary)] hover:border-[var(--color-accent)]">
                      <input
                        type="checkbox"
                        checked={selected.includes(group.id)}
                        onChange={() => toggle(group.id)}
                        className="h-4 w-4"
                      />
                      <span className="flex-1">{group.name}</span>
                      <span className="text-xs text-[var(--color-text-secondary)]">
                        {group.stats.contract_count}
                      </span>
                    </label>
                  </li>
                ))}
              </ul>
            )}

            {error && (
              <p className="mb-3 text-sm text-[var(--color-danger)]" role="alert">
                {error}
              </p>
            )}

            <div className="flex gap-3">
              <button
                type="button"
                onClick={() => setOpen(false)}
                className="flex-1 rounded-lg border border-[var(--color-border)] px-4 py-2 text-sm font-medium text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
              >
                Cancel
              </button>
              <button
                id="add-to-group-submit"
                type="button"
                onClick={handleAdd}
                disabled={selected.length === 0 || busy}
                className="flex-1 rounded-lg bg-[var(--color-accent)] px-4 py-2 text-sm font-semibold text-[var(--color-bg-page)] transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {busy ? "Adding…" : "Add"}
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
