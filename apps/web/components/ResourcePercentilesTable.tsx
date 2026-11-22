import type { PercentileValues, ResourcePercentiles } from "@/lib/types";

interface Props {
  percentiles?: ResourcePercentiles;
  /** Invocations in the window; the percentiles are all zero without any. */
  invocationCount: number;
  windowLabel: string;
}

const METRICS: {
  key: keyof ResourcePercentiles;
  label: string;
  unit: string;
}[] = [
  { key: "cpu_insn", label: "CPU", unit: "instructions" },
  { key: "mem_byte", label: "Memory", unit: "bytes" },
  { key: "fee", label: "Fee", unit: "stroops" },
];

const COLUMNS: (keyof PercentileValues)[] = ["p50", "p95", "p99"];

function formatValue(value: number): string {
  return Math.round(value).toLocaleString();
}

export function ResourcePercentilesTable({
  percentiles,
  invocationCount,
  windowLabel,
}: Props) {
  if (!percentiles || invocationCount === 0) return null;

  return (
    <section aria-labelledby="resource-percentiles-heading">
      <h2
        id="resource-percentiles-heading"
        className="mb-4 text-xl font-semibold"
      >
        Resource percentiles
      </h2>
      <div className="overflow-x-auto rounded-lg bg-[var(--color-bg-card)]">
        <table className="w-full text-sm">
          <caption className="sr-only">
            Resource usage percentiles per invocation over {windowLabel}
          </caption>
          <thead>
            <tr className="border-b border-[var(--color-border)] text-left text-xs text-[var(--color-text-secondary)]">
              <th scope="col" className="px-4 py-3 font-medium">
                Metric
              </th>
              {COLUMNS.map((column) => (
                <th
                  key={column}
                  scope="col"
                  className="px-4 py-3 text-right font-medium"
                >
                  {column}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {METRICS.map((metric) => (
              <tr
                key={metric.key}
                className="border-b border-[var(--color-border)] last:border-b-0"
              >
                <th scope="row" className="px-4 py-3 text-left font-medium">
                  {metric.label}{" "}
                  <span className="text-xs font-normal text-[var(--color-text-secondary)]">
                    {metric.unit}
                  </span>
                </th>
                {COLUMNS.map((column) => (
                  <td
                    key={column}
                    className="px-4 py-3 text-right font-mono text-xs"
                  >
                    {formatValue(percentiles[metric.key][column])}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
