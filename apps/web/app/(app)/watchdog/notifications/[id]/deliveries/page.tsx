"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { TableSkeleton } from "@/components/Skeleton";
import {
  getDelivery,
  listDeliveries,
  retryDelivery,
  ApiError,
} from "@/lib/api";
import { getUserId } from "@/lib/user";
import type { WebhookDelivery } from "@/lib/types";

// ---- status badge ----------------------------------------------------------

const STATUS_CLASSES: Record<string, string> = {
  delivered:
    "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200",
  failed: "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200",
  pending:
    "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200",
};

function StatusBadge({ status }: { status: string }) {
  const cls = STATUS_CLASSES[status] ?? "bg-gray-100 text-gray-800";
  return (
    <span
      className={`inline-flex items-center rounded px-2 py-0.5 text-xs font-medium ${cls}`}
    >
      {status}
    </span>
  );
}

// ---- detail drawer ---------------------------------------------------------

function DeliveryDrawer({
  subscriptionId,
  deliveryId,
  onClose,
  onRetry,
  userId,
}: {
  subscriptionId: string;
  deliveryId: string;
  onClose: () => void;
  onRetry: (d: WebhookDelivery) => void;
  userId: string;
}) {
  const [delivery, setDelivery] = useState<WebhookDelivery | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [retrying, setRetrying] = useState(false);
  const [showRetryConfirm, setShowRetryConfirm] = useState(false);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    setLoading(true);
    getDelivery(subscriptionId, deliveryId, userId)
      .then(setDelivery)
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load"))
      .finally(() => setLoading(false));
  }, [subscriptionId, deliveryId, userId]);

  const curlCommand = delivery
    ? `curl -X POST \\\n  -H "Content-Type: application/json" \\\n  -d '${JSON.stringify(delivery.signed_payload ?? {})}' \\\n  <webhook_url>`
    : "";

  function copyToClipboard() {
    navigator.clipboard.writeText(curlCommand).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  }

  async function handleRetry() {
    setRetrying(true);
    try {
      const updated = await retryDelivery(subscriptionId, deliveryId, userId);
      onRetry(updated);
      onClose();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Retry failed");
    } finally {
      setRetrying(false);
      setShowRetryConfirm(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex justify-end" onClick={onClose}>
      <div
        className="relative h-full w-full max-w-2xl overflow-y-auto bg-[var(--color-bg-card)] shadow-xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="sticky top-0 flex items-center justify-between border-b border-[var(--color-border)] bg-[var(--color-bg-card)] p-4">
          <h2 className="text-base font-semibold text-[var(--color-text-primary)]">
            Delivery details
          </h2>
          <button
            onClick={onClose}
            className="rounded p-1 text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-hover)]"
          >
            ✕
          </button>
        </div>

        {loading && (
          <div className="p-6 text-sm text-[var(--color-text-secondary)]">
            Loading…
          </div>
        )}
        {error && <div className="p-6 text-sm text-red-500">{error}</div>}
        {delivery && (
          <div className="space-y-6 p-6">
            {/* Summary row */}
            <div className="flex flex-wrap gap-4 text-sm">
              <div>
                <span className="text-[var(--color-text-secondary)]">
                  Status{" "}
                </span>
                <StatusBadge status={delivery.status} />
              </div>
              <div>
                <span className="text-[var(--color-text-secondary)]">
                  Attempts{" "}
                </span>
                <span className="font-mono">{delivery.attempt_number}</span>
              </div>
              {delivery.response_status != null && (
                <div>
                  <span className="text-[var(--color-text-secondary)]">
                    HTTP{" "}
                  </span>
                  <span className="font-mono">{delivery.response_status}</span>
                </div>
              )}
            </div>

            {/* Payload */}
            <div>
              <h3 className="mb-2 text-xs font-medium uppercase tracking-wide text-[var(--color-text-secondary)]">
                Signed payload
              </h3>
              <pre className="overflow-x-auto rounded-md bg-[var(--color-bg)] p-3 text-xs text-[var(--color-text-primary)]">
                {JSON.stringify(delivery.signed_payload ?? {}, null, 2)}
              </pre>
            </div>

            {/* Response body */}
            {delivery.response_body && (
              <div>
                <h3 className="mb-2 text-xs font-medium uppercase tracking-wide text-[var(--color-text-secondary)]">
                  Response body
                </h3>
                <pre className="overflow-x-auto rounded-md bg-[var(--color-bg)] p-3 text-xs text-[var(--color-text-primary)]">
                  {delivery.response_body}
                </pre>
              </div>
            )}

            {/* Last error */}
            {delivery.last_error && (
              <div>
                <h3 className="mb-2 text-xs font-medium uppercase tracking-wide text-[var(--color-text-secondary)]">
                  Last error
                </h3>
                <p className="rounded-md bg-red-50 p-3 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">
                  {delivery.last_error}
                </p>
              </div>
            )}

            {/* cURL */}
            <div>
              <div className="mb-2 flex items-center justify-between">
                <h3 className="text-xs font-medium uppercase tracking-wide text-[var(--color-text-secondary)]">
                  Replay as cURL
                </h3>
                <button
                  onClick={copyToClipboard}
                  className="text-xs text-[var(--color-accent)] hover:underline"
                >
                  {copied ? "Copied!" : "Copy"}
                </button>
              </div>
              <pre className="overflow-x-auto rounded-md bg-[var(--color-bg)] p-3 text-xs text-[var(--color-text-primary)]">
                {curlCommand}
              </pre>
            </div>

            {/* Retry button (only for failed deliveries) */}
            {delivery.status === "failed" && (
              <div>
                <button
                  onClick={() => setShowRetryConfirm(true)}
                  disabled={retrying}
                  className="rounded-md bg-[var(--color-accent)] px-4 py-2 text-sm font-medium text-white hover:opacity-90 disabled:opacity-50"
                >
                  {retrying ? "Scheduling retry…" : "Retry now"}
                </button>
              </div>
            )}
          </div>
        )}
      </div>

      <ConfirmDialog
        isOpen={showRetryConfirm}
        title="Retry delivery?"
        description="This will reset the next_retry_at to now and increment the attempt counter. The indexer will pick it up on its next dispatch cycle."
        confirmText="Retry"
        onConfirm={handleRetry}
        onClose={() => setShowRetryConfirm(false)}
      />
    </div>
  );
}

// ---- main page -------------------------------------------------------------

export default function DeliveriesPage() {
  const params = useParams<{ id: string }>();
  const subscriptionId = params.id;
  const userId = getUserId();

  const [deliveries, setDeliveries] = useState<WebhookDelivery[]>([]);
  const [cursor, setCursor] = useState("");
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const loadDeliveries = useCallback(
    async (nextCursor = "") => {
      setLoading(true);
      setLoadError(null);
      try {
        const res = await listDeliveries(
          subscriptionId,
          nextCursor,
          50,
          userId
        );
        setDeliveries((prev) =>
          nextCursor ? [...prev, ...res.deliveries] : res.deliveries
        );
        setCursor(res.next_cursor);
        setHasMore(!!res.next_cursor);
      } catch (e) {
        setLoadError(
          e instanceof ApiError && (e.status === 401 || e.status === 403)
            ? "Delivery history requires contributor access."
            : e instanceof Error
              ? e.message
              : "Failed to load deliveries."
        );
      } finally {
        setLoading(false);
      }
    },
    [subscriptionId, userId]
  );

  useEffect(() => {
    loadDeliveries();
  }, [loadDeliveries]);

  function handleRetry(updated: WebhookDelivery) {
    setDeliveries((prev) =>
      prev.map((d) => (d.id === updated.id ? updated : d))
    );
  }

  return (
    <div className="mx-auto max-w-5xl px-4 py-8">
      <div className="mb-6 flex items-center gap-3">
        <Link
          href="/watchdog/notifications"
          className="text-sm text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
        >
          ← Notifications
        </Link>
        <span className="text-[var(--color-text-secondary)]">/</span>
        <h1 className="text-lg font-semibold text-[var(--color-text-primary)]">
          Delivery log
        </h1>
        <span className="font-mono text-xs text-[var(--color-text-secondary)]">
          {subscriptionId}
        </span>
      </div>

      {loadError && (
        <div className="mb-4 rounded-md border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-300">
          {loadError}
        </div>
      )}

      {loading && deliveries.length === 0 ? (
        <TableSkeleton rows={8} />
      ) : deliveries.length === 0 ? (
        <p className="text-sm text-[var(--color-text-secondary)]">
          No deliveries recorded for this subscription yet.
        </p>
      ) : (
        <>
          <div className="overflow-hidden rounded-lg border border-[var(--color-border)]">
            <table className="w-full text-sm">
              <thead className="bg-[var(--color-bg-muted)]">
                <tr>
                  <th className="px-4 py-3 text-left font-medium text-[var(--color-text-secondary)]">
                    Status
                  </th>
                  <th className="px-4 py-3 text-left font-medium text-[var(--color-text-secondary)]">
                    Attempt
                  </th>
                  <th className="px-4 py-3 text-left font-medium text-[var(--color-text-secondary)]">
                    HTTP
                  </th>
                  <th className="px-4 py-3 text-left font-medium text-[var(--color-text-secondary)]">
                    Last error
                  </th>
                  <th className="px-4 py-3 text-left font-medium text-[var(--color-text-secondary)]">
                    Next retry
                  </th>
                  <th className="px-4 py-3 text-left font-medium text-[var(--color-text-secondary)]">
                    Created
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--color-border)]">
                {deliveries.map((d) => (
                  <tr
                    key={d.id}
                    onClick={() => setSelectedId(d.id)}
                    className="cursor-pointer hover:bg-[var(--color-bg-hover)]"
                  >
                    <td className="px-4 py-3">
                      <StatusBadge status={d.status} />
                    </td>
                    <td className="px-4 py-3 font-mono text-[var(--color-text-primary)]">
                      {d.attempt_number}
                    </td>
                    <td className="px-4 py-3 font-mono text-[var(--color-text-secondary)]">
                      {d.response_status ?? "—"}
                    </td>
                    <td className="max-w-xs truncate px-4 py-3 text-[var(--color-text-secondary)]">
                      {d.last_error ?? "—"}
                    </td>
                    <td className="px-4 py-3 text-[var(--color-text-secondary)]">
                      {d.next_retry_at
                        ? new Date(d.next_retry_at).toLocaleString()
                        : "—"}
                    </td>
                    <td className="px-4 py-3 text-[var(--color-text-secondary)]">
                      {new Date(d.created_at).toLocaleString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {hasMore && (
            <div className="mt-4 text-center">
              <button
                onClick={() => loadDeliveries(cursor)}
                disabled={loading}
                className="text-sm text-[var(--color-accent)] hover:underline disabled:opacity-50"
              >
                {loading ? "Loading…" : "Load more"}
              </button>
            </div>
          )}
        </>
      )}

      {selectedId && (
        <DeliveryDrawer
          subscriptionId={subscriptionId}
          deliveryId={selectedId}
          onClose={() => setSelectedId(null)}
          onRetry={handleRetry}
          userId={userId}
        />
      )}
    </div>
  );
}
