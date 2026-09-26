/**
 * ContractChangelog – dashboard page component for the per-contract Wasm hash
 * changelog (issue #276).
 *
 * Route: /contracts/{id}/changelog
 *
 * Receives the changelog array from the API (GET /contracts/{id}/changelog)
 * and renders a vertical timeline of every recorded Wasm hash transition with
 * links to the upgrade transaction and the verification source when available.
 */

import React from "react";

/** Single changelog entry as returned by GET /contracts/{id}/changelog */
export interface ContractVersionEntry {
  id: number;
  contract_id: string;
  wasm_hash: string;
  first_seen_ledger: number;
  tx_hash?: string;
  verified_source_ref?: string;
  recorded_at: string; // ISO-8601
}

export interface ContractChangelogProps {
  contractId: string;
  entries: ContractVersionEntry[];
  /** Base URL of the Sorolens API, e.g. "https://api.sorolens.xyz" */
  apiBase?: string;
}

/** Returns the first 8 characters of a hash string. */
function shortHash(h: string): string {
  return h.length > 8 ? h.slice(0, 8) : h;
}

/** Formats an ISO-8601 timestamp as a human-readable local date string. */
function formatDate(iso: string): string {
  try {
    return new Date(iso).toLocaleString(undefined, {
      dateStyle: "medium",
      timeStyle: "short",
    });
  } catch {
    return iso;
  }
}

/**
 * ContractChangelog renders the full Wasm hash history for a contract as a
 * vertical timeline, newest entry last (chronological order matches the API).
 */
export function ContractChangelog({
  contractId,
  entries,
  apiBase = "",
}: ContractChangelogProps) {
  if (entries.length === 0) {
    return (
      <section aria-label="Wasm changelog">
        <p className="changelog-empty">
          No Wasm hash transitions have been recorded for this contract yet.
        </p>
      </section>
    );
  }

  const feedHref = `${apiBase}/contracts/${contractId}/changelog/feed`;
  const badgeHref = `${apiBase}/contracts/${contractId}/changelog/badge`;
  const badgeMd = `![wasm badge](${badgeHref})`;

  return (
    <section aria-label="Wasm changelog" className="changelog-root">
      <header className="changelog-header">
        <h2 className="changelog-title">Wasm hash changelog</h2>
        <div className="changelog-meta">
          <a href={feedHref} className="changelog-feed-link" aria-label="Atom feed">
            ⚛ Atom feed
          </a>
          <span className="changelog-badge-hint" title={badgeMd}>
            🏷 Badge available
          </span>
        </div>
      </header>

      <ol className="changelog-timeline" reversed>
        {[...entries].reverse().map((entry, idx) => (
          <li
            key={entry.id}
            className={`changelog-entry${idx === 0 ? " changelog-entry--latest" : ""}`}
            data-testid="changelog-entry"
          >
            <span className="changelog-dot" aria-hidden="true" />

            <div className="changelog-card">
              <div className="changelog-card-header">
                <code className="changelog-hash" title={entry.wasm_hash}>
                  {shortHash(entry.wasm_hash)}…
                </code>
                {idx === 0 && (
                  <span className="changelog-badge-latest">latest</span>
                )}
              </div>

              <dl className="changelog-details">
                <dt>Full hash</dt>
                <dd>
                  <code className="changelog-hash-full">{entry.wasm_hash}</code>
                </dd>

                <dt>First seen ledger</dt>
                <dd>{entry.first_seen_ledger.toLocaleString()}</dd>

                <dt>Recorded at</dt>
                <dd>
                  <time dateTime={entry.recorded_at}>
                    {formatDate(entry.recorded_at)}
                  </time>
                </dd>

                {entry.tx_hash && (
                  <>
                    <dt>Upgrade tx</dt>
                    <dd>
                      <code className="changelog-hash" title={entry.tx_hash}>
                        {shortHash(entry.tx_hash)}…
                      </code>
                    </dd>
                  </>
                )}

                {entry.verified_source_ref && (
                  <>
                    <dt>Verified source</dt>
                    <dd>
                      <a
                        href={entry.verified_source_ref}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="changelog-source-link"
                      >
                        {entry.verified_source_ref}
                      </a>
                    </dd>
                  </>
                )}
              </dl>
            </div>
          </li>
        ))}
      </ol>
    </section>
  );
}

export default ContractChangelog;
