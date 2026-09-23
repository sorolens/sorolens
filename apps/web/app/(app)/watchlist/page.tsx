"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import {
  listWatchlist,
  addToWatchlist,
  removeFromWatchlist,
} from "@/lib/api";
import type { WatchlistItem } from "@/lib/types";
import { StarButton } from "@/components/StarButton";

const STORAGE_KEY = "sorolens_user_id";

function getUserId(): string {
  if (typeof window === "undefined") return "";
  let id = localStorage.getItem(STORAGE_KEY);
  if (!id) {
    id = `user_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`;
    localStorage.setItem(STORAGE_KEY, id);
  }
  return id;
}

export default function WatchlistPage() {
  const [userId] = useState<string>(getUserId);
  const [items, setItems] = useState<WatchlistItem[]>([]);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await listWatchlist(userId);
      setItems(data.items ?? []);
    } catch {
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, [userId]);

  useEffect(() => {
    load();
  }, [load]);

  const handleToggle = async (contractId: string, currentlyInList: boolean) => {
    try {
      if (currentlyInList) {
        await removeFromWatchlist(contractId, userId);
      } else {
        await addToWatchlist(contractId, userId);
      }
      await load();
    } catch {
      // error handled by UI
    }
  };

  return (
    <div>
      <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <h1 className="text-2xl font-semibold text-[var(--color-text-primary)]">
          Watchlist
        </h1>
      </div>

      {loading && <div className="space-y-4">{Array.from({ length: 5 }).map((_, i) => (
        <div key={i} className="h-16 rounded-lg bg-[var(--color-bg-card)] animate-pulse" />
      ))}</div>}

      {!loading && items.length === 0 && (
        <div className="rounded-lg bg-[var(--color-bg-card)] px-8 py-16 text-center border border-[var(--color-border)]">
          <p className="text-lg font-medium text-[var(--color-text-primary)]">
            No bookmarks yet
          </p>
          <p className="mt-2 text-sm text-[var(--color-text-secondary)]">
            Star a contract on the Contracts page to add it to your watchlist.
          </p>
        </div>
      )}

      {!loading && items.length > 0 && (
        <div className="overflow-x-auto rounded-lg border border-[var(--color-border)]">
          <table className="min-w-full text-left text-sm">
            <thead className="bg-[var(--color-bg-card)] text-[var(--color-text-secondary)]">
              <tr>
                <th className="px-4 py-2 font-medium">Contract</th>
                <th className="px-4 py-2 font-medium">Added</th>
                <th className="px-4 py-2 font-medium"></th>
              </tr>
            </thead>
            <tbody>
              {items.map((item) => (
                <tr
                  key={item.contract_id}
                  className="border-t border-[var(--color-border)] transition-colors hover:bg-[var(--color-bg-card)]"
                >
                  <td className="px-4 py-3 font-mono text-xs text-[var(--color-text-primary)]">
                    <Link
                      href={`/contracts/${item.contract_id}`}
                      className="hover:text-[var(--color-accent)]"
                    >
                      {item.contract_id}
                    </Link>
                  </td>
                  <td className="px-4 py-3 tabular-nums text-[var(--color-text-secondary)]">
                    {new Date(item.added_at).toLocaleDateString()}
                  </td>
                  <td className="px-4 py-3">
                    <StarButton
                      contractId={item.contract_id}
                      userId={userId}
                      initialInWatchlist
                    />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
