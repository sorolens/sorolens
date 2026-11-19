/**
 * Widget registry and persisted layout for the personal dashboard (#833).
 *
 * The layout is an ordered list of widgets; `span` is the width in grid
 * columns (1 or 2) and is what the resize control toggles. Everything here is
 * pure so it can be unit tested without a DOM; reads/writes accept an
 * injectable Storage for the same reason.
 */

export type WidgetId = "stats" | "volume" | "alerts" | "hot_contracts";

export interface WidgetDefinition {
  id: WidgetId;
  title: string;
  description: string;
  /** Width in grid columns on a wide screen. */
  defaultSpan: 1 | 2;
}

export interface WidgetLayout {
  id: WidgetId;
  span: 1 | 2;
}

export const WIDGET_REGISTRY: WidgetDefinition[] = [
  {
    id: "stats",
    title: "Network stats",
    description: "Contracts indexed, events, invocations and storage entries.",
    defaultSpan: 2,
  },
  {
    id: "volume",
    title: "Event volume",
    description: "Events per minute across every tracked contract.",
    defaultSpan: 2,
  },
  {
    id: "alerts",
    title: "Recent alerts",
    description: "The newest watchdog alerts across all contracts.",
    defaultSpan: 1,
  },
  {
    id: "hot_contracts",
    title: "Hot contracts",
    description: "Contracts emitting the most events right now.",
    defaultSpan: 1,
  },
];

export const DASHBOARD_STORAGE_KEY = "sorolens.dashboard.layout.v1";

/** Sensible default: two wide rows followed by a two-up row. */
export const DEFAULT_LAYOUT: WidgetLayout[] = [
  { id: "stats", span: 2 },
  { id: "volume", span: 2 },
  { id: "alerts", span: 1 },
  { id: "hot_contracts", span: 1 },
];

export function widgetDefinition(id: WidgetId): WidgetDefinition {
  const found = WIDGET_REGISTRY.find((widget) => widget.id === id);
  if (!found) throw new Error(`Unknown widget: ${id}`);
  return found;
}

export function isWidgetId(value: unknown): value is WidgetId {
  return WIDGET_REGISTRY.some((widget) => widget.id === value);
}

export function defaultLayout(): WidgetLayout[] {
  return DEFAULT_LAYOUT.map((widget) => ({ ...widget }));
}

/**
 * Validates a value read from storage or a caller. Unknown widgets, duplicate
 * entries and malformed spans are dropped instead of breaking the page.
 */
export function sanitizeLayout(raw: unknown): WidgetLayout[] {
  if (!Array.isArray(raw)) return [];
  const seen = new Set<WidgetId>();
  const layout: WidgetLayout[] = [];
  for (const candidate of raw) {
    if (!candidate || typeof candidate !== "object") continue;
    const { id, span } = candidate as { id?: unknown; span?: unknown };
    if (!isWidgetId(id) || seen.has(id)) continue;
    seen.add(id);
    layout.push({ id, span: span === 1 ? 1 : 2 });
  }
  return layout;
}

/** Reads the saved layout; anything unreadable falls back to the default. */
export function loadLayout(storage?: Storage): WidgetLayout[] {
  const store = storage ?? safeStorage();
  if (!store) return defaultLayout();
  try {
    const raw = store.getItem(DASHBOARD_STORAGE_KEY);
    if (raw === null) return defaultLayout();
    const parsed: unknown = JSON.parse(raw);
    if (!Array.isArray(parsed)) return defaultLayout();
    return sanitizeLayout(parsed);
  } catch {
    return defaultLayout();
  }
}

export function saveLayout(layout: WidgetLayout[], storage?: Storage): void {
  const store = storage ?? safeStorage();
  if (!store) return;
  try {
    store.setItem(DASHBOARD_STORAGE_KEY, JSON.stringify(sanitizeLayout(layout)));
  } catch {
    // A full or disabled storage must not break an interaction.
  }
}

export function clearLayout(storage?: Storage): void {
  const store = storage ?? safeStorage();
  if (!store) return;
  try {
    store.removeItem(DASHBOARD_STORAGE_KEY);
  } catch {
    // Ignore: the reset still applies in memory.
  }
}

/** Widgets that exist in the registry but are not on the dashboard. */
export function availableWidgets(layout: WidgetLayout[]): WidgetDefinition[] {
  const present = new Set(layout.map((widget) => widget.id));
  return WIDGET_REGISTRY.filter((widget) => !present.has(widget.id));
}

export function addWidget(layout: WidgetLayout[], id: WidgetId): WidgetLayout[] {
  if (layout.some((widget) => widget.id === id)) return layout;
  return [...layout, { id, span: widgetDefinition(id).defaultSpan }];
}

export function removeWidget(
  layout: WidgetLayout[],
  id: WidgetId
): WidgetLayout[] {
  return layout.filter((widget) => widget.id !== id);
}

/** Moves a widget by `delta` positions, clamped at either end. */
export function moveWidget(
  layout: WidgetLayout[],
  id: WidgetId,
  delta: number
): WidgetLayout[] {
  const index = layout.findIndex((widget) => widget.id === id);
  if (index === -1 || delta === 0) return layout;
  const target = Math.min(layout.length - 1, Math.max(0, index + delta));
  if (target === index) return layout;
  return reorderWidget(layout, index, target);
}

/** Moves the widget at `from` so it lands at index `to`. */
export function reorderWidget(
  layout: WidgetLayout[],
  from: number,
  to: number
): WidgetLayout[] {
  if (
    from < 0 ||
    to < 0 ||
    from >= layout.length ||
    to >= layout.length ||
    from === to
  ) {
    return layout;
  }
  const next = [...layout];
  const [moved] = next.splice(from, 1);
  next.splice(to, 0, moved);
  return next;
}

/** Toggles between half and full width; this is the resize control. */
export function resizeWidget(
  layout: WidgetLayout[],
  id: WidgetId
): WidgetLayout[] {
  return layout.map((widget) =>
    widget.id === id ? { ...widget, span: widget.span === 2 ? 1 : 2 } : widget
  );
}

function safeStorage(): Storage | null {
  try {
    return typeof window !== "undefined" ? window.localStorage : null;
  } catch {
    return null;
  }
}
