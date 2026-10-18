import React from "react";

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
   * When provided, every row renders a real link (in the first cell) to the
   * given href. Rows become keyboard and screen-reader accessible; pass
   * `linkComponent` to use the router's client-side navigation.
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
}

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
}: DataTableProps<T>) {
  if (loading) {
    return (
      <div className={`rounded-lg bg-[var(--color-bg-card)] p-4 ${className}`}>
        <div className="space-y-3">
          {Array.from({ length: 5 }).map((_, i) => (
            <div
              key={i}
              className="h-12 w-full animate-pulse rounded bg-[var(--color-bg-elevated)]"
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
      <div className="overflow-x-auto">
        <table className="w-full text-left border-collapse">
          <thead>
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
                      {col.sortable ? (
                        <button
                          type="button"
                          onClick={(e) => {
                            // The th onClick already fires; this keeps the
                            // control keyboard-operable without double-firing.
                            e.stopPropagation();
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
                        <>
                          <span>{col.header}</span>
                        </>
                      )}
                    </div>
                  </th>
                );
              })}
            </tr>
          </thead>
          <tbody>
            {data.map((item) => {
              const href = rowHref?.(item) ?? null;
              return (
                <tr
                  key={rowKey(item)}
                  onClick={() => {
                    // Backstop only: rows with a real link rely on the link,
                    // which is keyboard reachable.
                    if (!href) onRowClick?.(item);
                  }}
                  className={`border-b border-[var(--color-border)] transition-colors hover:bg-[var(--color-bg-hover)] ${
                    onRowClick && !href ? "cursor-pointer" : ""
                  }`}
                >
                  {columns.map((col) => {
                    const content = col.accessor
                      ? col.accessor(item)
                      : String(
                          (item as Record<string, unknown>)[col.key] ?? "-"
                        );
                    // The link column (default: first) carries the row link so
                    // each row has exactly one keyboard-focusable, announced
                    // link (issue #610).
                    const linkKey = linkColumnKey ?? columns[0]?.key;
                    const withLink =
                      href !== null && col.key === linkKey ? (
                        <Link
                          href={href}
                          className="text-[var(--color-accent)] underline-offset-2 hover:underline focus:outline-none focus-visible:underline"
                        >
                          {content}
                        </Link>
                      ) : (
                          content
                      );
                    return (
                      <td key={col.key} className="px-4 py-3 text-sm">
                        {withLink}
                      </td>
                    );
                  })}
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
