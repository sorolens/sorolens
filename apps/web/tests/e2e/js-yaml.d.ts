/**
 * Minimal ambient types for `js-yaml`, which ships without declarations and is
 * only used by the E2E OpenAPI fixture validator (#766).
 */
declare module "js-yaml" {
  export function load(input: string): unknown;
}
