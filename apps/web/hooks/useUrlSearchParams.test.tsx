import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { useUrlSearchParams } from "@/hooks/useUrlSearchParams";

describe("useUrlSearchParams", () => {
  afterEach(() => {
    window.history.replaceState({}, "", "/");
  });

  it("writes query state and restores it through browser history", () => {
    window.history.replaceState({}, "", "/events?type=contract");
    const { result } = renderHook(() => useUrlSearchParams());

    act(() => {
      result.current[1]((params) => {
        params.set("type", "system");
        params.append("cursor", "page-two");
      });
    });
    expect(result.current[0].get("type")).toBe("system");
    expect(result.current[0].getAll("cursor")).toEqual(["page-two"]);

    act(() => {
      window.history.replaceState({}, "", "/events?type=contract");
      window.dispatchEvent(new PopStateEvent("popstate"));
    });
    expect(result.current[0].get("type")).toBe("contract");
    expect(result.current[0].getAll("cursor")).toEqual([]);
  });
});
