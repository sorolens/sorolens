"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { MonoId } from "@sorolens/ui";
import { listMonitoredContracts } from "@/lib/api";
import type { MonitoredContract } from "@/lib/types";

export interface ContractPickerProps {
  value: string;
  onChange: (contractId: string) => void;
  placeholder?: string;
  disabled?: boolean;
  network?: string;
}

const PAGE_SIZE = 8;

export function ContractPicker({
  value,
  onChange,
  placeholder = "Select or enter a monitored contract (C…)",
  disabled = false,
  network,
}: ContractPickerProps) {
  const [open, setOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [contracts, setContracts] = useState<MonitoredContract[]>([]);
  const [loading, setLoading] = useState(false);
  const [cursors, setCursors] = useState<(string | null)[]>([null]);
  const [cursorIndex, setCursorIndex] = useState(0);
  const [nextCursor, setNextCursor] = useState("");
  const [error, setError] = useState<string | null>(null);

  const containerRef = useRef<HTMLDivElement>(null);
  const searchInputRef = useRef<HTMLInputElement>(null);

  // Close dropdown on outside click
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (
        containerRef.current &&
        !containerRef.current.contains(event.target as Node)
      ) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  const fetchPage = useCallback(
    async (cursor: string | null) => {
      setLoading(true);
      setError(null);
      try {
        const res = await listMonitoredContracts({
          cursor: cursor ?? undefined,
          limit: PAGE_SIZE,
          network,
        });
        setContracts(res.contracts ?? []);
        setNextCursor(res.next_cursor ?? "");
      } catch (err: unknown) {
        setError(
          err instanceof Error
            ? err.message
            : "Failed to load monitored contracts."
        );
        setContracts([]);
      } finally {
        setLoading(false);
      }
    },
    [network]
  );

  // Load contracts when first opened
  useEffect(() => {
    if (open && contracts.length === 0 && !loading && !error) {
      void fetchPage(cursors[cursorIndex]);
    }
  }, [open, contracts.length, loading, error, cursors, cursorIndex, fetchPage]);

  // Focus search input when opening
  useEffect(() => {
    if (open && searchInputRef.current) {
      searchInputRef.current.focus();
    }
  }, [open]);

  const handleNext = () => {
    if (!nextCursor || loading) return;
    const targetCursor = nextCursor;
    setCursors((prev) => [...prev.slice(0, cursorIndex + 1), targetCursor]);
    setCursorIndex((prev) => prev + 1);
    void fetchPage(targetCursor);
  };

  const handlePrev = () => {
    if (cursorIndex === 0 || loading) return;
    const targetCursor = cursors[cursorIndex - 1];
    setCursorIndex((prev) => prev - 1);
    void fetchPage(targetCursor);
  };

  const handleSelect = (contractId: string) => {
    onChange(contractId);
    setOpen(false);
  };

  const filteredContracts = contracts.filter((c) => {
    if (!searchQuery.trim()) return true;
    const q = searchQuery.toLowerCase().trim();
    return (
      c.contract_id.toLowerCase().includes(q) ||
      (c.name && c.name.toLowerCase().includes(q))
    );
  });

  const selectedContract = contracts.find((c) => c.contract_id === value);

  return (
    <div ref={containerRef} className="relative w-full">
      {/* Trigger button / Input display */}
      <div className="flex items-center gap-2">
        <div
          role="combobox"
          aria-expanded={open}
          aria-haspopup="listbox"
          tabIndex={disabled ? -1 : 0}
          onClick={() => !disabled && setOpen((prev) => !prev)}
          onKeyDown={(e) => {
            if (!disabled && (e.key === "Enter" || e.key === " ")) {
              e.preventDefault();
              setOpen((prev) => !prev);
            }
          }}
          data-testid="contract-picker-trigger"
          className={`flex min-h-[42px] w-full cursor-pointer items-center justify-between rounded-md border border-[var(--color-border)] bg-[var(--color-bg-card)] px-3 py-2 text-sm transition-colors focus:outline-none focus:ring-1 focus:ring-[var(--color-accent)] ${
            disabled
              ? "cursor-not-allowed opacity-50"
              : "hover:border-[var(--color-text-secondary)]"
          }`}
        >
          {value ? (
            <div className="flex items-center gap-2 font-mono text-xs">
              <span className="font-semibold text-[var(--color-text-primary)]">
                {selectedContract?.name || "Contract:"}
              </span>
              <MonoId value={value} headChars={6} tailChars={6} />
            </div>
          ) : (
            <span className="text-[var(--color-text-secondary)] font-mono text-xs">
              {placeholder}
            </span>
          )}

          <div className="flex items-center gap-1.5 shrink-0 text-xs text-[var(--color-text-secondary)]">
            {value && (
              <button
                type="button"
                data-testid="contract-picker-clear"
                onClick={(e) => {
                  e.stopPropagation();
                  onChange("");
                }}
                className="rounded p-1 hover:text-[var(--color-text-primary)]"
                title="Clear selection"
              >
                ✕
              </button>
            )}
            <span className="text-[10px]">{open ? "▲" : "▼"}</span>
          </div>
        </div>
      </div>

      {/* Popover list */}
      {open && (
        <div
          role="listbox"
          data-testid="contract-picker-dropdown"
          className="absolute z-50 mt-1 max-h-96 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg-card)] shadow-xl"
        >
          {/* Search Header */}
          <div className="border-b border-[var(--color-border)] p-2">
            <input
              ref={searchInputRef}
              type="text"
              data-testid="contract-picker-search"
              value={searchQuery}
              onChange={(e) => {
                setSearchQuery(e.target.value);
                // Also allow direct typing/pasting of contract ID into picker
                if (e.target.value.trim().toUpperCase().startsWith("C")) {
                  onChange(e.target.value.trim().toUpperCase());
                }
              }}
              placeholder="Search by contract ID or name…"
              className="w-full rounded bg-[var(--color-bg-page)] px-3 py-1.5 text-xs text-[var(--color-text-primary)] placeholder-[var(--color-text-secondary)] focus:outline-none focus:ring-1 focus:ring-[var(--color-accent)]"
            />
          </div>

          {/* List options */}
          <div className="max-h-56 overflow-y-auto p-1 text-xs">
            {loading ? (
              <div
                data-testid="contract-picker-loading"
                role="status"
                aria-busy="true"
                className="py-6 text-center text-[var(--color-text-secondary)]"
              >
                Loading monitored contracts…
              </div>
            ) : error ? (
              <div className="p-3 text-center text-red-400">
                {error}
                <button
                  type="button"
                  onClick={() => fetchPage(cursors[cursorIndex])}
                  className="mt-1 block w-full text-xs underline hover:text-red-300"
                >
                  Retry
                </button>
              </div>
            ) : filteredContracts.length === 0 ? (
              <div
                data-testid="contract-picker-empty"
                className="py-6 text-center text-[var(--color-text-secondary)]"
              >
                {searchQuery ? (
                  <>
                    <p>No contracts match &ldquo;{searchQuery}&rdquo;</p>
                    {searchQuery.trim().length >= 4 && (
                      <button
                        type="button"
                        onClick={() =>
                          handleSelect(searchQuery.trim().toUpperCase())
                        }
                        className="mt-2 text-xs text-[var(--color-accent)] underline hover:opacity-80"
                      >
                        Use &ldquo;{searchQuery.trim().toUpperCase()}&rdquo;
                        directly
                      </button>
                    )}
                  </>
                ) : (
                  "No monitored contracts found."
                )}
              </div>
            ) : (
              filteredContracts.map((c) => {
                const isSelected = c.contract_id === value;
                return (
                  <div
                    key={c.contract_id}
                    role="option"
                    tabIndex={0}
                    aria-selected={isSelected}
                    data-testid="contract-picker-item"
                    onClick={() => handleSelect(c.contract_id)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" || e.key === " ") {
                        e.preventDefault();
                        handleSelect(c.contract_id);
                      }
                    }}
                    className={`flex w-full cursor-pointer items-center justify-between rounded px-2.5 py-2 text-left transition-colors ${
                      isSelected
                        ? "bg-[var(--color-accent)]/15 text-[var(--color-text-primary)]"
                        : "hover:bg-[var(--color-bg-page)] text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
                    }`}
                  >
                    <div className="min-w-0 pr-2">
                      <div className="truncate font-medium text-[var(--color-text-primary)]">
                        {c.name || "Unnamed Contract"}
                      </div>
                      <div className="font-mono text-[11px] opacity-80">
                        <MonoId
                          value={c.contract_id}
                          headChars={8}
                          tailChars={8}
                        />
                      </div>
                    </div>
                    {isSelected && (
                      <span className="shrink-0 text-xs text-[var(--color-accent)]">
                        ✓
                      </span>
                    )}
                  </div>
                );
              })
            )}
          </div>

          {/* Pagination Footer */}
          <div className="flex items-center justify-between border-t border-[var(--color-border)] bg-[var(--color-bg-page)]/50 px-3 py-2 text-xs">
            <span className="text-[var(--color-text-secondary)]">
              Page {cursorIndex + 1}
            </span>
            <div className="flex items-center gap-2">
              <button
                type="button"
                data-testid="contract-picker-prev"
                onClick={handlePrev}
                disabled={cursorIndex === 0 || loading}
                className="rounded border border-[var(--color-border)] px-2.5 py-1 text-xs text-[var(--color-text-primary)] hover:bg-[var(--color-bg-page)] disabled:opacity-40"
              >
                Previous
              </button>
              <button
                type="button"
                data-testid="contract-picker-next"
                onClick={handleNext}
                disabled={!nextCursor || loading}
                className="rounded border border-[var(--color-border)] px-2.5 py-1 text-xs text-[var(--color-text-primary)] hover:bg-[var(--color-bg-page)] disabled:opacity-40"
              >
                Next
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
