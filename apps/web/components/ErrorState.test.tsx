import * as matchers from "@testing-library/jest-dom/matchers";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ErrorState, errorMessage } from "@/components/ErrorState";

expect.extend(matchers);

describe("ErrorState", () => {
  afterEach(() => {
    cleanup();
  });

  it("announces the failure as an alert with title and message", () => {
    render(<ErrorState title="Couldn't load things." message="API down" />);

    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("Couldn't load things.");
    expect(alert).toHaveTextContent("API down");
  });

  it("calls onRetry when the retry button is pressed", () => {
    const onRetry = vi.fn();
    render(<ErrorState onRetry={onRetry} />);

    fireEvent.click(screen.getByRole("button", { name: "Try again" }));

    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it("supports a custom retry label", () => {
    render(<ErrorState onRetry={() => {}} retryLabel="Reload" />);

    expect(screen.getByRole("button", { name: "Reload" })).toBeInTheDocument();
  });

  it("disables the button and shows progress while retrying", () => {
    const onRetry = vi.fn();
    render(<ErrorState onRetry={onRetry} retrying />);

    const button = screen.getByRole("button", { name: "Retrying…" });
    expect(button).toBeDisabled();
    fireEvent.click(button);
    expect(onRetry).not.toHaveBeenCalled();
  });

  it("renders no retry button without onRetry", () => {
    render(<ErrorState message="nope" />);

    expect(screen.queryByRole("button")).toBeNull();
  });

  it("falls back to a default title and omits an empty message", () => {
    render(<ErrorState />);

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Couldn't load this data."
    );
    expect(screen.getByRole("alert").querySelectorAll("p")).toHaveLength(1);
  });
});

describe("errorMessage", () => {
  it("uses an Error's message", () => {
    expect(errorMessage(new Error("boom"), "fallback")).toBe("boom");
  });

  it("falls back for empty messages and non-errors", () => {
    expect(errorMessage(new Error(""), "fallback")).toBe("fallback");
    expect(errorMessage("a string", "fallback")).toBe("fallback");
    expect(errorMessage(undefined, "fallback")).toBe("fallback");
  });
});
