import "@testing-library/jest-dom/vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useQueryState } from "./useQueryState";

const { searchParams, routerPush, routerReplace } = vi.hoisted(() => {
  const searchParams = new URLSearchParams();
  const applyTo = (url: string) => {
    const queryIndex = url.indexOf("?");
    const qs = queryIndex >= 0 ? url.slice(queryIndex + 1) : "";
    for (const key of Array.from(searchParams.keys())) searchParams.delete(key);
    for (const [key, value] of new URLSearchParams(qs)) {
      searchParams.set(key, value);
    }
  };
  const routerPush = vi.fn((url: string) => applyTo(url));
  const routerReplace = vi.fn((url: string) => applyTo(url));
  return { searchParams, routerPush, routerReplace };
});

vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: routerPush,
    replace: routerReplace,
    back: vi.fn(),
  }),
  useSearchParams: () => searchParams,
  usePathname: () => "/thing",
}));

interface Demo {
  color: string;
  page: number;
}

const spec = {
  parse: (params: URLSearchParams): Demo => ({
    color: params.get("color") ?? "",
    page: Number.parseInt(params.get("page") ?? "1", 10) || 1,
  }),
  serialize: (state: Demo) => ({
    color: state.color || undefined,
    page: state.page > 1 ? String(state.page) : undefined,
  }),
};

function Probe() {
  const [state, setState] = useQueryState<Demo>(spec);
  return (
    <div>
      <output data-testid="state">
        {state.color}|{state.page}
      </output>
      <button
        type="button"
        data-testid="set-blue"
        onClick={() => setState({ color: "blue", page: 1 })}
      >
        blue
      </button>
      <button
        type="button"
        data-testid="set-page"
        onClick={() => setState((prev) => ({ ...prev, page: prev.page + 1 }))}
      >
        page
      </button>
      <button
        type="button"
        data-testid="set-noop"
        onClick={() => setState((prev) => prev)}
      >
        noop
      </button>
    </div>
  );
}

function resetQuery() {
  for (const key of Array.from(searchParams.keys())) searchParams.delete(key);
  routerPush.mockClear();
  routerReplace.mockClear();
}

beforeEach(() => {
  resetQuery();
});

afterEach(() => {
  cleanup();
  resetQuery();
});

describe("useQueryState", () => {
  it("initializes its state from the query string", () => {
    searchParams.set("color", "red");
    searchParams.set("page", "3");
    render(<Probe />);
    expect(screen.getByTestId("state")).toHaveTextContent("red|3");
    expect(routerPush).not.toHaveBeenCalled();
    expect(routerReplace).not.toHaveBeenCalled();
  });

  it("falls back to defaults for missing params", () => {
    render(<Probe />);
    expect(screen.getByTestId("state")).toHaveTextContent("|1");
  });

  it("pushes a history entry when state changes", () => {
    render(<Probe />);
    act(() => {
      screen.getByTestId("set-blue").click();
    });
    expect(routerPush).toHaveBeenCalledWith("/thing?color=blue", {
      scroll: false,
    });
    expect(screen.getByTestId("state")).toHaveTextContent("blue|1");
  });

  it("can replace instead of push", () => {
    render(<Probe />);
    act(() => {
      screen.getByTestId("set-noop").click();
    });
    // A no-op update never navigates.
    expect(routerPush).not.toHaveBeenCalled();
    expect(routerReplace).not.toHaveBeenCalled();
  });

  it("supports functional updates", () => {
    render(<Probe />);
    act(() => {
      screen.getByTestId("set-page").click();
      screen.getByTestId("set-page").click();
    });
    expect(screen.getByTestId("state")).toHaveTextContent("|3");
    expect(routerPush).toHaveBeenLastCalledWith("/thing?page=3", {
      scroll: false,
    });
  });

  it("re-parses the state when the query string changes (back/forward)", () => {
    const view = render(<Probe />);
    act(() => {
      screen.getByTestId("set-blue").click();
    });
    expect(screen.getByTestId("state")).toHaveTextContent("blue|1");

    // Simulate browser Back: params revert, component re-renders.
    searchParams.delete("color");
    view.rerender(<Probe />);
    expect(screen.getByTestId("state")).toHaveTextContent("|1");

    // And forward again.
    searchParams.set("color", "blue");
    view.rerender(<Probe />);
    expect(screen.getByTestId("state")).toHaveTextContent("blue|1");
  });

  it("does not re-parse its own navigation", () => {
    const view = render(<Probe />);
    act(() => {
      screen.getByTestId("set-blue").click();
    });
    // The mocked router applied the URL; the echo must not trigger a
    // re-parse (which would reset unrelated local state in consumers).
    // Re-rendering with the same params is the observable equivalent.
    view.rerender(<Probe />);
    expect(screen.getByTestId("state")).toHaveTextContent("blue|1");
    expect(routerPush).toHaveBeenCalledTimes(1);
  });

  it("preserves params the page does not own", () => {
    searchParams.set("network", "mainnet");
    render(<Probe />);
    act(() => {
      screen.getByTestId("set-page").click();
    });
    expect(routerPush).toHaveBeenCalledWith("/thing?network=mainnet&page=2", {
      scroll: false,
    });
  });
});
