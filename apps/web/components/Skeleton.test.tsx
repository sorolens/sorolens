import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { CardSkeleton, ChartSkeleton, TableSkeleton } from "./Skeleton";

afterEach(cleanup);

describe("loading skeleton accessibility", () => {
  it.each([
    ["card", CardSkeleton],
    ["chart", ChartSkeleton],
    ["table", TableSkeleton],
  ] as const)(
    "announces the %s as a busy status",
    (kind, SkeletonComponent) => {
      render(<SkeletonComponent />);

      const status = screen.getByRole("status", { name: `Loading ${kind}` });
      expect(status).toHaveAttribute("aria-busy", "true");
      expect(status.querySelector('[aria-hidden="true"]')).toBeInTheDocument();
    }
  );
});
