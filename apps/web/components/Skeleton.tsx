export function Skeleton({ className = "" }: { className?: string }) {
  return (
    <div
      className={`animate-pulse rounded-md bg-[var(--color-border)] ${className}`}
    />
  );
}

export function CardSkeleton() {
  return (
    <div className="rounded-lg bg-[var(--color-bg-card)] p-4">
      <Skeleton className="mb-2 h-4 w-20" />
      <Skeleton className="h-8 w-16" />
    </div>
  );
}

export function ChartSkeleton() {
  return (
    <div className="rounded-lg bg-[var(--color-bg-card)] p-4">
      <Skeleton className="mb-4 h-5 w-32" />
      <Skeleton className="h-64 w-full" />
    </div>
  );
}

export function TableSkeleton({ rows = 5 }: { rows?: number }) {
  return (
    <div className="rounded-lg bg-[var(--color-bg-card)] p-4">
      <Skeleton className="mb-4 h-5 w-24" />
      {Array.from({ length: rows }).map((_, i) => (
        <Skeleton key={i} className="mb-2 h-10 w-full" />
      ))}
    </div>
  );
}
