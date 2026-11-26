/**
 * Tests for the label registry page (issue #590): browsing, filtering,
 * validation, creation and error handling. The API module is mocked so no
 * request leaves the test.
 */

import "@testing-library/jest-dom/vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { LabelEntry } from "@/lib/types";

const mockListLabels = vi.fn();
const mockCreateLabel = vi.fn();

vi.mock("@/lib/api", () => ({
  listLabels: (...args: unknown[]) => mockListLabels(...args),
  createLabel: (...args: unknown[]) => mockCreateLabel(...args),
}));

vi.mock("@/lib/useSession", () => ({
  useSessionUserId: () => "user-1",
}));

function label(overrides: Partial<LabelEntry> = {}): LabelEntry {
  return {
    label: "Stellar DEX",
    value: "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC",
    scope: "public",
    ...overrides,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  mockListLabels.mockResolvedValue({ labels: [label()] });
  mockCreateLabel.mockResolvedValue(label({ scope: "workspace" }));
});

afterEach(() => {
  cleanup();
});

async function renderPage() {
  const { default: LabelsPage } = await import("@/app/(app)/labels/page");
  render(<LabelsPage />);
  await screen.findAllByTestId("label-row");
}

describe("Label registry page", () => {
  it("lists labels with their scope", async () => {
    await renderPage();

    const row = screen.getByTestId("label-row");
    expect(row).toHaveTextContent("Stellar DEX");
    expect(row).toHaveTextContent(label().value);
    expect(row).toHaveTextContent("Public");
    expect(mockListLabels).toHaveBeenCalledWith("", "user-1");
  });

  it("filters the list by scope without another request", async () => {
    mockListLabels.mockResolvedValue({
      labels: [label(), label({ label: "Mine", scope: "workspace" })],
    });
    await renderPage();
    expect(screen.getAllByTestId("label-row")).toHaveLength(2);

    fireEvent.change(screen.getByLabelText("Filter by scope"), {
      target: { value: "workspace" },
    });

    expect(screen.getAllByTestId("label-row")).toHaveLength(1);
    expect(screen.getByTestId("label-row")).toHaveTextContent("Mine");
  });

  it("searches through the API with the typed query", async () => {
    await renderPage();

    fireEvent.change(screen.getByLabelText("Search"), {
      target: { value: "DEX" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Search" }));

    await waitFor(() =>
      expect(mockListLabels).toHaveBeenLastCalledWith("DEX", "user-1")
    );
  });

  it("validates the form before calling the API", async () => {
    await renderPage();

    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "DEX" },
    });
    fireEvent.change(screen.getByLabelText("Account or contract ID"), {
      target: { value: "not-an-id" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save label" }));

    expect(
      await screen.findByText(/56-character Stellar account/)
    ).toBeVisible();
    expect(mockCreateLabel).not.toHaveBeenCalled();
  });

  it("creates a label and reloads the registry", async () => {
    await renderPage();

    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "DEX" },
    });
    fireEvent.change(screen.getByLabelText("Account or contract ID"), {
      target: { value: label().value },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save label" }));

    await waitFor(() =>
      expect(mockCreateLabel).toHaveBeenCalledWith(
        { label: "DEX", value: label().value, scope: "workspace" },
        "user-1"
      )
    );
    expect(await screen.findByText("Label saved.")).toBeVisible();
    expect(mockListLabels).toHaveBeenCalledTimes(2);
  });

  it("reports a failed creation instead of swallowing it", async () => {
    mockCreateLabel.mockRejectedValue(new Error("Label already taken."));
    await renderPage();

    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "DEX" },
    });
    fireEvent.change(screen.getByLabelText("Account or contract ID"), {
      target: { value: label().value },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save label" }));

    expect(await screen.findByText("Label already taken.")).toBeVisible();
    expect(screen.queryByText("Label saved.")).toBeNull();
  });

  it("offers a retry when the registry cannot be loaded", async () => {
    mockListLabels.mockRejectedValueOnce(new Error("API is down."));
    const { default: LabelsPage } = await import("@/app/(app)/labels/page");
    render(<LabelsPage />);

    expect(await screen.findByText("Couldn't load labels.")).toBeVisible();
    expect(screen.getByText("API is down.")).toBeVisible();

    mockListLabels.mockResolvedValue({ labels: [label()] });
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));

    expect(await screen.findByTestId("label-row")).toBeVisible();
  });
});
