"use client";

/**
 * A short, copyable command or request snippet with a visible copy action.
 *
 * The snippet itself is a <pre><code> so screen readers get the exact text;
 * the copy button reports success through a status region rather than changing
 * focus.
 */

import { useEffect, useRef, useState } from "react";

interface CopySnippetProps {
  /** Text placed in the clipboard, without line-number decoration. */
  code: string;
  /** What is being copied, used in the button's accessible name. */
  label: string;
  /** Language shown as a small caption, e.g. "bash". */
  language?: string;
}

export function CopySnippet({ code, label, language }: CopySnippetProps) {
  const [copied, setCopied] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current);
    },
    []
  );

  async function copy() {
    setError(null);
    try {
      await navigator.clipboard.writeText(code);
      setCopied(true);
      if (timer.current) clearTimeout(timer.current);
      timer.current = setTimeout(() => setCopied(false), 2000);
    } catch {
      setError("Could not copy to the clipboard.");
    }
  }

  return (
    <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)]">
      <div className="flex items-center justify-between gap-3 border-b border-[var(--color-border)] px-3 py-2">
        <span className="font-mono text-xs uppercase tracking-wide text-[var(--color-text-secondary)]">
          {language ?? "shell"}
        </span>
        <button
          type="button"
          onClick={() => void copy()}
          className="rounded-md border border-[var(--color-border)] px-2.5 py-1 text-xs font-medium text-[var(--color-text-primary)] transition-colors hover:border-[var(--color-accent)]"
        >
          {copied ? "Copied" : `Copy ${label}`}
        </button>
      </div>
      <pre className="overflow-x-auto px-3 py-3 text-left text-xs leading-relaxed">
        <code className="font-mono text-[var(--color-text-primary)]">
          {code}
        </code>
      </pre>
      <span aria-live="polite" className="sr-only">
        {copied ? `${label} copied to the clipboard.` : ""}
      </span>
      {error && (
        <p
          role="alert"
          className="px-3 pb-2 text-xs text-[var(--color-danger-text)]"
        >
          {error}
        </p>
      )}
    </div>
  );
}
