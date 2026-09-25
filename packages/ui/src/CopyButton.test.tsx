/**
 * Tests for packages/ui/src/CopyButton.tsx
 *
 * We use vitest + @testing-library/react + jsdom, same as MonoId.test.tsx.
 */

import * as matchers from "@testing-library/jest-dom/matchers";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { CopyButton } from "./CopyButton";

expect.extend(matchers);

describe("CopyButton", () => {
  const writeText = vi.fn().mockResolvedValue(undefined);

  beforeEach(() => {
    writeText.mockClear();
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it("copies the text on click and shows feedback", async () => {
    const text = JSON.stringify({ hello: "world" }, null, 2);
    render(<CopyButton text={text} label="Copy JSON" />);

    const button = screen.getByRole("button", { name: "Copy JSON" });
    expect(button).toHaveAttribute("title", "Copy JSON");

    fireEvent.click(button);

    await waitFor(() => expect(writeText).toHaveBeenCalledWith(text));
    expect(screen.getByText("Copied")).toBeVisible();
  });

  it("does not trigger outer click handlers (e.g. row expand)", async () => {
    const onRowClick = vi.fn();
    render(
      <div onClick={onRowClick}>
        <CopyButton text="x" label="Copy" />
      </div>
    );

    fireEvent.click(screen.getByRole("button", { name: "Copy" }));

    await waitFor(() => expect(writeText).toHaveBeenCalledWith("x"));
    expect(onRowClick).not.toHaveBeenCalled();
  });

  it("clears the Copied feedback after a short delay", async () => {
    vi.useFakeTimers();
    try {
      render(<CopyButton text="x" label="Copy" />);
      fireEvent.click(screen.getByRole("button", { name: "Copy" }));
      // Flush the async clipboard promise so "Copied" renders.
      await act(async () => {});
      expect(screen.getByText("Copied")).toBeVisible();

      act(() => {
        vi.advanceTimersByTime(1500);
      });
      expect(screen.queryByText("Copied")).not.toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });
});
