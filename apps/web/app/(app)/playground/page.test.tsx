/**
 * Tests for apps/web/app/(app)/playground/page.tsx
 *
 * The page uses the global fetch API directly, so we stub it. No Next.js
 * runtime is required (the page imports no next/* modules).
 */

import * as matchers from "@testing-library/jest-dom/matchers";
import {
  cleanup,
  render,
  screen,
  fireEvent,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import PlaygroundPage from "./page";

expect.extend(matchers);

function mockResponse(
  status = 200,
  body = JSON.stringify({ contracts: [] }, null, 2)
) {
  return {
    status,
    statusText: status === 200 ? "OK" : "Forbidden",
    headers: {
      forEach: (cb: (value: string, key: string) => void) =>
        cb("application/json", "content-type"),
    },
    text: async () => body,
  };
}

describe("PlaygroundPage", () => {
  const fetchMock = vi.fn();

  beforeEach(() => {
    fetchMock.mockReset();
    fetchMock.mockResolvedValue(mockResponse());
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("lists endpoints grouped by tag", () => {
    render(<PlaygroundPage />);
    expect(
      screen.getByRole("heading", { name: /api playground/i })
    ).toBeDefined();
    expect(screen.getByTestId("endpoint-GET-/api/v1/contracts")).toBeDefined();
    expect(screen.getByTestId("endpoint-POST-/api/v1/contracts")).toBeDefined();
    expect(
      screen.getByTestId("endpoint-GET-/api/v1/contracts/{id}/snapshot")
    ).toBeDefined();
  });

  it("does not auto-populate an API key", () => {
    render(<PlaygroundPage />);
    const keyInput = document.getElementById(
      "playground-api-key"
    ) as HTMLInputElement;
    expect(keyInput.value).toBe("");
  });

  it("sends a request and renders status, headers, and body", async () => {
    render(<PlaygroundPage />);

    fireEvent.click(screen.getByTestId("endpoint-GET-/api/v1/contracts/{id}"));

    const idInput = document.getElementById("param-id") as HTMLInputElement;
    fireEvent.change(idInput, { target: { value: "CABC" } });

    fireEvent.click(document.getElementById("playground-send")!);

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    const [url, options] = fetchMock.mock.calls[0];
    expect(String(url)).toContain("/api/v1/contracts/CABC");
    expect(options.method).toBe("GET");

    await waitFor(() =>
      expect(screen.getByTestId("response-status").textContent).toContain("200")
    );
    expect(screen.getByTestId("response-body").textContent).toContain(
      "contracts"
    );
  });

  it("includes a user-supplied key in the curl command but nothing by default", () => {
    render(<PlaygroundPage />);

    const curlBlock = screen.getByTestId("curl-command");
    expect(curlBlock.textContent).toContain("curl -X GET");
    expect(curlBlock.textContent).not.toContain("Authorization");

    const keyInput = document.getElementById(
      "playground-api-key"
    ) as HTMLInputElement;
    fireEvent.change(keyInput, { target: { value: "sl_user_key" } });

    expect(screen.getByTestId("curl-command").textContent).toContain(
      "Authorization: Bearer sl_user_key"
    );
  });

  it("renders a non-2xx status", async () => {
    fetchMock.mockResolvedValue(
      mockResponse(403, JSON.stringify({ error: "missing scope" }))
    );
    render(<PlaygroundPage />);

    fireEvent.click(document.getElementById("playground-send")!);

    await waitFor(() =>
      expect(screen.getByTestId("response-status").textContent).toContain("403")
    );
    expect(screen.getByTestId("response-body").textContent).toContain(
      "missing scope"
    );
  });
});
