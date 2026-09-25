import * as matchers from "@testing-library/jest-dom/matchers";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { MonoId } from "./MonoId";

expect.extend(matchers);

describe("MonoId", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("truncates, exposes the full value in the tooltip, copies on click, and shows feedback", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });

    const value = "abcdef1234567890fedcba0987654321abcdef1234567890fedcba";
    render(<MonoId value={value} />);

    const button = screen.getByRole("button", { name: `Copy ${value}` });
    expect(button).toHaveAttribute("title", value);
    expect(button).toHaveStyle("font-family: \"JetBrains Mono\", \"SFMono-Regular\", Consolas, \"Liberation Mono\", monospace");
    expect(button).toHaveTextContent("abcdef...dcba");

    fireEvent.click(button);

    await waitFor(() => expect(writeText).toHaveBeenCalledWith(value));
    expect(screen.getByText("Copied")).toBeVisible();
  });

  it("renders a non-interactive span with the full value in the title for the text variant", () => {
    const value = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQAHHAGQFW2J";
    render(<MonoId value={value} headChars={8} tailChars={6} variant="text" />);

    const span = screen.getByTitle(value);
    expect(span.tagName).toBe("SPAN");
    expect(span).toHaveTextContent("CDLZFC3S...");
    expect(span).not.toHaveTextContent(value);
    expect(screen.queryByRole("button")).toBeNull();
  });
});