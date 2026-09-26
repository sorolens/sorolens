# Sorolens Python SDK

Official, typed Python client for the [Sorolens](https://github.com/sorolens/sorolens)
Soroban indexing API.

It gives you two layers:

* **`sorolens._generated`** — the low-level client generated from
  [`docs/openapi.yaml`](https://github.com/sorolens/sorolens/blob/main/docs/openapi.yaml)
  with `openapi-python-client`. One module per operation, one model per schema.
* **`sorolens.Client` / `sorolens.AsyncClient`** — a hand-written facade with
  namespaced resources (`client.contracts`, `client.events`, ...), typed return
  values, sync and async `httpx` transports, and a small exception hierarchy.

## Install

```bash
pip install sorolens
```

## Next steps

* [Quickstart](quickstart.md) — authenticate, page through results, handle errors.
* [API reference](api.md) — the facade and the exception hierarchy.
