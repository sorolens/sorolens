"use client";

/**
 * "Hot contracts" leaderboard for the /live dashboard.
 *
 * Contracts are already ordered hottest-first by the API, so this component
 * only renders. Each row carries a sparkline of that contract's events per
 * minute over the window.
 */

import { MonoId } from "@sorolens/ui";
import { Sparkline, sparklineColor } from "./Sparkline";
import type { ContractEventRate } from "@/lib/types";

export interface HotContractsProps {
  contracts: ContractEventRate[];
  minutes: number;
}

/** Shortened contract id used when a contract has no label. */
function ContractName({ rate }: { rate: ContractEventRate }) {
  if (rate.label) {
    return <span className="truncate">{rate.label}</span>;
  }
  return (
    <span className="font-mono">
      <MonoId value={rate.contract_id} headChars={6} tailChars={4} />
    </span>
  );
}

export function HotContracts({ contracts, minutes }: HotContractsProps) {
  if (contracts.length === 0) {
    return (
      <p
        data-testid="hot-contracts-empty"
        className="px-4 py-8 text-center text-sm text-[var(--color-text-secondary)]"
      >
        No activity in the last {minutes} minutes.
      </p>
    );
  }

  // The hottest contract anchors the colour scale so a glance at the wall
  // shows which contract is currently doing the most work.
  const hottest = contracts[0]?.total ?? 0;

  return (
    <ol data-testid="hot-contracts" className="divide-y divide-[var(--color-border)]">
      {contracts.map((rate, i) => (
        <li
          key={rate.contract_id}
          data-testid="hot-contract-row"
          data-contract-id={rate.contract_id}
          className="flex items-center gap-3 px-4 py-2.5 text-sm"
        >
          <span className="w-5 shrink-0 text-right text-xs tabular-nums text-[var(--color-text-secondary)]">
            {i + 1}
          </span>

          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <ContractName rate={rate} />
              <span className="shrink-0 text-xs text-[var(--color-text-secondary)]">
                {rate.network}
              </span>
            </div>
          </div>

          <div className="w-24 shrink-0 sm:w-32">
            <Sparkline
              data={rate.per_minute}
              stroke={sparklineColor(rate.total, hottest)}
              label={`Events per minute for ${rate.label || rate.contract_id} over the last ${minutes} minutes`}
              testId="hot-contract-sparkline"
            />
          </div>

          <span className="w-10 shrink-0 text-right font-mono text-xs tabular-nums">
            {rate.total}
          </span>
        </li>
      ))}
    </ol>
  );
}
