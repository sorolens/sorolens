import { generatedOpenApiDocument } from "./openapi.generated";

export type HttpMethod =
  "GET" | "POST" | "PUT" | "PATCH" | "DELETE" | "OPTIONS" | "HEAD";

export interface OpenAPIParameter {
  name: string;
  in: "path" | "query";
  required?: boolean;
  description?: string;
  type?: "string" | "integer";
  default?: string;
}

export interface OpenAPIRequestBody {
  description?: string;
  example: string;
}

export interface OpenAPIOperation {
  operationId: string;
  summary: string;
  tags: string[];
  parameters: OpenAPIParameter[];
  requestBody?: OpenAPIRequestBody;
}

export type OpenAPIPathItem = Partial<
  Record<Lowercase<HttpMethod>, OpenAPIOperation>
>;

export interface OpenAPIDocument {
  openapi: string;
  info: { title: string; version: string };
  paths: Record<string, OpenAPIPathItem>;
}

export const openApiDocument = generatedOpenApiDocument;

export interface EndpointEntry {
  method: HttpMethod;
  path: string;
  operation: OpenAPIOperation;
}

const methods: HttpMethod[] = [
  "GET",
  "POST",
  "PUT",
  "PATCH",
  "DELETE",
  "OPTIONS",
  "HEAD",
];

export function listEndpoints(
  doc: OpenAPIDocument = openApiDocument
): EndpointEntry[] {
  const endpoints: EndpointEntry[] = [];
  for (const [path, pathItem] of Object.entries(doc.paths)) {
    for (const method of methods) {
      const operation = pathItem[method.toLowerCase() as Lowercase<HttpMethod>];
      if (operation) endpoints.push({ method, path, operation });
    }
  }
  return endpoints;
}

export function endpointsByTag(
  doc: OpenAPIDocument = openApiDocument
): { tag: string; endpoints: EndpointEntry[] }[] {
  const groups = new Map<string, EndpointEntry[]>();
  for (const endpoint of listEndpoints(doc)) {
    const tag = endpoint.operation.tags[0] ?? "Other";
    const group = groups.get(tag) ?? [];
    group.push(endpoint);
    groups.set(tag, group);
  }
  return Array.from(groups, ([tag, endpoints]) => ({ tag, endpoints }));
}
