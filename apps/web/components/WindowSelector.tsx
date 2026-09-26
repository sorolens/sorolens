import type { TimeWindow } from "@/lib/types";

const WINDOWS: { label: string; value: TimeWindow }[] = [
  { label: "24h", value: "24h" },
  { label: "7d", value: "7d" },
  { label: "30d", value: "30d" },
  { label: "All", value: "all" },
];

interface WindowSelectorProps {
  selected: TimeWindow;
  onChange: (window: TimeWindow) => void;
  /** Optional subset/order of windows to show. Defaults to all of them. */
  options?: TimeWindow[];
}

export function WindowSelector({
  selected,
  onChange,
  options,
}: WindowSelectorProps) {
  const windows = options
    ? options
        .map((value) => WINDOWS.find((w) => w.value === value))
        .filter((w): w is { label: string; value: TimeWindow } => Boolean(w))
    : WINDOWS;

  return (
    <div className="flex gap-1 rounded-lg bg-[var(--color-bg-card)] p-1">
      {windows.map((w) => (
        <button
          key={w.value}
          type="button"
          onClick={() => onChange(w.value)}
          className={`rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${
            selected === w.value
              ? "bg-[var(--color-accent)] text-[var(--color-bg-page)]"
              : "text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
          }`}
        >
          {w.label}
        </button>
      ))}
    </div>
  );
}
