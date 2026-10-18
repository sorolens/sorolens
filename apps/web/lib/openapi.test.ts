import { describe, expect, it } from "vitest";
import { listEndpoints } from "./openapi";

describe("generated OpenAPI playground catalog", () => {
  it("includes the complete operation surface and request metadata", () => {
    const endpoints = listEndpoints();

    expect(endpoints.length).toBeGreaterThan(50);
    expect(
      endpoints.some(
        ({ method, path }) =>
          method === "GET" && path === "/api/v1/contracts/{id}/snapshot"
      )
    ).toBe(true);
    expect(
      endpoints.some(
        ({ method, path }) => method === "POST" && path === "/graphql"
      )
    ).toBe(true);
    expect(
      endpoints.every(
        ({ operation }) =>
          operation.operationId && operation.summary && operation.tags.length
      )
    ).toBe(true);
  });
});
