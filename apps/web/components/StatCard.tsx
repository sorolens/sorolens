interface StatCardProps {
  label: string;
  value: string | number;
  subtext?: string;
  variant?: "default" | "warning" | "danger" | "safe";
  /** Alias for `variant`, kept for readability on new call sites. */
  tone?: "default" | "warning" | "danger" | "safe";
}

export function StatCard({
  label,
  value,
  subtext,
  variant,
  tone,
}: StatCardProps) {
  const v = tone ?? variant ?? "default";
  const borderColor =
    v === "warning"
      ? "border-[var(--color-warning)]"
      : v === "danger"
        ? "border-[var(--color-danger)]"
        : v === "safe"
          ? "border-[var(--color-safe)]"
          : "border-[var(--color-border)]";

  const textColor =
    v === "warning"
      ? "text-[var(--color-warning)]"
      : v === "danger"
        ? "text-[var(--color-danger)]"
        : v === "safe"
          ? "text-[var(--color-safe)]"
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
