"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { MonoId } from "@sorolens/ui";
import {
  ApiError,
  createSubscription,
  deleteSubscription,
  listSubscriptions,
  sendTestNotification,
} from "@/lib/api";
import {
  CHANNELS,
  SEVERITIES,
  buildSubscriptionRequest,
  type SeverityFilter,
} from "@/lib/notifications";
import { ensureSession, getUserId } from "@/lib/user";
import { TableSkeleton } from "@/components/Skeleton";
import { ContractPicker } from "@/components/ContractPicker";
import { API_URL } from "@/lib/config";
import type { AlertSubscription, ChannelType } from "@/lib/types";

const STORAGE_KEY = "sorolens_user_id";

const inputClass =
  "w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg-card)] px-3 py-2 text-sm text-[var(--color-text-primary)] placeholder-[var(--color-text-secondary)] focus:outline-none focus:ring-1 focus:ring-[var(--color-accent)]";

function readUserId(): string {
  if (typeof window === "undefined") return "";
  try {
    return window.localStorage.getItem(STORAGE_KEY) || "";
  } catch {
    return "";
  }
}

/** Turns API failures into something actionable for this page. */
function describeError(err: unknown): string {
  if (err instanceof ApiError && (err.status === 401 || err.status === 403)) {
    return "Notification settings need a contributor account. Ask an admin to grant your user the contributor role.";
  }
  return err instanceof Error ? err.message : "Something went wrong.";
}

function destination(sub: AlertSubscription): string {
  if (sub.channel_type === "pagerduty") {
    return sub.has_routing_key ? "Routing key set" : "No routing key";
  }
  return sub.webhook_url;
}

export default function NotificationsPage() {
  const [userId, setUserId] = useState("");
  const [subs, setSubs] = useState<AlertSubscription[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);

  const [contractId, setContractId] = useState("");
  const [channel, setChannel] = useState<ChannelType>("slack");
  const [dest, setDest] = useState("");
  const [severity, setSeverity] = useState<SeverityFilter>("Critical");
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [testSending, setTestSending] = useState(false);
  const [testFeedback, setTestFeedback] = useState<{
    type: "success" | "error";
    message: string;
  } | null>(null);
  const [deleting, setDeleting] = useState<string | null>(null);

  const load = useCallback(async (uid: string) => {
    setLoading(true);
    setLoadError(null);
    try {
      const res = await listSubscriptions(uid || undefined);
      setSubs(res.subscriptions ?? []);
    } catch (err) {
      setLoadError(describeError(err));
    } finally {
      setLoading(false);
    }
  }, []);

  // Bootstrap user ID on first visit and fetch subscriptions
  useEffect(() => {
    let active = true;
    const cached = getUserId() || readUserId();
    if (cached) {
      setUserId(cached);
      void load(cached);
    }

    ensureSession()
      .then((session) => {
        if (!active) return;
        setUserId(session.user_id);
        if (!cached) {
          void load(session.user_id);
        }
      })
      .catch((err) => {
        if (!active) return;
        if (!cached) {
          setLoadError(describeError(err));
          setLoading(false);
        }
      });

    return () => {
      active = false;
    };
  }, [load]);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setTestFeedback(null);
    const built = buildSubscriptionRequest({
      contractId,
      channel,
      destination: dest,
      severity,
    });
    if ("error" in built) {
      setFormError(built.error);
      return;
    }
    setSaving(true);
    setFormError(null);

    let activeUid = userId;
    if (!activeUid) {
      try {
        const session = await ensureSession();
        activeUid = session.user_id;
        setUserId(activeUid);
      } catch {
        // proceed with current uid
      }
    }

    try {
      const created = await createSubscription(
        built.request,
        activeUid || undefined
      );
      setSubs((prev) => [created, ...prev]);
      setDest("");
    } catch (err) {
      setFormError(describeError(err));
    } finally {
      setSaving(false);
    }
  }

  async function handleTestSend() {
    setTestFeedback(null);
    setFormError(null);

    const built = buildSubscriptionRequest({
      contractId,
      channel,
      destination: dest,
      severity,
    });
    if ("error" in built) {
      setFormError(built.error);
      return;
    }

    setTestSending(true);
    try {
      const res = await sendTestNotification({
        contractId,
        channel,
        destination: dest,
        severity,
      });
      setTestFeedback({
        type: "success",
        message: res.message || "Test notification sent successfully!",
      });
    } catch (err: unknown) {
      setTestFeedback({
        type: "error",
        message:
          err instanceof Error
            ? err.message
            : "Failed to send test notification.",
      });
    } finally {
      setTestSending(false);
    }
  }

  async function handleDelete(id: string) {
    setDeleting(id);
    let activeUid = userId;
    if (!activeUid) {
      const session = await ensureSession().catch(() => null);
      if (session) {
        activeUid = session.user_id;
        setUserId(activeUid);
      }
    }

    try {
      await deleteSubscription(id, activeUid || undefined);
      setSubs((prev) => prev.filter((s) => s.id !== id));
    } catch (err) {
      setLoadError(describeError(err));
    } finally {
      setDeleting(null);
    }
  }

  const meta = CHANNELS[channel];

  return (
    <div className="space-y-8">
      <div>
        <Link
          href="/watchdog"
          className="text-sm text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
        >
          ← Watchdog
        </Link>
        <h1 className="mt-2 text-3xl font-bold tracking-tight">
          Notification channels
        </h1>
        <p className="mt-2 max-w-2xl text-sm text-[var(--color-text-secondary)]">
          Send watchdog alerts for a contract to Slack, Discord, PagerDuty, or
          any webhook, formatted natively for each channel. Integration secrets
          are stored server side and never shown again after you save them.
        </p>
      </div>

      <form
        onSubmit={handleSubmit}
        className="grid gap-4 rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-5 md:grid-cols-2"
        aria-label="Add notification channel"
      >
        <div className="flex flex-col gap-1 text-sm md:col-span-2">
          <label
            htmlFor="contract-picker-trigger"
            className="text-[var(--color-text-secondary)]"
          >
            Contract
          </label>
          <ContractPicker
            value={contractId}
            onChange={(val) => {
              setContractId(val);
              setFormError(null);
            }}
            placeholder="Select or search a monitored contract (C…)"
          />
        </div>

        <fieldset className="flex flex-col gap-1 text-sm">
          <legend className="mb-1 text-[var(--color-text-secondary)]">
            Channel
          </legend>
          <div className="flex flex-wrap gap-2">
            {(Object.keys(CHANNELS) as ChannelType[]).map((c) => (
              <label
                key={c}
                onClick={() => {
                  setChannel(c);
                  setDest("");
                  setFormError(null);
                  setTestFeedback(null);
                }}
                className={`cursor-pointer rounded-md border px-3 py-1.5 text-xs ${
                  channel === c
                    ? "border-[var(--color-accent)] text-[var(--color-text-primary)]"
                    : "border-[var(--color-border)] text-[var(--color-text-secondary)]"
                }`}
              >
                <input
                  type="radio"
                  name="channel"
                  value={c}
                  checked={channel === c}
                  onChange={() => {
                    setChannel(c);
                    setDest("");
                    setFormError(null);
                    setTestFeedback(null);
                  }}
                  className="sr-only"
                />
                {CHANNELS[c].label}
              </label>
            ))}
          </div>
        </fieldset>

        <fieldset className="flex flex-col gap-1 text-sm">
          <legend className="mb-1 text-[var(--color-text-secondary)]">
            Severity filter
          </legend>
          <div className="flex flex-wrap gap-2">
            {SEVERITIES.map((s) => (
              <label
                key={s}
                onClick={() => {
                  setSeverity(s);
                  setTestFeedback(null);
                }}
                className={`cursor-pointer rounded-md border px-3 py-1.5 text-xs font-medium transition-colors ${
                  severity === s
                    ? "border-[var(--color-accent)] bg-[var(--color-accent)]/15 text-[var(--color-text-primary)]"
                    : "border-[var(--color-border)] text-[var(--color-text-secondary)] hover:border-[var(--color-text-secondary)]"
                }`}
              >
                <input
                  type="radio"
                  name="severity"
                  value={s}
                  checked={severity === s}
                  onChange={() => {
                    setSeverity(s);
                    setTestFeedback(null);
                  }}
                  className="sr-only"
                />
                {s}
              </label>
            ))}
          </div>
          <span className="text-xs text-[var(--color-text-secondary)]">
            {severity === "Critical" &&
              "Deliver only Critical watchdog alerts."}
            {severity === "Warning" &&
              "Deliver Warning and Critical watchdog alerts."}
            {severity === "Info" &&
              "Deliver all alerts (Info, Warning, and Critical)."}
          </span>
        </fieldset>

        <label className="flex flex-col gap-1 text-sm md:col-span-2">
          <span className="text-[var(--color-text-secondary)]">
            {meta.destinationLabel}
          </span>
          <input
            type={channel === "pagerduty" ? "password" : "url"}
            autoComplete="off"
            value={dest}
            onChange={(e) => {
              setDest(e.target.value);
              setFormError(null);
              setTestFeedback(null);
            }}
            placeholder={meta.placeholder}
            className={`${inputClass} font-mono`}
          />
          <span className="text-xs text-[var(--color-text-secondary)]">
            {meta.help}
          </span>
        </label>

        <div className="flex flex-wrap items-center gap-3 md:col-span-2">
          <button
            type="submit"
            disabled={saving || testSending}
            className="rounded-md bg-[var(--color-accent)] px-4 py-2 text-sm font-medium text-[var(--color-bg-page)] transition-opacity hover:opacity-90 disabled:opacity-50"
          >
            {saving ? "Saving…" : `Add ${meta.label} channel`}
          </button>

          <button
            type="button"
            data-testid="test-send-btn"
            disabled={saving || testSending}
            onClick={handleTestSend}
            className="rounded-md border border-[var(--color-border)] bg-[var(--color-bg-card)] px-4 py-2 text-sm font-medium text-[var(--color-text-primary)] transition-colors hover:border-[var(--color-text-secondary)] disabled:opacity-50"
          >
            {testSending ? "Sending test…" : "Send test notification"}
          </button>

          {formError && (
            <p role="alert" className="text-sm text-red-400">
              {formError}
            </p>
          )}

          {testFeedback && (
            <p
              role="alert"
              data-testid="test-feedback"
              className={`text-sm ${
                testFeedback.type === "success"
                  ? "text-emerald-400"
                  : "text-red-400"
              }`}
            >
              {testFeedback.message}
            </p>
          )}
        </div>
      </form>

      <section className="space-y-3">
        <h2 className="text-xl font-semibold">Active channels</h2>
        {loadError ? (
          <p
            role="alert"
            className="rounded-lg border border-red-900/60 bg-red-950/30 p-4 text-sm text-red-300"
          >
            {loadError}
          </p>
        ) : loading ? (
          <TableSkeleton rows={3} />
        ) : subs.length === 0 ? (
          <p className="rounded-lg border border-dashed border-[var(--color-border)] p-8 text-center text-sm text-[var(--color-text-secondary)]">
            No notification channels yet. Add one above to get alerts where your
            team works.
          </p>
        ) : (
          <div className="overflow-x-auto rounded-lg border border-[var(--color-border)]">
            <table className="w-full min-w-[640px] text-left text-sm">
              <thead className="border-b border-[var(--color-border)] text-xs uppercase tracking-wide text-[var(--color-text-secondary)]">
                <tr>
                  <th scope="col" className="px-4 py-3 font-medium">
                    Contract
                  </th>
                  <th scope="col" className="px-4 py-3 font-medium">
                    Channel
                  </th>
                  <th scope="col" className="px-4 py-3 font-medium">
                    Severity
                  </th>
                  <th scope="col" className="px-4 py-3 font-medium">
                    Destination
                  </th>
                  <th scope="col" className="px-4 py-3 font-medium">
                    Added
                  </th>
                  <th scope="col" className="px-4 py-3">
                    <span className="sr-only">Actions</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {subs.map((s) => (
                  <tr
                    key={s.id}
                    className="border-b border-[var(--color-border)] last:border-0"
                  >
                    <td className="px-4 py-3 font-mono text-xs">
                      <MonoId
                        value={s.contract_id}
                        headChars={6}
                        tailChars={6}
                      />
                    </td>
                    <td className="px-4 py-3">
                      {CHANNELS[s.channel_type]?.label ?? s.channel_type}
                    </td>
                    <td className="px-4 py-3">
                      <span
                        className={`inline-flex items-center rounded px-2 py-0.5 text-xs font-medium ${
                          s.severity_filter === "Critical"
                            ? "border border-red-500/20 bg-red-500/10 text-red-400"
                            : s.severity_filter === "Warning"
                              ? "border border-amber-500/20 bg-amber-500/10 text-amber-400"
                              : "border border-blue-500/20 bg-blue-500/10 text-blue-400"
                        }`}
                      >
                        {s.severity_filter || "Critical"}
                      </span>
                    </td>
                    <td className="max-w-xs truncate px-4 py-3 font-mono text-xs text-[var(--color-text-secondary)]">
                      {destination(s)}
                    </td>
                    <td className="whitespace-nowrap px-4 py-3 text-xs text-[var(--color-text-secondary)]">
                      {new Date(s.created_at).toLocaleDateString()}
                    </td>
                    <td className="px-4 py-3 text-right">
                      <div className="flex items-center justify-end gap-3">
                        <button
                          type="button"
                          onClick={() => {
                            setContractId(s.contract_id);
                            setChannel(s.channel_type);
                            if (
                              s.severity_filter === "Critical" ||
                              s.severity_filter === "Warning" ||
                              s.severity_filter === "Info"
                            ) {
                              setSeverity(s.severity_filter);
                            }
                            if (
                              s.channel_type === "webhook" &&
                              s.webhook_url &&
                              !s.webhook_url.includes("***")
                            ) {
                              setDest(s.webhook_url);
                            }
                            window.scrollTo({ top: 0, behavior: "smooth" });
                          }}
                          className="text-xs text-[var(--color-accent)] hover:underline"
                        >
                          Configure
                        </button>
                        <button
                          type="button"
                          onClick={() => handleDelete(s.id)}
                          disabled={deleting === s.id}
                          className="text-xs text-red-400 hover:underline disabled:opacity-50"
                        >
                          {deleting === s.id ? "Removing…" : "Remove"}
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section className="rounded-lg border border-[var(--color-border)] p-5 text-sm">
        <h2 className="font-semibold">Slack slash command</h2>
        <p className="mt-2 text-[var(--color-text-secondary)]">
          Check a contract&apos;s status from Slack with{" "}
          <code>/sorolens &lt;contract_id&gt;</code>. Create a slash command in
          your Slack app with this request URL, and set the app&apos;s signing
          secret as <code>SLACK_SIGNING_SECRET</code> on the API so requests are
          verified.
        </p>
        <code className="mt-3 block overflow-x-auto rounded bg-black/30 p-3 font-mono text-xs">
          {API_URL}/integrations/slack/commands
        </code>
      </section>
    </div>
  );
}
