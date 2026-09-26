import { trackContract, ApiError } from "@/lib/api";
import type { ContractSummary, TrackContractRequest } from "@/lib/types";

// Optimistic "track contract" for the contracts list (#197).
//
// This runs on plain React state for now. When the list moves to SWR (#196),
// only the body of trackContractOptimistically changes: it becomes
//   mutate(key, trackContract(input, userId).then(() => undefined), {
//     optimisticData: (list) => [buildOptimisticContract(input, network), ...list],
//     rollbackOnError: true,
//     populateCache: false,
//   })
// and the page keeps calling it with the same arguments and result shape.

/** A list row; `optimisticId` is set only while the API has not confirmed it. */
export interface ContractRow extends ContractSummary {
  optimisticId?: string;
}

export type TrackResult = { ok: true } | { ok: false; message: string };

const FALLBACK_MESSAGE = "Couldn't track contract. Please try again.";

let optimisticCounter = 0;

export function isPendingRow(row: ContractRow): boolean {
  return row.optimisticId !== undefined;
}

/** Stable React key: the temporary id while pending, the contract id after. */
export function contractRowKey(row: ContractRow): string {
  return row.optimisticId ?? row.id;
}

export function buildOptimisticContract(
  input: TrackContractRequest,
  network: string | undefined
): ContractRow {
  optimisticCounter += 1;
  return {
    id: input.id,
    network: network ?? "",
    label: input.label ?? null,
    // Matches the status the API assigns to a newly registered contract.
    status: "pending",
    wasm_hash: null,
    added_at: new Date().toISOString(),
    last_activity_at: null,
    optimisticId: `optimistic-${optimisticCounter}`,
  };
}

/**
 * Prepends an optimistic row to `list`, then registers the contract. On
 * failure, restores `list` exactly (unless `shouldRollback` says a newer
 * load already replaced it) and returns the message to show the user.
 * On success the caller refetches, which replaces the optimistic row.
 */
export async function trackContractOptimistically(
  list: ContractRow[],
  setList: (list: ContractRow[]) => void,
  input: TrackContractRequest,
  options: {
    userId: string;
    network?: string;
    shouldRollback?: () => boolean;
  }
): Promise<TrackResult> {
  const snapshot = list;
  setList([buildOptimisticContract(input, options.network), ...snapshot]);

  try {
    await trackContract(input, options.userId);
    return { ok: true };
  } catch (err) {
    if (options.shouldRollback?.() ?? true) setList(snapshot);
    const detail = err instanceof ApiError ? err.message : "";
    return {
      ok: false,
      message: detail ? `Couldn't track contract: ${detail}` : FALLBACK_MESSAGE,
    };
  }
}
