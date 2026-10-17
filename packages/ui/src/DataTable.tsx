import React, { useRef } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";

export interface Column<T> {
  key: string;
  header: React.ReactNode;
  accessor?: (item: T) => React.ReactNode;
  sortable?: boolean;
}

export interface DataTableProps<T> {
  columns: Column<T>[];
  data: T[];
  loading?: boolean;
  emptyState?: React.ReactNode;
  rowKey: (item: T) => string;
  onRowClick?: (item: T) => void;
  sortColumn?: string;
  sortDirection?: "asc" | "desc";
  onSort?: (columnKey: string) => void;
  className?: string;
  virtualized?: boolean;
  virtualizeAfter?: number;
}

export function DataTable<T>({
  columns,
  data,
  loading = false,
  emptyState,
  rowKey,
  onRowClick,
  sortColumn,
  sortDirection,
  onSort,
  className = "",
  virtualized = false,
  virtualizeAfter = 100,
}: DataTableProps<T>) {
  const scrollElementRef = useRef<HTMLDivElement>(null);
  const shouldVirtualize = virtualized && data.length > virtualizeAfter;
  const rowVirtualizer = useVirtualizer({
    count: shouldVirtualize ? data.length : 0,
    getScrollElement: () => scrollElementRef.current,
    estimateSize: () => 56,
    getItemKey: (index) => rowKey(data[index]),
    initialRect: { width: 1024, height: 560 },
    overscan: 10,
  });

  if (loading) {
    return (
      <div className={`rounded-lg bg-[var(--color-bg-card)] p-4 ${className}`}>
        <div className="space-y-3">
          {Array.from({ length: 5 }).map((_, i) => (
            <div
              key={i}
              className="h-12 w-full animate-pulse rounded bg-white/5"
            />
          ))}
        </div>
      </div>
    );
  }

  if (data.length === 0 && emptyState) {
    return <div className={className}>{emptyState}</div>;
  }

  return (
    <div
      className={`rounded-lg bg-[var(--color-bg-card)] overflow-hidden ${className}`}
    >
      <div
        ref={scrollElementRef}
        className={
          shouldVirtualize ? "max-h-[70vh] overflow-auto" : "overflow-x-auto"
        }
      >
        <table className="w-full text-left border-collapse">
          <thead
            className={
              shouldVirtualize
                ? "sticky top-0 z-10 bg-[var(--color-bg-card)]"
                : undefined
            }
          >
            <tr className="border-b border-[var(--color-border)] text-xs font-medium text-[var(--color-text-secondary)]">
              {columns.map((col) => {
                const isSorted = sortColumn === col.key;
                return (
                  <th
                    key={col.key}
                    className={`px-4 py-3 ${
                      col.sortable
                        ? "cursor-pointer select-none hover:text-[var(--color-text-primary)]"
                        : ""
                    }`}
                    onClick={() => {
                      if (col.sortable && onSort) {
                        onSort(col.key);
                      }
                    }}
                  >
                    <div className="flex items-center gap-1.5">
                      <span>{col.header}</span>
                      {col.sortable && (
                        <span className="text-xs opacity-60">
                          {isSorted
                            ? sortDirection === "asc"
                              ? "▲"
                              : "▼"
                            : "↕"}
                        </span>
                      )}
                    </div>
                  </th>
                );
              })}
            </tr>
          </thead>
          <tbody>
            {shouldVirtualize ? (
              <>
                {rowVirtualizer.getVirtualItems()[0]?.start > 0 && (
                  <tr aria-hidden="true">
                    <td
                      colSpan={columns.length}
                      style={{
                        height: rowVirtualizer.getVirtualItems()[0].start,
                      }}
                    />
                  </tr>
                )}
                {rowVirtualizer.getVirtualItems().map((virtualRow) => {
                  const item = data[virtualRow.index];
                  return (
                    <tr
                      key={rowKey(item)}
                      onClick={() => onRowClick?.(item)}
                      className={`border-b border-[var(--color-border)] transition-colors hover:bg-white/5 ${
                        onRowClick ? "cursor-pointer" : ""
                      }`}
                      style={{ height: virtualRow.size }}
                    >
                      {columns.map((col) => (
                        <td key={col.key} className="px-4 py-3 text-sm">
                          {col.accessor
                            ? col.accessor(item)
                            : String(
                                (item as Record<string, unknown>)[col.key] ??
                                  "-"
                              )}
                        </td>
                      ))}
                    </tr>
                  );
                })}
                {rowVirtualizer.getVirtualItems().length > 0 && (
                  <tr aria-hidden="true">
                    <td
                      colSpan={columns.length}
                      style={{
                        height:
                          rowVirtualizer.getTotalSize() -
                          rowVirtualizer.getVirtualItems()[
                            rowVirtualizer.getVirtualItems().length - 1
                          ].end,
                      }}
                    />
                  </tr>
                )}
              </>
            ) : (
              data.map((item) => (
                <tr
                  key={rowKey(item)}
                  onClick={() => onRowClick?.(item)}
                  className={`border-b border-[var(--color-border)] transition-colors hover:bg-white/5 ${
                    onRowClick ? "cursor-pointer" : ""
                  }`}
                >
                  {columns.map((col) => (
                    <td key={col.key} className="px-4 py-3 text-sm">
                      {col.accessor
                        ? col.accessor(item)
                        : String(
                            (item as Record<string, unknown>)[col.key] ?? "-"
                          )}
                    </td>
                  ))}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
