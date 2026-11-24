import * as matchers from "@testing-library/jest-dom/matchers";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { EmptyState } from "@/components/EmptyState";

expect.extend(matchers);

describe("EmptyState", () => {
  afterEach(() => {
    cleanup();
  });

  it("renders a first-run empty state as a polite status with copy", () => {
    render(
      <EmptyState
        title="No contracts tracked yet"
        description="Track a contract to get started."
      />
    );

    const status = screen.getByRole("status");
    expect(status).toHaveAttribute("data-empty-variant", "first-run");
    expect(status).toHaveTextContent("No contracts tracked yet");
    expect(status).toHaveTextContent("Track a contract to get started.");
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("distinguishes a filtered no-results state from a first run", () => {
    render(<EmptyState variant="no-results" title="No matches" />);

    expect(screen.getByRole("status")).toHaveAttribute(
      "data-empty-variant",
      "no-results"
    );
  });

  it("links the next action to its route", () => {
    render(
      <EmptyState
        title="No contracts tracked yet"
        action={{ label: "Track a contract", href: "/contracts/new" }}
      />
    );

    expect(
      screen.getByRole("link", { name: "Track a contract" })
    ).toHaveAttribute("href", "/contracts/new");
  });

  it("calls onClick for a button action", () => {
    const onClick = vi.fn();
    render(
      <EmptyState
        variant="no-results"
        title="No matches"
        action={{ label: "Clear filters", onClick }}
      />
    );

    fireEvent.click(screen.getByRole("button", { name: "Clear filters" }));

    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("renders no action without one", () => {
    render(<EmptyState title="Nothing here yet" />);

    expect(screen.queryByRole("button")).toBeNull();
    expect(screen.queryByRole("link")).toBeNull();
  });

  it("delegates the error variant to ErrorState with a retry", () => {
    const onRetry = vi.fn();
    render(
      <EmptyState
        variant="error"
        title="Couldn't load contracts."
        message="API down"
        onRetry={onRetry}
      />
    );

    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("Couldn't load contracts.");
    expect(alert).toHaveTextContent("API down");

    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });
});
