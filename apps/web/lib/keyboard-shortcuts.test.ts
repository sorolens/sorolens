import { describe, expect, it, vi } from "vitest";

import {
  createSequencer,
  isKeyboardShortcutsDisabled,
  isTypingTarget,
  parseSequence,
  SEQUENCE_TIMEOUT_MS,
  setKeyboardShortcutsDisabled,
  SHORTCUT_HELP,
  SHORTCUTS_STORAGE_KEY,
  type ShortcutBinding,
} from "./keyboard-shortcuts";

/** Minimal in-memory Storage so the helpers can be tested without a DOM. */
function memoryStorage(): Storage {
  const data = new Map<string, string>();
  return {
    get length() {
      return data.size;
    },
    clear: () => data.clear(),
    getItem: (key: string) => (data.has(key) ? (data.get(key) as string) : null),
    key: (index: number) => Array.from(data.keys())[index] ?? null,
    removeItem: (key: string) => {
      data.delete(key);
    },
    setItem: (key: string, value: string) => {
      data.set(key, value);
    },
  };
}

function bindings(runs: Record<string, ReturnType<typeof vi.fn>>): ShortcutBinding[] {
  return [
    { id: "contracts", sequence: "g c", label: "Contracts", run: runs.contracts },
    { id: "events", sequence: "g e", label: "Events", run: runs.events },
    { id: "search", sequence: "/", label: "Search", run: runs.search },
  ];
}

describe("parseSequence", () => {
  it("splits, lowercases and collapses whitespace", () => {
    expect(parseSequence("  G   C ")).toEqual(["g", "c"]);
    expect(parseSequence("/")).toEqual(["/"]);
    expect(parseSequence("")).toEqual([]);
  });
});

describe("shortcut preference", () => {
  it("defaults to enabled when nothing is stored", () => {
    expect(isKeyboardShortcutsDisabled(memoryStorage())).toBe(false);
  });

  it("round-trips the disabled flag", () => {
    const storage = memoryStorage();

    expect(setKeyboardShortcutsDisabled(true, storage)).toBe(true);
    expect(storage.getItem(SHORTCUTS_STORAGE_KEY)).toBe("1");
    expect(isKeyboardShortcutsDisabled(storage)).toBe(true);

    setKeyboardShortcutsDisabled(false, storage);
    expect(isKeyboardShortcutsDisabled(storage)).toBe(false);
  });

  it("reports failure when storage throws", () => {
    const storage = memoryStorage();
    storage.setItem = () => {
      throw new Error("quota exceeded");
    };
    expect(setKeyboardShortcutsDisabled(true, storage)).toBe(false);
  });
});

describe("isTypingTarget", () => {
  it("detects form fields", () => {
    for (const tag of ["input", "textarea", "select"]) {
      const element = document.createElement(tag);
      expect(isTypingTarget(element)).toBe(true);
    }
  });

  it("detects contenteditable regions", () => {
    const editable = document.createElement("div");
    editable.contentEditable = "true";
    // jsdom does not compute isContentEditable from the attribute.
    Object.defineProperty(editable, "isContentEditable", { value: true });
    expect(isTypingTarget(editable)).toBe(true);
  });

  it("ignores ordinary elements and non-elements", () => {
    expect(isTypingTarget(document.createElement("div"))).toBe(false);
    expect(isTypingTarget(null)).toBe(false);
    expect(isTypingTarget("input")).toBe(false);
  });
});

describe("createSequencer", () => {
  function make() {
    const runs = { contracts: vi.fn(), events: vi.fn(), search: vi.fn() };
    return { runs, sequencer: createSequencer(bindings(runs)) };
  }

  it("fires single-key shortcuts immediately", () => {
    const { runs, sequencer } = make();

    const fired = sequencer.handle("/");
    expect(fired?.id).toBe("search");
    expect(fired?.run).toBe(runs.search);
  });

  it("fires a two-key sequence on its second key", () => {
    const { runs, sequencer } = make();

    expect(sequencer.handle("g")).toBeNull();
    const fired = sequencer.handle("c");
    expect(fired?.id).toBe("contracts");
    expect(fired?.run).toBe(runs.contracts);
  });

  it("routes each suffix of g to its own page", () => {
    const { sequencer } = make();

    sequencer.handle("g");
    expect(sequencer.handle("e")?.id).toBe("events");
  });

  it("is case-insensitive", () => {
    const { sequencer } = make();

    sequencer.handle("G");
    expect(sequencer.handle("C")?.id).toBe("contracts");
  });

  it("cancels an armed sequence when the wrong key follows", () => {
    const { sequencer } = make();

    sequencer.handle("g");
    expect(sequencer.handle("x")).toBeNull();

    // The cancelled prefix is not reusable: "c" alone must not fire.
    expect(sequencer.handle("c")).toBeNull();
  });

  it("expires an armed sequence after the timeout", () => {
    const custom = createSequencer(bindings(make().runs), 500);

    custom.handle("g", 1000);
    expect(custom.handle("c", 1000 + 500)).not.toBeNull();

    custom.handle("g", 1000);
    expect(custom.handle("c", 1000 + 500 + 1)).toBeNull();
  });

  it("honours the default timeout", () => {
    const { sequencer } = make();

    sequencer.handle("g", 0);
    expect(sequencer.handle("c", SEQUENCE_TIMEOUT_MS + 1)).toBeNull();
  });

  it("ignores unbound keys", () => {
    const { sequencer } = make();

    expect(sequencer.handle("z")).toBeNull();
    expect(sequencer.handle("1")).toBeNull();
  });

  it("reset drops an armed prefix", () => {
    const { sequencer } = make();

    sequencer.handle("g");
    sequencer.reset();
    expect(sequencer.handle("c")).toBeNull();
  });
});

describe("SHORTCUT_HELP", () => {
  it("documents every navigation shortcut", () => {
    const labels = SHORTCUT_HELP.map((entry) => entry.keys.join(" "));
    expect(labels).toContain("g c");
    expect(labels).toContain("g e");
    expect(labels).toContain("g i");
    expect(labels).toContain("/");
    expect(labels).toContain("?");
  });
});
