import { afterEach, expect, it, vi } from "vitest";
import { getAPIHealth } from "./api";

afterEach(() => {
  vi.unstubAllGlobals();
});

it("returns the parsed health data when the API responds successfully", async () => {
  const health = { status: "ok" };
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json(health)));

  const result = await getAPIHealth();

  expect(result).toEqual(health);
});

it("turns a 503 response into an ApiError with the server's message and code", async () => {
  const response = Response.json(
    { error: "Service unavailable", code: "UNAVAILABLE" },
    { status: 503 }
  );
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response));

  await expect(getAPIHealth()).rejects.toMatchObject({
    name: "ApiError",
    status: 503,
    message: "Service unavailable",
    code: "UNAVAILABLE",
  });
});

it("uses the HTTP status text when the error body is plain text", async () => {
  const response = new Response("Server temporarily offline", {
    status: 502,
    statusText: "Bad Gateway",
  });
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response));

  await expect(getAPIHealth()).rejects.toMatchObject({
    name: "ApiError",
    status: 502,
    message: "Bad Gateway",
  });
});

it("passes the timeout error back to the caller when fetch times out", async () => {
  const timeout = new DOMException("Request timed out", "TimeoutError");
  vi.stubGlobal("fetch", vi.fn().mockRejectedValue(timeout));

  await expect(getAPIHealth()).rejects.toBe(timeout);
});

it("rejects with a SyntaxError when a successful response contains invalid JSON", async () => {
  const response = new Response('{"status":', {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response));

  await expect(getAPIHealth()).rejects.toMatchObject({
    name: "SyntaxError",
  });
});
