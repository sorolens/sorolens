"use client";

import { useEffect, useState } from "react";
import { getContractVerification } from "@/lib/api";
import type { ContractVerification } from "@/lib/types";

interface VerifiedBadgeProps {
  contractId: string;
}

/**
 * Shows a `Verified` pill when the backend has a matching source verification
 * for the contract (issue #263). Renders nothing while the verdict is unknown,
 * pending, or failed, so unverified contracts look exactly as they did before.
 */
export function VerifiedBadge({ contractId }: VerifiedBadgeProps) {
  const [verification, setVerification] = useState<ContractVerification | null>(
    null,
  );

  useEffect(() => {
    let cancelled = false;
    getContractVerification(contractId)
      .then((data) => {
        if (!cancelled) setVerification(data);
      })
      .catch(() => {
        // A contract that has never been submitted 404s; treat that as
        // "no badge" rather than an error.
      });
    return () => {
      cancelled = true;
    };
  }, [contractId]);

  if (!verification || verification.status !== "verified") {
    return null;
  }

  const hash = verification.compiled_wasm_hash ?? "";

  return (
    <span
      title={`Source verified: the rebuilt Wasm hash matches the on-chain hash${
        hash ? ` (${hash})` : ""
      }`}
      className="inline-block rounded-full bg-emerald-900/40 px-2.5 py-0.5 text-xs font-medium text-emerald-400"
    >
      Verified
    </span>
  );
}
