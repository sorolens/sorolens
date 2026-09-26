import type { AlertSeverity, HealthStatus } from "@/lib/types";

const healthTone: Record<string, { border: string; text: string }> = {
  Healthy: {
    border: "border-[var(--color-safe)]",
    text: "text-[var(--color-safe)]",
  },
  Degraded: {
    border: "border-[var(--color-warning)]",
    text: "text-[var(--color-warning)]",
  },
  Unresponsive: {
    border: "border-[var(--color-danger)]",
    text: "text-[var(--color-danger)]",
  },
};

export function HealthBadge({ status }: { status: HealthStatus }) {
  const tone = healthTone[status] ?? {
    border: "border-[var(--color-border)]",
    text: "text-[var(--color-text-secondary)]",
  };
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-xs font-medium ${tone.border} ${tone.text}`}
    >
      <span
        className={`h-1.5 w-1.5 rounded-full ${tone.text.replace("text-", "bg-")}`}
      />
      {status}
    </span>
  );
}

const severityTone: Record<AlertSeverity, { border: string; text: string }> = {
  Info: {
    border: "border-[var(--color-accent)]",
    text: "text-[var(--color-accent)]",
  },
  Warning: {
    border: "border-[var(--color-warning)]",
    text: "text-[var(--color-warning)]",
  },
  Critical: {
    border: "border-[var(--color-danger)]",
    text: "text-[var(--color-danger)]",
  },
};

export function SeverityBadge({ severity }: { severity: AlertSeverity }) {
  const tone = severityTone[severity] ?? severityTone.Info;
  return (
    <span
      className={`inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-medium ${tone.border} ${tone.text}`}
    >
      {severity}
    </span>
  );
}
