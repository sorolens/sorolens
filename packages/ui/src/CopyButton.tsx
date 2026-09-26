import { useEffect, useState, type MouseEvent } from "react";

export interface CopyButtonProps {
  /** Text copied to the clipboard on click. */
  text: string;
  /** Accessible name and tooltip for the button. */
  label: string;
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

/**
 * Small icon button that copies `text` to the clipboard and briefly shows a
 * "Copied" badge, mirroring the copy affordance in MonoId.
 */
export function CopyButton({ text, label }: CopyButtonProps) {
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!copied) {
      return;
    }

    const timer = window.setTimeout(() => setCopied(false), 1400);
    return () => window.clearTimeout(timer);
  }, [copied, text]);

  async function handleClick(event: MouseEvent<HTMLButtonElement>) {
    // The button often sits inside clickable rows; don't trigger them.
    event.stopPropagation();
    await copyText(text);
    setCopied(true);
  }

  return (
    <button
      type="button"
      onClick={handleClick}
      title={label}
      aria-label={label}
      style={{
        alignItems: "center",
        background: "transparent",
        border: "none",
        color: "inherit",
        cursor: "copy",
        display: "inline-flex",
        gap: "0.35rem",
        lineHeight: 1.2,
        padding: 0,
      }}
    >
      <svg
        aria-hidden="true"
        fill="none"
        height="14"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="2"
        viewBox="0 0 24 24"
        width="14"
      >
        <rect height="13" rx="2" width="13" x="9" y="9" />
        <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
      </svg>
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
