import { afterEach, describe, expect, it, vi } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { PlausibleAnalytics } from "./PlausibleAnalytics";

describe("PlausibleAnalytics (#191)", () => {
  afterEach(() => vi.unstubAllEnvs());

  it("renders the cookieless script in production when a domain is set", () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("NEXT_PUBLIC_PLAUSIBLE_DOMAIN", "sorolens.dev");

    const html = renderToStaticMarkup(<PlausibleAnalytics />);

    expect(html).toContain('src="https://plausible.io/js/script.js"');
    expect(html).toContain('data-domain="sorolens.dev"');
    expect(html).toContain("defer");
    // Cookieless: the standard script needs no extra attributes and sets no cookies.
    expect(html).not.toContain("cookie");
  });

  it("renders nothing outside a production build", () => {
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("NEXT_PUBLIC_PLAUSIBLE_DOMAIN", "sorolens.dev");

    expect(renderToStaticMarkup(<PlausibleAnalytics />)).toBe("");
  });

  it("renders nothing when no domain is configured", () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("NEXT_PUBLIC_PLAUSIBLE_DOMAIN", "");

    expect(renderToStaticMarkup(<PlausibleAnalytics />)).toBe("");
  });
});
