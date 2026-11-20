import { useState, useEffect, useCallback } from "react";
import { API_URL } from "../lib/config";

export type StorageDurability = "temporary" | "persistent" | "instance";

export interface StorageDiffEntry {
  key: string;
  durability: StorageDurability;
  live_until_ledger?: number;
  val?: string;
  val_decoded?: {
    type: string;
    value: any;
    human: string;
  };
}

export interface StorageDiffChange {
  key: string;
  durability: StorageDurability;
  old_val?: string;
  old_val_decoded?: {
    type: string;
    value: any;
    human: string;
  };
  new_val?: string;
  new_val_decoded?: {
    type: string;
    value: any;
    human: string;
  };
  old_live_until?: number;
  new_live_until?: number;
}

export interface StorageDiffResult {
  contract_id: string;
  from_ledger: number;
  to_ledger: number;
  created: StorageDiffEntry[];
  changed: StorageDiffChange[];
  removed: StorageDiffEntry[];
  expired: StorageDiffEntry[];
}

export interface UseStorageDiffOptions {
  contractId: string;
  from?: number;
  to?: number;
  prefix?: string;
  durability?: StorageDurability | "";
}

export function useStorageDiff({
  contractId,
  from,
  to,
  prefix,
  durability,
}: UseStorageDiffOptions) {
  const [data, setData] = useState<StorageDiffResult | null>(null);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  const fetchDiff = useCallback(async () => {
    if (!contractId || from === undefined || to === undefined) {
      setData(null);
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const query = new URLSearchParams();
      query.set("from", String(from));
      query.set("to", String(to));
      if (prefix) query.set("prefix", prefix);
      if (durability) query.set("durability", durability);

      const res = await fetch(
        `${API_URL}/api/v1/contracts/${encodeURIComponent(contractId)}/storage/diff?${query.toString()}`
      );
      if (!res.ok) {
        const errBody = await res.text();
        throw new Error(errBody || `Failed to fetch storage diff (HTTP ${res.status})`);
      }
      const json = await res.json();
      setData(json);
    } catch (err: any) {
      setError(err.message || "An unexpected error occurred");
    } finally {
      setLoading(false);
    }
  }, [contractId, from, to, prefix, durability]);

  useEffect(() => {
    fetchDiff();
  }, [fetchDiff]);

  return { data, loading, error, refetch: fetchDiff };
}
