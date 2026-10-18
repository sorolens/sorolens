import type { StatsResponse } from "@/lib/types";

export interface ContractOgCard {
  alt: string;
  title: string;
  subtitle: string;
  shortId: string;
  stats: { label: string; value: string }[];
}

export function shortContractId(id: string): string {
  return id.length > 12 ? `${id.slice(0, 6)}…${id.slice(-4)}` : id;
}

export function formatCount(value: number): string {
  if (!Number.isFinite(value)) return "0";
  const abs = Math.abs(value);
  if (abs >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`;
  if (abs >= 1_000) return `${(value / 1_000).toFixed(1)}K`;
  return String(value);
}

/**
 * Builds the copy shown on a per-contract social preview image. Pure so the
 * fallback path (no contract, no stats) is unit-testable without rendering.
 */
export function contractOgCard(input: {
  id: string;
  label?: string | null;
  network?: string | null;
  stats?: StatsResponse | null;
}): ContractOgCard {
  const shortId = shortContractId(input.id);
  const title = input.label?.trim() || `Contract ${shortId}`;
  const subtitle = input.network
    ? `${input.network} · Soroban contract`
    : "Soroban contract";
  const stats = input.stats
    ? [
        { label: "Events", value: formatCount(input.stats.event_count) },
        {
          label: "Invocations",
          value: formatCount(input.stats.invocation_count),
        },
        { label: "Storage", value: formatCount(input.stats.storage_count) },
      ]
    : [];

  return {
    alt: `Sorolens preview for ${title} on ${input.network || "Stellar"}`,
    title,
    subtitle,
    shortId,
    stats,
  };
}
