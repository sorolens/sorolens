"use client";

import { useMemo } from "react";
import type {
  RuleCatalogResponse,
  RulePreview,
  RuleValidation,
} from "@/lib/types";
import { RULE_AGGREGATIONS, RULE_METRICS, RULE_UNITS } from "@/lib/ruleSyntax";
import { SUPPORTED_OPERATORS, type VisualRuleTree } from "@/lib/ruleTree";

interface VisualRuleBuilderProps {
  value: VisualRuleTree;
  onChange: (next: VisualRuleTree) => void;
  catalog?: RuleCatalogResponse | null;
  validation?: RuleValidation | null;
  preview?: RulePreview | null;
  previewing?: boolean;
}

export function VisualRuleBuilder({
  value,
  onChange,
  catalog,
  validation,
  preview,
  previewing,
}: VisualRuleBuilderProps) {
  const metricsList = useMemo(() => {
    if (catalog?.metrics && catalog.metrics.length > 0) {
      return catalog.metrics.map((m) => m.name);
    }
    return Array.from(RULE_METRICS);
  }, [catalog]);

  const update = (patch: Partial<VisualRuleTree>) => {
    onChange({
      ...value,
      ...patch,
    });
  };

  return (
    <div className="space-y-6">
      {/* Visual Rule Builder Form */}
      <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-surface)] p-5 shadow-xs">
        <h3 className="mb-4 text-sm font-semibold tracking-wide text-[var(--color-text-secondary)] uppercase">
          Condition Builder
        </h3>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {/* Aggregation */}
          <div>
            <label
              htmlFor="rule-aggregation"
              className="mb-1 block text-xs font-medium text-[var(--color-text-secondary)]"
            >
              Aggregation (Optional)
            </label>
            <select
              id="rule-aggregation"
              className="w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg-base)] px-3 py-2 text-sm text-[var(--color-text-primary)] focus:border-[var(--color-accent)] focus:outline-hidden"
              value={value.aggregation || ""}
              onChange={(e) =>
                update({ aggregation: e.target.value || undefined })
              }
            >
              <option value="">(None - Raw Metric)</option>
              {Array.from(RULE_AGGREGATIONS).map((agg) => (
                <option key={agg} value={agg}>
                  {agg}
                </option>
              ))}
            </select>
          </div>

          {/* Metric */}
          <div>
            <label
              htmlFor="rule-metric"
              className="mb-1 block text-xs font-medium text-[var(--color-text-secondary)]"
            >
              Metric
            </label>
            <select
              id="rule-metric"
              className="w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg-base)] px-3 py-2 text-sm text-[var(--color-text-primary)] focus:border-[var(--color-accent)] focus:outline-hidden"
              value={value.metric}
              onChange={(e) => update({ metric: e.target.value })}
            >
              {metricsList.map((m) => (
                <option key={m} value={m}>
                  {m}
                </option>
              ))}
            </select>
          </div>

          {/* Operator */}
          <div>
            <label
              htmlFor="rule-operator"
              className="mb-1 block text-xs font-medium text-[var(--color-text-secondary)]"
            >
              Operator
            </label>
            <select
              id="rule-operator"
              className="w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg-base)] px-3 py-2 text-sm text-[var(--color-text-primary)] focus:border-[var(--color-accent)] focus:outline-hidden"
              value={value.operator}
              onChange={(e) => update({ operator: e.target.value })}
            >
              {SUPPORTED_OPERATORS.map((op) => (
                <option key={op} value={op}>
                  {op}
                </option>
              ))}
            </select>
          </div>

          {/* Threshold & Unit */}
          <div>
            <label
              htmlFor="rule-threshold"
              className="mb-1 block text-xs font-medium text-[var(--color-text-secondary)]"
            >
              Threshold & Unit
            </label>
            <div className="flex gap-2">
              <input
                id="rule-threshold"
                type="number"
                step="any"
                className="w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg-base)] px-3 py-2 text-sm text-[var(--color-text-primary)] focus:border-[var(--color-accent)] focus:outline-hidden"
                value={Number.isNaN(value.threshold) ? "" : value.threshold}
                onChange={(e) =>
                  update({ threshold: parseFloat(e.target.value) || 0 })
                }
              />
              <select
                aria-label="Unit"
                className="w-28 rounded-md border border-[var(--color-border)] bg-[var(--color-bg-base)] px-2 py-2 text-sm text-[var(--color-text-primary)] focus:border-[var(--color-accent)] focus:outline-hidden"
                value={value.unit || ""}
                onChange={(e) => update({ unit: e.target.value || undefined })}
              >
                <option value="">(None)</option>
                {Array.from(RULE_UNITS).map((u) => (
                  <option key={u} value={u}>
                    {u}
                  </option>
                ))}
              </select>
            </div>
          </div>
        </div>

        {/* Duration & Scoping */}
        <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-3">
          <div>
            <label
              htmlFor="rule-duration"
              className="mb-1 block text-xs font-medium text-[var(--color-text-secondary)]"
            >
              Duration (for)
            </label>
            <input
              id="rule-duration"
              type="text"
              placeholder="e.g. 5m, 1h"
              className="w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg-base)] px-3 py-2 text-sm text-[var(--color-text-primary)] focus:border-[var(--color-accent)] focus:outline-hidden"
              value={value.duration || ""}
              onChange={(e) =>
                update({ duration: e.target.value || undefined })
              }
            />
          </div>

          <div>
            <label
              htmlFor="rule-contract-id"
              className="mb-1 block text-xs font-medium text-[var(--color-text-secondary)]"
            >
              Contract ID Scope (Optional)
            </label>
            <input
              id="rule-contract-id"
              type="text"
              placeholder="e.g. CDLZFC3..."
              className="w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg-base)] px-3 py-2 text-sm text-[var(--color-text-primary)] focus:border-[var(--color-accent)] focus:outline-hidden"
              value={value.contractId || ""}
              onChange={(e) =>
                update({ contractId: e.target.value || undefined })
              }
            />
          </div>

          <div>
            <label
              htmlFor="rule-network"
              className="mb-1 block text-xs font-medium text-[var(--color-text-secondary)]"
            >
              Network Scope (Optional)
            </label>
            <select
              id="rule-network"
              className="w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg-base)] px-3 py-2 text-sm text-[var(--color-text-primary)] focus:border-[var(--color-accent)] focus:outline-hidden"
              value={value.network || ""}
              onChange={(e) => update({ network: e.target.value || undefined })}
            >
              <option value="">All Networks</option>
              <option value="testnet">testnet</option>
              <option value="mainnet">mainnet</option>
              <option value="futurenet">futurenet</option>
            </select>
          </div>
        </div>

        {/* Validation Feedback */}
        {validation?.errors && validation.errors.length > 0 && (
          <div className="mt-4 rounded-md border border-[var(--color-danger-border)] bg-[var(--color-danger-bg)] p-3">
            <h4 className="text-xs font-semibold text-[var(--color-danger-text)]">
              Validation Errors
            </h4>
            <ul className="mt-1 space-y-1 text-xs text-[var(--color-danger-text)]">
              {validation.errors.map((d, i) => (
                <li key={i}>{d.message}</li>
              ))}
            </ul>
          </div>
        )}
      </div>

      {/* Live Preview Panel */}
      <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-surface)] p-5 shadow-xs">
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-sm font-semibold tracking-wide text-[var(--color-text-secondary)] uppercase">
            Live Matches Preview
          </h3>
          {previewing && (
            <span className="text-xs text-[var(--color-text-secondary)] animate-pulse">
              Evaluating...
            </span>
          )}
        </div>

        {preview ? (
          <div>
            <div className="flex items-center gap-3 mb-2">
              <span
                className={`inline-flex items-center rounded px-2 py-0.5 text-xs font-medium ${preview.fired ? "bg-[var(--color-danger-bg)] text-[var(--color-danger-text)] border border-[var(--color-danger-border)]" : "bg-[var(--color-safe-bg)] text-[var(--color-safe)] border border-[var(--color-safe-border)]"}`}
              >
                {preview.fired
                  ? "Triggered (Rule Fired)"
                  : "Passing (Not Fired)"}
              </span>
              {preview.value !== null && preview.value !== undefined && (
                <span className="text-xs text-[var(--color-text-secondary)]">
                  Current Value:{" "}
                  <strong className="text-[var(--color-text-primary)]">
                    {preview.value}
                  </strong>
                </span>
              )}
              {preview.window && (
                <span className="text-xs text-[var(--color-text-secondary)]">
                  Window: {preview.window}
                </span>
              )}
            </div>

            {preview.points && preview.points.length > 0 ? (
              <div className="max-h-48 overflow-y-auto rounded-md border border-[var(--color-border)] bg-[var(--color-bg-base)] p-2">
                <ul className="space-y-1 text-xs">
                  {preview.points.map((p, i) => (
                    <li
                      key={i}
                      className="flex justify-between border-b border-[var(--color-border)]/50 pb-1 last:border-0"
                    >
                      <span className="font-mono text-[var(--color-text-secondary)]">
                        {p.at}
                      </span>
                      <span
                        className={
                          p.fired
                            ? "text-[var(--color-danger-text)] font-semibold"
                            : "text-[var(--color-text-primary)]"
                        }
                      >
                        {p.value !== null ? p.value : "null"}{" "}
                        {p.fired ? "(fired)" : ""}
                      </span>
                    </li>
                  ))}
                </ul>
              </div>
            ) : (
              <p className="text-xs text-[var(--color-text-secondary)] italic">
                {preview.reason || "Evaluated against latest indexer sample."}
              </p>
            )}
          </div>
        ) : (
          <p className="text-xs text-[var(--color-text-secondary)] italic">
            Enter a valid rule condition above to see live evaluation and
            matching points.
          </p>
        )}
      </div>
    </div>
  );
}
