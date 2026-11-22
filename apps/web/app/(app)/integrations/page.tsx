"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import {
  listIntegrations,
  testIntegration,
  type Integration,
} from "@/lib/integrations";

/**
 * `/integrations` — documents the notification integrations the backend
 * already supports (Slack slash command, Discord bot, Telegram) and lets a user
 * send a real test message to check their deployment's configuration
 * (issue #612).
 */
export default function IntegrationsPage() {
  const [integrations, setIntegrations] = useState<Integration[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [testing, setTesting] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<
    Record<string, { type: "success" | "error"; message: string }>
  >({});

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    try {
      setIntegrations(await listIntegrations());
    } catch (err) {
      setLoadError(
        err instanceof Error ? err.message : "Could not load integrations."
      );
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  async function handleTest(integration: Integration) {
    setTesting(integration.id);
    setFeedback((current) => {
      const next = { ...current };
      delete next[integration.id];
      return next;
    });
    try {
      const result = await testIntegration(integration.id);
      setFeedback((current) => ({
        ...current,
        [integration.id]: {
          type: "success",
          message: result.detail || "Test message delivered.",
        },
      }));
    } catch (err) {
      setFeedback((current) => ({
        ...current,
        [integration.id]: {
          type: "error",
          message: err instanceof Error ? err.message : "Test message failed.",
        },
      }));
    } finally {
      setTesting(null);
    }
  }

  return (
    <div className="mx-auto max-w-4xl">
      <Link
        href="/watchdog/notifications"
        className="text-sm text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
      >
        ← Notification channels
      </Link>
      <h1 className="mt-2 text-2xl font-semibold text-[var(--color-text-primary)]">
        Integrations
      </h1>
      <p className="mt-2 max-w-2xl text-sm text-[var(--color-text-secondary)]">
        Deliver watchdog alerts where your team works. Per-contract channels are
        configured on the{" "}
        <Link
          href="/watchdog/notifications"
          className="text-[var(--color-accent)] underline underline-offset-2"
        >
          notification channels
        </Link>{" "}
        page; these integrations describe the deployment-level setup and let you
        send a test message.
      </p>

      {loadError ? (
        <div
          role="alert"
          className="mt-6 rounded-lg border border-[var(--color-danger-border)]/60 bg-[var(--color-danger-bg)] p-4 text-sm text-[var(--color-danger-text)]"
        >
          <p>{loadError}</p>
          <button
            type="button"
            onClick={() => void load()}
            className="mt-2 rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm text-[var(--color-text-primary)]"
          >
            Retry
          </button>
        </div>
      ) : loading ? (
        <p
          role="status"
          className="mt-6 text-sm text-[var(--color-text-secondary)]"
        >
          Loading integrations…
        </p>
      ) : (
        <div className="mt-6 space-y-6">
          {integrations.map((integration) => {
            const result = feedback[integration.id];
            return (
              <section
                key={integration.id}
                data-testid={`integration-${integration.id}`}
                aria-labelledby={`integration-${integration.id}-heading`}
                className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-5"
              >
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <h2
                      id={`integration-${integration.id}-heading`}
                      className="text-base font-semibold text-[var(--color-text-primary)]"
                    >
                      {integration.name}
                    </h2>
                    <p className="mt-1 max-w-2xl text-sm text-[var(--color-text-secondary)]">
                      {integration.description}
                    </p>
                  </div>
                  <span
                    data-testid={`integration-${integration.id}-status`}
                    className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${
                      integration.configured
                        ? "bg-[var(--color-safe-bg)] text-[var(--color-safe)]"
                        : "bg-[var(--color-warning-bg)] text-[var(--color-warning-text)]"
                    }`}
                  >
                    {integration.configured ? "Configured" : "Not configured"}
                  </span>
                </div>

                <ol className="mt-4 list-decimal space-y-1 pl-5 text-sm text-[var(--color-text-secondary)]">
                  {integration.setup_steps.map((step) => (
                    <li key={step}>{step}</li>
                  ))}
                </ol>

                <div className="mt-4 flex flex-wrap items-center gap-3 border-t border-[var(--color-border)] pt-3">
                  <a
                    href={integration.install_url}
                    target="_blank"
                    rel="noreferrer noopener"
                    className="rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm text-[var(--color-text-primary)] transition-colors hover:border-[var(--color-accent)]"
                  >
                    Install {integration.name}
                  </a>
                  <button
                    type="button"
                    data-testid={`integration-${integration.id}-test`}
                    disabled={testing === integration.id}
                    onClick={() => void handleTest(integration)}
                    className="rounded-md bg-[var(--color-accent)] px-3 py-1.5 text-sm font-medium text-[var(--color-text-primary)] transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {testing === integration.id
                      ? "Sending…"
                      : "Send test message"}
                  </button>

                  {result && (
                    <p
                      role="alert"
                      data-testid={`integration-${integration.id}-feedback`}
                      className={`text-sm ${
                        result.type === "success"
                          ? "text-[var(--color-safe)]"
                          : "text-[var(--color-danger-text)]"
                      }`}
                    >
                      {result.message}
                    </p>
                  )}
                </div>
              </section>
            );
          })}
        </div>
      )}
    </div>
  );
}
