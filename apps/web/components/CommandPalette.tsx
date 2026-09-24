"use client";

/**
 * GlobalCommandPalette — Cmd+K (or Ctrl+K) command palette for Sorolens.
 *
 * Behaviour:
 * - Press Cmd+K / Ctrl+K anywhere in the app to open.
 * - Type to filter contracts (searched by ID / label) AND built-in commands.
 * - Results are split into two sections: "Commands" and "Contracts".
 * - Arrow keys navigate, Enter runs the highlighted item, Escape closes.
 * - Clicking outside the modal also closes it.
 *
 * Commands registered here:
 * - "Track contract"  → opens the Track Contract modal on the /contracts page
 *                       (dispatches a custom event so the page can respond)
 * - "Go to contracts" → navigates to /contracts
 * - "Go to watchdog"  → navigates to /watchdog
 * - "Go to playground"→ navigates to /playground
 * - "Toggle theme"    → toggles between light and dark CSS variables
 */

import {
  Fragment,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useRouter } from "next/navigation";
import { listContractsAll } from "@/lib/api";
import type { ContractSummary } from "@/lib/types";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface PaletteCommand {
  id: string;
  label: string;
  description?: string;
  keywords?: string[];
  icon?: string;
  run: () => void;
}

// ---------------------------------------------------------------------------
// Theme toggle helpers
// ---------------------------------------------------------------------------

const LIGHT_VARS: Record<string, string> = {
  "--color-warning": "#d97706",
  "--color-danger": "#dc2626",
  "--color-safe": "#16a34a",
  "--color-bg-card": "#f1f5f9",
  "--color-bg-page": "#f8fafc",
  "--color-border": "#cbd5e1",
  "--color-text-primary": "#0f172a",
  "--color-text-secondary": "#475569",
  "--color-accent": "#3b82f6",
};

const DARK_VARS: Record<string, string> = {
  "--color-warning": "#f59e0b",
  "--color-danger": "#ef4444",
  "--color-safe": "#22c55e",
  "--color-bg-card": "#1e1e2e",
  "--color-bg-page": "#11111b",
  "--color-border": "#313244",
  "--color-text-primary": "#cdd6f4",
  "--color-text-secondary": "#a6adc8",
  "--color-accent": "#89b4fa",
};

const THEME_KEY = "sorolens_theme";

function getStoredTheme(): "dark" | "light" {
  if (typeof window === "undefined") return "dark";
  try {
    return (localStorage.getItem(THEME_KEY) as "dark" | "light") ?? "dark";
  } catch {
    return "dark";
  }
}

function applyTheme(theme: "dark" | "light") {
  const vars = theme === "light" ? LIGHT_VARS : DARK_VARS;
  const root = document.documentElement;
  Object.entries(vars).forEach(([k, v]) => root.style.setProperty(k, v));
  try {
    localStorage.setItem(THEME_KEY, theme);
  } catch {
    // ignore
  }
}

// ---------------------------------------------------------------------------
// Custom event for "open Track modal"
// ---------------------------------------------------------------------------
export const OPEN_TRACK_MODAL_EVENT = "sorolens:open-track-modal";

// ---------------------------------------------------------------------------
// Main hook: manages palette open state + global keydown
// ---------------------------------------------------------------------------

export function useCommandPalette() {
  const [open, setOpen] = useState(false);

  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        setOpen((v) => !v);
      }
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);

  return { open, setOpen };
}

// ---------------------------------------------------------------------------
// Contract search result row
// ---------------------------------------------------------------------------

function ContractRow({
  contract,
  highlighted,
  onSelect,
}: {
  contract: ContractSummary;
  highlighted: boolean;
  onSelect: () => void;
}) {
  const ref = useRef<HTMLLIElement>(null);
  useEffect(() => {
    if (highlighted) ref.current?.scrollIntoView?.({ block: "nearest" });
  }, [highlighted]);

  return (
    <li
      ref={ref}
      role="option"
      aria-selected={highlighted}
      onClick={onSelect}
      className={`flex cursor-pointer items-center gap-3 px-4 py-2.5 text-sm transition-colors ${
        highlighted
          ? "bg-[var(--color-accent)]/10 text-[var(--color-text-primary)]"
          : "text-[var(--color-text-secondary)] hover:bg-[var(--color-border)]/40"
      }`}
    >
      {/* Network dot */}
      <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-md bg-[var(--color-border)] text-xs font-bold uppercase text-[var(--color-accent)]">
        {contract.network.slice(0, 1).toUpperCase()}
      </span>

      <span className="min-w-0 flex-1">
        {contract.label ? (
          <>
            <span className="block truncate font-medium text-[var(--color-text-primary)]">
              {contract.label}
            </span>
            <span className="block truncate font-mono text-xs opacity-60">
              {contract.id}
            </span>
          </>
        ) : (
          <span className="block truncate font-mono text-xs">
            {contract.id}
          </span>
        )}
      </span>

      <span className="shrink-0 text-xs opacity-50">{contract.network}</span>
    </li>
  );
}

// ---------------------------------------------------------------------------
// Command result row
// ---------------------------------------------------------------------------

function CommandRow({
  command,
  highlighted,
  onSelect,
}: {
  command: PaletteCommand;
  highlighted: boolean;
  onSelect: () => void;
}) {
  const ref = useRef<HTMLLIElement>(null);
  useEffect(() => {
    if (highlighted) ref.current?.scrollIntoView?.({ block: "nearest" });
  }, [highlighted]);

  return (
    <li
      ref={ref}
      role="option"
      aria-selected={highlighted}
      onClick={onSelect}
      className={`flex cursor-pointer items-center gap-3 px-4 py-2.5 text-sm transition-colors ${
        highlighted
          ? "bg-[var(--color-accent)]/10 text-[var(--color-text-primary)]"
          : "text-[var(--color-text-secondary)] hover:bg-[var(--color-border)]/40"
      }`}
    >
      {/* Icon */}
      <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-md bg-[var(--color-border)] text-base">
        {command.icon ?? "⌘"}
      </span>

      <span className="min-w-0 flex-1">
        <span className="block truncate font-medium text-[var(--color-text-primary)]">
          {command.label}
        </span>
        {command.description && (
          <span className="block truncate text-xs opacity-60">
            {command.description}
          </span>
        )}
      </span>
    </li>
  );
}

// ---------------------------------------------------------------------------
// Section header
// ---------------------------------------------------------------------------

function SectionHeader({ label }: { label: string }) {
  return (
    <li
      role="presentation"
      className="px-4 pb-1 pt-3 text-[10px] font-semibold uppercase tracking-widest text-[var(--color-text-secondary)] opacity-60"
    >
      {label}
    </li>
  );
}

// ---------------------------------------------------------------------------
// The palette dialog
// ---------------------------------------------------------------------------

interface CommandPaletteProps {
  onClose: () => void;
  /** Extra commands injected by the host (e.g. "Open track modal"). */
  extraCommands?: PaletteCommand[];
}

export function CommandPalette({
  onClose,
  extraCommands = [],
}: CommandPaletteProps) {
  const router = useRouter();
  const inputRef = useRef<HTMLInputElement>(null);
  const backdropRef = useRef<HTMLDivElement>(null);

  const [query, setQuery] = useState("");
  const [contracts, setContracts] = useState<ContractSummary[]>([]);
  const [loadingContracts, setLoadingContracts] = useState(false);
  const [theme, setTheme] = useState<"dark" | "light">(getStoredTheme);
  const [highlightedIndex, setHighlightedIndex] = useState(0);

  // Focus input on open
  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  // Apply stored theme on mount
  useEffect(() => {
    applyTheme(theme);
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  // Fetch contracts once for the search section
  useEffect(() => {
    setLoadingContracts(true);
    listContractsAll()
      .then((r) => setContracts(r.contracts ?? []))
      .catch(() => setContracts([]))
      .finally(() => setLoadingContracts(false));
  }, []);

  // ---------------------------------------------------------------------------
  // Built-in commands
  // ---------------------------------------------------------------------------

  const toggleTheme = useCallback(() => {
    setTheme((prev) => {
      const next = prev === "dark" ? "light" : "dark";
      applyTheme(next);
      return next;
    });
    onClose();
  }, [onClose]);

  const builtinCommands: PaletteCommand[] = useMemo(
    () => [
      {
        id: "track-contract",
        label: "Track contract",
        description: "Open the Track Contract dialog",
        icon: "＋",
        keywords: ["add", "register", "new"],
        run: () => {
          onClose();
          // Navigate to contracts page then open the modal via custom event.
          router.push("/contracts");
          // Small delay so the page has time to mount before it handles the event.
          setTimeout(
            () => window.dispatchEvent(new CustomEvent(OPEN_TRACK_MODAL_EVENT)),
            120,
          );
        },
      },
      {
        id: "goto-contracts",
        label: "Go to Contracts",
        description: "Navigate to the contracts list",
        icon: "📋",
        keywords: ["contracts", "list", "dashboard"],
        run: () => {
          router.push("/contracts");
          onClose();
        },
      },
      {
        id: "goto-watchdog",
        label: "Go to Watchdog",
        description: "Navigate to the watchdog monitoring page",
        icon: "🐶",
        keywords: ["watchdog", "monitor", "health"],
        run: () => {
          router.push("/watchdog");
          onClose();
        },
      },
      {
        id: "goto-playground",
        label: "Go to Playground",
        description: "Open the interactive API playground",
        icon: "🎮",
        keywords: ["playground", "api", "explore"],
        run: () => {
          router.push("/playground");
          onClose();
        },
      },
      {
        id: "toggle-theme",
        label: `Toggle theme (${theme === "dark" ? "switch to light" : "switch to dark"})`,
        description: "Switch between dark and light mode",
        icon: theme === "dark" ? "☀️" : "🌙",
        keywords: ["theme", "dark", "light", "mode"],
        run: toggleTheme,
      },
      ...extraCommands,
    ],
    [router, onClose, toggleTheme, theme, extraCommands],
  );

  // ---------------------------------------------------------------------------
  // Filtering
  // ---------------------------------------------------------------------------

  const q = query.toLowerCase().trim();

  const filteredCommands = useMemo(() => {
    if (!q) return builtinCommands;
    return builtinCommands.filter(
      (c) =>
        c.label.toLowerCase().includes(q) ||
        c.description?.toLowerCase().includes(q) ||
        c.keywords?.some((k) => k.includes(q)),
    );
  }, [q, builtinCommands]);

  const filteredContracts = useMemo(() => {
    if (!q) return contracts.slice(0, 5);
    return contracts
      .filter(
        (c) =>
          c.id.toLowerCase().includes(q) ||
          (c.label ?? "").toLowerCase().includes(q),
      )
      .slice(0, 8);
  }, [q, contracts]);

  // Flat ordered list for keyboard nav
  const flatItems = useMemo<
    Array<{ type: "command"; item: PaletteCommand } | { type: "contract"; item: ContractSummary }>
  >(
    () => [
      ...filteredCommands.map((c) => ({ type: "command" as const, item: c })),
      ...filteredContracts.map((c) => ({
        type: "contract" as const,
        item: c,
      })),
    ],
    [filteredCommands, filteredContracts],
  );

  // Reset highlight when results change
  useEffect(() => {
    setHighlightedIndex(0);
  }, [query]);

  // ---------------------------------------------------------------------------
  // Keyboard navigation
  // ---------------------------------------------------------------------------

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === "Escape") {
      e.preventDefault();
      onClose();
      return;
    }
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setHighlightedIndex((i) => (i + 1) % Math.max(1, flatItems.length));
      return;
    }
    if (e.key === "ArrowUp") {
      e.preventDefault();
      setHighlightedIndex((i) =>
        i === 0 ? Math.max(0, flatItems.length - 1) : i - 1,
      );
      return;
    }
    if (e.key === "Enter") {
      e.preventDefault();
      const current = flatItems[highlightedIndex];
      if (!current) return;
      if (current.type === "command") {
        current.item.run();
      } else {
        router.push(`/contracts/${current.item.id}`);
        onClose();
      }
    }
  }

  // Close on backdrop click
  function handleBackdropClick(e: React.MouseEvent) {
    if (e.target === backdropRef.current) onClose();
  }

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  const isEmpty = flatItems.length === 0;
  let globalIdx = -1;

  return (
    <div
      ref={backdropRef}
      onClick={handleBackdropClick}
      className="fixed inset-0 z-[9999] flex items-start justify-center bg-black/70 backdrop-blur-sm pt-[10vh]"
      aria-modal="true"
      role="dialog"
      aria-label="Command palette"
    >
      <div
        className="w-full max-w-xl rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-card)] shadow-2xl overflow-hidden"
        onKeyDown={handleKeyDown}
      >
        {/* Input */}
        <div className="flex items-center gap-3 border-b border-[var(--color-border)] px-4 py-3">
          {/* Search icon */}
          <svg
            className="h-4 w-4 shrink-0 text-[var(--color-text-secondary)]"
            viewBox="0 0 20 20"
            fill="currentColor"
            aria-hidden="true"
          >
            <path
              fillRule="evenodd"
              d="M9 3.5a5.5 5.5 0 100 11 5.5 5.5 0 000-11zM2 9a7 7 0 1112.452 4.391l3.328 3.329a.75.75 0 11-1.06 1.06l-3.329-3.328A7 7 0 012 9z"
              clipRule="evenodd"
            />
          </svg>
          <input
            ref={inputRef}
            type="text"
            role="combobox"
            aria-autocomplete="list"
            aria-expanded={flatItems.length > 0}
            aria-label="Search commands or contracts"
            placeholder="Search commands or contracts…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            className="flex-1 bg-transparent text-sm text-[var(--color-text-primary)] placeholder-[var(--color-text-secondary)] outline-none"
          />
          <kbd className="hidden rounded border border-[var(--color-border)] px-1.5 py-0.5 text-[10px] font-mono text-[var(--color-text-secondary)] sm:block">
            esc
          </kbd>
        </div>

        {/* Results */}
        <ul
          role="listbox"
          aria-label="Commands and contracts"
          className="max-h-[400px] overflow-y-auto py-1"
        >
          {isEmpty && (!loadingContracts || q !== "") && (
            <li
              data-testid="no-results"
              className="px-4 py-6 text-center text-sm text-[var(--color-text-secondary)]"
            >
              No results for{" "}
              <span className="font-medium text-[var(--color-text-primary)]">
                &quot;{query}&quot;
              </span>
            </li>
          )}

          {isEmpty && loadingContracts && q === "" && (
            <li className="px-4 py-6 text-center text-sm text-[var(--color-text-secondary)] animate-pulse">
              Loading…
            </li>
          )}

          {/* Commands section */}
          {filteredCommands.length > 0 && (
            <Fragment>
              <SectionHeader label="Commands" />
              {filteredCommands.map((cmd) => {
                globalIdx++;
                const idx = globalIdx;
                return (
                  <CommandRow
                    key={cmd.id}
                    command={cmd}
                    highlighted={highlightedIndex === idx}
                    onSelect={() => cmd.run()}
                  />
                );
              })}
            </Fragment>
          )}

          {/* Contracts section */}
          {filteredContracts.length > 0 && (
            <Fragment>
              <SectionHeader
                label={
                  loadingContracts ? "Contracts (loading…)" : "Contracts"
                }
              />
              {filteredContracts.map((c) => {
                globalIdx++;
                const idx = globalIdx;
                return (
                  <ContractRow
                    key={c.id}
                    contract={c}
                    highlighted={highlightedIndex === idx}
                    onSelect={() => {
                      router.push(`/contracts/${c.id}`);
                      onClose();
                    }}
                  />
                );
              })}
            </Fragment>
          )}
        </ul>

        {/* Footer hint */}
        <div className="flex items-center gap-4 border-t border-[var(--color-border)] px-4 py-2 text-[10px] text-[var(--color-text-secondary)]">
          <span>
            <kbd className="rounded border border-[var(--color-border)] px-1 py-0.5 font-mono">↑↓</kbd>{" "}
            navigate
          </span>
          <span>
            <kbd className="rounded border border-[var(--color-border)] px-1 py-0.5 font-mono">↵</kbd>{" "}
            select
          </span>
          <span>
            <kbd className="rounded border border-[var(--color-border)] px-1 py-0.5 font-mono">esc</kbd>{" "}
            close
          </span>
          <span className="ml-auto opacity-60">Cmd+K</span>
        </div>
      </div>
    </div>
  );
}
