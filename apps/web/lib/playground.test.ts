import { expect, it } from "vitest";
import { buildCurl, highlightJson } from "./playground";

it("builds a GET command without credentials or a request body", () => {
  const url = "https://example.com/api/v1/health";
  const command = buildCurl("GET", url, "", '{"unused":true}');

  expect(command).toContain(`curl -X GET '${url}'`);
  expect(command).not.toContain("Authorization:");
  expect(command).not.toContain("-d ");
});

it("includes the API key and JSON body in a POST command", () => {
  const url = "https://example.com/api/v1/contracts";
  const body = '{"id":"contract-123"}';

  const command = buildCurl("POST", url, "test-key", body);

  expect(command).toContain(`curl -X POST '${url}'`);
  expect(command).toContain("-H 'Authorization: Bearer test-key'");
  expect(command).toContain("-H 'Content-Type: application/json'");
  expect(command).toContain(`-d '${body}'`);
});

it("preserves JSON text when a string contains HTML tags", () => {
  const json = '{"message":"<b>ready</b>","count":2,"active":true}';
  const container = document.createElement("div");

  container.innerHTML = highlightJson(json);

  expect(container.textContent).toBe(json);
  expect(container.querySelector("b")).toBeNull();
});
