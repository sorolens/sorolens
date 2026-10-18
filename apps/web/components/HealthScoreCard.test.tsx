/**
 * Tests for apps/web/components/HealthScoreCard.tsx (issue #627).
 *
 * The card fetches its score on mount; the API call is mocked so each test
 * drives one branch of the render state machine.
 */

// Registers jest-dom matchers with vitest, including their types.
import "@testing-library/jest-dom/vitest";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { HealthScoreCard } from "./HealthScoreCard";
import type { HealthScoreResponse } from "@/lib/types";

const getContractHealthScore = vi.fn();

vi.mock("@/lib/api", () => ({
  getContractHealthScore: (...args: unknown[]) =>
    getContractHealthScore(...args),
}));

function healthScore(overrides: Partial<HealthScoreResponse> = {}) {
  return {
    contract_id: "CAVRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C33",
    score: 92,
    components: {
      uptime: 99,
      error_rate: 95,
      performance: 88,
      storage_ttl: 90,
    },
    computed_at: "2026-07-03T08:00:00Z",
    ...overrides,
  } as HealthScoreResponse;
}

beforeEach(() => {
  vi.spyOn(console, "error").mockImplementation(() => {});
});

afterEach(() => {
  cleanup();
  getContractHealthScore.mockReset();
  vi.restoreAllMocks();
});

describe("HealthScoreCard", () => {
  it("renders nothing while the score is still loading", () => {
    getContractHealthScore.mockReturnValue(new Promise(() => {}));
    const { container } = render(<HealthScoreCard contractId="C1" />);

    expect(container).toBeEmptyDOMElement();
  });

  it("requests the score for the given contract", () => {
    getContractHealthScore.mockReturnValue(new Promise(() => {}));
    render(<HealthScoreCard contractId="C1" />);

    expect(getContractHealthScore).toHaveBeenCalledWith("C1");
  });

  it("renders the score and component breakdown, collapsed at first", async () => {
    getContractHealthScore.mockResolvedValue(healthScore());
    render(<HealthScoreCard contractId="C1" />);

    expect(await screen.findByText("Health Score")).toBeVisible();
    expect(screen.getByText("92")).toBeVisible();
    expect(screen.getByText("/100")).toBeVisible();
    for (const label of [
      "Uptime",
      "Error rate",
      "Performance",
      "Storage TTL",
    ]) {
      expect(screen.getByText(label)).toBeVisible();
    }

    const toggle = screen.getByRole("button");
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(screen.getByText("Show component breakdown")).toBeVisible();

    fireEvent.click(toggle);
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByText("Hide component breakdown")).toBeVisible();

    fireEvent.click(toggle);
    expect(toggle).toHaveAttribute("aria-expanded", "false");
  });

  it("marks a safe score with the safe border colour", async () => {
    getContractHealthScore.mockResolvedValue(healthScore({ score: 80 }));
    render(<HealthScoreCard contractId="C1" />);

    const header = (await screen.findByText("Health Score"))
      .parentElement as HTMLElement;
    expect(header.style.borderColor).toBe("var(--color-safe)");
  });

  it("marks a middling score with the warning border colour", async () => {
    getContractHealthScore.mockResolvedValue(healthScore({ score: 60 }));
    render(<HealthScoreCard contractId="C1" />);

    const header = (await screen.findByText("Health Score"))
      .parentElement as HTMLElement;
    expect(header.style.borderColor).toBe("var(--color-warning)");
  });

  it("marks a low score with the danger border colour", async () => {
    getContractHealthScore.mockResolvedValue(healthScore({ score: 10 }));
    render(<HealthScoreCard contractId="C1" />);

    const header = (await screen.findByText("Health Score"))
      .parentElement as HTMLElement;
    expect(header.style.borderColor).toBe("var(--color-danger)");
  });

  it("reports a not-yet-computed score as such", async () => {
    getContractHealthScore.mockRejectedValue(
      new Error("health score not found")
    );
    render(<HealthScoreCard contractId="C1" />);

    expect(
      await screen.findByText("Health score: Not yet computed")
    ).toBeVisible();
  });

  it("reports other failures as unavailable", async () => {
    getContractHealthScore.mockRejectedValue(new Error("boom"));
    render(<HealthScoreCard contractId="C1" />);

    expect(await screen.findByText("Health score: Unavailable")).toBeVisible();
  });

  it("reports a payload without components as unavailable", async () => {
    getContractHealthScore.mockResolvedValue({
      contract_id: "C1",
      score: 92,
      computed_at: "2026-07-03T08:00:00Z",
    });
    render(<HealthScoreCard contractId="C1" />);

    expect(await screen.findByText("Health score: Unavailable")).toBeVisible();
  });

  it("re-fetches when the contract changes", () => {
    getContractHealthScore.mockReturnValue(new Promise(() => {}));
    const { rerender } = render(<HealthScoreCard contractId="C1" />);
    rerender(<HealthScoreCard contractId="C2" />);

    expect(getContractHealthScore).toHaveBeenLastCalledWith("C2");
  });

  it("ignores a score that resolves after unmount", async () => {
    let resolveScore: (value: HealthScoreResponse) => void = () => {};
    getContractHealthScore.mockReturnValue(
      new Promise<HealthScoreResponse>((resolve) => {
        resolveScore = resolve;
      })
    );
    const { unmount } = render(<HealthScoreCard contractId="C1" />);
    unmount();

    // Must not throw or set state on the unmounted component.
    await act(async () => {
      resolveScore(healthScore());
    });
    expect(screen.queryByText("Health Score")).not.toBeInTheDocument();
  });
});
