"use client";

/**
 * Label registry (issue #590).
 *
 * Browsing and creating readable account/contract names was API-only: the
 * dashboard only resolved a label through LabelledId. This page exposes the
 * registry: search + scope filter, a create form with client-side validation,
 * and a clear failure state for rejected requests.
 */

import { useCallback, useEffect, useState } from "react";
import { createLabel, listLabels } from "@/lib/api";
import type { LabelEntry, LabelScope } from "@/lib/types";
import { useSessionUserId } from "@/lib/useSession";
import { CardSkeleton } from "@/components/Skeleton";
import { EmptyState } from "@/components/EmptyState";
import { ErrorState, errorMessage } from "@/components/ErrorState";

const STRKEY_RE = /^[GC][A-Z2-7]{55}$/;

type ScopeFilter = "all" | LabelScope;

function scopeBadge(scope: string) {
  return scope === "public" ? "Public" : "Workspace";
}

export default function LabelsPage() {
  const userId = useSessionUserId();
  const [labels, setLabels] = useState<LabelEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [scopeFilter, setScopeFilter] = useState<ScopeFilter>("all");

  const [label, setLabel] = useState("");
  const [value, setValue] = useState("");
  const [scope, setScope] = useState<LabelScope>("workspace");
  const [fieldErrors, setFieldErrors] = useState<{
    label?: string;
    value?: string;
  }>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  const load = useCallback(
    async (search: string) => {
      setLoading(true);
      setLoadError(null);
      try {
        const data = await listLabels(search, userId || undefined);
        setLabels(data.labels ?? []);
      } catch (err) {
        setLabels([]);
        setLoadError(errorMessage(err, "The label registry request failed."));
      } finally {
        setLoading(false);
      }
    },
    [userId]
  );

  useEffect(() => {
    void load(query);
    // The initial load waits for the session only long enough to include
    // workspace labels; changing the query reloads through the form.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [load]);

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const errors: { label?: string; value?: string } = {};
    const trimmedLabel = label.trim();
    const trimmedValue = value.trim();
    if (!trimmedLabel) errors.label = "Enter a name for this label.";
    else if (trimmedLabel.length > 64)
      errors.label = "Labels are limited to 64 characters.";
    if (!trimmedValue) errors.value = "Enter an account or contract ID.";
    else if (!STRKEY_RE.test(trimmedValue))
      errors.value =
        "Enter a 56-character Stellar account (G…) or contract (C…) ID.";
    setFieldErrors(errors);
    setFormError(null);
    setSaved(false);
    if (Object.keys(errors).length > 0) return;

    setSubmitting(true);
    try {
      await createLabel(
        { label: trimmedLabel, value: trimmedValue, scope },
        userId || undefined
      );
      setSaved(true);
      setLabel("");
      setValue("");
      await load(query);
    } catch (err) {
      setFormError(errorMessage(err, "Could not save the label."));
    } finally {
      setSubmitting(false);
    }
  }

  const visible = labels.filter(
    (entry) => scopeFilter === "all" || entry.scope === scopeFilter
  );

  return (
    <div className="mx-auto max-w-4xl">
      <h1 className="text-2xl font-semibold text-[var(--color-text-primary)]">
        Label registry
      </h1>
      <p className="mt-2 text-sm text-[var(--color-text-secondary)]">
        Give accounts and contracts a readable name. Public labels are visible
        to everyone; workspace labels are only visible to this workspace.
      </p>

      <form
        role="search"
        aria-label="Search labels"
        className="mt-6 flex flex-wrap items-end gap-3"
        onSubmit={(event) => {
          event.preventDefault();
          void load(query);
        }}
      >
        <div className="min-w-0 flex-1">
          <label
            htmlFor="label-search"
            className="block text-sm font-medium text-[var(--color-text-primary)]"
          >
            Search
          </label>
          <input
            id="label-search"
            type="search"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Name or address…"
            className="mt-1 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg-card)] px-3 py-2 text-sm text-[var(--color-text-primary)]"
          />
        </div>
        <div>
          <label
            htmlFor="label-scope-filter"
            className="block text-sm font-medium text-[var(--color-text-primary)]"
          >
            Filter by scope
          </label>
          <select
            id="label-scope-filter"
            value={scopeFilter}
            onChange={(event) =>
              setScopeFilter(event.target.value as ScopeFilter)
            }
            className="mt-1 rounded-md border border-[var(--color-border)] bg-[var(--color-bg-card)] px-3 py-2 text-sm text-[var(--color-text-primary)]"
          >
            <option value="all">All scopes</option>
            <option value="public">Public</option>
            <option value="workspace">Workspace</option>
          </select>
        </div>
        <button
          type="submit"
          className="rounded-md border border-[var(--color-border)] px-4 py-2 text-sm font-medium text-[var(--color-text-primary)] transition-colors hover:border-[var(--color-accent)]"
        >
          Search
        </button>
      </form>

      <section aria-labelledby="label-list-heading" className="mt-8">
        <h2
          id="label-list-heading"
          className="text-lg font-semibold text-[var(--color-text-primary)]"
        >
          Labels
        </h2>
        {loadError ? (
          <ErrorState
            className="mt-3"
            title="Couldn't load labels."
            message={loadError}
            onRetry={() => void load(query)}
            retrying={loading}
          />
        ) : loading ? (
          <div className="mt-3">
            <CardSkeleton />
          </div>
        ) : visible.length === 0 ? (
          <EmptyState
            className="mt-3"
            title="No labels found"
            description={
              labels.length === 0
                ? "Create the first label for this workspace with the form below."
                : "No label matches this scope filter."
            }
          />
        ) : (
          <div className="mt-3 overflow-x-auto rounded-lg border border-[var(--color-border)]">
            <table className="min-w-full text-left text-sm">
              <caption className="sr-only">
                Public and workspace labels, with their Stellar identifier.
              </caption>
              <thead className="bg-[var(--color-bg-card)] text-xs uppercase tracking-wide text-[var(--color-text-secondary)]">
                <tr>
                  <th scope="col" className="px-3 py-2 font-medium">
                    Label
                  </th>
                  <th scope="col" className="px-3 py-2 font-medium">
                    Account or contract
                  </th>
                  <th scope="col" className="px-3 py-2 font-medium">
                    Scope
                  </th>
                </tr>
              </thead>
              <tbody>
                {visible.map((entry) => (
                  <tr
                    key={`${entry.scope}:${entry.label}`}
                    data-testid="label-row"
                    className="border-t border-[var(--color-border)]"
                  >
                    <td className="px-3 py-2 font-medium text-[var(--color-text-primary)]">
                      {entry.label}
                    </td>
                    <td className="break-all px-3 py-2 font-mono text-xs text-[var(--color-text-secondary)]">
                      {entry.value}
                    </td>
                    <td className="px-3 py-2">
                      <span className="rounded-full border border-[var(--color-border)] px-2 py-0.5 text-xs text-[var(--color-text-secondary)]">
                        {scopeBadge(entry.scope)}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section aria-labelledby="label-create-heading" className="mt-10">
        <h2
          id="label-create-heading"
          className="text-lg font-semibold text-[var(--color-text-primary)]"
        >
          Create a label
        </h2>
        <form className="mt-3 space-y-4" onSubmit={handleSubmit} noValidate>
          <div>
            <label
              htmlFor="label-name"
              className="block text-sm font-medium text-[var(--color-text-primary)]"
            >
              Name
            </label>
            <input
              id="label-name"
              value={label}
              onChange={(event) => setLabel(event.target.value)}
              aria-invalid={fieldErrors.label ? true : undefined}
              aria-describedby={
                fieldErrors.label ? "label-name-error" : undefined
              }
              className="mt-1 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg-card)] px-3 py-2 text-sm text-[var(--color-text-primary)]"
            />
            {fieldErrors.label && (
              <p
                id="label-name-error"
                role="alert"
                className="mt-1 text-xs text-[var(--color-danger-text)]"
              >
                {fieldErrors.label}
              </p>
            )}
          </div>

          <div>
            <label
              htmlFor="label-value"
              className="block text-sm font-medium text-[var(--color-text-primary)]"
            >
              Account or contract ID
            </label>
            <input
              id="label-value"
              value={value}
              onChange={(event) => setValue(event.target.value)}
              aria-invalid={fieldErrors.value ? true : undefined}
              aria-describedby={
                fieldErrors.value ? "label-value-error" : "label-value-hint"
              }
              className="mt-1 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg-card)] px-3 py-2 font-mono text-sm text-[var(--color-text-primary)]"
            />
            {fieldErrors.value ? (
              <p
                id="label-value-error"
                role="alert"
                className="mt-1 text-xs text-[var(--color-danger-text)]"
              >
                {fieldErrors.value}
              </p>
            ) : (
              <p
                id="label-value-hint"
                className="mt-1 text-xs text-[var(--color-text-secondary)]"
              >
                A 56-character Stellar account or contract ID.
              </p>
            )}
          </div>

          <div>
            <label
              htmlFor="label-scope"
              className="block text-sm font-medium text-[var(--color-text-primary)]"
            >
              Visibility
            </label>
            <select
              id="label-scope"
              value={scope}
              onChange={(event) => setScope(event.target.value as LabelScope)}
              className="mt-1 rounded-md border border-[var(--color-border)] bg-[var(--color-bg-card)] px-3 py-2 text-sm text-[var(--color-text-primary)]"
            >
              <option value="workspace">Workspace</option>
              <option value="public">Public</option>
            </select>
          </div>

          {formError && (
            <p
              role="alert"
              className="rounded-md border border-[var(--color-border)] bg-[var(--color-bg-page)] p-3 text-sm text-[var(--color-text-secondary)]"
            >
              {formError}
            </p>
          )}

          <div className="flex items-center gap-3">
            <button
              type="submit"
              disabled={submitting}
              className="rounded-md bg-[var(--color-accent)] px-4 py-2 text-sm font-medium text-[var(--color-text-primary)] transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {submitting ? "Saving…" : "Save label"}
            </button>
            {saved && (
              <p
                role="status"
                className="text-sm text-[var(--color-text-secondary)]"
              >
                Label saved.
              </p>
            )}
          </div>
        </form>
      </section>
    </div>
  );
}
