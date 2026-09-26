import * as matchers from "@testing-library/jest-dom/matchers";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, it, expect } from "vitest";

expect.extend(matchers);

afterEach(() => {
  cleanup();
});
import {
  ContractChangelog,
  ContractVersionEntry,
} from "./ContractChangelog.js";

const CONTRACT_ID = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC";

const sampleEntries: ContractVersionEntry[] = [
  {
    id: 1,
    contract_id: CONTRACT_ID,
    wasm_hash:
      "aabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccddaabbccdd",
    first_seen_ledger: 100_000,
    tx_hash: "tx_genesis_hash_placeholder",
    recorded_at: "2026-07-01T10:00:00Z",
  },
  {
    id: 2,
    contract_id: CONTRACT_ID,
    wasm_hash:
      "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
    first_seen_ledger: 200_000,
    tx_hash: "tx_upgrade_hash_placeholder",
    verified_source_ref: "https://github.com/example/contract/tree/v2",
    recorded_at: "2026-08-15T14:30:00Z",
  },
];

describe("ContractChangelog", () => {
  it("renders all changelog entries", () => {
    render(
      <ContractChangelog contractId={CONTRACT_ID} entries={sampleEntries} />,
    );
    const entries = screen.getAllByTestId("changelog-entry");
    expect(entries).toHaveLength(2);
  });

  it("shows empty message when no entries", () => {
    render(<ContractChangelog contractId={CONTRACT_ID} entries={[]} />);
    expect(
      screen.getByText(/No Wasm hash transitions have been recorded/i),
    ).toBeInTheDocument();
  });

  it("marks the latest entry with a 'latest' badge", () => {
    render(
      <ContractChangelog contractId={CONTRACT_ID} entries={sampleEntries} />,
    );
    expect(screen.getByText("latest")).toBeInTheDocument();
  });

  it("displays truncated hash with ellipsis for each entry", () => {
    render(
      <ContractChangelog contractId={CONTRACT_ID} entries={sampleEntries} />,
    );
    // Both hashes should be truncated to 8 chars with "…" suffix.
    expect(screen.getByText(/^aabbccdd…/)).toBeInTheDocument();
    expect(screen.getByText(/^deadbeef…/)).toBeInTheDocument();
  });

  it("renders Atom feed link with correct href", () => {
    render(
      <ContractChangelog
        contractId={CONTRACT_ID}
        entries={sampleEntries}
        apiBase="https://api.sorolens.xyz"
      />,
    );
    const feedLink = screen.getByRole("link", { name: /Atom feed/i });
    expect(feedLink).toHaveAttribute(
      "href",
      `https://api.sorolens.xyz/contracts/${CONTRACT_ID}/changelog/feed`,
    );
  });

  it("renders verified source ref as a link when present", () => {
    render(
      <ContractChangelog contractId={CONTRACT_ID} entries={sampleEntries} />,
    );
    const sourceLink = screen.getByRole("link", {
      name: /github\.com\/example\/contract/i,
    });
    expect(sourceLink).toHaveAttribute(
      "href",
      "https://github.com/example/contract/tree/v2",
    );
  });

  it("does not render verified source section when absent", () => {
    const singleEntry: ContractVersionEntry[] = [sampleEntries[0]];
    render(
      <ContractChangelog contractId={CONTRACT_ID} entries={singleEntry} />,
    );
    expect(screen.queryByText("Verified source")).not.toBeInTheDocument();
  });
});
