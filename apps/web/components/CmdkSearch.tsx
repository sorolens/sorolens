"use client";

import * as React from "react";
import { Command } from "cmdk";
import { useDebounce } from "use-debounce";
import { useRouter } from "next/navigation";
import { ContractSummary } from "@/lib/types";

// Need to match the generic Next.js standard API url
const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export function CmdkSearch() {
  const [open, setOpen] = React.useState(false);
  const [inputValue, setInputValue] = React.useState("");
  const [debouncedValue] = useDebounce(inputValue, 300);
  const [loading, setLoading] = React.useState(false);
  const [results, setResults] = React.useState<ContractSummary[]>([]);
  const router = useRouter();

  // Toggle the menu when ⌘K is pressed
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
    }
  }, [open]);

  React.useEffect(() => {
    if (!debouncedValue) {
      setResults([]);
      return;
    }

    let active = true;
    setLoading(true);

    fetch(`${API_URL}/api/v1/search?q=${encodeURIComponent(debouncedValue)}`)
      .then((res) => res.json())
      .then((data) => {
        if (active) {
          setResults(data.items || []);
          setLoading(false);
        }
      })
      .catch((err) => {
        console.error("Failed to search contracts:", err);
        if (active) setLoading(false);
      });

    return () => {
      active = false;
    };
  }, [debouncedValue]);

  return (
    <>
      {/* We can add a button somewhere else to open it, or just rely on shortcut. 
          The instructions say "Cmd+K opens a search modal". */}
      <button
        onClick={() => setOpen(true)}
        className="hidden md:flex items-center gap-2 px-3 py-1.5 text-sm text-[var(--color-text-secondary)] bg-[var(--color-surface)] border border-[var(--color-border)] rounded-md hover:border-[var(--color-border-hover)] transition-colors"
      >
        <svg
          width="15"
          height="15"
          viewBox="0 0 15 15"
          fill="none"
          xmlns="http://www.w3.org/2000/svg"
          className="opacity-70"
        >
          <path
            d="M10 6.5C10 8.433 8.433 10 6.5 10C4.567 10 3 8.433 3 6.5C3 4.567 4.567 3 6.5 3C8.433 3 10 4.567 10 6.5ZM9.30884 10.0159C8.53901 10.6318 7.56251 11 6.5 11C4.01472 11 2 8.98528 2 6.5C2 4.01472 4.01472 2 6.5 2C8.98528 2 11 4.01472 11 6.5C11 7.56251 10.6318 8.53901 10.0159 9.30884L12.8536 12.1464C13.0488 12.3417 13.0488 12.6583 12.8536 12.8536C12.6583 13.0488 12.3417 13.0488 12.1464 12.8536L9.30884 10.0159Z"
            fill="currentColor"
            fillRule="evenodd"
            clipRule="evenodd"
          ></path>
        </svg>
        <span>Search contracts...</span>
        <kbd className="ml-2 px-1.5 py-0.5 text-xs rounded bg-[var(--color-bg)] border border-[var(--color-border)] opacity-70">
          ⌘K
        </kbd>
      </button>

      <Command.Dialog
        open={open}
        onOpenChange={setOpen}
        label="Search Contracts"
        className="fixed inset-0 z-50 flex items-start justify-center pt-[20vh] bg-black/50 backdrop-blur-sm"
      >
        <div className="w-full max-w-xl bg-[var(--color-surface)] rounded-xl border border-[var(--color-border)] shadow-2xl overflow-hidden animate-in fade-in zoom-in-95">
          <Command.Input
            value={inputValue}
            onValueChange={setInputValue}
            placeholder="Search contracts by ID or label..."
            className="w-full px-4 py-4 bg-transparent outline-none text-[var(--color-text-primary)] placeholder-[var(--color-text-secondary)] border-b border-[var(--color-border)]"
          />

          <Command.List className="max-h-[60vh] overflow-y-auto p-2">
            <Command.Empty className="p-6 text-center text-sm text-[var(--color-text-secondary)]">
              {loading ? "Searching..." : "No contracts found."}
            </Command.Empty>

            {results.map((contract) => (
              <Command.Item
                key={contract.id}
                value={contract.id}
                onSelect={() => {
                  router.push(`/contracts/${contract.id}`);
                  setOpen(false);
                }}
                className="flex items-center gap-3 px-3 py-3 text-sm text-[var(--color-text-primary)] rounded-md cursor-pointer data-[selected=true]:bg-[var(--color-bg-hover)] data-[selected=true]:text-[var(--color-text-primary)]"
              >
                <div>
                  <div className="font-medium">
                    {contract.label || "Unnamed Contract"}
                  </div>
                  <div className="text-xs text-[var(--color-text-secondary)] font-mono truncate max-w-md">
                    {contract.id}
                  </div>
                </div>
                <div className="ml-auto text-xs px-2 py-1 rounded-full bg-[var(--color-bg)] border border-[var(--color-border)] capitalize text-[var(--color-text-secondary)]">
                  {contract.network}
                </div>
              </Command.Item>
            ))}
          </Command.List>
        </div>
      </Command.Dialog>
    </>
  );
}
