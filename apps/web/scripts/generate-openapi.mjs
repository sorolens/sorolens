import { readFile, writeFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import yaml from "js-yaml";
import prettier from "prettier";

const scriptDirectory = dirname(fileURLToPath(import.meta.url));
const repositoryRoot = resolve(scriptDirectory, "../../..");
const sourcePath = resolve(repositoryRoot, "docs/openapi.yaml");
const outputPath = resolve(scriptDirectory, "../lib/openapi.generated.ts");
const httpMethods = [
  "get",
  "post",
  "put",
  "patch",
  "delete",
  "options",
  "head",
];

function makeParameter(parameter) {
  if (parameter.in !== "path" && parameter.in !== "query") return null;
  const schema = parameter.schema ?? {};
  return {
    name: parameter.name,
    in: parameter.in,
    required: Boolean(parameter.required),
    ...(parameter.description || schema.description
      ? { description: parameter.description ?? schema.description }
      : {}),
    type: schema.type === "integer" ? "integer" : "string",
    ...(schema.default !== undefined
      ? { default: String(schema.default) }
      : {}),
  };
}

function makeRequestBody(requestBody) {
  if (!requestBody) return undefined;
  const content = requestBody.content ?? {};
  const entries = Object.entries(content);
  const [mediaType, media] =
    entries.find(([type]) => type.includes("json")) ?? entries[0] ?? [];
  if (!mediaType || !media) return undefined;

  const example =
    media.example ??
    Object.values(media.examples ?? {}).find(
      (value) => value?.value !== undefined
    )?.value ??
    media.schema?.example ??
    {};
  return {
    ...(requestBody.description
      ? { description: requestBody.description }
      : {}),
    example:
      typeof example === "string" ? example : JSON.stringify(example, null, 2),
  };
}

function resolveParameter(parameter, source) {
  if (!parameter.$ref) return parameter;
  const name = parameter.$ref
    .split("/")
    .at(-1)
    .replaceAll("~1", "/")
    .replaceAll("~0", "~");
  return source.components?.parameters?.[name];
}

function makeDocument(source) {
  const paths = {};
  for (const [path, pathItem] of Object.entries(source.paths ?? {})) {
    const operations = {};
    for (const method of httpMethods) {
      const operation = pathItem[method];
      if (!operation || typeof operation !== "object") continue;

      const parameters = new Map();
      for (const parameter of [
        ...(pathItem.parameters ?? []),
        ...(operation.parameters ?? []),
      ]) {
        const resolved = resolveParameter(parameter, source);
        const converted = resolved ? makeParameter(resolved) : null;
        if (converted)
          parameters.set(`${converted.in}:${converted.name}`, converted);
      }

      const requestBody = makeRequestBody(operation.requestBody);
      operations[method] = {
        operationId: operation.operationId ?? `${method}_${path}`,
        summary:
          operation.summary ?? operation.operationId ?? `${method} ${path}`,
        tags: operation.tags?.length ? operation.tags : ["Other"],
        parameters: [...parameters.values()],
        ...(requestBody ? { requestBody } : {}),
      };
    }
    if (Object.keys(operations).length > 0) paths[path] = operations;
  }

  return {
    openapi: source.openapi,
    info: { title: source.info.title, version: source.info.version },
    paths,
  };
}

const source = yaml.load(await readFile(sourcePath, "utf8"));
const document = makeDocument(source);
const generated = await prettier.format(
  `import type { OpenAPIDocument } from "./openapi";\n\nexport const generatedOpenApiDocument: OpenAPIDocument = ${JSON.stringify(document, null, 2)};\n`,
  { filepath: outputPath }
);

if (process.argv.includes("--check")) {
  let current;
  try {
    current = await readFile(outputPath, "utf8");
  } catch {
    throw new Error("OpenAPI output is missing; run pnpm openapi:generate");
  }
  if (current !== generated) {
    throw new Error("OpenAPI output is stale; run pnpm openapi:generate");
  }
} else {
  await writeFile(outputPath, generated);
}
