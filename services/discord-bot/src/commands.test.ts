import { describe, expect, it, vi } from "vitest";
import type { AutocompleteInteraction } from "discord.js";
import {
  contractAutocompleteChoices,
  createContractAutocompleteHandler,
} from "./commands.js";

const CONTRACT = "C" + "A".repeat(55);

function autocompleteInteraction(commandName = "status", query = "watch") {
  const respond = vi.fn().mockResolvedValue(undefined);
  const options = {
    getFocused: vi.fn((full?: boolean) =>
      full ? { name: "contract", value: query } : query
    ),
  };
  return {
    interaction: {
      commandName,
      options,
      respond,
    } as unknown as AutocompleteInteraction,
    respond,
  };
}

describe("contract autocomplete", () => {
  it("matches labels and displays a short ID while preserving the full value", () => {
    expect(
      contractAutocompleteChoices([
        { id: CONTRACT, label: "Watchdog" },
        { id: "C" + "B".repeat(55), label: null },
      ])
    ).toEqual([
      {
        name: "Watchdog (CAAAAA…AAAAAA)",
        value: CONTRACT,
      },
      {
        name: "CBBBBB…BBBBBB",
        value: "C" + "B".repeat(55),
      },
    ]);
  });

  it("limits choices to Discord's 25-choice maximum and removes duplicates", () => {
    const results = Array.from({ length: 30 }, (_, i) => ({
      id: `contract-${i}`,
      label: `Contract ${i}`,
    }));
    results.push(results[0]);

    const choices = contractAutocompleteChoices(results);

    expect(choices).toHaveLength(25);
    expect(new Set(choices.map((choice) => choice.value)).size).toBe(25);
  });

  it("queries the API, caches repeated input, and degrades to an empty list on errors", async () => {
    const searchContracts = vi
      .fn()
      .mockResolvedValue([{ id: CONTRACT, label: "Watchdog" }]);
    const handler = createContractAutocompleteHandler({ searchContracts });
    const first = autocompleteInteraction("status", "watch");
    const second = autocompleteInteraction("status", "watch");

    await handler(first.interaction);
    await handler(second.interaction);

    expect(searchContracts).toHaveBeenCalledTimes(1);
    expect(first.respond).toHaveBeenCalledWith([
      { name: "Watchdog (CAAAAA…AAAAAA)", value: CONTRACT },
    ]);
    expect(second.respond).toHaveBeenCalledWith([
      { name: "Watchdog (CAAAAA…AAAAAA)", value: CONTRACT },
    ]);

    const failedHandler = createContractAutocompleteHandler({
      searchContracts: vi.fn().mockRejectedValue(new Error("API down")),
    });
    const failed = autocompleteInteraction("alerts", "watch");
    await failedHandler(failed.interaction);
    expect(failed.respond).toHaveBeenCalledWith([]);
  });
});
