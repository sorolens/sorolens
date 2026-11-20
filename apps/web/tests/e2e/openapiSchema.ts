/**
 * Validates Playwright fixtures against `docs/openapi.yaml`.
 *
 * The E2E specs mock the API with hand-written JSON, so a schema change in the
 * spec would otherwise drift silently. This module loads the committed OpenAPI
 * document, resolves local `$ref`s and validates fixtures against the response
 * schema of the operation they stand in for.
 *
 * Only the JSON Schema subset the Sorolens spec uses is implemented: `$ref`,
 * `type`, `required`, `properties`, `items`, `enum`, `nullable` and the
 * `allOf`/`anyOf`/`oneOf` composition keywords.
 *
 * Issue #766.
 */

import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
import yaml from "js-yaml";

interface JSONSchema {
  $ref?: string;
  type?: string;
  properties?: Record<string, JSONSchema>;
  required?: string[];
  items?: JSONSchema;
  enum?: unknown[];
  nullable?: boolean;
  allOf?: JSONSchema[];
  anyOf?: JSONSchema[];
  oneOf?: JSONSchema[];
}

interface OpenAPIDocument {
  paths: Record<
    string,
    Record<
      string,
      {
        responses?: Record<
          string,
          { content?: Record<string, { schema?: JSONSchema }> }
        >;
      }
    >
  >;
  components?: {
    schemas?: Record<string, JSONSchema>;
    responses?: Record<
      string,
      { content?: Record<string, { schema?: JSONSchema }> }
    >;
  };
}

function specPath(): string {
  for (const candidate of [
    resolve(process.cwd(), "docs/openapi.yaml"),
    resolve(process.cwd(), "../../docs/openapi.yaml"),
  ]) {
    if (existsSync(candidate)) return candidate;
  }
  throw new Error(
    "docs/openapi.yaml not found from the Playwright process cwd"
  );
}

let document: OpenAPIDocument | undefined;

function openApi(): OpenAPIDocument {
  if (!document) {
    document = yaml.load(readFileSync(specPath(), "utf8")) as OpenAPIDocument;
  }
  return document;
}

function resolveRef(ref: string, root: OpenAPIDocument): JSONSchema {
  const segments = ref.replace(/^#\//, "").split("/");
  let current: unknown = root;
  for (const segment of segments) {
    if (typeof current !== "object" || current === null) {
      throw new Error(`cannot resolve OpenAPI $ref ${ref}`);
    }
    current = (current as Record<string, unknown>)[segment];
  }
  if (typeof current !== "object" || current === null) {
    throw new Error(`cannot resolve OpenAPI $ref ${ref}`);
  }
  return current as JSONSchema;
}

function typeLabel(value: unknown): string {
  if (value === null) return "null";
  if (Array.isArray(value)) return "array";
  return typeof value;
}

function validate(
  schema: JSONSchema,
  value: unknown,
  root: OpenAPIDocument,
  path: string
): string[] {
  if (schema.$ref) {
    return validate(resolveRef(schema.$ref, root), value, root, path);
  }

  if (value === null && schema.nullable) return [];

  const errors: string[] = [];

  for (const composed of schema.allOf ?? []) {
    errors.push(...validate(composed, value, root, path));
  }

  const alternatives = schema.anyOf ?? schema.oneOf;
  if (alternatives && alternatives.length > 0) {
    const matches = alternatives.some(
      (alternative) => validate(alternative, value, root, path).length === 0
    );
    if (!matches) errors.push(`${path}: no anyOf/oneOf alternative matched`);
    return errors;
  }

  if (schema.enum && !schema.enum.includes(value)) {
    errors.push(
      `${path}: ${JSON.stringify(value)} is not one of ${JSON.stringify(schema.enum)}`
    );
  }

  switch (schema.type) {
    case "object": {
      if (typeof value !== "object" || value === null || Array.isArray(value)) {
        errors.push(`${path}: expected object, got ${typeLabel(value)}`);
        break;
      }
      const record = value as Record<string, unknown>;
      for (const key of schema.required ?? []) {
        if (!(key in record)) errors.push(`${path}: missing required "${key}"`);
      }
      for (const [key, property] of Object.entries(schema.properties ?? {})) {
        if (key in record) {
          errors.push(
            ...validate(property, record[key], root, `${path}.${key}`)
          );
        }
      }
      break;
    }
    case "array": {
      if (!Array.isArray(value)) {
        errors.push(`${path}: expected array, got ${typeLabel(value)}`);
        break;
      }
      if (schema.items) {
        value.forEach((item, index) => {
          errors.push(
            ...validate(schema.items!, item, root, `${path}[${index}]`)
          );
        });
      }
      break;
    }
    case "integer":
      if (!Number.isInteger(value)) {
        errors.push(`${path}: expected integer, got ${typeLabel(value)}`);
      }
      break;
    case "number":
      if (typeof value !== "number" || Number.isNaN(value)) {
        errors.push(`${path}: expected number, got ${typeLabel(value)}`);
      }
      break;
    case "string":
      if (typeof value !== "string") {
        errors.push(`${path}: expected string, got ${typeLabel(value)}`);
      }
      break;
    case "boolean":
      if (typeof value !== "boolean") {
        errors.push(`${path}: expected boolean, got ${typeLabel(value)}`);
      }
      break;
    default:
      break;
  }

  return errors;
}

function assertValid(
  schema: JSONSchema | undefined,
  value: unknown,
  label: string
): void {
  if (!schema) throw new Error(`no OpenAPI schema found for ${label}`);
  const errors = validate(schema, value, openApi(), label);
  if (errors.length > 0) {
    throw new Error(
      `Fixture does not match docs/openapi.yaml (${label}):\n${errors.join("\n")}`
    );
  }
}

/** Validate a fixture against a named schema under `components/schemas`. */
export function expectMatchesSchema(schemaName: string, value: unknown): void {
  assertValid(
    openApi().components?.schemas?.[schemaName],
    value,
    `components.schemas.${schemaName}`
  );
}

/**
 * Validate a fixture against the JSON response schema declared for an
 * operation, e.g. `expectMatchesResponse("/api/v1/compare", "get", body)`.
 */
export function expectMatchesResponse(
  path: string,
  method: "get" | "post" | "put" | "patch" | "delete",
  value: unknown,
  status = "200"
): void {
  const operation = openApi().paths[path]?.[method];
  const schema =
    operation?.responses?.[status]?.content?.["application/json"]?.schema;
  assertValid(schema, value, `${method.toUpperCase()} ${path} ${status}`);
}
