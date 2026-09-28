"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  createRule,
  deleteRule,
  listRuleLibrary,
  listRuleMetrics,
  listRules,
  previewRule,
  setRuleEnabled,
  validateRule,
} from "@/lib/api";
import type {
  AlertRule,
  RuleCatalogResponse,
  RuleDiagnostic,
  RuleLibraryEntry,
  RuleMetric,
  RulePreview,
  RuleSeverity,
  RuleValidation,
} from "@/lib/types";
import { classForToken, tokenizeRule } from "@/lib/ruleSyntax";
import { getUserId } from "@/lib/user";
import { StatCard } from "@/components/StatCard";

const SEVERITIES: RuleSeverity[] = ["Info", "Warning", "Critical"];

const DEFAULT_RULE = "fee_per_invocation > 0.5 XLM for 5m";

/**
 * RulesPage is the alert-rule editor. It keeps a two-way contract with the
 * server: the editor renders the same token stream the Go parser accepts, and
 * the preview pane calls the real evaluator so what you see is what the
 * indexer will decide.
 */
export default function RulesPage() {
  const [userId] = useState<string>(getUserId);
  const [source, setSource] = useState(DEFAULT_RULE);
  const [name, setName] = useState("Expensive invocation");
  const [severity, setSeverity] = useState<RuleSeverity>("Warning");
  const [contractId, setContractId] = useState("");
  const [network, setNetwork] = useState("");

  const [validation, setValidation] = useState<RuleValidation | null>(null);
  const [preview, setPreview] = useState<RulePreview | null>(null);
  const [previewing, setPreviewing] = useState(false);

  const [rules, setRules] = useState<AlertRule[]>([]);
  const [library, setLibrary] = useState<RuleLibraryEntry[]>([]);
  const [catalog, setCatalog] = useState<RuleCatalogResponse | null>(null);
  const [status, setStatus] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const previewSeq = useRef(0);

  const loadRules = useCallback(async () => {
    try {
      const data = await listRules();
      setRules(data.rules ?? []);
    } catch {
      setRules([]);
    }
  }, []);

  useEffect(() => {
    loadRules();
    listRuleLibrary()
      .then((d) => setLibrary(d.rules ?? []))
      .catch(() => setLibrary([]));
    listRuleMetrics()
      .then(setCatalog)
      .catch(() => setCatalog(null));
  }, [loadRules]);

  // Debounced validate + live preview. Every keystroke re-validates locally
  // cheaply, then asks the server for a fresh evaluation.
  useEffect(() => {
    const handle = setTimeout(async () => {
      const v = await validateRule(source).catch(() => null);
      setValidation(v);
      if (!v?.valid) {
        setPreview(null);
        return;
      }
      setPreviewing(true);
      const seq = ++previewSeq.current;
      try {
        const p = await previewRule(source, contractId || undefined).catch(
          () => null
        );
        if (seq === previewSeq.current) setPreview(p);
      } finally {
        if (seq === previewSeq.current) setPreviewing(false);
      }
    }, 400);
    return () => clearTimeout(handle);
  }, [source, contractId]);

  const tokens = useMemo(() => tokenizeRule(source), [source]);
  const errors: RuleDiagnostic[] = validation?.errors ?? [];
  const canSave = Boolean(validation?.valid && name.trim().length > 0);

  const save = async () => {
    if (!canSave) return;
    setSaving(true);
    setStatus(null);
    try {
      await createRule(
        {
          name: name.trim(),
          source: validation?.normalized ?? source,
          severity,
          contract_id: contractId.trim() || undefined,
          network: network || undefined,
        },
        userId
      );
      setStatus("Rule saved.");
      await loadRules();
    } catch (err) {
      setStatus(err instanceof Error ? err.message : "Failed to save rule");
    } finally {
      setSaving(false);
    }
  };

  const toggle = async (rule: AlertRule) => {
    try {
      const updated = await setRuleEnabled(rule.id, !rule.enabled, userId);
      setRules((prev) => prev.map((r) => (r.id === rule.id ? updated : r)));
    } catch {
      setStatus("Failed to update rule");
    }
  };

  const remove = async (rule: AlertRule) => {
    try {
      await deleteRule(rule.id, userId);
      setRules((prev) => prev.filter((r) => r.id !== rule.id));
    } catch {
      setStatus("Failed to delete rule");
    }
  };

  const useExample = (entry: RuleLibraryEntry) => {
    setSource(entry.source);
    setName(entry.name);
    setSeverity((entry.severity as RuleSeverity) ?? "Warning");
  };

  return (
    <div className="space-y-8">
      <header>
        <h1 className="text-2xl font-bold tracking-tight">Alert rules</h1>
        <p className="mt-1 text-sm text-[var(--color-text-secondary)]">
          Define your own SLOs with the rule language. Rules are validated on
          save and evaluated by the indexer every pass.
        </p>
      </header>

      <div className="grid gap-6 lg:grid-cols-2">
        {/* ---- editor ---- */}
        <section className="rounded-lg border border-[var(--color-border)] p-4">
          <h2 className="mb-3 text-lg font-semibold">Editor</h2>
          <div className="grid gap-3 sm:grid-cols-3">
            <label className="sm:col-span-2 text-sm">
              <span className="mb-1 block text-[var(--color-text-secondary)]">
                Name
              </span>
              <input
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="w-full rounded border border-[var(--color-border)] bg-transparent px-2 py-1"
                aria-label="Rule name"
              />
            </label>
            <label className="text-sm">
              <span className="mb-1 block text-[var(--color-text-secondary)]">
                Severity
              </span>
              <select
                value={severity}
                onChange={(e) => setSeverity(e.target.value as RuleSeverity)}
                className="w-full rounded border border-[var(--color-border)] bg-transparent px-2 py-1"
                aria-label="Severity"
              >
                {SEVERITIES.map((s) => (
                  <option key={s} value={s}>
                    {s}
                  </option>
                ))}
              </select>
            </label>
            <label className="text-sm">
              <span className="mb-1 block text-[var(--color-text-secondary)]">
                Contract (optional)
              </span>
              <input
                value={contractId}
                onChange={(e) => setContractId(e.target.value)}
                placeholder="C..."
                className="w-full rounded border border-[var(--color-border)] bg-transparent px-2 py-1 font-mono text-xs"
                aria-label="Contract id"
              />
            </label>
            <label className="text-sm">
              <span className="mb-1 block text-[var(--color-text-secondary)]">
                Network (optional)
              </span>
              <select
                value={network}
                onChange={(e) => setNetwork(e.target.value)}
                className="w-full rounded border border-[var(--color-border)] bg-transparent px-2 py-1"
                aria-label="Network"
              >
                <option value="">All networks</option>
                {(
                  catalog?.networks ?? [
                    "testnet",
                    "mainnet",
                    "futurenet",
                    "standalone",
                  ]
                ).map((n) => (
                  <option key={n} value={n}>
                    {n}
                  </option>
                ))}
              </select>
            </label>
          </div>

          {/* highlighted editor: a transparent textarea sits over coloured text */}
          <div className="relative mt-4 h-24 w-full overflow-hidden rounded border border-[var(--color-border)] font-mono text-sm">
            <pre
              aria-hidden
              className="pointer-events-none absolute inset-0 overflow-auto whitespace-pre-wrap break-words p-3"
            >
              {tokens.map((t, i) => (
                <span key={i} className={classForToken(t.kind)}>
                  {t.text}
                </span>
              ))}
              {"\n"}
            </pre>
            <textarea
              value={source}
              onChange={(e) => setSource(e.target.value)}
              spellCheck={false}
              aria-label="Rule source"
              className="absolute inset-0 h-full w-full resize-none overflow-auto whitespace-pre-wrap break-words bg-transparent p-3 font-mono text-sm text-transparent caret-[var(--color-text-primary)] outline-none"
            />
          </div>

          <EditorFeedback validation={validation} errors={errors} />

          <div className="mt-3 flex items-center gap-3">
            <button
              onClick={save}
              disabled={!canSave || saving}
              className="rounded bg-[var(--color-accent,#3b82f6)] px-3 py-1.5 text-sm font-medium text-white disabled:opacity-50"
            >
              {saving ? "Saving..." : "Save rule"}
            </button>
            {status && (
              <span className="text-sm text-[var(--color-text-secondary)]">
                {status}
              </span>
            )}
          </div>
        </section>

        {/* ---- live preview ---- */}
        <section className="rounded-lg border border-[var(--color-border)] p-4">
          <div className="mb-3 flex items-center justify-between">
            <h2 className="text-lg font-semibold">Live preview</h2>
            <span className="text-xs text-[var(--color-text-secondary)]">
              {previewing
                ? "evaluating..."
                : preview?.evaluated_at
                  ? "server evaluator"
                  : "waiting"}
            </span>
          </div>
          <Preview preview={preview} />
        </section>
      </div>

      {/* ---- sample library ---- */}
      <section>
        <h2 className="mb-3 text-lg font-semibold">Sample rules</h2>
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {library.map((entry) => (
            <button
              key={entry.name}
              onClick={() => useExample(entry)}
              className="rounded-lg border border-[var(--color-border)] p-3 text-left transition-colors hover:border-[var(--color-accent,#3b82f6)]"
            >
              <div className="flex items-center justify-between gap-2">
                <span className="font-medium">{entry.name}</span>
                <span className="text-xs text-[var(--color-text-secondary)]">
                  {entry.severity}
                </span>
              </div>
              <p className="mt-1 text-xs text-[var(--color-text-secondary)]">
                {entry.description}
              </p>
              <code className="mt-2 block break-all text-xs text-emerald-400">
                {entry.source}
              </code>
            </button>
          ))}
        </div>
      </section>

      {/* ---- saved rules ---- */}
      <section>
        <h2 className="mb-3 text-lg font-semibold">Saved rules</h2>
        {rules.length === 0 ? (
          <p className="text-sm text-[var(--color-text-secondary)]">
            No rules yet.
          </p>
        ) : (
          <ul className="divide-y divide-[var(--color-border)] rounded-lg border border-[var(--color-border)]">
            {rules.map((rule) => (
              <li
                key={rule.id}
                className="flex flex-wrap items-center gap-3 p-3"
              >
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="font-medium">{rule.name}</span>
                    <span className="text-xs text-[var(--color-text-secondary)]">
                      {rule.severity}
                    </span>
                    {!rule.enabled && (
                      <span className="text-xs text-amber-500">paused</span>
                    )}
                  </div>
                  <code className="block break-all text-xs text-emerald-400">
                    {rule.source}
                  </code>
                </div>
                <button
                  onClick={() => toggle(rule)}
                  className="rounded border border-[var(--color-border)] px-2 py-1 text-xs"
                >
                  {rule.enabled ? "Pause" : "Enable"}
                </button>
                <button
                  onClick={() => remove(rule)}
                  className="rounded border border-[var(--color-border)] px-2 py-1 text-xs text-rose-400"
                >
                  Delete
                </button>
              </li>
            ))}
          </ul>
        )}
      </section>

      {/* ---- metric reference ---- */}
      {catalog && catalog.metrics.length > 0 && (
        <section>
          <h2 className="mb-3 text-lg font-semibold">Metrics</h2>
          <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
            {catalog.metrics.map((m: RuleMetric) => (
              <div
                key={m.name}
                className="rounded border border-[var(--color-border)] p-2 text-sm"
              >
                <div className="flex items-center justify-between gap-2">
                  <code className="text-emerald-400">{m.name}</code>
                  <span className="text-xs text-[var(--color-text-secondary)]">
                    {m.unit}
                  </span>
                </div>
                <p className="mt-1 text-xs text-[var(--color-text-secondary)]">
                  {m.description}
                </p>
              </div>
            ))}
          </div>
        </section>
      )}
    </div>
  );
}

function EditorFeedback({
  validation,
  errors,
}: {
  validation: RuleValidation | null;
  errors: RuleDiagnostic[];
}) {
  if (validation?.valid) {
    return (
      <div className="mt-2 text-xs text-emerald-400">
        Valid
        {validation.window ? ` · window ${validation.window}` : ""}
        {validation.metrics?.length
          ? ` · uses ${validation.metrics.join(", ")}`
          : ""}
      </div>
    );
  }
  if (errors.length === 0) {
    return (
      <div className="mt-2 text-xs text-[var(--color-text-secondary)]">
        Type a rule to validate.
      </div>
    );
  }
  return (
    <ul className="mt-2 space-y-1 text-xs text-rose-400">
      {errors.map((e, i) => (
        <li key={i}>
          <span className="font-mono">
            {e.line}:{e.column}
          </span>{" "}
          {e.message}
          {e.hint ? (
            <span className="text-[var(--color-text-secondary)]">
              {" "}
              — {e.hint}
            </span>
          ) : null}
        </li>
      ))}
    </ul>
  );
}

function Preview({ preview }: { preview: RulePreview | null }) {
  if (!preview) {
    return (
      <p className="text-sm text-[var(--color-text-secondary)]">
        Enter a contract id (or add <code>on contract C...</code>) to evaluate
        against live samples.
      </p>
    );
  }
  if (!preview.valid) {
    return <p className="text-sm text-rose-400">Fix the rule to preview it.</p>;
  }
  const points = preview.points ?? [];
  return (
    <div className="space-y-3">
      <div className="grid gap-3 sm:grid-cols-3">
        <StatCard label="Verdict" value={preview.fired ? "Firing" : "OK"} />
        <StatCard label="Observed" value={formatNumber(preview.value)} />
        <StatCard label="Threshold" value={formatNumber(preview.threshold)} />
      </div>
      {points.length > 0 && <Sparkline points={points.map((p) => p.value)} />}
      <p className="text-sm text-[var(--color-text-secondary)]">
        {preview.reason}
      </p>
    </div>
  );
}

function Sparkline({ points }: { points: (number | null)[] }) {
  const values = points.filter((v): v is number => v !== null);
  if (values.length === 0) {
    return (
      <p className="text-xs text-[var(--color-text-secondary)]">
        No samples in this window.
      </p>
    );
  }
  const max = Math.max(...values, 1);
  const min = Math.min(...values, 0);
  const span = max - min || 1;
  return (
    <div className="flex h-12 items-end gap-0.5" aria-label="preview series">
      {points.map((v, i) => {
        const height = v === null ? 2 : Math.max(2, ((v - min) / span) * 48);
        return (
          <div
            key={i}
            title={v === null ? "no data" : String(v)}
            style={{ height: `${height}px` }}
            className={`flex-1 rounded-sm ${v === null ? "bg-[var(--color-border)]" : "bg-[var(--color-accent,#3b82f6)]"}`}
          />
        );
      })}
    </div>
  );
}

function formatNumber(v: number | null | undefined): string {
  if (v === null || v === undefined) return "—";
  if (Math.abs(v) >= 1000)
    return v.toLocaleString(undefined, { maximumFractionDigits: 2 });
  return String(Number(v.toFixed(6)));
}
