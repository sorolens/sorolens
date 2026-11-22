"use client";

import { useState } from "react";
import { decode, type DecodedScVal, type DecodedMapEntry } from "@sorolens/xdr";

interface ScValTreeProps {
  label: string;
  xdr?: string;
  decoded?: DecodedScVal | unknown;
  path: string;
  defaultExpanded?: boolean;
}

function getTypeColor(type: string): string {
  switch (type.toLowerCase()) {
    case "symbol":
      return "bg-[var(--color-warning-bg)] text-[var(--color-warning-text)] border-[var(--color-warning-border)]";
    case "string":
      return "bg-[var(--color-warning-bg)] text-[var(--color-warning-text)] border-[var(--color-warning-border)]";
    case "address":
      return "bg-[var(--color-safe-bg)] text-[var(--color-safe)] border-[var(--color-safe-border)]";
    case "vec":
      return "bg-[var(--color-info-bg)] text-[var(--color-info-text)] border-[var(--color-info-border)]";
    case "map":
      return "bg-[var(--color-accent-bg)] text-[var(--color-accent-text)] border-[var(--color-accent-border)]";
    case "i128":
    case "u128":
    case "i64":
    case "u64":
    case "i32":
    case "u32":
      return "bg-[var(--color-info-bg)] text-[var(--color-info-text)] border-[var(--color-info-border)]";
    case "bytes":
      return "bg-[var(--color-accent-bg)] text-[var(--color-accent-text)] border-[var(--color-accent-border)]";
    case "bool":
      return "bg-[var(--color-danger-bg)] text-[var(--color-danger-text)] border-[var(--color-danger-border)]";
    case "error":
      return "bg-[var(--color-danger-bg)] text-[var(--color-danger-text)] border-[var(--color-danger)]";
    default:
      return "bg-[var(--color-bg-card)] text-[var(--color-text-secondary)] border-[var(--color-border)]";
  }
}

export function ScValTree({
  label,
  xdr,
  decoded: initialDecoded,
  path,
  defaultExpanded = true,
}: ScValTreeProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);
  const [copiedPath, setCopiedPath] = useState(false);
  const [copiedValue, setCopiedValue] = useState(false);

  // If xdr is provided, decode it using @sorolens/xdr
  let scVal: DecodedScVal;
  let isDecodeError = false;

  if (xdr) {
    try {
      scVal = decode(xdr);
      if (scVal.type === "error") {
        isDecodeError = true;
      }
    } catch {
      isDecodeError = true;
      scVal = {
        type: "error",
        value: null,
        human: "<decode_error>",
      };
    }
  } else if (
    initialDecoded &&
    typeof initialDecoded === "object" &&
    "type" in initialDecoded
  ) {
    scVal = initialDecoded as DecodedScVal;
    if (scVal.type === "error") {
      isDecodeError = true;
    }
  } else {
    scVal = {
      type: typeof initialDecoded,
      value: initialDecoded,
      human: String(initialDecoded ?? ""),
    };
  }

  const handleCopyPath = (e: React.MouseEvent) => {
    e.stopPropagation();
    navigator.clipboard.writeText(path);
    setCopiedPath(true);
    setTimeout(() => setCopiedPath(false), 1500);
  };

  const handleCopyValue = (e: React.MouseEvent) => {
    e.stopPropagation();
    const textToCopy =
      typeof scVal.value === "object"
        ? JSON.stringify(scVal.value, null, 2)
        : String(scVal.human || scVal.value);
    navigator.clipboard.writeText(textToCopy);
    setCopiedValue(true);
    setTimeout(() => setCopiedValue(false), 1500);
  };

  if (isDecodeError) {
    return (
      <div
        data-testid={`scval-node-${path}`}
        className="my-1 rounded border border-[var(--color-danger-border)] bg-[var(--color-danger-bg)] p-2.5 font-mono text-xs text-[var(--color-danger-text)]"
      >
        <div className="flex items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            <span className="font-semibold text-[var(--color-danger-text)]">
              {label}:
            </span>
            <span className="rounded border border-[var(--color-danger)] bg-[var(--color-danger-bg)] px-1.5 py-0.5 text-[10px] font-bold uppercase tracking-wider text-[var(--color-danger-text)]">
              Decode Error
            </span>
          </div>
          <button
            type="button"
            onClick={handleCopyPath}
            title="Copy path"
            className="rounded px-1.5 py-0.5 text-[10px] text-[var(--color-danger-text)] hover:bg-[var(--color-danger-bg)]"
          >
            {copiedPath ? "Copied Path!" : path}
          </button>
        </div>
        <p className="mt-1 text-[11px] text-[var(--color-danger-text)]/80">
          Failed to decode ScVal XDR payload. The XDR data may be corrupted or
          non-standard.
        </p>
        {xdr && (
          <div className="mt-1.5 flex items-center gap-2 rounded bg-[var(--color-bg)] p-1.5">
            <span className="truncate text-[var(--color-danger-text)]">
              {xdr}
            </span>
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                if (xdr) {
                  navigator.clipboard.writeText(xdr);
                  setCopiedValue(true);
                  setTimeout(() => setCopiedValue(false), 1500);
                }
              }}
              className="ml-auto shrink-0 text-[10px] text-[var(--color-danger-text)] hover:underline"
            >
              {copiedValue ? "Copied Raw!" : "Copy Raw XDR"}
            </button>
          </div>
        )}
      </div>
    );
  }

  const isContainer = scVal.type === "vec" || scVal.type === "map";
  const vecItems =
    scVal.type === "vec" && Array.isArray(scVal.value)
      ? (scVal.value as DecodedScVal[])
      : [];
  const mapEntries =
    scVal.type === "map" && Array.isArray(scVal.value)
      ? (scVal.value as DecodedMapEntry[])
      : [];

  return (
    <div
      data-testid={`scval-node-${path}`}
      className="font-mono text-xs leading-relaxed"
    >
      <div
        className={`group flex items-center gap-2 rounded px-2 py-1 transition-colors hover:bg-white/5 ${
          isContainer ? "cursor-pointer" : ""
        }`}
        onClick={() => isContainer && setExpanded(!expanded)}
      >
        {isContainer ? (
          <span
            className="w-3 text-center text-[var(--color-text-secondary)] transition-transform select-none"
            aria-hidden="true"
          >
            {expanded ? "▼" : "▶"}
          </span>
        ) : (
          <span
            className="w-3 text-[var(--color-text-muted)]"
            aria-hidden="true"
          >
            •
          </span>
        )}

        <span className="font-medium text-[var(--color-text-primary)]">
          {label}:
        </span>

        <span
          className={`rounded border px-1.5 py-0.2 text-[10px] font-semibold uppercase tracking-wider ${getTypeColor(
            scVal.type
          )}`}
        >
          {scVal.type}
        </span>

        {!isContainer && (
          <span
            className="truncate text-[var(--color-text-secondary)] select-all"
            title={scVal.human}
          >
            {scVal.human}
          </span>
        )}

        {isContainer && (
          <span className="text-[var(--color-text-muted)] text-[11px]">
            {scVal.type === "vec"
              ? `[${vecItems.length} items]`
              : `{${mapEntries.length} entries}`}
          </span>
        )}

        <div className="ml-auto flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
          <button
            type="button"
            onClick={handleCopyPath}
            title={`Copy path: ${path}`}
            className="rounded bg-[var(--color-bg-hover)] px-1.5 py-0.5 text-[10px] text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-hover)] hover:text-[var(--color-text-primary)]"
          >
            {copiedPath ? "Copied Path!" : "Path"}
          </button>
          <button
            type="button"
            onClick={handleCopyValue}
            title="Copy value"
            className="rounded bg-[var(--color-bg-hover)] px-1.5 py-0.5 text-[10px] text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-hover)] hover:text-[var(--color-text-primary)]"
          >
            {copiedValue ? "Copied!" : "Copy"}
          </button>
        </div>
      </div>

      {isContainer && expanded && (
        <div className="ml-4 border-l border-[var(--color-border)] pl-2">
          {scVal.type === "vec" &&
            vecItems.map((item, idx) => (
              <ScValTree
                key={idx}
                label={`[${idx}]`}
                decoded={item}
                path={`${path}[${idx}]`}
                defaultExpanded={false}
              />
            ))}

          {scVal.type === "map" &&
            mapEntries.map((entry, idx) => (
              <div key={idx} className="my-0.5 border-b border-white/5 pb-0.5">
                <ScValTree
                  label={`key`}
                  decoded={entry.key}
                  path={`${path}[${idx}].key`}
                  defaultExpanded={false}
                />
                <ScValTree
                  label={`value`}
                  decoded={entry.value}
                  path={`${path}[${idx}].value`}
                  defaultExpanded={false}
                />
              </div>
            ))}
        </div>
      )}
    </div>
  );
}
