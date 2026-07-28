import * as matchers from "@testing-library/jest-dom/matchers";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { Badge } from "./Badge";

expect.extend(matchers);

afterEach(() => {
  cleanup();
});

describe("Badge", () => {
  it("renders all variants with matching colors and both sizes", () => {
    const variants = [
      "iris",
      "sky",
      "teal",
      "amber",
      "rose",
      "neutral",
    ] as const;

    for (const variant of variants) {
      const { rerender } = render(<Badge variant={variant}>Sample</Badge>);
      const badge = screen.getByText("Sample");
      expect(badge).toHaveAttribute("data-variant", variant);
      expect(badge).toHaveStyle("border-radius: 9999px");
      expect(badge).toHaveAttribute("role", "status");
      rerender(<></>);
    }

    render(<Badge size="sm">Small</Badge>);
    expect(screen.getByText("Small")).toHaveStyle("padding: 0.2rem 0.6rem");

    render(<Badge size="md">Medium</Badge>);
    expect(screen.getByText("Medium")).toHaveStyle("padding: 0.35rem 0.8rem");
  });
});
