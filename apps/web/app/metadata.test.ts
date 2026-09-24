import { describe, expect, it } from "vitest";

import { metadata as rootMetadata } from "./layout";
import { metadata as compareMetadata } from "./(app)/compare/layout";
import { metadata as contractsMetadata } from "./(app)/contracts/layout";
import { metadata as eventsMetadata } from "./(app)/events/layout";
import { metadata as playgroundMetadata } from "./(app)/playground/layout";
import { metadata as watchdogMetadata } from "./(app)/watchdog/layout";
import { metadata as watchlistMetadata } from "./(app)/watchlist/layout";

describe("page titles", () => {
  it("suffixes every page title with | Sorolens via the root template", () => {
    expect(rootMetadata.title).toEqual({
      default: "Sorolens",
      template: "%s | Sorolens",
    });
  });

  it("gives each section a page title for the template", () => {
    expect(
      [
        compareMetadata,
        contractsMetadata,
        eventsMetadata,
        playgroundMetadata,
        watchdogMetadata,
        watchlistMetadata,
      ].map((m) => m.title)
    ).toEqual([
      "Compare Contracts",
      "Contracts",
      "Live Events",
      "API Playground",
      "Watchdog",
      "Watchlist",
    ]);
  });
});
