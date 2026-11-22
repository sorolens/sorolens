import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import {
  CardSkeleton,
  ChartSkeleton,
  Skeleton,
  TableSkeleton,
} from "./Skeleton";

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

describe("Skeleton placeholder (issue #627)", () => {
  it("renders an aria-hidden pulse with the caller's sizing classes", () => {
    const { container } = render(<Skeleton className="h-4 w-20" />);

    const skeleton = container.firstElementChild as HTMLElement;
    expect(skeleton).toHaveAttribute("aria-hidden", "true");
    expect(skeleton.className).toContain("animate-pulse");
    expect(skeleton.className).toContain("h-4");
    expect(skeleton.className).toContain("w-20");
  });

  it("renders cleanly with no extra classes", () => {
    const { container } = render(<Skeleton />);

    const skeleton = container.firstElementChild as HTMLElement;
    expect(skeleton.className).toContain("animate-pulse");
  });
});

describe("TableSkeleton rows prop", () => {
  it("renders five rows by default (header skeleton plus rows)", () => {
    render(<TableSkeleton />);

    const status = screen.getByRole("status", { name: "Loading table" });
    expect(status.querySelectorAll(".animate-pulse")).toHaveLength(6);
  });

  it("renders the requested number of rows", () => {
    render(<TableSkeleton rows={2} />);

    const status = screen.getByRole("status", { name: "Loading table" });
    expect(status.querySelectorAll(".animate-pulse")).toHaveLength(3);
  });
});
