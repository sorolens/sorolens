import * as matchers from "@testing-library/jest-dom/matchers";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { Toast } from "./Toast";

expect.extend(matchers);

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("Toast", () => {
  it("renders the message", () => {
    render(<Toast message="Contract tracked" onDismiss={() => {}} />);
    expect(screen.getByText("Contract tracked")).toBeInTheDocument();
  });

  it("uses role=alert with assertive live region for errors", () => {
    render(<Toast message="Boom" variant="error" onDismiss={() => {}} />);
    const toast = screen.getByRole("alert");
    expect(toast).toHaveTextContent("Boom");
    expect(toast).toHaveAttribute("aria-live", "assertive");
    expect(toast).toHaveAttribute("aria-atomic", "true");
  });

  it("uses role=status with polite live region by default", () => {
    render(<Toast message="Saved" onDismiss={() => {}} />);
    const toast = screen.getByRole("status");
    expect(toast).toHaveTextContent("Saved");
    expect(toast).toHaveAttribute("aria-live", "polite");
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("auto-dismisses after the timeout", () => {
    const onDismiss = vi.fn();
    render(<Toast message="Bye" duration={3000} onDismiss={onDismiss} />);

    act(() => {
      vi.advanceTimersByTime(2999);
    });
    expect(onDismiss).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(onDismiss).toHaveBeenCalledTimes(1);
  });

  it("does not restart the timer when re-rendered with a new callback", () => {
    const first = vi.fn();
    const second = vi.fn();
    const { rerender } = render(
      <Toast message="Hi" duration={3000} onDismiss={first} />,
    );

    act(() => {
      vi.advanceTimersByTime(2000);
    });
    rerender(<Toast message="Hi" duration={3000} onDismiss={second} />);
    act(() => {
      vi.advanceTimersByTime(1000);
    });

    expect(first).not.toHaveBeenCalled();
    expect(second).toHaveBeenCalledTimes(1);
  });

  it("clears its timer on unmount", () => {
    const onDismiss = vi.fn();
    const { unmount } = render(
      <Toast message="Gone" duration={3000} onDismiss={onDismiss} />,
    );
    expect(vi.getTimerCount()).toBe(1);

    unmount();
    expect(vi.getTimerCount()).toBe(0);

    act(() => {
      vi.advanceTimersByTime(5000);
    });
    expect(onDismiss).not.toHaveBeenCalled();
  });

  it("can be dismissed manually", () => {
    const onDismiss = vi.fn();
    render(<Toast message="Close me" onDismiss={onDismiss} />);

    fireEvent.click(screen.getByRole("button", { name: /dismiss notification/i }));
    expect(onDismiss).toHaveBeenCalledTimes(1);
  });
});
