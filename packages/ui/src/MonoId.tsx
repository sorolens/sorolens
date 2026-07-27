import { useEffect, useMemo, useState } from "react";

export interface MonoIdProps {
  value: string;
  headChars?: number;
  tailChars?: number;
}

async function copyText(value: string): Promise<void> {
  if (typeof navigator !== "undefined" && navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(value);
    return;
  }

  if (typeof document === "undefined") {
    throw new Error("Clipboard API is unavailable in this environment.");
  }

  const textarea = document.createElement("textarea");
  textarea.value = value;
  textarea.setAttribute("readonly", "true");
  textarea.style.position = "fixed";
  textarea.style.opacity = "0";
  document.body.appendChild(textarea);
  textarea.select();

  const copied = document.execCommand("copy");
  document.body.removeChild(textarea);

  if (!copied) {
    throw new Error("Copy command was rejected.");
  }
}

function truncateValue(value: string, headChars: number, tailChars: number): string {
  if (value.length <= headChars + tailChars + 3) {
    return value;
  }

  return `${value.slice(0, headChars)}...${value.slice(-tailChars)}`;
}

export function MonoId({ value, headChars = 6, tailChars = 4 }: MonoIdProps) {
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!copied) {
      return;
    }

    const timer = window.setTimeout(() => setCopied(false), 1400);
    return () => window.clearTimeout(timer);
  }, [copied, value]);

  const displayValue = useMemo(
    () => truncateValue(value, headChars, tailChars),
    [value, headChars, tailChars],
  );

  async function handleClick() {
    await copyText(value);
    setCopied(true);
  }

  return (
    <button
      type="button"
      onClick={handleClick}
      title={value}
      aria-label={`Copy ${value}`}
      style={{
        alignItems: "center",
        background: "transparent",
        border: "none",
        color: "inherit",
        cursor: "copy",
        display: "inline-flex",
        fontFamily: '"JetBrains Mono", "SFMono-Regular", Consolas, "Liberation Mono", monospace',
        fontSize: "inherit",
        gap: "0.5rem",
        lineHeight: 1.2,
        padding: 0,
      }}
    >
      <span>{displayValue}</span>
      {copied ? (
        <span
          aria-live="polite"
          style={{
            borderRadius: 999,
            color: "#0f5132",
            fontSize: "0.75em",
            fontWeight: 600,
            padding: "0.1rem 0.45rem",
          }}
        >
          Copied
        </span>
      ) : null}
    </button>
  );
}