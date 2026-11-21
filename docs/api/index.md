# API

Sorolens exposes a REST API, described by an OpenAPI 3.0 spec in
[`docs/openapi.yaml`](https://github.com/sorolens/sorolens/blob/main/docs/openapi.yaml).

- **[API reference](reference.md)**: every endpoint, parameter and schema,
  generated from that spec when the site is built.
- **[Authentication and API keys](../authentication.md)**: scopes, roles and
  key rotation, with curl examples.
- **[API v2](../api-v2.md)**: what changed between `/api/v1` and `/api/v2`.
- **[Webhooks](../webhooks.md)**: signing and verifying deliveries.

The production server is `https://api.sorolens.xyz`. A local API listens on
`http://localhost:8080`.

## Keeping the reference current

The reference is never edited by hand. Change `docs/openapi.yaml` together
with the route, and run `make openapi`: it fails if a route is undocumented,
lints the spec and regenerates the Go client. The next docs build picks up the
change.
