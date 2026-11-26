/**
 * Fixture/spec drift guard (issue #599).
 *
 * Playwright mocks responses by hand. Those fixtures drifted from the API
 * before, so every fixture that stands in for a documented response is
 * validated against docs/openapi.yaml here. A spec change that the fixtures do
 * not follow fails this test instead of surfacing only in a browser run.
 */
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import Ajv from "ajv";
import yaml from "js-yaml";
import { describe, expect, it } from "vitest";
import {
  contractDetail,
  contractEvent,
  contractInvocation,
  resourceTrendResponse,
} from "./e2e/helpers";

const specPath = resolve(__dirname, "../../../docs/openapi.yaml");
const spec = yaml.load(readFileSync(specPath, "utf8")) as Record<string, any>;

const ajv = new Ajv({ strict: false, allErrors: true });
// OpenAPI numeric formats are annotations; JSON numbers already cover them.
ajv.addFormat("double", true);
ajv.addFormat("int64", true);
ajv.addFormat("int32", true);
ajv.addFormat("date-time", true);

/** Resolves local JSON pointers and converts OpenAPI `nullable` to JSON Schema. */
function normalize(node: any, seen = new Set<string>()): any {
  if (Array.isArray(node)) return node.map((item) => normalize(item, seen));
  if (!node || typeof node !== "object") return node;

  if (typeof node.$ref === "string" && node.$ref.startsWith("#/")) {
    if (seen.has(node.$ref)) return {};
    const target = node.$ref
      .slice(2)
      .split("/")
      .reduce((value: any, key: string) => value?.[key], spec);
    return normalize(target, new Set([...seen, node.$ref]));
  }

  const out: Record<string, any> = {};
  for (const [key, value] of Object.entries(node)) {
    if (key === "nullable") continue;
    out[key] = normalize(value, seen);
  }
  if (node.nullable && typeof out.type === "string") {
    out.type = [out.type, "null"];
  }
  return out;
}

function validateFixture(name: string, body: unknown, schema: unknown) {
  const validate = ajv.compile(normalize(schema));
  const ok = validate(body);
  expect(
    ok,
    `${name} does not match the OpenAPI schema: ${JSON.stringify(
      validate.errors,
      null,
      2
    )}`
  ).toBe(true);
}

const operationResponse = (path: string, operation: string) =>
  spec.paths[path][operation].responses["200"].content["application/json"]
    .schema;

describe("Playwright fixtures match docs/openapi.yaml", () => {
  it("contract detail matches the Contract schema", () => {
    validateFixture(
      "contractDetail",
      contractDetail(),
      spec.components.schemas.Contract
    );
  });

  it("contract event matches the Event schema", () => {
    validateFixture(
      "contractEvent",
      contractEvent(),
      spec.components.schemas.Event
    );
  });

  it("contract invocation matches the Invocation schema", () => {
    validateFixture(
      "contractInvocation",
      contractInvocation(),
      spec.components.schemas.Invocation
    );
  });

  it("resource trend matches the documented response", () => {
    validateFixture(
      "resourceTrendResponse",
      resourceTrendResponse(),
      operationResponse("/api/v1/contracts/{id}/resource-trend", "get")
    );
  });
});
