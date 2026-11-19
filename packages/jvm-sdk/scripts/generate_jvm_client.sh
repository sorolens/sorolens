#!/usr/bin/env bash
# Regenerate the generated JVM models from docs/openapi.yaml.
#
# Generated sources are NOT committed: only openapi-generator-config.yaml is
# reviewed. This script runs in the existing `sdks` CI job (not a new workflow)
# so the pinned generator and the spec are exercised on every relevant change.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
repo_root="$(cd "$here/../.." && pwd)"
spec="${1:-$repo_root/docs/openapi.yaml}"
config="$here/openapi-generator-config.yaml"
generator_version="7.10.0"
jar="$here/build/openapi-generator-cli-$generator_version.jar"

if [[ ! -f "$spec" ]]; then
  echo "OpenAPI spec not found at $spec" >&2
  exit 1
fi
if [[ ! -f "$config" ]]; then
  echo "Generator config not found at $config" >&2
  exit 1
fi

mkdir -p "$here/build"
if [[ ! -f "$jar" ]]; then
  curl -fsSL -o "$jar" \
    "https://repo1.maven.org/maven2/org/openapitools/openapi-generator-cli/$generator_version/openapi-generator-cli-$generator_version.jar"
fi

rm -rf "$here/build/generated/openapi"
# docs/openapi.yaml uses `deprecated` on security schemes, a valid OpenAPI 3.1
# field that OpenAPI Generator 7.10's strict validator rejects.
java -jar "$jar" generate \
  --config "$config" \
  --input-spec "$spec" \
  --output "$here/build/generated/openapi" \
  --skip-validate-spec

echo "Regenerated $here/build/generated/openapi (openapi-generator $generator_version)"
