"use client";

import { useState } from "react";
import { resolveLabel } from "@/lib/api";
import { MonoId } from "@sorolens/ui";

export function LabelledId({ value, knownLabel }: { value: string; knownLabel?: string | null }) {
  const [label, setLabel] = useState<string | null>(knownLabel ?? null);

  const loadLabel = () => {
    if (label) return;
    resolveLabel(value).then((result) => {
      if (result.label !== value) setLabel(result.label);
    }).catch(() => undefined);
  };

  return (
    <span title={value} onMouseEnter={loadLabel}>
      {label ?? <MonoId value={value} headChars={8} tailChars={8} />}
      {label && (
        <span className="sr-only">
          <MonoId value={value} headChars={8} tailChars={8} />
        </span>
      )}
    </span>
  );
}