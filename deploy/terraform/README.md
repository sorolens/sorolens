# Sorolens Terraform

Terraform modules that provision a self-hosted Sorolens stack (API, indexer,
dashboard, PostgreSQL, Redis).

```
deploy/terraform/
├── modules/
│   ├── core/     cloud-agnostic settings: validation + per-service environment
│   └── aws/      ECS Fargate, Aurora PostgreSQL, ElastiCache, ALB, Secrets Manager
└── examples/
    └── aws/      ready-to-apply root module (region, provider, backend)
```

Status: **AWS only** so far (#274 is being delivered in phases; GCP and Fly.io
are not started). Start with the [AWS install guide](../../docs/self-hosting/aws.md).
A Pulumi (TypeScript) equivalent is in [deploy/pulumi](../pulumi).

## Design

- One module per cloud, sharing `modules/core`. Each cloud module takes a
  region through its provider and the same app settings, so adding a cloud
  does not change the others.
- Secure by default: private data stores, encryption at rest and in transit,
  generated secrets in the cloud secret manager, HTTPS unless explicitly opted
  out, least-privilege IAM. See [modules/aws](modules/aws/README.md).
- No published images are assumed: image references are required inputs
  (see #96 / #381).

## Versions

| | |
|---|---|
| Terraform | >= 1.9.0 to use the modules; >= 1.11 to run their tests |
| hashicorp/aws | ~> 6.0 |
| hashicorp/random | ~> 3.6 |

`.terraform.lock.hcl` files are not committed: modules are consumed with the
caller's provider versions, and the example is a template you copy.

## Development

Everything below runs without cloud credentials and creates nothing.

```sh
terraform fmt -check -recursive

for dir in modules/core modules/aws examples/aws; do
  terraform -chdir=$dir init -backend=false
  terraform -chdir=$dir validate
  terraform -chdir=$dir test      # mock providers, plan only
done

tflint --init --config "$PWD/.tflint.hcl"
for dir in modules/core modules/aws examples/aws; do
  tflint --chdir=$dir --config "$PWD/.tflint.hcl"
done
```

CI runs the same steps in `.github/workflows/iac.yml`, plus a validate-only
pass on Terraform 1.9 to keep the minimum version honest.
