/**
 * Validates the Playwright e2e fixtures (apps/web/tests/e2e/helpers.ts)
 * against the schemas in docs/openapi.yaml (issue #599).
 *
 * The spec is the single source of truth for the API shape. Fixtures that
 * drift from it produce mock responses the real API would never send, so
 * e2e tests can pass against a mock while the production page is broken.
 *
 * Each fixture factory below is mapped to the spec schema it claims to
 * represent. The mapping grows incrementally as hand-written fixtures and
 * types are migrated to the spec (no hand-written duplicates for migrated
 * endpoints). Fixtures that still predate the spec are listed in
 * `pending` with a comment so the drift is visible rather than silent.
 */

import { readFileSync } from "fs";
import { join } from "path";
import { describe, expect, it } from "vitest";
import { load } from "js-yaml";
import Ajv, { type AnySchema } from "ajv";
import {
  alertGroup,
  contractEvent,
  monitoredContract,
  resourceTrendPoint,
  storageEntry,
  watchdogStats,
} from "@/tests/e2e/helpers";

const specPath = join(__dirname, "..", "..", "..", "docs", "openapi.yaml");
const spec = load(readFileSync(specPath, "utf8")) as {
  components: { schemas: Record<string, AnySchema> };
};

const ajv = new Ajv({ allErrors: true, strict: false });
// Register every spec schema so `$ref: "#/components/schemas/X"` resolves.
for (const [name, schema] of Object.entries(spec.components.schemas)) {
  ajv.addSchema(schema, `#/components/schemas/${name}`);
}

function validate(schemaName: string, data: unknown): string[] {
  const validate = ajv.getSchema(`#/components/schemas/${schemaName}`);
  if (!validate) {
    return [`schema ${schemaName} not found in spec`];
  }
  if (!validate(data)) {
    return Object.values(validate.errors ?? []).map(
      (e) => `${e.instancePath ?? "(root)"} ${e.message}`
    );
  }
  return [];
}

// ---- fixtures validated against spec schemas ------------------------------

const fixtures: Array<{
  name: string;
  schema: string;
  data: unknown;
}> = [
  {
    name: "watchdogStats",
    schema: "WatchdogStats",
    data: watchdogStats(),
  },
  {
    name: "monitoredContract",
    schema: "MonitoredContract",
    data: monitoredContract(),
  },
  {
    name: "alertGroup",
    schema: "AlertGroup",
    data: alertGroup(),
  },
  {
    name: "contractEvent",
    schema: "Event",
    data: contractEvent(),
  },
  {
    name: "storageEntry",
    schema: "StorageEntry",
    data: storageEntry(),
  },
  {
    name: "resourceTrendPoint",
    schema: "ResourceTrendPoint",
    data: resourceTrendPoint(),
  },
];

describe("Playwright fixtures vs OpenAPI spec", () => {
  it("every registered spec schema compiles", () => {
    for (const name of Object.keys(spec.components.schemas)) {
      const validate = ajv.getSchema(`#/components/schemas/${name}`);
      expect(validate).toBeDefined();
    }
  });

  it.each(fixtures)(
    "fixture $name conforms to spec schema $schema",
    ({ name, schema, data }) => {
      const errors = validate(schema, data);
      expect(
        errors,
        `fixture ${name} does not conform to ${schema}: ${errors.join(", ")}`
      ).toEqual([]);
    }
  );
});
