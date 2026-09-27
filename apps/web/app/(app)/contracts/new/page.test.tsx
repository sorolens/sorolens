/**
 * Tests for apps/web/app/(app)/contracts/new/page.tsx — the tracking wizard
 * (issue #140).
 *
 * vitest + @testing-library/react + jsdom. next/link and next/navigation are
 * mocked so we can assert on hrefs and on the post-create redirect without a
 * Next.js runtime, and @/lib/api is mocked so every validation branch is
 * reachable.
 */

import * as matchers from "@testing-library/jest-dom/matchers";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import React from "react";

expect.extend(matchers);

// ── Mocks ───────────────────────────────────────────────────────────────────

const { push } = vi.hoisted(() => ({ push: vi.fn() }));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push }),
}));

vi.mock("next/link", () => ({
  default: ({
    children,
    href,
    ...rest
  }: React.AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

const mockValidateContract = vi.fn();
const mockTrackContract = vi.fn();

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    validateContract: (...args: unknown[]) => mockValidateContract(...args),
    trackContract: (...args: unknown[]) => mockTrackContract(...args),
  };
});

import NewContractPage from "./page";
import { ApiError } from "@/lib/api";

// A well-formed id: exactly 56 characters, leading 'C', A-Z0-9 only.
const VALID_ID = "CAVRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5";

function validResponse(overrides: Record<string, unknown> = {}) {
  return {
    valid: true,
    contract_id: VALID_ID,
    network: "testnet",
    already_tracked: false,
    label: null,
    reason: null,
    ...overrides,
  };
}

/** Walks steps 1 and 2 with a valid id so tests can start at step 3. */
async function advanceToConfigure() {
  fireEvent.change(screen.getByTestId("wizard-contract-id"), {
    target: { value: VALID_ID },
  });
  fireEvent.click(screen.getByTestId("wizard-next"));
  await waitFor(() => expect(screen.getByTestId("wizard-valid")).toBeDefined());
  fireEvent.click(screen.getByTestId("wizard-next"));
  await waitFor(() =>
    expect(screen.getByTestId("wizard-step-configure")).toBeDefined()
  );
}

beforeEach(() => {
  push.mockReset();
  mockValidateContract.mockReset();
  mockTrackContract.mockReset();
  mockValidateContract.mockResolvedValue(validResponse());
  mockTrackContract.mockResolvedValue({
    id: VALID_ID,
    network: "testnet",
    label: null,
    status: "pending",
    wasm_hash: null,
    added_at: "2026-07-02T10:15:00Z",
  });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

// ── Step 1: identify ────────────────────────────────────────────────────────

describe("Wizard step 1 (identify)", () => {
  it("renders the contract id and network inputs", () => {
    render(<NewContractPage />);
    expect(screen.getByLabelText(/contract id/i)).toBeDefined();
    expect(screen.getByLabelText(/network/i)).toBeDefined();
  });

  it("disables Next until a well-formed contract id is entered", () => {
    render(<NewContractPage />);
    const next = screen.getByTestId("wizard-next") as HTMLButtonElement;
    expect(next.disabled).toBe(true);

    fireEvent.change(screen.getByTestId("wizard-contract-id"), {
      target: { value: VALID_ID },
    });
    expect(
      (screen.getByTestId("wizard-next") as HTMLButtonElement).disabled
    ).toBe(false);
  });

  it("shows a format error and keeps Next disabled for a malformed id", () => {
    render(<NewContractPage />);
    fireEvent.change(screen.getByTestId("wizard-contract-id"), {
      target: { value: "NOT-A-CONTRACT" },
    });

    expect(screen.getByTestId("wizard-contract-id-error")).toBeDefined();
    expect(
      (screen.getByTestId("wizard-next") as HTMLButtonElement).disabled
    ).toBe(true);
  });

  it("disables Back on the first step", () => {
    render(<NewContractPage />);
    expect(
      (screen.getByTestId("wizard-back") as HTMLButtonElement).disabled
    ).toBe(true);
  });
});

// ── Step 2: validate ────────────────────────────────────────────────────────

describe("Wizard step 2 (validate)", () => {
  it("calls the API and enables Next for a valid contract", async () => {
    render(<NewContractPage />);
    fireEvent.change(screen.getByTestId("wizard-contract-id"), {
      target: { value: VALID_ID },
    });
    fireEvent.click(screen.getByTestId("wizard-next"));

    await waitFor(() =>
      expect(mockValidateContract).toHaveBeenCalledWith({
        contract_id: VALID_ID,
        network: "testnet",
      })
    );
    await waitFor(() =>
      expect(screen.getByTestId("wizard-valid")).toBeDefined()
    );
    expect(
      (screen.getByTestId("wizard-next") as HTMLButtonElement).disabled
    ).toBe(false);
  });

  it("shows the API reason and blocks Next for an invalid contract", async () => {
    mockValidateContract.mockResolvedValue(
      validResponse({
        valid: false,
        reason:
          "contract id checksum does not match (check for a mistyped character)",
      })
    );

    render(<NewContractPage />);
    fireEvent.change(screen.getByTestId("wizard-contract-id"), {
      target: { value: VALID_ID },
    });
    fireEvent.click(screen.getByTestId("wizard-next"));

    await waitFor(() =>
      expect(screen.getByTestId("wizard-invalid")).toBeDefined()
    );
    expect(screen.getByTestId("wizard-invalid").textContent).toContain(
      "checksum"
    );
    expect(
      (screen.getByTestId("wizard-next") as HTMLButtonElement).disabled
    ).toBe(true);
  });

  it("points at the existing contract when already tracked", async () => {
    mockValidateContract.mockResolvedValue(
      validResponse({ already_tracked: true, label: "Escrow DEX" })
    );

    render(<NewContractPage />);
    fireEvent.change(screen.getByTestId("wizard-contract-id"), {
      target: { value: VALID_ID },
    });
    fireEvent.click(screen.getByTestId("wizard-next"));

    await waitFor(() =>
      expect(screen.getByTestId("wizard-already-tracked")).toBeDefined()
    );
    expect(screen.getByTestId("wizard-already-tracked").textContent).toContain(
      "Escrow DEX"
    );
    // Creating a duplicate is blocked.
    expect(
      (screen.getByTestId("wizard-next") as HTMLButtonElement).disabled
    ).toBe(true);
  });

  it("treats a network failure as a validation failure", async () => {
    mockValidateContract.mockRejectedValue(new ApiError(500, "boom"));

    render(<NewContractPage />);
    fireEvent.change(screen.getByTestId("wizard-contract-id"), {
      target: { value: VALID_ID },
    });
    fireEvent.click(screen.getByTestId("wizard-next"));

    await waitFor(() =>
      expect(screen.getByTestId("wizard-invalid")).toBeDefined()
    );
    expect(
      (screen.getByTestId("wizard-next") as HTMLButtonElement).disabled
    ).toBe(true);
  });

  it("preserves the contract id when going Back", async () => {
    render(<NewContractPage />);
    fireEvent.change(screen.getByTestId("wizard-contract-id"), {
      target: { value: VALID_ID },
    });
    fireEvent.click(screen.getByTestId("wizard-next"));
    await waitFor(() =>
      expect(screen.getByTestId("wizard-valid")).toBeDefined()
    );

    fireEvent.click(screen.getByTestId("wizard-back"));

    const input = screen.getByTestId("wizard-contract-id") as HTMLInputElement;
    expect(input.value).toBe(VALID_ID);
  });
});

// ── Step 3: configure and create ────────────────────────────────────────────

describe("Wizard step 3 (configure)", () => {
  it("creates the contract and redirects to its detail page", async () => {
    render(<NewContractPage />);
    await advanceToConfigure();

    fireEvent.change(screen.getByTestId("wizard-label"), {
      target: { value: "Escrow DEX" },
    });
    fireEvent.click(screen.getByTestId("wizard-create"));

    await waitFor(() =>
      expect(mockTrackContract).toHaveBeenCalledWith(
        { id: VALID_ID, label: "Escrow DEX", network: "testnet" },
        ""
      )
    );
    await waitFor(() =>
      expect(push).toHaveBeenCalledWith(`/contracts/${VALID_ID}`)
    );
  });

  it("omits an empty label rather than sending an empty string", async () => {
    render(<NewContractPage />);
    await advanceToConfigure();
    fireEvent.click(screen.getByTestId("wizard-create"));

    await waitFor(() =>
      expect(mockTrackContract).toHaveBeenCalledWith(
        { id: VALID_ID, label: undefined, network: "testnet" },
        ""
      )
    );
  });

  it("routes to the Watchdog page when enrollment was requested", async () => {
    render(<NewContractPage />);
    await advanceToConfigure();

    fireEvent.click(screen.getByTestId("wizard-watchdog"));
    fireEvent.click(screen.getByTestId("wizard-create"));

    await waitFor(() => expect(push).toHaveBeenCalledWith("/watchdog"));
  });

  it("shows the API error and stays on the step when creation fails", async () => {
    mockTrackContract.mockRejectedValue(
      new ApiError(409, "contract already being tracked")
    );

    render(<NewContractPage />);
    await advanceToConfigure();

    fireEvent.click(screen.getByTestId("wizard-create"));

    await waitFor(() =>
      expect(screen.getByTestId("wizard-create-error")).toBeDefined()
    );
    expect(screen.getByTestId("wizard-create-error").textContent).toContain(
      "already being tracked"
    );
    expect(push).not.toHaveBeenCalled();
  });

  it("rejects an over-long label before calling the API", async () => {
    render(<NewContractPage />);
    await advanceToConfigure();

    fireEvent.change(screen.getByTestId("wizard-label"), {
      target: { value: "x".repeat(65) },
    });

    expect(screen.getByTestId("wizard-label-error")).toBeDefined();
    expect(
      (screen.getByTestId("wizard-create") as HTMLButtonElement).disabled
    ).toBe(true);
  });
});
