export function Skeleton({ className = "" }: { className?: string }) {
  return (
    <div
      aria-hidden="true"
      className={`animate-pulse rounded-md bg-[var(--color-border)] ${className}`}
    />
  );
}

export function CardSkeleton() {
  return (
    <div role="status" aria-busy="true" aria-label="Loading card">
      <span className="sr-only">Loading card</span>
      <div
        aria-hidden="true"
        className="rounded-lg bg-[var(--color-bg-card)] p-4"
      >
        <Skeleton className="mb-2 h-4 w-20" />
        <Skeleton className="h-8 w-16" />
      </div>
    </div>
  );
}

export function ChartSkeleton() {
  return (
    <div role="status" aria-busy="true" aria-label="Loading chart">
      <span className="sr-only">Loading chart</span>
      <div
        aria-hidden="true"
        className="rounded-lg bg-[var(--color-bg-card)] p-4"
      >
        <Skeleton className="mb-4 h-5 w-32" />
        <Skeleton className="h-64 w-full" />
      </div>
    </div>
  );
}

export function TableSkeleton({ rows = 5 }: { rows?: number }) {
  return (
    <div role="status" aria-busy="true" aria-label="Loading table">
      <span className="sr-only">Loading table</span>
      <div
        aria-hidden="true"
        className="rounded-lg bg-[var(--color-bg-card)] p-4"
      >
        <Skeleton className="mb-4 h-5 w-24" />
        {Array.from({ length: rows }).map((_, i) => (
          <Skeleton key={i} className="mb-2 h-10 w-full" />
        ))}
      </div>
    </div>
  );
}
