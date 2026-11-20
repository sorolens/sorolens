/**
 * Data layer for the contract page's Tokens tab (issue #834).
 *
 * The transfers and holders endpoints are specified by issue #801 and are not
 * in docs/openapi.yaml yet, so the response shapes below are provisional.
 * Until the API serves them `hasTokenActivity` resolves to false and the tab
 * stays hidden.
 */
import { API_URL } from "./config";

export type TokenTransferKind = "transfer" | "mint" | "burn";

export interface TokenTransfer {
  id: string;
  kind: TokenTransferKind;
  /** Null for mints. */
  from: string | null;
  /** Null for burns. */
  to: string | null;
  /** Raw integer amount; i128 values do not fit in a JS number. */
  amount: string;
  tx_hash: string;
  ledger: number;
  ledger_closed_at: string;
}

export interface TokenHolder {
  address: string;
  /** Raw integer balance. */
  balance: string;
}

export interface TokenTransfersPage {
  transfers: TokenTransfer[];
  next_cursor: string;
  decimals?: number;
}

export interface TokenHoldersPage {
  holders: TokenHolder[];
  next_cursor: string;
  decimals?: number;
  total_supply?: string;
}

export interface HolderShare {
  label: string;
  share: number;
}

/** SEP-41 tokens on Stellar conventionally use seven decimals. */
export const DEFAULT_TOKEN_DECIMALS = 7;

async function fetchTokenPage<T>(
  contractId: string,
  resource: "transfers" | "holders",
  params: { cursor?: string; limit?: number }
): Promise<T> {
  const query = new URLSearchParams();
  if (params.limit) query.set("limit", String(params.limit));
  if (params.cursor) query.set("cursor", params.cursor);
  const qs = query.toString();
  const res = await fetch(
    `${API_URL}/api/v1/contracts/${encodeURIComponent(contractId)}/${resource}${qs ? `?${qs}` : ""}`
  );
  if (!res.ok) {
    throw new Error(`The ${resource} request failed (HTTP ${res.status}).`);
  }
  return res.json();
}

export function getTokenTransfers(
  contractId: string,
  params: { cursor?: string; limit?: number } = {}
): Promise<TokenTransfersPage> {
  return fetchTokenPage(contractId, "transfers", params);
}

export function getTokenHolders(
  contractId: string,
  params: { cursor?: string; limit?: number } = {}
): Promise<TokenHoldersPage> {
  return fetchTokenPage(contractId, "holders", params);
}

/** True only when the API reports at least one transfer for the contract. */
export async function hasTokenActivity(contractId: string): Promise<boolean> {
  try {
    const page = await getTokenTransfers(contractId, { limit: 1 });
    return Array.isArray(page.transfers) && page.transfers.length > 0;
  } catch {
    return false;
  }
}

/** Formats a raw integer amount using the token's decimals. */
export function formatTokenAmount(raw: string, decimals: number): string {
  if (!/^-?\d+$/.test(raw)) return raw;
  const negative = raw.startsWith("-");
  const digits = (negative ? raw.slice(1) : raw).padStart(decimals + 1, "0");
  const split = digits.length - decimals;
  const whole = BigInt(digits.slice(0, split)).toLocaleString("en-US");
  const fraction = digits.slice(split).replace(/0+$/, "");
  return `${negative ? "-" : ""}${whole}${fraction ? `.${fraction}` : ""}`;
}

/** Parses a human-entered amount into raw units; null when it is not valid. */
export function parseTokenAmount(
  input: string,
  decimals: number
): bigint | null {
  const match = /^(\d*)(?:\.(\d*))?$/.exec(input.trim().replace(/,/g, ""));
  if (!match || (!match[1] && !match[2])) return null;
  const fraction = match[2] ?? "";
  if (fraction.length > decimals) return null;
  return BigInt(`${match[1] || "0"}${fraction.padEnd(decimals, "0")}`);
}

/** Percentage of `total` held by `balance`, to two decimal places. */
export function shareOfSupply(balance: string, total: bigint): number {
  if (total <= BigInt(0)) return 0;
  return Number((BigInt(balance) * BigInt(10000)) / total) / 100;
}

/** The supply figure shares are measured against. */
export function supplyTotal(
  holders: TokenHolder[],
  totalSupply?: string
): bigint {
  if (totalSupply && /^\d+$/.test(totalSupply)) return BigInt(totalSupply);
  return holders.reduce((sum, h) => sum + BigInt(h.balance), BigInt(0));
}

export function filterTransfers(
  transfers: TokenTransfer[],
  filters: { address: string; minAmount: bigint | null }
): TokenTransfer[] {
  const address = filters.address.trim().toUpperCase();
  return transfers.filter((t) => {
    if (
      address &&
      !t.from?.toUpperCase().includes(address) &&
      !t.to?.toUpperCase().includes(address)
    ) {
      return false;
    }
    return filters.minAmount === null || BigInt(t.amount) >= filters.minAmount;
  });
}

export function filterHolders(
  holders: TokenHolder[],
  filters: { address: string; minAmount: bigint | null }
): TokenHolder[] {
  const address = filters.address.trim().toUpperCase();
  return holders.filter((h) => {
    if (address && !h.address.toUpperCase().includes(address)) return false;
    return filters.minAmount === null || BigInt(h.balance) >= filters.minAmount;
  });
}

/** Top holders by balance, with everyone else rolled into "Others". */
export function holderDistribution(
  holders: TokenHolder[],
  total: bigint,
  top = 10
): HolderShare[] {
  const sorted = [...holders].sort((a, b) => {
    const diff = BigInt(b.balance) - BigInt(a.balance);
    return diff > BigInt(0) ? 1 : diff < BigInt(0) ? -1 : 0;
  });
  const shares = sorted.slice(0, top).map((h) => ({
    label: `${h.address.slice(0, 4)}…${h.address.slice(-4)}`,
    share: shareOfSupply(h.balance, total),
  }));
  const held = shares.reduce((sum, s) => sum + s.share, 0);
  const others = Math.round((100 - held) * 100) / 100;
  return others > 0 && total > BigInt(0)
    ? [...shares, { label: "Others", share: others }]
    : shares;
}

/**
 * Quotes a CSV cell. Addresses and hashes come from the chain, so a leading
 * formula character is neutralised before a spreadsheet can evaluate it.
 */
function csvCell(value: string | number): string {
  const text = String(value);
  const safe = /^[=+\-@\t\r]/.test(text) ? `'${text}` : text;
  return /[",\n\r]/.test(safe) ? `"${safe.replace(/"/g, '""')}"` : safe;
}

function toCsv(rows: (string | number)[][]): string {
  return rows.map((row) => row.map(csvCell).join(",")).join("\n");
}

export function transfersToCsv(
  transfers: TokenTransfer[],
  decimals: number
): string {
  return toCsv([
    ["kind", "from", "to", "amount", "tx_hash", "ledger", "ledger_closed_at"],
    ...transfers.map((t) => [
      t.kind,
      t.from ?? "",
      t.to ?? "",
      formatTokenAmount(t.amount, decimals).replace(/,/g, ""),
      t.tx_hash,
      t.ledger,
      t.ledger_closed_at,
    ]),
  ]);
}

export function holdersToCsv(
  holders: TokenHolder[],
  decimals: number,
  total: bigint
): string {
  return toCsv([
    ["address", "balance", "share_of_supply_pct"],
    ...holders.map((h) => [
      h.address,
      formatTokenAmount(h.balance, decimals).replace(/,/g, ""),
      shareOfSupply(h.balance, total),
    ]),
  ]);
}
