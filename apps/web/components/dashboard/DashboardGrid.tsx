"use client";

import { useEffect, useRef, useState } from "react";
import {
  DEFAULT_LAYOUT,
  addWidget,
  availableWidgets,
  clearLayout,
  defaultLayout,
  loadLayout,
  moveWidget,
  removeWidget,
  reorderWidget,
  resizeWidget,
  saveLayout,
  widgetDefinition,
  type WidgetId,
  type WidgetLayout,
} from "@/lib/dashboardLayout";
import { WidgetErrorBoundary } from "./WidgetErrorBoundary";
import {
  AlertsWidget,
  HotContractsWidget,
  StatsWidget,
  VolumeWidget,
} from "./widgets";

const WIDGET_COMPONENTS: Record<WidgetId, () => React.JSX.Element> = {
  stats: StatsWidget,
  volume: VolumeWidget,
  alerts: AlertsWidget,
  hot_contracts: HotContractsWidget,
};

/**
 * Personal dashboard (#833).
 *
 * The layout is owned by `lib/dashboardLayout` and persisted locally, so it
 * survives reloads without an account. Widgets are independent: each one is
 * wrapped in its own error boundary and own data fetch.
 */
export function DashboardGrid() {
  const [layout, setLayout] = useState<WidgetLayout[]>(DEFAULT_LAYOUT);
  const [hydrated, setHydrated] = useState(false);
  const dragFrom = useRef<number | null>(null);

  // Read storage after mount so the server-rendered default and the first
  // client render agree (no hydration mismatch).
  useEffect(() => {
    setLayout(loadLayout());
    setHydrated(true);
  }, []);

  useEffect(() => {
    if (!hydrated) return;
    saveLayout(layout);
  }, [layout, hydrated]);

  function update(next: WidgetLayout[]) {
    setLayout(next);
  }

  function onDrop(target: number) {
    const from = dragFrom.current;
    dragFrom.current = null;
    if (from === null) return;
    update(reorderWidget(layout, from, target));
  }

  function reset() {
    clearLayout();
    setLayout(defaultLayout());
  }

  const missing = availableWidgets(layout);

  return (
    <section aria-label="Personal dashboard" data-testid="dashboard-grid">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-[var(--color-text-secondary)]">
          Drag a widget by its card, or use the arrow buttons, to reorder it.
          Layouts are saved in this browser.
        </p>
        <button
          type="button"
          onClick={reset}
          className="rounded-lg border border-[var(--color-border)] px-3 py-1.5 text-sm hover:border-[var(--color-accent)]"
        >
          Reset layout
        </button>
      </div>

      {layout.length === 0 ? (
        <p className="rounded-lg border border-dashed border-[var(--color-border)] px-4 py-12 text-center text-sm text-[var(--color-text-secondary)]">
          Every widget is hidden. Add one below or reset the layout.
        </p>
      ) : (
        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          {layout.map((widget, index) => {
            const definition = widgetDefinition(widget.id);
            const Widget = WIDGET_COMPONENTS[widget.id];
            return (
              <article
                key={widget.id}
                data-testid={`widget-${widget.id}`}
                data-span={widget.span}
                draggable
                onDragStart={() => {
                  dragFrom.current = index;
                }}
                onDragOver={(event) => event.preventDefault()}
                onDrop={() => onDrop(index)}
                className={`rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] ${
                  widget.span === 2 ? "lg:col-span-2" : ""
                }`}
              >
                <header className="flex items-start justify-between gap-3 border-b border-[var(--color-border)] px-4 py-3">
                  <div className="min-w-0">
                    <h2 className="truncate font-semibold">
                      {definition.title}
                    </h2>
                    <p className="truncate text-xs text-[var(--color-text-secondary)]">
                      {definition.description}
                    </p>
                  </div>
                  <div className="flex shrink-0 items-center gap-1 text-xs">
                    <button
                      type="button"
                      aria-label={`Move ${definition.title} up`}
                      disabled={index === 0}
                      onClick={() => update(moveWidget(layout, widget.id, -1))}
                      className="rounded border border-[var(--color-border)] px-1.5 py-0.5 disabled:opacity-40"
                    >
                      ↑
                    </button>
                    <button
                      type="button"
                      aria-label={`Move ${definition.title} down`}
                      disabled={index === layout.length - 1}
                      onClick={() => update(moveWidget(layout, widget.id, 1))}
                      className="rounded border border-[var(--color-border)] px-1.5 py-0.5 disabled:opacity-40"
                    >
                      ↓
                    </button>
                    <button
                      type="button"
                      aria-label={`Resize ${definition.title}`}
                      onClick={() => update(resizeWidget(layout, widget.id))}
                      className="rounded border border-[var(--color-border)] px-1.5 py-0.5"
                    >
                      {widget.span === 2 ? "½" : "⤢"}
                    </button>
                    <button
                      type="button"
                      aria-label={`Remove ${definition.title}`}
                      onClick={() => update(removeWidget(layout, widget.id))}
                      className="rounded border border-[var(--color-border)] px-1.5 py-0.5 hover:border-[var(--color-danger)] hover:text-[var(--color-danger)]"
                    >
                      ×
                    </button>
                  </div>
                </header>
                <div className="py-2">
                  <WidgetErrorBoundary title={definition.title}>
                    <Widget />
                  </WidgetErrorBoundary>
                </div>
              </article>
            );
          })}
        </div>
      )}

      {missing.length > 0 && (
        <div className="mt-4 flex flex-wrap items-center gap-2">
          <span className="text-sm text-[var(--color-text-secondary)]">
            Add a widget:
          </span>
          {missing.map((widget) => (
            <button
              key={widget.id}
              type="button"
              onClick={() => update(addWidget(layout, widget.id))}
              className="rounded-lg border border-[var(--color-border)] px-3 py-1.5 text-sm hover:border-[var(--color-accent)]"
            >
              + {widget.title}
            </button>
          ))}
        </div>
      )}
    </section>
  );
}
