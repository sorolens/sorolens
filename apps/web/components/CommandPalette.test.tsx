/**
 * Unit tests for the CommandPalette component (issue #189).
 *
 * Covers:
 * - Cmd+K / Ctrl+K opens the palette
 * - Escape closes the palette
 * - Typing filters commands
 * - Typing filters contracts (search section)
 * - Arrow key navigation
 * - Enter runs the highlighted command
 * - "Track contract" command fires the custom event
 * - Backdrop click closes the palette
 */
import * as matchers from "@testing-library/jest-dom/matchers";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

expect.extend(matchers);

// ── Mock next/navigation ─────────────────────────────────────────────────────
const mockPush = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
}));

// ── Mock next/link ───────────────────────────────────────────────────────────
vi.mock("next/link", () => ({
  default: ({
    children,
    href,
  }: {
    children: React.ReactNode;
    href: string;
  }) => <a href={href}>{children}</a>,
}));

// ── Mock @/lib/api ───────────────────────────────────────────────────────────
const mockListContractsAll = vi.fn();
vi.mock("@/lib/api", () => ({
  listContractsAll: (...args: unknown[]) => mockListContractsAll(...args),
}));

// ── Mock localStorage ────────────────────────────────────────────────────────
const localStorageMock = (() => {
  let store: Record<string, string> = {};
  return {
    getItem: (k: string) => store[k] ?? null,
    setItem: (k: string, v: string) => {
      store[k] = v;
    },
    clear: () => {
      store = {};
    },
  };
})();
Object.defineProperty(window, "localStorage", { value: localStorageMock });

// ── Fixtures ─────────────────────────────────────────────────────────────────

const CONTRACT_A = {
  id: "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
  label: "My Contract",
  network: "testnet",
  status: "active",
  wasm_hash: null,
  added_at: "2024-01-01T00:00:00Z",
};

const CONTRACT_B = {
  id: "CBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB",
  label: null,
  network: "mainnet",
  status: "backfilling",
  wasm_hash: null,
  added_at: "2024-02-01T00:00:00Z",
};

// ── Helpers ──────────────────────────────────────────────────────────────────

async function renderPalette(onClose = vi.fn()) {
  const { CommandPalette } = await import("./CommandPalette");
  return render(<CommandPalette onClose={onClose} />);
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe("CommandPalette", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorageMock.clear();
    // Default: no contracts
    mockListContractsAll.mockResolvedValue({ contracts: [] });
  });

  afterEach(() => {
    cleanup();
  });

  // ── Renders ───────────────────────────────────────────────────────────────

  it("renders the search input", async () => {
    await renderPalette();
    expect(screen.getByRole("combobox")).toBeDefined();
    expect(screen.getByPlaceholderText(/search commands or contracts/i)).toBeDefined();
  });

  it("renders the Commands section header", async () => {
    await renderPalette();
    expect(screen.getByText("Commands")).toBeDefined();
  });

  it("shows built-in commands", async () => {
    await renderPalette();
    // Use getAllByText since command label + description may both contain text
    expect(screen.getAllByText(/track contract/i).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/go to contracts/i).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/go to watchdog/i).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/go to playground/i).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/toggle theme/i).length).toBeGreaterThan(0);
  });

  // ── Filtering commands ────────────────────────────────────────────────────

  it("filters commands when typing", async () => {
    await renderPalette();
    const input = screen.getByRole("combobox");
    fireEvent.change(input, { target: { value: "watchdog" } });

    expect(screen.getByText(/go to watchdog/i)).toBeDefined();
    // Other commands not matching "watchdog" should be gone
    expect(screen.queryByText(/go to playground/i)).toBeNull();
  });

  it("shows no results message when nothing matches", async () => {
    await renderPalette();
    const input = screen.getByRole("combobox");
    fireEvent.change(input, { target: { value: "zzzzzzz_no_match" } });

    // The text is split across elements, so use a custom function matcher
    expect(
      screen.getByTestId("no-results"),
    ).toBeDefined();
  });

  // ── Filtering contracts ───────────────────────────────────────────────────

  it("shows a Contracts section when contracts are loaded", async () => {
    mockListContractsAll.mockResolvedValue({
      contracts: [CONTRACT_A, CONTRACT_B],
    });
    await renderPalette();
    await waitFor(() => expect(screen.getByText("Contracts")).toBeDefined());
  });

  it("filters contracts by label", async () => {
    mockListContractsAll.mockResolvedValue({
      contracts: [CONTRACT_A, CONTRACT_B],
    });
    await renderPalette();
    await waitFor(() => screen.getByText("Contracts"));

    const input = screen.getByRole("combobox");
    fireEvent.change(input, { target: { value: "my contract" } });

    expect(screen.getByText("My Contract")).toBeDefined();
  });

  it("filters contracts by ID prefix", async () => {
    mockListContractsAll.mockResolvedValue({
      contracts: [CONTRACT_A, CONTRACT_B],
    });
    await renderPalette();
    await waitFor(() => screen.getByText("Contracts"));

    const input = screen.getByRole("combobox");
    fireEvent.change(input, { target: { value: "CBBBB" } });

    const items = screen.getAllByRole("option");
    const contractItems = items.filter((el) =>
      el.textContent?.includes("CBBBB"),
    );
    expect(contractItems.length).toBeGreaterThan(0);
  });

  // ── Keyboard navigation ───────────────────────────────────────────────────

  it("closes on Escape key", async () => {
    const onClose = vi.fn();
    await renderPalette(onClose);

    const input = screen.getByRole("combobox");
    fireEvent.keyDown(input, { key: "Escape" });
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("ArrowDown cycles through items", async () => {
    await renderPalette();
    const input = screen.getByRole("combobox");

    // Initially the first item should be highlighted
    const options = screen.getAllByRole("option");
    expect(options[0].getAttribute("aria-selected")).toBe("true");

    fireEvent.keyDown(input, { key: "ArrowDown" });
    const updatedOptions = screen.getAllByRole("option");
    expect(updatedOptions[1].getAttribute("aria-selected")).toBe("true");
  });

  it("ArrowUp cycles backwards", async () => {
    await renderPalette();
    const input = screen.getByRole("combobox");

    // Move down first
    fireEvent.keyDown(input, { key: "ArrowDown" });
    fireEvent.keyDown(input, { key: "ArrowDown" });
    fireEvent.keyDown(input, { key: "ArrowUp" });

    const options = screen.getAllByRole("option");
    expect(options[1].getAttribute("aria-selected")).toBe("true");
  });

  it("Enter navigates to contract when a contract row is highlighted", async () => {
    mockListContractsAll.mockResolvedValue({
      contracts: [CONTRACT_A],
    });
    await renderPalette();
    await waitFor(() => screen.getByText("Contracts"));

    const input = screen.getByRole("combobox");

    // Navigate past all commands to the first contract
    // (number of built-in commands is 5)
    for (let i = 0; i < 5; i++) {
      fireEvent.keyDown(input, { key: "ArrowDown" });
    }
    fireEvent.keyDown(input, { key: "Enter" });

    expect(mockPush).toHaveBeenCalledWith(
      `/contracts/${CONTRACT_A.id}`,
    );
  });

  it("Enter runs a command when a command row is highlighted", async () => {
    const onClose = vi.fn();
    await renderPalette(onClose);

    const input = screen.getByRole("combobox");
    // Filter to just "watchdog" command
    fireEvent.change(input, { target: { value: "Go to Watchdog" } });

    await waitFor(() => screen.getByText(/go to watchdog/i));
    fireEvent.keyDown(input, { key: "Enter" });

    expect(mockPush).toHaveBeenCalledWith("/watchdog");
    expect(onClose).toHaveBeenCalled();
  });

  // ── "Track contract" command ──────────────────────────────────────────────

  it('"Track contract" dispatches the OPEN_TRACK_MODAL_EVENT and closes', async () => {
    const onClose = vi.fn();
    const dispatchSpy = vi.spyOn(window, "dispatchEvent");
    await renderPalette(onClose);

    const input = screen.getByRole("combobox");
    // Filter to just "track contract" — the option with exact label "Track contract"
    fireEvent.change(input, { target: { value: "track" } });

    // Find the [role="option"] that contains "Track contract" as its label text
    await waitFor(() => {
      const options = screen.getAllByRole("option");
      expect(options.some((o) => o.textContent?.includes("Track contract"))).toBe(true);
    });

    const options = screen.getAllByRole("option");
    const trackOption = options.find((o) => o.textContent?.includes("Track contract"))!;
    fireEvent.click(trackOption);

    expect(onClose).toHaveBeenCalled();
    // router.push("/contracts") should be called
    expect(mockPush).toHaveBeenCalledWith("/contracts");
    // The custom event should be dispatched (after a timeout)
    await waitFor(() =>
      expect(
        dispatchSpy.mock.calls.some(
          (c) => (c[0] as CustomEvent).type === "sorolens:open-track-modal",
        ),
      ).toBe(true),
      { timeout: 500 },
    );
  });

  // ── Backdrop click ────────────────────────────────────────────────────────

  it("closes when clicking the backdrop", async () => {
    const onClose = vi.fn();
    const { container } = await renderPalette(onClose);

    // The outermost div is the backdrop
    const backdrop = container.firstChild as HTMLElement;
    fireEvent.click(backdrop);

    expect(onClose).toHaveBeenCalled();
  });

  // ── Theme toggle ──────────────────────────────────────────────────────────

  it('"Toggle theme" calls document.documentElement.style.setProperty', async () => {
    const setPropSpy = vi.spyOn(document.documentElement.style, "setProperty");
    await renderPalette();

    const input = screen.getByRole("combobox");
    fireEvent.change(input, { target: { value: "toggle theme" } });
    await waitFor(() => screen.getByText(/toggle theme/i));

    const themeOption = screen.getByText(/toggle theme/i).closest("[role='option']")!;
    fireEvent.click(themeOption);

    expect(setPropSpy).toHaveBeenCalled();
  });
});

// ── useCommandPalette hook ────────────────────────────────────────────────────

describe("useCommandPalette", () => {
  afterEach(cleanup);

  it("opens on Cmd+K", async () => {
    const { useCommandPalette } = await import("./CommandPalette");

    function Harness() {
      const { open, setOpen } = useCommandPalette();
      return (
        <div>
          <span data-testid="state">{open ? "open" : "closed"}</span>
          <button onClick={() => setOpen(false)}>close</button>
        </div>
      );
    }

    render(<Harness />);
    expect(screen.getByTestId("state").textContent).toBe("closed");

    await act(async () => {
      fireEvent.keyDown(window, { key: "k", metaKey: true });
    });

    expect(screen.getByTestId("state").textContent).toBe("open");
  });

  it("opens on Ctrl+K", async () => {
    const { useCommandPalette } = await import("./CommandPalette");

    function Harness() {
      const { open, setOpen } = useCommandPalette();
      return (
        <div>
          <span data-testid="state">{open ? "open" : "closed"}</span>
          <button onClick={() => setOpen(false)}>close</button>
        </div>
      );
    }

    render(<Harness />);

    await act(async () => {
      fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    });

    expect(screen.getByTestId("state").textContent).toBe("open");
  });

  it("toggles closed if already open on second Cmd+K", async () => {
    const { useCommandPalette } = await import("./CommandPalette");

    function Harness() {
      const { open } = useCommandPalette();
      return <span data-testid="state">{open ? "open" : "closed"}</span>;
    }

    render(<Harness />);

    await act(async () => {
      fireEvent.keyDown(window, { key: "k", metaKey: true });
    });
    expect(screen.getByTestId("state").textContent).toBe("open");

    await act(async () => {
      fireEvent.keyDown(window, { key: "k", metaKey: true });
    });
    expect(screen.getByTestId("state").textContent).toBe("closed");
  });
});
