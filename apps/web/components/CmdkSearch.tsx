"use client";

import * as React from "react";
import { Command } from "cmdk";
import { useDebounce } from "use-debounce";
import { useRouter } from "next/navigation";
import { useTheme } from "./ThemeProvider";
import { useNetwork, networkFilter } from "@/lib/network";
import { ApiError, searchContracts } from "@/lib/api";
import type { SearchContract } from "@/lib/types";

const searchItemClassName =
  "flex items-center gap-3 px-3 py-3 text-sm text-[var(--color-text-primary)] rounded-md cursor-pointer data-[selected=true]:bg-[var(--color-bg-card)] data-[selected=true]:text-[var(--color-text-primary)]";

function SearchIcon() {
  return (
    <svg
      width="15"
      height="15"
      viewBox="0 0 15 15"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className="opacity-70"
      aria-hidden
    >
      <path
        d="M10 6.5C10 8.433 8.433 10 6.5 10C4.567 10 3 8.433 3 6.5C3 4.567 4.567 3 6.5 3C8.433 3 10 4.567 10 6.5ZM9.30884 10.0159C8.53901 10.6318 7.56251 11 6.5 11C4.01472 11 2 8.98528 2 6.5C2 4.01472 4.01472 2 6.5 2C8.98528 2 11 4.01472 11 6.5C11 7.56251 10.6318 8.53901 10.0159 9.30884L12.8536 12.1464C13.0488 12.3417 13.0488 12.6583 12.8536 12.8536C12.6583 13.0488 12.3417 13.0488 12.1464 12.8536L9.30884 10.0159Z"
        fill="currentColor"
        fillRule="evenodd"
        clipRule="evenodd"
      />
    </svg>
  );
}

export function CmdkSearch() {
  const [open, setOpen] = React.useState(false);
  const [inputValue, setInputValue] = React.useState("");
  const [debouncedValue] = useDebounce(inputValue, 300);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [results, setResults] = React.useState<SearchContract[]>([]);
  const router = useRouter();
  const { theme, setTheme } = useTheme();
  const { network } = useNetwork();
  const networkParam = networkFilter(network);

  // Toggle the menu when ⌘K / Ctrl+K is pressed.
  React.useEffect(() => {
    const down = (e: KeyboardEvent) => {
      if (e.key === "k" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        setOpen((open) => !open);
      }
    };

    document.addEventListener("keydown", down);
    return () => document.removeEventListener("keydown", down);
  }, []);

  React.useEffect(() => {
    if (!open) {
      setInputValue("");
      setResults([]);
      setError(null);
    }
  }, [open]);

  React.useEffect(() => {
    if (!debouncedValue) {
      setResults([]);
      setError(null);
      return;
    }

    let active = true;
    setLoading(true);
    setError(null);

    searchContracts(debouncedValue, {
      limit: 10,
      network: networkParam,
    })
      .then((data) => {
        if (!active) return;
        const items = Array.isArray(data.items) ? data.items : [];
        // The endpoint does not filter by network yet, so honour the selected
        // network here instead (issue #619).
        setResults(
          networkParam ? items.filter((c) => c.network === networkParam) : items
        );
        setLoading(false);
      })
      .catch((err) => {
        if (!active) return;
        setResults([]);
        setLoading(false);
        setError(
          err instanceof ApiError
            ? err.message
            : "Search is unavailable right now."
        );
      });

    return () => {
      active = false;
    };
  }, [debouncedValue, networkParam]);

  return (
    <>
      <button
        onClick={() => setOpen(true)}
        aria-label="Search (Ctrl+K)"
        className="hidden md:flex items-center gap-2 px-3 py-1.5 text-sm text-[var(--color-text-secondary)] bg-[var(--color-bg-card)] border border-[var(--color-border)] rounded-md hover:border-[var(--color-text-secondary)] transition-colors"
      >
        <SearchIcon />
        <span>Search contracts...</span>
        <kbd className="ml-2 px-1.5 py-0.5 text-xs rounded bg-[var(--color-bg-page)] border border-[var(--color-border)] opacity-70">
          ⌘K
        </kbd>
      </button>

      {/* Mobile entry point: the shortcut is keyboard-only, so expose a button. */}
      <button
        onClick={() => setOpen(true)}
        aria-label="Search (Ctrl+K)"
        className="md:hidden flex items-center justify-center h-9 w-9 rounded-md border border-[var(--color-border)] bg-[var(--color-bg-card)] text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
      >
        <SearchIcon />
      </button>

      <Command.Dialog
        open={open}
        onOpenChange={setOpen}
        label="Search Contracts"
        className="fixed inset-0 z-50 flex items-start justify-center pt-[20vh] bg-black/50 backdrop-blur-sm"
      >
        <div className="w-full max-w-xl bg-[var(--color-bg-card)] rounded-xl border border-[var(--color-border)] shadow-2xl overflow-hidden animate-in fade-in zoom-in-95">
          <Command.Input
            value={inputValue}
            onValueChange={setInputValue}
            placeholder="Search contracts by ID or label..."
            className="w-full px-4 py-4 bg-transparent outline-none text-[var(--color-text-primary)] placeholder-[var(--color-text-secondary)] border-b border-[var(--color-border)]"
          />

          <Command.List className="max-h-[60vh] overflow-y-auto p-2">
            <Command.Empty
              role={loading ? "status" : undefined}
              aria-busy={loading || undefined}
              className="p-6 text-center text-sm text-[var(--color-text-secondary)]"
            >
              {loading ? "Searching..." : error ? error : "No results found."}
            </Command.Empty>

            <Command.Group
              heading="Commands"
              className="text-xs font-semibold text-[var(--color-text-secondary)] mb-2 px-2 mt-4 first:mt-0"
            >
              <Command.Item
                onSelect={() => {
                  router.push("/contracts/new");
                  setOpen(false);
                }}
                className={searchItemClassName}
              >
                Track contract
              </Command.Item>
              <Command.Item
                onSelect={() => {
                  setTheme(theme === "dark" ? "light" : "dark");
                  setOpen(false);
                }}
                className={searchItemClassName}
              >
                Toggle theme
              </Command.Item>
            </Command.Group>

            {results.length > 0 && (
              <Command.Group
                heading="Contracts"
                className="text-xs font-semibold text-[var(--color-text-secondary)] px-2 mt-4"
              >
                {results.map((contract) => (
                  <Command.Item
                    key={contract.id}
                    // cmdk filters items client-side, so the label has to be
                    // part of the value or a label-only query hides the result.
                    value={[contract.label ?? "", contract.id].join(" ").trim()}
                    onSelect={() => {
                      router.push(`/contracts/${contract.id}`);
                      setOpen(false);
                    }}
                    className={searchItemClassName}
                  >
                    <div>
                      <div className="font-medium">
                        {contract.label || "Unnamed Contract"}
                      </div>
                      <div
                        className="text-xs text-[var(--color-text-secondary)] font-mono truncate max-w-md"
                        title={contract.id}
                      >
                        {contract.id}
                      </div>
                    </div>
                    <div className="ml-auto text-xs px-2 py-1 rounded-full bg-[var(--color-bg-page)] border border-[var(--color-border)] capitalize text-[var(--color-text-secondary)]">
                      {contract.network}
                    </div>
                  </Command.Item>
                ))}
              </Command.Group>
            )}

            {debouncedValue && (
              <Command.Group
                heading="Pages"
                className="text-xs font-semibold text-[var(--color-text-secondary)] px-2 mt-4"
              >
                <Command.Item
                  value={`all-results ${debouncedValue}`}
                  onSelect={() => {
                    router.push(
                      `/search?q=${encodeURIComponent(debouncedValue)}`
                    );
                    setOpen(false);
                  }}
                  className={searchItemClassName}
                >
                  View all results for &ldquo;{debouncedValue}&rdquo;
                </Command.Item>
              </Command.Group>
            )}
          </Command.List>
        </div>
      </Command.Dialog>
    </>
  );
}
