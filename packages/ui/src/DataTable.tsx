import React, { useRef } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";

export interface Column<T> {
  key: string;
  header: React.ReactNode;
  accessor?: (item: T) => React.ReactNode;
  sortable?: boolean;
}

/** Anything renderable as a row link. Defaults to a plain `<a>`. */
export type LinkComponent = React.ComponentType<{
  href: string;
  className?: string;
  children: React.ReactNode;
}>;

function DefaultLink({
  href,
  className,
  children,
}: {
  href: string;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <a href={href} className={className}>
      {children}
    </a>
  );
}

export interface DataTableProps<T> {
  columns: Column<T>[];
  data: T[];
  loading?: boolean;
  emptyState?: React.ReactNode;
  rowKey: (item: T) => string;
  /**
   * When provided, every row renders a real link (in the link column) to the
   * given href. Rows become keyboard and screen-reader accessible; pass
   * `linkComponent` to use the router's client-side navigation (issue #610).
   */
  rowHref?: (item: T) => string | null;
  /**
   * Key of the column that receives the row link. Defaults to the first
   * column; set it when the first column is decorative (e.g. a checkbox).
   */
  linkColumnKey?: string;
  linkComponent?: LinkComponent;
  onRowClick?: (item: T) => void;
  sortColumn?: string;
  sortDirection?: "asc" | "desc";
  onSort?: (columnKey: string) => void;
  className?: string;
  virtualized?: boolean;
  virtualizeAfter?: number;
}

const linkClass =
  "text-[var(--color-accent)] underline-offset-2 hover:underline focus:outline-none focus-visible:underline";

export function DataTable<T>({
  columns,
  data,
  loading = false,
  emptyState,
  rowKey,
  rowHref,
  linkColumnKey,
  linkComponent: Link = DefaultLink,
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

  const linkKey = linkColumnKey ?? columns[0]?.key;

  const renderRow = (item: T, style?: React.CSSProperties) => {
    const href = rowHref?.(item) ?? null;
    return (
      <tr
        key={rowKey(item)}
        onClick={() => {
          // Rows with a real link navigate through that link, which is
          // keyboard reachable; this is only a backstop for click-only rows.
          if (!href) onRowClick?.(item);
        }}
        className={`border-b border-[var(--color-border)] transition-colors hover:bg-white/5 ${
          onRowClick && !href ? "cursor-pointer" : ""
        }`}
        style={style}
      >
        {columns.map((col) => {
          const content = col.accessor
            ? col.accessor(item)
            : String((item as Record<string, unknown>)[col.key] ?? "-");
          // Exactly one announced, focusable link per row: the row link lives
          // in linkColumnKey so a decorative first cell (checkbox) does not
          // swallow it (issue #610).
          const cellContent =
            href !== null && col.key === linkKey ? (
              <Link href={href} className={linkClass}>
                {content}
              </Link>
            ) : (
              content
            );
          return (
            <td key={col.key} className="px-4 py-3 text-sm">
              {cellContent}
            </td>
          );
        })}
      </tr>
    );
  };

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
                    data-key={col.key}
                    scope="col"
                    aria-sort={
                      !col.sortable
                        ? undefined
                        : isSorted
                          ? sortDirection === "asc"
                            ? "ascending"
                            : "descending"
                          : "none"
                    }
                    className={`px-4 py-3 ${
                      col.sortable
                        ? "select-none hover:text-[var(--color-text-primary)]"
                        : ""
                    }`}
                    onClick={() => {
                      if (col.sortable && onSort) {
                        onSort(col.key);
                      }
                    }}
                  >
                    <div className="flex items-center gap-1.5">
                      {col.sortable ? (
                        // A real button makes sorting keyboard operable; the
                        // th click handler stays for pointer users.
                        <button
                          type="button"
                          onClick={(event) => {
                            event.stopPropagation();
                            onSort?.(col.key);
                          }}
                          className="flex items-center gap-1.5 font-medium"
                        >
                          <span>{col.header}</span>
                          <span className="text-xs opacity-60" aria-hidden>
                            {isSorted
                              ? sortDirection === "asc"
                                ? "▲"
                                : "▼"
                              : "↕"}
                          </span>
                        </button>
                      ) : (
                        <span>{col.header}</span>
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
                  return renderRow(item, { height: virtualRow.size });
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
              data.map((item) => renderRow(item))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
