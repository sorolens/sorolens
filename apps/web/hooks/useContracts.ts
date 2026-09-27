"use client";

import useSWR from "swr";

import { getContract, listContracts } from "@/lib/api";
import type { ContractsListResponse } from "@/lib/types";

/**
 * Query accepted by {@link useContracts}. Mirrors the parameters of the
 * underlying `listContracts` client plus the sort options added for #359.
 */
export interface ContractsQuery {
  cursor?: string;
  limit?: number;
  network?: string;
  status?: string;
  tag?: string;
  sort?: string;
  dir?: "asc" | "desc";
}

/** Stable SWR cache key for a contracts-list query. */
export function contractsKey(query: ContractsQuery = {}) {
  return ["contracts", query] as const;
}

/**
 * SWR-backed wrapper around `listContracts`.
 *
 * Each distinct query (cursor, network, tag, …) is its own cache entry, so
 * moving between pages or filters is instant once a page has been seen, and
 * revisiting it revalidates in the background. Concurrent components asking
 * for the same query share a single request.
 */
export function useContracts(query: ContractsQuery = {}) {
  return useSWR<
    ContractsListResponse,
    Error,
    readonly ["contracts", ContractsQuery]
  >(contractsKey(query), ([, params]) => listContracts(params), {
    keepPreviousData: true,
  });
}

/** Stable SWR cache key for a single contract's detail payload. */
export function contractKey(id: string) {
  return ["contract", id] as const;
}

/**
 * SWR-backed wrapper around `getContract`. Passing a falsy `id` skips the
 * request (SWR's conditional fetching) instead of firing a request for an
 * empty resource.
 */
export function useContract(id: string | null | undefined) {
  return useSWR(
    id ? contractKey(id) : null,
    (key: readonly ["contract", string] | null) => {
      // SWR only runs the fetcher for a non-null key.
      const [, contractId] = key!;
      return getContract(contractId);
    },
    { keepPreviousData: true }
  );
}
