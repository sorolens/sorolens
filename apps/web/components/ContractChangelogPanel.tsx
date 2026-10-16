"use client";

import { useEffect, useState } from "react";
import { ContractChangelog, CopyButton } from "@sorolens/ui";
import type { ContractVersionEntry } from "@sorolens/ui";
import {
  ApiError,
  changelogApiBase,
  contractChangelogBadgeUrl,
  contractChangelogFeedUrl,
  getContractChangelog,
} from "@/lib/api";

interface ContractChangelogPanelProps {
  contractId: string;
}

type State =
  | { status: "loading" }
  | { status: "ready"; entries: ContractVersionEntry[] }
  | { status: "error"; message: string };

/**
 * ContractChangelogPanel shows a contract's Wasm upgrade history (issue #601)
 * using the shared ContractChangelog timeline, plus the Atom feed and the
 * embeddable badge so they are discoverable even before the first upgrade.
 */
export function ContractChangelogPanel({
  contractId,
}: ContractChangelogPanelProps) {
  const [state, setState] = useState<State>({ status: "loading" });

  useEffect(() => {
    let cancelled = false;
    setState({ status: "loading" });
    getContractChangelog(contractId)
      .then((entries) => {
        if (cancelled) return;
        // Guard the shape so an unexpected body can never crash the page.
        if (!Array.isArray(entries)) {
          setState({ status: "error", message: "Changelog unavailable." });
          return;
        }
        setState({ status: "ready", entries });
      })
      .catch((e: unknown) => {
        if (cancelled) return;
        setState({
          status: "error",
          message:
            e instanceof ApiError && e.status === 404
              ? "No changelog is available for this contract yet."
              : "Changelog unavailable.",
        });
      });
    return () => {
      cancelled = true;
    };
  }, [contractId]);

  const feedUrl = contractChangelogFeedUrl(contractId);
  const badgeUrl = contractChangelogBadgeUrl(contractId);
  const badgeMarkdown = `![Wasm hash](${badgeUrl})`;

  return (
    <div className="space-y-4">
      {state.status === "loading" && (
        <p className="text-sm text-[var(--color-text-secondary)]" role="status">
          Loading changelog…
        </p>
      )}
      {state.status === "error" && (
        <p className="text-sm text-[var(--color-text-secondary)]">
          {state.message}
        </p>
      )}
      {state.status === "ready" && (
        <ContractChangelog
          contractId={contractId}
          entries={state.entries}
          apiBase={changelogApiBase()}
        />
      )}

      <div
        className="flex flex-wrap items-center gap-4 rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-4 text-sm"
        data-testid="changelog-links"
      >
        <a
          href={feedUrl}
          target="_blank"
          rel="noopener noreferrer"
          className="text-[var(--color-accent)] hover:underline"
        >
          Subscribe (RSS / Atom)
        </a>
        <a
          href={badgeUrl}
          target="_blank"
          rel="noopener noreferrer"
          aria-label="Wasm hash badge"
        >
          {/* eslint-disable-next-line @next/next/no-img-element -- external SVG badge */}
          <img src={badgeUrl} alt="Wasm hash badge" height={20} />
        </a>
        <CopyButton text={badgeMarkdown} label="Copy badge Markdown" />
      </div>
    </div>
  );
}
