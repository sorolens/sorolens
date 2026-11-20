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
                  ? "bg-green-900/40 text-green-400"
                  : "bg-red-900/40 text-red-400"
              }`}
            >
              {event.in_successful_call ? "SUCCESS" : "FAILED"}
            </span>
            <span className="font-mono text-xs text-gray-400">
              ID: {event.id}
            </span>
          </div>

          <button
            ref={closeBtnRef}
            type="button"
            onClick={onClose}
            aria-label="Close event inspector"
            className="rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-white/10 hover:text-white"
          >
            ✕
          </button>
        </div>

        {/* Metadata Bar */}
        <div className="grid grid-cols-2 gap-4 border-b border-[var(--color-border)] bg-black/20 px-6 py-3 text-xs">
          <div>
            <span className="text-gray-500">Contract: </span>
            <span className="font-mono font-medium text-gray-300">
              <MonoId value={event.contract_id} headChars={10} tailChars={8} />
            </span>
          </div>
          <div>
            <span className="text-gray-500">Tx Hash: </span>
            <span className="font-mono font-medium text-gray-300">
              <MonoId value={event.tx_hash} headChars={8} tailChars={8} />
            </span>
          </div>
          <div>
            <span className="text-gray-500">Ledger: </span>
            <span className="font-mono text-gray-300">{event.ledger}</span>
          </div>
          <div>
            <span className="text-gray-500">Timestamp: </span>
            <span className="text-gray-300">
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
                : "border-transparent text-gray-400 hover:text-gray-200"
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
                : "border-transparent text-gray-400 hover:text-gray-200"
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
                  <h3 className="text-xs font-semibold uppercase tracking-wider text-gray-400">
                    Topics ({topics.length})
                  </h3>
                  <span className="text-[11px] text-gray-500">
                    Click nodes to expand • Hover to copy path
                  </span>
                </div>
                <div className="rounded-lg border border-[var(--color-border)] bg-black/30 p-3 space-y-2">
                  {topics.length === 0 ? (
                    <div className="text-xs text-gray-500 italic">
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
                  <h3 className="text-xs font-semibold uppercase tracking-wider text-gray-400">
                    Event Value (Payload)
                  </h3>
                  {event.value_xdr && (
                    <CopyButton
                      text={event.value_xdr}
                      label="Copy Raw Value XDR"
                    />
                  )}
                </div>
                <div className="rounded-lg border border-[var(--color-border)] bg-black/30 p-3">
                  {event.value_xdr ? (
                    <ScValTree
                      label="Value"
                      xdr={event.value_xdr}
                      path="value"
                      defaultExpanded={true}
                    />
                  ) : (
                    <div className="text-xs text-gray-500 italic">
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
                        ? "bg-white/10 text-white"
                        : "text-gray-400 hover:text-gray-200"
                    }`}
                  >
                    Raw XDR
                  </button>
                  <button
                    type="button"
                    onClick={() => setRawSubTab("json")}
                    className={`rounded px-3 py-1 text-xs font-medium transition-colors ${
                      rawSubTab === "json"
                        ? "bg-white/10 text-white"
                        : "text-gray-400 hover:text-gray-200"
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
                    <h4 className="mb-1 text-xs font-medium text-gray-400">
                      Topic XDR Payloads
                    </h4>
                    {topics.map((tXdr, i) => (
                      <div
                        key={i}
                        className="mb-2 flex items-center justify-between rounded bg-black/40 p-2 font-mono text-xs text-gray-300"
                      >
                        <span className="truncate mr-2">
                          <strong className="text-gray-500">[{i}]:</strong>{" "}
                          {tXdr}
                        </span>
                        <CopyButton text={tXdr} label={`Copy Topic ${i} XDR`} />
                      </div>
                    ))}
                  </div>

                  <div>
                    <h4 className="mb-1 text-xs font-medium text-gray-400">
                      Value XDR Payload
                    </h4>
                    <div className="flex items-center justify-between rounded bg-black/40 p-2 font-mono text-xs text-gray-300">
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
                  <pre className="max-h-[50vh] overflow-auto rounded bg-black/40 p-4 font-mono text-xs leading-relaxed text-gray-300">
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
            className="rounded-md bg-white/10 px-4 py-1.5 text-xs font-medium text-white transition-colors hover:bg-white/20"
          >
            Close
          </button>
        </div>
      </div>
    </div>
  );
}
