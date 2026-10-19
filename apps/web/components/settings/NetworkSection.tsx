"use client";

import { SettingsCard, SettingsRow, type SaveState } from "./SettingsCard";
import { ALL_NETWORKS, NETWORKS } from "@/lib/network";

/**
 * Default-network card. Saving stores the choice and switches the header
 * selector, so the filter changes immediately rather than only next visit.
 */
export function NetworkSection({
  value,
  status,
  onChange,
  onSave,
}: {
  value: string;
  status: SaveState;
  onChange: (value: string) => void;
  onSave: () => boolean;
}) {
  return (
    <SettingsCard
      testId="settings-network"
      title="Default network"
      description="The network the dashboard filters by when a new session starts."
      status={status}
      onSave={onSave}
    >
      <SettingsRow
        htmlFor="settings-default-network"
        title="Default network"
        description="Applies to the contracts, events, and watchdog lists."
      >
        <select
          id="settings-default-network"
          value={value}
          onChange={(event) => onChange(event.target.value)}
          className="rounded-md border border-[var(--color-border)] bg-[var(--color-bg-page)] px-2 py-1 text-sm text-[var(--color-text-primary)] focus:border-[var(--color-accent)] focus:outline-none"
        >
          {NETWORKS.map((network) => (
            <option key={network} value={network}>
              {network === ALL_NETWORKS ? "All networks" : network}
            </option>
          ))}
        </select>
      </SettingsRow>
    </SettingsCard>
  );
}
