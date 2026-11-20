"use client";

import React, { useState, useMemo } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useStorageDiff, StorageDurability, StorageDiffEntry, StorageDiffChange } from "../../hooks/useStorageDiff";

interface StorageDiffViewProps {
  contractId: string;
  initialFrom?: number;
  initialTo?: number;
}

type GroupTab = "all" | "created" | "changed" | "removed" | "expired";

export function StorageDiffView({
  contractId,
  initialFrom = 1,
  initialTo = 100,
}: StorageDiffViewProps) {
  const [fromLedger, setFromLedger] = useState<number>(initialFrom);
  const [toLedger, setToLedger] = useState<number>(initialTo);
  const [prefixFilter, setPrefixFilter] = useState<string>("");
  const [durabilityFilter, setDurabilityFilter] = useState<StorageDurability | "">("");
  const [activeTab, setActiveTab] = useState<GroupTab>("all");

  const {
    data,
    loading,
    error,
    refetch,
  } = useStorageDiff({
    contractId,
    from: fromLedger,
    to: toLedger,
    prefix: prefixFilter,
    durability: durabilityFilter,
  });

  // Flatten rows for virtualization based on active tab and filters
  const rows = useMemo(() => {
    if (!data) return [];
    const items: Array<{ type: "created" | "changed" | "removed" | "expired"; item: StorageDiffEntry | StorageDiffChange }> = [];

    if (activeTab === "all" || activeTab === "created") {
      data.created.forEach(i => items.push({ type: "created", item: i }));
    }
    if (activeTab === "all" || activeTab === "changed") {
      data.changed.forEach(i => items.push({ type: "changed", item: i }));
    }
    if (activeTab === "all" || activeTab === "removed") {
      data.removed.forEach(i => items.push({ type: "removed", item: i }));
    }
    if (activeTab === "all" || activeTab === "expired") {
      data.expired.forEach(i => items.push({ type: "expired", item: i }));
    }
    return items;
  }, [data, activeTab]);

  const parentRef = React.useRef<HTMLDivElement>(null);

  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 72,
    overscan: 5,
  });

  const handleExportCSV = () => {
    if (!data) return;
    let csv = "Type,Key,Durability,Old Value,New Value,Old TTL,New TTL\n";
    data.created.forEach(i => {
      csv += `created,"${i.key}",${i.durability},,"${i.val_decoded?.human || i.val || ""}",,${i.live_until_ledger || ""}\n`;
    });
    data.changed.forEach(i => {
      csv += `changed,"${i.key}",${i.durability},"${i.old_val_decoded?.human || i.old_val || ""}","${i.new_val_decoded?.human || i.new_val || ""}",${i.old_live_until || ""},${i.new_live_until || ""}\n`;
    });
    data.removed.forEach(i => {
      csv += `removed,"${i.key}",${i.durability},"${i.val_decoded?.human || i.val || ""}",,,${i.live_until_ledger || ""}\n`;
    });
    data.expired.forEach(i => {
      csv += `expired,"${i.key}",${i.durability},"${i.val_decoded?.human || i.val || ""}",,,${i.live_until_ledger || ""}\n`;
    });

    const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.setAttribute("href", url);
    link.setAttribute("download", `storage-diff-${contractId}-${fromLedger}-${toLedger}.csv`);
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  };

  const handleExportJSON = () => {
    if (!data) return;
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json;charset=utf-8;" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.setAttribute("href", url);
    link.setAttribute("download", `storage-diff-${contractId}-${fromLedger}-${toLedger}.json`);
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  };

  return (
    <div className="flex flex-col gap-6">
      {/* Controls & Filters Header */}
      <div className="flex flex-wrap items-center justify-between gap-4 rounded-xl border border-gray-200 bg-white p-4 shadow-sm dark:border-gray-800 dark:bg-gray-900">
        <div className="flex flex-wrap items-center gap-3">
          <div className="flex items-center gap-2">
            <label className="text-sm font-medium text-gray-700 dark:text-gray-300">From Ledger:</label>
            <input
              type="number"
              value={fromLedger}
              onChange={e => setFromLedger(Number(e.target.value))}
              className="w-28 rounded-lg border border-gray-300 px-3 py-1.5 text-sm dark:border-gray-700 dark:bg-gray-800 dark:text-white"
            />
          </div>
          <div className="flex items-center gap-2">
            <label className="text-sm font-medium text-gray-700 dark:text-gray-300">To Ledger:</label>
            <input
              type="number"
              value={toLedger}
              onChange={e => setToLedger(Number(e.target.value))}
              className="w-28 rounded-lg border border-gray-300 px-3 py-1.5 text-sm dark:border-gray-700 dark:bg-gray-800 dark:text-white"
            />
          </div>
          <div className="flex items-center gap-2">
            <input
              type="text"
              placeholder="Key prefix..."
              value={prefixFilter}
              onChange={e => setPrefixFilter(e.target.value)}
              className="w-36 rounded-lg border border-gray-300 px-3 py-1.5 text-sm dark:border-gray-700 dark:bg-gray-800 dark:text-white"
            />
          </div>
          <div className="flex items-center gap-2">
            <select
              value={durabilityFilter}
              onChange={e => setDurabilityFilter(e.target.value as StorageDurability | "")}
              className="rounded-lg border border-gray-300 px-3 py-1.5 text-sm dark:border-gray-700 dark:bg-gray-800 dark:text-white"
            >
              <option value="">All Durabilities</option>
              <option value="temporary">Temporary</option>
              <option value="persistent">Persistent</option>
              <option value="instance">Instance</option>
            </select>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <button
            onClick={handleExportCSV}
            disabled={!data}
            className="rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300 dark:hover:bg-gray-700"
          >
            Export CSV
          </button>
          <button
            onClick={handleExportJSON}
            disabled={!data}
            className="rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300 dark:hover:bg-gray-700"
          >
            Export JSON
          </button>
        </div>
      </div>

      {/* Group Navigation Tabs */}
      {data && (
        <div className="flex gap-2 border-b border-gray-200 pb-2 dark:border-gray-800">
          <button
            onClick={() => setActiveTab("all")}
            className={`rounded-lg px-3 py-1.5 text-sm font-medium transition-colors ${
              activeTab === "all"
                ? "bg-blue-600 text-white"
                : "bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-gray-800 dark:text-gray-400 dark:hover:bg-gray-700"
            }`}
          >
            All ({(data.created.length + data.changed.length + data.removed.length + data.expired.length)})
          </button>
          <button
            onClick={() => setActiveTab("created")}
            className={`rounded-lg px-3 py-1.5 text-sm font-medium transition-colors ${
              activeTab === "created"
                ? "bg-green-600 text-white"
                : "bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-gray-800 dark:text-gray-400 dark:hover:bg-gray-700"
            }`}
          >
            Created ({data.created.length})
          </button>
          <button
            onClick={() => setActiveTab("changed")}
            className={`rounded-lg px-3 py-1.5 text-sm font-medium transition-colors ${
              activeTab === "changed"
                ? "bg-amber-600 text-white"
                : "bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-gray-800 dark:text-gray-400 dark:hover:bg-gray-700"
            }`}
          >
            Changed ({data.changed.length})
          </button>
          <button
            onClick={() => setActiveTab("removed")}
            className={`rounded-lg px-3 py-1.5 text-sm font-medium transition-colors ${
              activeTab === "removed"
                ? "bg-red-600 text-white"
                : "bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-gray-800 dark:text-gray-400 dark:hover:bg-gray-700"
            }`}
          >
            Removed ({data.removed.length})
          </button>
          <button
            onClick={() => setActiveTab("expired")}
            className={`rounded-lg px-3 py-1.5 text-sm font-medium transition-colors ${
              activeTab === "expired"
                ? "bg-purple-600 text-white"
                : "bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-gray-800 dark:text-gray-400 dark:hover:bg-gray-700"
            }`}
          >
            Expired ({data.expired.length})
          </button>
        </div>
      )}

      {/* Main Content Area */}
      <div className="relative rounded-xl border border-gray-200 bg-white p-4 shadow-sm dark:border-gray-800 dark:bg-gray-900">
        {loading && (
          <div className="flex h-48 items-center justify-center text-sm text-gray-500">
            Loading storage diff...
          </div>
        )}

        {error && (
          <div className="flex flex-col items-center justify-center gap-3 py-12 text-center">
            <p className="text-sm font-medium text-red-600 dark:text-red-400">{error}</p>
            <button
              onClick={() => refetch()}
              className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
            >
              Retry
            </button>
          </div>
        )}

        {!loading && !error && data && rows.length === 0 && (
          <div className="flex h-48 items-center justify-center text-sm text-gray-500">
            No storage entry changes found between ledger {fromLedger} and {toLedger}.
          </div>
        )}

        {!loading && !error && data && rows.length > 0 && (
          <>
            {rows.length > 500 && (
              <div className="mb-3 rounded-lg bg-amber-50 p-3 text-xs text-amber-800 dark:bg-amber-950/50 dark:text-amber-200">
                Notice: Large diff detected ({rows.length} entries). Rendering is virtualized for optimal performance.
              </div>
            )}
            <div
              ref={parentRef}
              className="h-[500px] overflow-auto rounded-lg border border-gray-100 dark:border-gray-800"
            >
              <div
                style={{
                  height: `${virtualizer.getTotalSize()}px`,
                  width: "100%",
                  position: "relative",
                }}
              >
                {virtualizer.getVirtualItems().map(virtualRow => {
                  const row = rows[virtualRow.index];
                  const isChange = row.type === "changed";
                  const entry = row.item as StorageDiffEntry;
                  const change = row.item as StorageDiffChange;

                  return (
                    <div
                      key={virtualRow.index}
                      style={{
                        position: "absolute",
                        top: 0,
                        left: 0,
                        width: "100%",
                        height: `${virtualRow.size}px`,
                        transform: `translateY(${virtualRow.start}px)`,
                      }}
                      className="flex items-center justify-between border-b border-gray-100 px-4 py-3 hover:bg-gray-50 dark:border-gray-800 dark:hover:bg-gray-800/50"
                    >
                      <div className="flex flex-col gap-1 truncate pr-4">
                        <div className="flex items-center gap-2">
                          <span
                            className={`inline-flex items-center rounded px-2 py-0.5 text-xs font-semibold ${
                              row.type === "created"
                                ? "bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400"
                                : row.type === "changed"
                                ? "bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-400"
                                : row.type === "removed"
                                ? "bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400"
                                : "bg-purple-100 text-purple-800 dark:bg-purple-900/30 dark:text-purple-400"
                            }`}
                          >
                            {row.type}
                          </span>
                          <span className="font-mono text-xs text-gray-500 dark:text-gray-400">
                            {isChange ? change.durability : entry.durability}
                          </span>
                          <span className="truncate font-mono text-xs font-medium text-gray-900 dark:text-gray-100">
                            {isChange ? change.key : entry.key}
                          </span>
                        </div>

                        {/* Side-by-side / values view */}
                        <div className="flex items-center gap-4 text-xs">
                          {isChange ? (
                            <div className="flex items-center gap-2">
                              <span className="text-red-600 dark:text-red-400">
                                Old: {change.old_val_decoded?.human || change.old_val || "(none)"}
                              </span>
                              <span>→</span>
                              <span className="text-green-600 dark:text-green-400">
                                New: {change.new_val_decoded?.human || change.new_val || "(none)"}
                              </span>
                            </div>
                          ) : (
                            <span className="text-gray-600 dark:text-gray-300">
                              Value: {entry.val_decoded?.human || entry.val || "(none)"}
                            </span>
                          )}
                        </div>
                      </div>

                      <div className="flex shrink-0 flex-col items-end gap-0.5 text-xs text-gray-500 dark:text-gray-400">
                        {isChange ? (
                          <span>TTL: {change.old_live_until ?? "-"} → {change.new_live_until ?? "-"}</span>
                        ) : (
                          <span>TTL: {entry.live_until_ledger ?? "-"}</span>
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
