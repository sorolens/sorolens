"use client";

import { useEffect, useRef, useState } from "react";
import { CopyButton, MonoId } from "@sorolens/ui";
import { useLocale } from "@/lib/i18n";
import type { ContractEvent } from "@/lib/types";
import { ScValTree } from "./ScValTree";

interface EventInspectorDrawerProps {
  event: ContractEvent | null;
  isOpen: boolean;
  onClose: () => void;
}

export function EventInspectorDrawer({
  event,
  isOpen,
  onClose,
}: EventInspectorDrawerProps) {
  const [activeTab, setActiveTab] = useState<"tree" | "raw">("tree");
  const [rawSubTab, setRawSubTab] = useState<"xdr" | "json">("xdr");
  const drawerRef = useRef<HTMLDivElement>(null);
  const closeBtnRef = useRef<HTMLButtonElement>(null);
  const { formatDateTime } = useLocale();

  // Escape key and focus management
  useEffect(() => {
    if (!isOpen) return;

    const timer = setTimeout(() => {
      closeBtnRef.current?.focus();
    }, 50);

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        onClose();
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => {
      clearTimeout(timer);
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [isOpen, onClose]);

  if (!isOpen || !event) return null;

  const topics = event.topic_xdr || [];
  const fullJson = JSON.stringify(event, null, 2);

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="event-inspector-title"
      className="fixed inset-0 z-50 flex justify-end bg-black/60 backdrop-blur-xs transition-opacity"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        ref={drawerRef}
        data-testid="event-inspector-drawer"
        className="flex h-full w-full max-w-2xl flex-col border-l border-[var(--color-border)] bg-[var(--color-bg-card)] shadow-2xl animate-in slide-in-from-right duration-200"
      >
        {/* Drawer Header */}
        <div className="flex items-center justify-between border-b border-[var(--color-border)] px-6 py-4">
          <div className="flex items-center gap-3">
            <h2
              id="event-inspector-title"
              className="text-base font-semibold text-[var(--color-text-primary)]"
            >
              Event Inspector
            </h2>
            <span
              className={`rounded-full px-2 py-0.5 text-xs font-medium ${
                event.in_successful_call
                  ? "bg-[var(--color-safe-bg)] text-[var(--color-safe)]"
                  : "bg-[var(--color-danger-bg)]/40 text-[var(--color-danger-text)]"
              }`}
            >
              {event.in_successful_call ? "SUCCESS" : "FAILED"}
            </span>
            <span className="font-mono text-xs text-[var(--color-text-secondary)]">
              ID: {event.id}
            </span>
          </div>

          <button
            ref={closeBtnRef}
            type="button"
            onClick={onClose}
            aria-label="Close event inspector"
            className="rounded-lg p-1.5 text-[var(--color-text-secondary)] transition-colors hover:bg-[var(--color-bg-hover)] hover:text-[var(--color-text-primary)]"
          >
            ✕
          </button>
        </div>

        {/* Metadata Bar */}
        <div className="grid grid-cols-2 gap-4 border-b border-[var(--color-border)] bg-black/20 px-6 py-3 text-xs">
          <div>
            <span className="text-[var(--color-text-muted)]">Contract: </span>
            <span className="font-mono font-medium text-[var(--color-text-secondary)]">
              <MonoId value={event.contract_id} headChars={10} tailChars={8} />
            </span>
          </div>
          <div>
            <span className="text-[var(--color-text-muted)]">Tx Hash: </span>
            <span className="font-mono font-medium text-[var(--color-text-secondary)]">
              <MonoId value={event.tx_hash} headChars={8} tailChars={8} />
            </span>
          </div>
          <div>
            <span className="text-[var(--color-text-muted)]">Ledger: </span>
            <span className="font-mono text-[var(--color-text-secondary)]">
              {event.ledger}
            </span>
          </div>
          <div>
            <span className="text-[var(--color-text-muted)]">Timestamp: </span>
            <span className="text-[var(--color-text-secondary)]">
              {formatDateTime(event.ledger_closed_at)}
            </span>
          </div>
        </div>

        {/* Tab Controls */}
        <div className="flex border-b border-[var(--color-border)] px-6 pt-3">
          <button
            type="button"
            onClick={() => setActiveTab("tree")}
            data-testid="tab-tree"
            className={`border-b-2 px-4 pb-2.5 text-xs font-medium transition-colors ${
              activeTab === "tree"
                ? "border-[var(--color-accent,#6366f1)] text-[var(--color-text-primary)]"
                : "border-transparent text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
            }`}
          >
            Decoded ScVal Tree
          </button>
          <button
            type="button"
            onClick={() => setActiveTab("raw")}
            data-testid="tab-raw"
            className={`border-b-2 px-4 pb-2.5 text-xs font-medium transition-colors ${
              activeTab === "raw"
                ? "border-[var(--color-accent,#6366f1)] text-[var(--color-text-primary)]"
                : "border-transparent text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
            }`}
          >
            Raw XDR & JSON
          </button>
        </div>

        {/* Drawer Body */}
        <div className="flex-1 overflow-y-auto p-6 space-y-6">
          {activeTab === "tree" ? (
            <div className="space-y-6">
              {/* Topics Section */}
              <div>
                <div className="mb-2 flex items-center justify-between">
                  <h3 className="text-xs font-semibold uppercase tracking-wider text-[var(--color-text-secondary)]">
                    Topics ({topics.length})
                  </h3>
                  <span className="text-[11px] text-[var(--color-text-muted)]">
                    Click nodes to expand • Hover to copy path
                  </span>
                </div>
                <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] p-3 space-y-2">
                  {topics.length === 0 ? (
                    <div className="text-xs text-[var(--color-text-muted)] italic">
                      No topics emitted
                    </div>
                  ) : (
                    topics.map((topicXdr, idx) => (
                      <ScValTree
                        key={idx}
                        label={`Topic [${idx}]`}
                        xdr={topicXdr}
                        path={`topics[${idx}]`}
                        defaultExpanded={true}
                      />
                    ))
                  )}
                </div>
              </div>

              {/* Event Value Section */}
              <div>
                <div className="mb-2 flex items-center justify-between">
                  <h3 className="text-xs font-semibold uppercase tracking-wider text-[var(--color-text-secondary)]">
                    Event Value (Payload)
                  </h3>
                  {event.value_xdr && (
                    <CopyButton
                      text={event.value_xdr}
                      label="Copy Raw Value XDR"
                    />
                  )}
                </div>
                <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] p-3">
                  {event.value_xdr ? (
                    <ScValTree
                      label="Value"
                      xdr={event.value_xdr}
                      path="value"
                      defaultExpanded={true}
                    />
                  ) : (
                    <div className="text-xs text-[var(--color-text-muted)] italic">
                      No value payload
                    </div>
                  )}
                </div>
              </div>
            </div>
          ) : (
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <div className="flex rounded-md bg-white/5 p-0.5">
                  <button
                    type="button"
                    onClick={() => setRawSubTab("xdr")}
                    className={`rounded px-3 py-1 text-xs font-medium transition-colors ${
                      rawSubTab === "xdr"
                        ? "bg-[var(--color-bg-hover)] text-[var(--color-text-primary)]"
                        : "text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
                    }`}
                  >
                    Raw XDR
                  </button>
                  <button
                    type="button"
                    onClick={() => setRawSubTab("json")}
                    className={`rounded px-3 py-1 text-xs font-medium transition-colors ${
                      rawSubTab === "json"
                        ? "bg-[var(--color-bg-hover)] text-[var(--color-text-primary)]"
                        : "text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
                    }`}
                  >
                    Event JSON
                  </button>
                </div>
                <CopyButton
                  text={rawSubTab === "xdr" ? event.value_xdr : fullJson}
                  label={
                    rawSubTab === "xdr" ? "Copy Value XDR" : "Copy Full JSON"
                  }
                />
              </div>

              {rawSubTab === "xdr" ? (
                <div className="space-y-4">
                  <div>
                    <h4 className="mb-1 text-xs font-medium text-[var(--color-text-secondary)]">
                      Topic XDR Payloads
                    </h4>
                    {topics.map((tXdr, i) => (
                      <div
                        key={i}
                        className="mb-2 flex items-center justify-between rounded bg-[var(--color-bg)] p-2 font-mono text-xs text-[var(--color-text-secondary)]"
                      >
                        <span className="truncate mr-2">
                          <strong className="text-[var(--color-text-muted)]">
                            [{i}]:
                          </strong>{" "}
                          {tXdr}
                        </span>
                        <CopyButton text={tXdr} label={`Copy Topic ${i} XDR`} />
                      </div>
                    ))}
                  </div>

                  <div>
                    <h4 className="mb-1 text-xs font-medium text-[var(--color-text-secondary)]">
                      Value XDR Payload
                    </h4>
                    <div className="flex items-center justify-between rounded bg-[var(--color-bg)] p-2 font-mono text-xs text-[var(--color-text-secondary)]">
                      <span className="truncate mr-2">{event.value_xdr}</span>
                      <CopyButton
                        text={event.value_xdr}
                        label="Copy Value XDR"
                      />
                    </div>
                  </div>
                </div>
              ) : (
                <div>
                  <pre className="max-h-[50vh] overflow-auto rounded bg-[var(--color-bg)] p-4 font-mono text-xs leading-relaxed text-[var(--color-text-secondary)]">
                    {fullJson}
                  </pre>
                </div>
              )}
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="border-t border-[var(--color-border)] bg-black/20 px-6 py-3 text-right">
          <button
            type="button"
            onClick={onClose}
            className="rounded-md bg-[var(--color-bg-hover)] px-4 py-1.5 text-xs font-medium text-[var(--color-text-primary)] transition-colors hover:bg-[var(--color-bg-hover)]"
          >
            Close
          </button>
        </div>
      </div>
    </div>
  );
}
