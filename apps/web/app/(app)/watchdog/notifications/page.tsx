"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { MonoId } from "@sorolens/ui";
import {
  ApiError,
  createSubscription,
  deleteSubscription,
  listMonitoredContracts,
  listSubscriptions,
} from "@/lib/api";
import { CHANNELS, buildSubscriptionRequest } from "@/lib/notifications";
import { TableSkeleton } from "@/components/Skeleton";
import type { AlertSubscription, ChannelType, MonitoredContract } from "@/lib/types";

const STORAGE_KEY = "sorolens_user_id";
const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

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
  const [monitored, setMonitored] = useState<MonitoredContract[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);

  const [contractId, setContractId] = useState("");
  const [channel, setChannel] = useState<ChannelType>("slack");
  const [dest, setDest] = useState("");
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
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

  useEffect(() => {
    const uid = readUserId();
    setUserId(uid);
    void load(uid);
    listMonitoredContracts({ limit: 200 })
      .then((res) => setMonitored(res.contracts ?? []))
      .catch(() => setMonitored([]));
  }, [load]);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    const built = buildSubscriptionRequest({ contractId, channel, destination: dest });
    if ("error" in built) {
      setFormError(built.error);
      return;
    }
    setSaving(true);
    setFormError(null);
    try {
      const created = await createSubscription(built.request, userId || undefined);
      setSubs((prev) => [created, ...prev]);
      setDest("");
    } catch (err) {
      setFormError(describeError(err));
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete(id: string) {
    setDeleting(id);
    try {
      await deleteSubscription(id, userId || undefined);
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
        <Link href="/watchdog" className="text-sm text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]">
          ← Watchdog
        </Link>
        <h1 className="mt-2 text-3xl font-bold tracking-tight">Notification channels</h1>
        <p className="mt-2 max-w-2xl text-sm text-[var(--color-text-secondary)]">
          Send Critical watchdog alerts for a contract to Slack, Discord, PagerDuty, or any
          webhook, formatted natively for each channel. Integration secrets are stored server
          side and never shown again after you save them.
        </p>
      </div>

      <form
        onSubmit={handleSubmit}
        className="grid gap-4 rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-5 md:grid-cols-2"
        aria-label="Add notification channel"
      >
        <label className="flex flex-col gap-1 text-sm">
          <span className="text-[var(--color-text-secondary)]">Contract</span>
          <input
            list="monitored-contracts"
            value={contractId}
            onChange={(e) => setContractId(e.target.value)}
            placeholder="C… (a monitored contract)"
            className={`${inputClass} font-mono`}
          />
          <datalist id="monitored-contracts">
            {monitored.map((m) => (
              <option key={m.contract_id} value={m.contract_id}>
                {m.name}
              </option>
            ))}
          </datalist>
        </label>

        <fieldset className="flex flex-col gap-1 text-sm">
          <legend className="mb-1 text-[var(--color-text-secondary)]">Channel</legend>
          <div className="flex flex-wrap gap-2">
            {(Object.keys(CHANNELS) as ChannelType[]).map((c) => (
              <label
                key={c}
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
                  }}
                  className="sr-only"
                />
                {CHANNELS[c].label}
              </label>
            ))}
          </div>
        </fieldset>

        <label className="flex flex-col gap-1 text-sm md:col-span-2">
          <span className="text-[var(--color-text-secondary)]">{meta.destinationLabel}</span>
          <input
            type={channel === "pagerduty" ? "password" : "url"}
            autoComplete="off"
            value={dest}
            onChange={(e) => setDest(e.target.value)}
            placeholder={meta.placeholder}
            className={`${inputClass} font-mono`}
          />
          <span className="text-xs text-[var(--color-text-secondary)]">{meta.help}</span>
        </label>

        <div className="flex items-center gap-3 md:col-span-2">
          <button
            type="submit"
            disabled={saving}
            className="rounded-md bg-[var(--color-accent)] px-4 py-2 text-sm font-medium text-[var(--color-bg-page)] transition-opacity hover:opacity-90 disabled:opacity-50"
          >
            {saving ? "Saving…" : `Add ${meta.label} channel`}
          </button>
          {formError && (
            <p role="alert" className="text-sm text-red-400">
              {formError}
            </p>
          )}
        </div>
      </form>

      <section className="space-y-3">
        <h2 className="text-xl font-semibold">Active channels</h2>
        {loadError ? (
          <p role="alert" className="rounded-lg border border-red-900/60 bg-red-950/30 p-4 text-sm text-red-300">
            {loadError}
          </p>
        ) : loading ? (
          <TableSkeleton rows={3} />
        ) : subs.length === 0 ? (
          <p className="rounded-lg border border-dashed border-[var(--color-border)] p-8 text-center text-sm text-[var(--color-text-secondary)]">
            No notification channels yet. Add one above to get Critical alerts where your team works.
          </p>
        ) : (
          <div className="overflow-x-auto rounded-lg border border-[var(--color-border)]">
            <table className="w-full min-w-[640px] text-left text-sm">
              <thead className="border-b border-[var(--color-border)] text-xs uppercase tracking-wide text-[var(--color-text-secondary)]">
                <tr>
                  <th scope="col" className="px-4 py-3 font-medium">Contract</th>
                  <th scope="col" className="px-4 py-3 font-medium">Channel</th>
                  <th scope="col" className="px-4 py-3 font-medium">Destination</th>
                  <th scope="col" className="px-4 py-3 font-medium">Added</th>
                  <th scope="col" className="px-4 py-3"><span className="sr-only">Actions</span></th>
                </tr>
              </thead>
              <tbody>
                {subs.map((s) => (
                  <tr key={s.id} className="border-b border-[var(--color-border)] last:border-0">
                    <td className="px-4 py-3 font-mono text-xs">
                      <MonoId value={s.contract_id} headChars={6} tailChars={6} />
                    </td>
                    <td className="px-4 py-3">{CHANNELS[s.channel_type]?.label ?? s.channel_type}</td>
                    <td className="max-w-xs truncate px-4 py-3 font-mono text-xs text-[var(--color-text-secondary)]">
                      {destination(s)}
                    </td>
                    <td className="whitespace-nowrap px-4 py-3 text-xs text-[var(--color-text-secondary)]">
                      {new Date(s.created_at).toLocaleDateString()}
                    </td>
                    <td className="px-4 py-3 text-right">
                      <button
                        type="button"
                        onClick={() => handleDelete(s.id)}
                        disabled={deleting === s.id}
                        className="text-xs text-red-400 hover:underline disabled:opacity-50"
                      >
                        {deleting === s.id ? "Removing…" : "Remove"}
                      </button>
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
          Check a contract&apos;s status from Slack with <code>/sorolens &lt;contract_id&gt;</code>.
          Create a slash command in your Slack app with this request URL, and set the app&apos;s
          signing secret as <code>SLACK_SIGNING_SECRET</code> on the API so requests are verified.
        </p>
        <code className="mt-3 block overflow-x-auto rounded bg-black/30 p-3 font-mono text-xs">
          {API_URL}/integrations/slack/commands
        </code>
      </section>
    </div>
  );
}
