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
      return "bg-amber-900/40 text-amber-300 border-amber-700/50";
    case "string":
      return "bg-yellow-900/40 text-yellow-300 border-yellow-700/50";
    case "address":
      return "bg-emerald-900/40 text-emerald-300 border-emerald-700/50";
    case "vec":
      return "bg-blue-900/40 text-blue-300 border-blue-700/50";
    case "map":
      return "bg-purple-900/40 text-purple-300 border-purple-700/50";
    case "i128":
    case "u128":
    case "i64":
    case "u64":
    case "i32":
    case "u32":
      return "bg-cyan-900/40 text-cyan-300 border-cyan-700/50";
    case "bytes":
      return "bg-indigo-900/40 text-indigo-300 border-indigo-700/50";
    case "bool":
      return "bg-rose-900/40 text-rose-300 border-rose-700/50";
    case "error":
      return "bg-red-900/50 text-red-300 border-red-600";
    default:
      return "bg-gray-800 text-gray-300 border-gray-700";
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
        className="my-1 rounded border border-red-700/60 bg-red-950/30 p-2.5 font-mono text-xs text-red-200"
      >
        <div className="flex items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            <span className="font-semibold text-red-400">{label}:</span>
            <span className="rounded border border-red-600 bg-red-900/50 px-1.5 py-0.5 text-[10px] font-bold uppercase tracking-wider text-red-300">
              Decode Error
            </span>
          </div>
          <button
            type="button"
            onClick={handleCopyPath}
            title="Copy path"
            className="rounded px-1.5 py-0.5 text-[10px] text-red-400 hover:bg-red-900/40"
          >
            {copiedPath ? "Copied Path!" : path}
          </button>
        </div>
        <p className="mt-1 text-[11px] text-red-300/80">
          Failed to decode ScVal XDR payload. The XDR data may be corrupted or
          non-standard.
        </p>
        {xdr && (
          <div className="mt-1.5 flex items-center gap-2 rounded bg-black/40 p-1.5">
            <span className="truncate text-red-300">{xdr}</span>
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
              className="ml-auto shrink-0 text-[10px] text-red-400 hover:underline"
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
            className="w-3 text-center text-gray-400 transition-transform select-none"
            aria-hidden="true"
          >
            {expanded ? "▼" : "▶"}
          </span>
        ) : (
          <span className="w-3 text-gray-600" aria-hidden="true">
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
            className="truncate text-gray-300 select-all"
            title={scVal.human}
          >
            {scVal.human}
          </span>
        )}

        {isContainer && (
          <span className="text-gray-500 text-[11px]">
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
            className="rounded bg-white/10 px-1.5 py-0.5 text-[10px] text-gray-400 hover:bg-white/20 hover:text-white"
          >
            {copiedPath ? "Copied Path!" : "Path"}
          </button>
          <button
            type="button"
            onClick={handleCopyValue}
            title="Copy value"
            className="rounded bg-white/10 px-1.5 py-0.5 text-[10px] text-gray-400 hover:bg-white/20 hover:text-white"
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
