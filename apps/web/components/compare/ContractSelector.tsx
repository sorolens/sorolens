import type { ContractSummary } from "@/lib/types";

interface ContractSelectorProps {
  selected: string[];
  onSelect: (ids: string[]) => void;
  contracts: ContractSummary[];
  max?: number;
}

function shortId(id: string): string {
  return id.length > 12 ? `${id.slice(0, 6)}…${id.slice(-4)}` : id;
}

export function ContractSelector({
  selected,
  onSelect,
  contracts,
  max = 4,
}: ContractSelectorProps) {
  const byId = new Map(contracts.map((c) => [c.id, c]));
  const slots = Array.from({ length: max }, (_, i) => selected[i] ?? null);

  const toggle = (id: string) => {
    if (selected.includes(id)) {
      onSelect(selected.filter((s) => s !== id));
    } else if (selected.length < max) {
      onSelect([...selected, id]);
    }
  };

  const clear = () => onSelect([]);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-[var(--color-text-primary)]">
          Contract slots
        </h2>
        {selected.length > 0 && (
          <button
            type="button"
            onClick={clear}
            className="text-xs text-[var(--color-accent)] hover:underline"
          >
            Clear all
          </button>
        )}
      </div>

      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        {slots.map((id, i) => {
          const contract = id ? byId.get(id) : undefined;
          return (
            <div
              key={id ?? `empty-slot-${i}`}
              className={`flex items-center justify-between gap-2 rounded-lg border px-3 py-2 text-sm ${
                id
                  ? "border-[var(--color-accent)] bg-[var(--color-bg-card)]"
                  : "border-dashed border-[var(--color-border)]"
              }`}
              data-testid={id ? "compare-slot-filled" : "compare-slot-empty"}
            >
              <span className="min-w-0 truncate text-[var(--color-text-secondary)]">
                {id ? contract?.label ?? shortId(id) : `Slot ${i + 1}`}
              </span>
              {id && (
                <button
                  type="button"
                  onClick={() => toggle(id)}
                  aria-label={`Remove ${id}`}
                  className="shrink-0 text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
                >
                  ✕
                </button>
              )}
            </div>
          );
        })}
      </div>

      <p className="text-sm text-[var(--color-text-secondary)]">
        Select 2–{max} contracts to compare
      </p>

      <div className="flex flex-wrap gap-2">
        {contracts.map((c) => {
          const isSelected = selected.includes(c.id);
          const isDisabled = !isSelected && selected.length >= max;
          return (
            <button
              key={c.id}
              type="button"
              onClick={() => toggle(c.id)}
              disabled={isDisabled}
              title={c.id}
              className={`rounded-lg border px-3 py-1.5 text-sm font-medium transition-colors ${
                isSelected
                  ? "border-[var(--color-accent)] bg-[var(--color-accent)] text-[var(--color-bg-page)]"
                  : isDisabled
                    ? "cursor-not-allowed border-[var(--color-border)] bg-[var(--color-bg-card)] text-[var(--color-text-secondary)] opacity-50"
                    : "border-[var(--color-border)] bg-[var(--color-bg-card)] text-[var(--color-text-primary)] hover:border-[var(--color-accent)]"
              }`}
            >
              {c.label ?? c.id}
            </button>
          );
        })}
      </div>

      {selected.length > 0 && (
        <div className="flex items-center gap-2 text-sm text-[var(--color-text-secondary)]">
          <span>
            {selected.length}/{max} selected
          </span>
          <span className="text-[var(--color-text-secondary)]">
            {selected.length >= 2 ? "Ready to compare" : "Select at least 2 contracts"}
          </span>
        </div>
      )}
    </div>
  );
}
