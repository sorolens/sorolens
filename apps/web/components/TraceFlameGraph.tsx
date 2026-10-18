"use client";

import type { TraceNode } from "@/lib/types";

interface TraceFlameGraphProps {
  root: TraceNode;
  hasEdges?: boolean;
  truncated?: boolean;
}

/**
 * The metric the frame widths are proportional to. CPU instructions are used
 * when the host emitted core_metrics for any frame (consistent units across the
 * tree); otherwise the fee each top-level frame was charged. When neither is
 * known every frame gets an equal-width bar.
 */
type Metric = "cpu" | "fee_share" | "none";

function hasPositive(node: TraceNode, key: "cpu" | "fee_share"): boolean {
  if (node[key] > 0) return true;
  return node.children.some((child) => hasPositive(child, key));
}

function pickMetric(root: TraceNode): Metric {
  if (hasPositive(root, "cpu")) return "cpu";
  if (hasPositive(root, "fee_share")) return "fee_share";
  return "none";
}

function metricValue(node: TraceNode, metric: Metric): number {
  if (metric === "cpu") return node.cpu;
  if (metric === "fee_share") return node.fee_share;
  return 1;
}

function maxMetric(node: TraceNode, metric: Metric): number {
  return node.children.reduce(
    (max, child) => Math.max(max, maxMetric(child, metric)),
    metricValue(node, metric)
  );
}

function frameLabel(node: TraceNode): string {
  if (node.function_name) return node.function_name;
  if (node.contract_id) return "contract call";
  return node.span_id === "0" ? "transaction" : "call";
}

function metricLabel(node: TraceNode, metric: Metric): string {
  if (metric === "cpu" && node.cpu > 0) {
    return `${node.cpu.toLocaleString()} cpu`;
  }
  if (metric === "fee_share" && node.fee_share > 0) {
    return `${node.fee_share.toLocaleString()} fee`;
  }
  return "";
}

// Theme-aware categorical palette (--chart-N in globals.css), one shade per
// depth, so nested frames read like a flame graph in BOTH themes (issue #609).
const DEPTH_COLORS = [
  "bg-[var(--chart-3)]",
  "bg-[var(--chart-5)]",
  "bg-[var(--chart-6)]",
  "bg-[var(--chart-4)]",
  "bg-[var(--chart-1)]",
  "bg-[var(--chart-2)]",
];

function FlameFrame({
  node,
  metric,
  max,
}: {
  node: TraceNode;
  metric: Metric;
  max: number;
}) {
  const value = metricValue(node, metric);
  // Keep a floor so a frame with zero attributed resources is still visible.
  const width =
    max > 0 && value > 0 ? Math.max(8, Math.round((value / max) * 100)) : 100;
  const color = DEPTH_COLORS[node.depth % DEPTH_COLORS.length];
  const label = metricLabel(node, metric);
  const isRoot = node.depth === 0;

  return (
    <div
      data-testid="trace-frame"
      data-span-id={node.span_id}
      data-depth={node.depth}
    >
      <div
        className="flex items-center"
        style={{ paddingLeft: `${node.depth * 16}px` }}
      >
        <div
          className={`flex h-8 items-center gap-2 overflow-hidden rounded px-2 text-xs ${color} ${
            isRoot ? "ring-1 ring-[var(--color-warning)]" : ""
          }`}
          style={{ width: `${width}%` }}
          title={`${frameLabel(node)} (${node.span_id})`}
        >
          <span className="truncate font-medium text-[var(--color-on-chart)]">
            {frameLabel(node)}
          </span>
          {node.contract_id && (
            <span className="truncate font-mono text-[10px] text-[var(--color-on-chart)]">
              {node.contract_id.slice(0, 8)}…{node.contract_id.slice(-4)}
            </span>
          )}
          {label && (
            <span className="ml-auto shrink-0 font-mono text-[10px] text-[var(--color-on-chart)]">
              {label}
            </span>
          )}
        </div>
      </div>
      {node.children.map((child) => (
        <FlameFrame
          key={child.span_id}
          node={child}
          metric={metric}
          max={max}
        />
      ))}
    </div>
  );
}

/**
 * Renders a transaction's cross-contract call tree as a flame-graph-style
 * stack: each frame is indented by call depth and its bar is proportional to
 * the frame's CPU (or fee) relative to the widest frame in the tree.
 */
export function TraceFlameGraph({
  root,
  hasEdges,
  truncated,
}: TraceFlameGraphProps) {
  const metric = pickMetric(root);
  const max = maxMetric(root, metric);

  return (
    <div className="rounded-lg bg-[var(--color-bg-card)] p-4">
      <div className="mb-3 flex flex-wrap items-center gap-3 text-xs text-[var(--color-text-secondary)]">
        <span data-testid="flamegraph-metric">
          {metric === "cpu"
            ? "Bar width ∝ CPU instructions"
            : metric === "fee_share"
              ? "Bar width ∝ fee share"
              : "Equal-width frames (no resource metrics)"}
        </span>
        {hasEdges === false && (
          <span data-testid="flamegraph-empty">
            No cross-contract calls recorded for this transaction
          </span>
        )}
        {truncated && (
          <span
            className="rounded-full bg-[var(--color-warning-bg)] px-2 py-0.5 font-medium text-[var(--color-warning)]"
            data-testid="flamegraph-truncated"
          >
            Truncated - some frames were dropped
          </span>
        )}
      </div>
      <div data-testid="trace-flamegraph">
        <FlameFrame node={root} metric={metric} max={max} />
      </div>
    </div>
  );
}
