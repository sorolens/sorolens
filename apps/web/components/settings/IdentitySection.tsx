"use client";

import { useState } from "react";

/**
 * Identity card (issue #613): surfaces the verified workspace user ID the
 * dashboard already mints via `ensureSession` (lib/user.ts), with a copy
 * button, so the settings page shows a real identifier instead of implying a
 * key/localStorage blob is who you are. Display-only — there is nothing to
 * save — so it renders as a plain card rather than a SettingsCard with a
 * Save button.
 */
export function IdentitySection({ userId }: { userId: string }) {
  const [copied, setCopied] = useState(false);

  async function copyUserId() {
    if (!userId) return;
    await navigator.clipboard?.writeText(userId);
    setCopied(true);
  }

  return (
    <section
      data-testid="settings-identity"
      aria-labelledby="settings-identity-heading"
      className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-5"
    >
      <h2
        id="settings-identity-heading"
        className="text-base font-semibold text-[var(--color-text-primary)]"
      >
        Identity
      </h2>
      <p className="mt-1 text-sm text-[var(--color-text-secondary)]">
        Your Sorolens workspace ID, issued by the API when this browser signed
        in. API calls made from the dashboard are attributed to it.
      </p>

      <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
        <code
          data-testid="settings-identity-user-id"
          className="rounded bg-[var(--color-bg-page)] px-2 py-1 font-mono text-xs text-[var(--color-text-primary)]"
        >
          {userId || "Signing in…"}
        </code>

        <button
          type="button"
          disabled={!userId}
          onClick={() => {
            void copyUserId();
          }}
          className="rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm text-[var(--color-text-primary)] transition-colors hover:border-[var(--color-accent)] disabled:cursor-not-allowed disabled:opacity-50"
        >
          {copied ? "Copied" : "Copy ID"}
        </button>
      </div>
    </section>
  );
}
