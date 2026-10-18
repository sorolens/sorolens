import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { LiveStatsClient } from "./LiveStatsClient";

describe("server-rendered landing stats", () => {
  it("renders server-provided counters before hydration", () => {
    const markup = renderToStaticMarkup(
      <LiveStatsClient
        initialData={{
          global: {
            tracked_contracts: 42,
            total_events: 12_345,
            total_invocations: 678,
            total_storage_entries: 90,
          },
          watchdog: null,
        }}
      />
    );

    expect(markup).toContain("42");
    expect(markup).toContain("12.3K");
    expect(markup).toContain("Contracts indexed");
  });
});
