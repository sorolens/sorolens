interface StatCardProps {
  label: string;
  value: string | number;
  subtext?: string;
  variant?: "default" | "warning" | "danger";
}

export function StatCard({ label, value, subtext, variant = "default" }: StatCardProps) {
  const borderColor =
    variant === "warning"
      ? "border-[var(--color-warning)]"
      : variant === "danger"
        ? "border-[var(--color-danger)]"
        : "border-[var(--color-border)]";

  const textColor =
    variant === "warning"
      ? "text-[var(--color-warning)]"
      : variant === "danger"
        ? "text-[var(--color-danger)]"
        : "";

  return (
    <div
      className={`rounded-lg border-l-4 bg-[var(--color-bg-card)] p-4 ${borderColor}`}
    >
      <div className="text-sm text-[var(--color-text-secondary)]">{label}</div>
      <div className={`mt-1 text-3xl font-bold ${textColor}`}>{value}</div>
      {subtext && (
        <div className="mt-1 text-xs text-[var(--color-text-secondary)]">
          {subtext}
        </div>
      )}
    </div>
  );
}
