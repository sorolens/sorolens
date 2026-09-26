import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import robots from "@/app/robots";

const captureException = vi.hoisted(() => vi.fn());

vi.mock("@sentry/nextjs", () => ({
  captureException,
}));

vi.mock("next/link", () => ({
  default: ({
    children,
    href,
  }: {
    children: React.ReactNode;
    href: string;
  }) => <a href={href}>{children}</a>,
}));

import RootError from "@/app/error";
import RootNotFound from "@/app/not-found";
import AppError from "@/app/(app)/error";
import AppNotFound from "@/app/(app)/not-found";

describe("route handling", () => {
  beforeEach(() => {
    captureException.mockClear();
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllEnvs();
  });

  it("derives the crawler rules and sitemap from NEXT_PUBLIC_APP_URL", () => {
    vi.stubEnv("NEXT_PUBLIC_APP_URL", "https://staging.sorolens.dev/");

    expect(robots()).toEqual({
      rules: {
        userAgent: "*",
        allow: "/",
      },
      sitemap: "https://staging.sorolens.dev/sitemap.xml",
      host: "https://staging.sorolens.dev",
    });
  });

  it("falls back to the production origin when NEXT_PUBLIC_APP_URL is unset", () => {
    vi.stubEnv("NEXT_PUBLIC_APP_URL", undefined);

    expect(robots()).toEqual({
      rules: {
        userAgent: "*",
        allow: "/",
      },
      sitemap: "https://sorolens.dev/sitemap.xml",
      host: "https://sorolens.dev",
    });
  });

  it.each([
    ["root", RootError, "This page hit a snag", "Try again"],
    ["app", AppError, "We lost the signal", "Retry dashboard"],
  ])(
    "reports errors and retries the %s route group",
    async (routeGroup, ErrorBoundary, heading, buttonName) => {
      const reset = vi.fn();
      const error = new Error("render failed");

      render(<ErrorBoundary error={error} reset={reset} />);

      expect(screen.getByRole("heading", { name: heading })).toBeDefined();
      expect(captureException).toHaveBeenCalledWith(error, {
        tags: { routeGroup },
      });

      fireEvent.click(screen.getByRole("button", { name: buttonName }));
      expect(reset).toHaveBeenCalledOnce();
    }
  );

  it("renders useful recovery links in both not-found boundaries", () => {
    const rootView = render(<RootNotFound />);
    expect(screen.getByRole("link", { name: "Back to home" })).toHaveProperty(
      "href",
      expect.stringContaining("/")
    );
    rootView.unmount();

    render(<AppNotFound />);
    expect(
      screen.getByRole("link", { name: "View contracts" }).getAttribute("href")
    ).toBe("/contracts");
    expect(
      screen.getByRole("link", { name: "Back to home" }).getAttribute("href")
    ).toBe("/");
  });
});
