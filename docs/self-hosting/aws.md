# Self-hosting Sorolens on AWS

> [!WARNING]
> **Unverified: this guide has not yet been run against a real AWS account.**
> The Terraform module and Pulumi component are checked in CI with mocked
> providers and an offline `pulumi preview`, but nobody has applied them yet.
> Expect rough edges, and please report what you find on
> [#274](https://github.com/sorolens/sorolens/issues/274).

This guide deploys the whole Sorolens stack (API, indexer, dashboard,
PostgreSQL, Redis) into one AWS region with the Terraform module in
[`deploy/terraform`](../../deploy/terraform). A Pulumi (TypeScript) option is
at the end.

**Time:** about 15 minutes of hands-on work, but `terraform apply` itself
takes roughly **20–30 minutes** wall-clock, mostly Aurora (AWS says a cluster
can take up to 20 minutes to become available) and ElastiCache, which are
created in parallel. Building images the first time adds to that.

## What you get

```
                 Internet
                    │ 443 (80 redirects)
          ┌─────────▼──────────┐   public subnets (2 AZs)
          │ Application LB     │   + NAT gateway
          └──┬──────────────┬──┘
   /api/*,   │              │  everything else
   /health   │              │
┌────────────▼───┐   ┌──────▼─────────┐   ┌─────────────────┐   private subnets
│ API (Fargate)  │   │ Dashboard      │   │ Indexer         │   (2 AZs)
│ 1..n tasks     │   │ (Fargate)      │   │ exactly 1 task  │
└──────┬─────────┘   └────────────────┘   └───────┬─────────┘
       │ TLS                                       │ TLS
┌──────▼───────────────────────────────────────────▼──────┐
│ Aurora PostgreSQL 16 Serverless v2  │  ElastiCache Redis 7 │
└──────────────────────────────────────────────────────────┘
   DATABASE_URL / REDIS_URL in Secrets Manager, logs in CloudWatch
```

- The dashboard and the API share one origin: the load balancer sends
  `/api/*`, `/health` and `/readyz` to the API, everything else to the
  dashboard.
- Nothing but the load balancer is reachable from the internet. Data stores
  are encrypted at rest and in transit; passwords are generated and kept in
  Secrets Manager. Details: [modules/aws](../../deploy/terraform/modules/aws/README.md).

## Current limitations (read first)

Sorolens is not fully packaged for self-hosting yet. This guide is honest
about the gaps; each links to where it is being fixed.

| Gap | Effect on this guide |
|---|---|
| **No published images** (#96; #381 adds `apps/api/Dockerfile`) | You build and push the three images yourself (step 1). The indexer and dashboard have no Dockerfile in the repository yet. |
| **Indexer is stubbed on `main`** (`services/indexer/main.go` uses stub RPC/store/Redis) | The indexer task starts, but indexes nothing until it is wired to real dependencies. |
| **Migrations** (duplicate version numbers on `main`; #381 renumbers them and packages them) | Step 4 applies them with psql, as the repository's `docker compose` setup does, until #381 lands. |
| **Dashboard API URL is fixed at build time** (`NEXT_PUBLIC_API_URL`) | Build the dashboard image for this deployment's URL (step 1 and step 5). |

## Prerequisites

- An AWS account (a sandbox or dedicated account is recommended).
- Credentials allowed to manage: VPC/EC2 networking, Elastic Load Balancing,
  ECS, IAM roles and inline policies (including `iam:PassRole` for the two
  roles the module creates), RDS/Aurora, ElastiCache, Secrets Manager,
  CloudWatch Logs, and (optionally) Route 53 and ACM. For a sandbox,
  `AdministratorAccess` is simplest; for anything else, scope it down.
- [Terraform](https://developer.hashicorp.com/terraform/install) 1.9 or newer.
- [AWS CLI v2](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html).
- Docker, to build the images.
- Recommended: a domain in a Route 53 hosted zone and an ACM certificate for
  it **in the same region**. Without them you can only run a plain-HTTP
  evaluation stack.

```sh
export AWS_REGION=us-east-1           # your region
aws sts get-caller-identity           # confirm which account you are in
```

## 1. Build and push the images

Create one ECR repository per image:

```sh
ACCOUNT=$(aws sts get-caller-identity --query Account --output text)
REGISTRY=$ACCOUNT.dkr.ecr.$AWS_REGION.amazonaws.com
for repo in sorolens-api sorolens-indexer sorolens-dashboard; do
  aws ecr create-repository --repository-name $repo \
    --image-scanning-configuration scanOnPush=true --image-tag-mutability IMMUTABLE
done
aws ecr get-login-password | docker login --username AWS --password-stdin $REGISTRY
```

Build with an explicit tag (the module rejects `:latest`). What each image
must do:

| Image | Requirements |
|---|---|
| API | Serves on `$PORT` (8080) and answers `GET /health`. With #381: `docker build -f apps/api/Dockerfile -t $REGISTRY/sorolens-api:0.1.0 .` |
| Indexer | Entrypoint is the indexer binary (`services/indexer`); the module passes `-mode=continuous -poll-interval=5m`. |
| Dashboard | Next.js server listening on `$PORT` (3000). Build it with `NEXT_PUBLIC_API_URL=https://<your domain>` (see step 5 if you have no domain yet). |

```sh
docker push $REGISTRY/sorolens-api:0.1.0
docker push $REGISTRY/sorolens-indexer:0.1.0
docker push $REGISTRY/sorolens-dashboard:0.1.0
```

Images in ECR in the same account need nothing else. For a private registry
elsewhere, store `{"username","password"}` in Secrets Manager and pass its ARN
as `image_pull_secret_arn`.

## 2. Configure

```sh
cd deploy/terraform/examples/aws
cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars`:

| Variable | What to set |
|---|---|
| `region` | Your region, e.g. `us-east-1`. |
| `api_image`, `indexer_image`, `dashboard_image` | The three images from step 1. |
| `certificate_arn`, `domain_name`, `route53_zone_id` | HTTPS setup (recommended). The certificate must cover `domain_name`. |
| `allow_http_only = true` | Only instead of the three above, for a throwaway evaluation stack. |
| `stellar_network` | `testnet` (default), `mainnet` or `futurenet`. |
| `soroban_rpc_urls` | Required for mainnet, e.g. `{ mainnet = "https://your-provider.example.com" }`. |
| `watchdog_contract_id` | Your deployed watchdog contract, if any. |

Every module input is documented in
[modules/aws](../../deploy/terraform/modules/aws/README.md#inputs).

**State holds secrets.** The generated database password and Redis token are
in Terraform state. For anything beyond a test, enable the S3 backend block
in `versions.tf` (encrypted bucket with restricted access) before `init`.

## 3. Deploy (about 20–30 minutes)

```sh
terraform init
terraform plan -out sorolens.tfplan    # review: roughly 70 resources
terraform apply sorolens.tfplan
```

Outputs you will use:

| Output | Use |
|---|---|
| `url` | The dashboard; the API is `url` + `/api/v1`. |
| `health_url` | Quick API check. |
| `dashboard_build_api_url` | The value to build the dashboard image with. |
| `migrations_run_task_command` | Runs the migrations task (step 4). |
| `cluster_name`, `service_names`, `log_group_names` | Operations and logs. |

## 4. Run database migrations

The module creates a one-off `migrations` task definition (same secrets as the
API, private subnets, no inbound traffic) and prints the command that runs it.
Pick one of the two ways to fill it:

**A. psql, like `docker compose` does today.** The migration files on `main`
have duplicate version numbers, so the repository's own `migrate` compose
service applies them with `psql` on a fresh database. Do the same on AWS with
a small image that contains the SQL files:

```sh
# from the repository root
aws ecr create-repository --repository-name sorolens-migrations --image-tag-mutability IMMUTABLE
cat > /tmp/Dockerfile.migrations <<'EOF'
FROM postgres:16-alpine
COPY apps/api/internal/db/migrations /migrations
EOF
docker build -f /tmp/Dockerfile.migrations -t $REGISTRY/sorolens-migrations:0.1.0 .
docker push $REGISTRY/sorolens-migrations:0.1.0
```

Then in `terraform.tfvars` (mirrors the compose service, including its
"skip if the schema already exists" guard; like compose and CI, psql does not
stop on the one known harmless error in `000003_partition_events`):

```hcl
migrations_image = "123456789012.dkr.ecr.us-east-1.amazonaws.com/sorolens-migrations:0.1.0"
migrations_command = ["/bin/sh", "-c", <<-EOT
  if psql "$DATABASE_URL" -tAc "SELECT to_regclass('public.contracts')" | grep -q contracts; then
    echo "schema already present; skipping migrations"; exit 0
  fi
  for f in $(ls /migrations/*.up.sql | sort); do echo "applying $f"; psql -q "$DATABASE_URL" -f "$f"; done
  EOT
]
```

**B. The API image, once #381 lands.** #381 renumbers the migrations and
applies them when the API starts; if the final version also offers a
"migrate and exit" command, set `migrations_command` to it instead. If the API
simply migrates on startup, this step is not needed.

Then run the task and follow it in the `/ecs/<name>/migrations` log group:

```sh
terraform apply
eval "$(terraform output -raw migrations_run_task_command)"
aws logs tail /ecs/sorolens/migrations --follow
```

## 5. Check it works

```sh
curl -fsS "$(terraform output -raw health_url)"          # API health
curl -fsS "$(terraform output -raw url)/api/version"     # running API version
```

Open `terraform output -raw url` in a browser for the dashboard.

**No domain?** With `allow_http_only = true` the URL is the load balancer's
DNS name, which only exists after the first apply. Rebuild the dashboard
image with `NEXT_PUBLIC_API_URL=$(terraform output -raw dashboard_build_api_url)`,
push it under a new tag, update `dashboard_image`, and `terraform apply`
again.

Logs:

```sh
aws logs tail /ecs/sorolens/api --follow
aws logs tail /ecs/sorolens/indexer --follow
```

## Updating

Push images under new tags, change the `*_image` variables and
`terraform apply`. ECS rolls the API and dashboard with a deployment circuit
breaker (automatic rollback on failed health checks). The indexer is stopped
before its replacement starts, so two indexers never run at once.

## Teardown

Aurora is deletion-protected by default. To delete everything:

```sh
# in terraform.tfvars:
#   db_deletion_protection = false
#   db_skip_final_snapshot = true    # only if you do not want a final snapshot
terraform apply
terraform destroy
```

Left behind on purpose: the final Aurora snapshot (unless skipped; it costs
storage until you delete it), the two Secrets Manager secrets for their 7-day
recovery window, and your ECR repositories.

## Cost (estimate)

> [!NOTE]
> **Rough estimate, not a quote.** us-east-1 on-demand list prices with the
> module defaults, before storage, I/O, data transfer and free tier. Check the
> [AWS Pricing Calculator](https://calculator.aws/) for your region and usage.

| Item | Default size | ≈ USD / month |
|---|---|---|
| Aurora PostgreSQL Serverless v2 | 0.5 ACU minimum, billed around the clock (`db_min_capacity = 0` allows auto-pause) | ~44 + storage/I/O |
| NAT gateway | 1 | ~33 + data processed |
| Application Load Balancer | 1 | ~16 + LCUs |
| ECS Fargate | 3 tasks × 0.25 vCPU / 0.5 GB | ~27 |
| ElastiCache | 1 × cache.t4g.micro | ~12 |
| Secrets Manager, CloudWatch Logs | 2 secrets, 30-day logs | a few dollars |
| **Total** | | **roughly 130–150** |

`single_nat_gateway = false`, `db_instance_count = 2` and
`redis_num_cache_clusters = 2` buy availability at extra cost.

## Pulumi instead of Terraform

The same stack is available as a Pulumi component,
[`SorolensAws`](../../deploy/pulumi). With the [Pulumi CLI](https://www.pulumi.com/docs/iac/download-install/):

```sh
cd deploy/pulumi && pnpm install && cd examples/aws
pulumi stack init prod
pulumi config set aws:region us-east-1
pulumi config set apiImage $REGISTRY/sorolens-api:0.1.0
pulumi config set indexerImage $REGISTRY/sorolens-indexer:0.1.0
pulumi config set dashboardImage $REGISTRY/sorolens-dashboard:0.1.0
pulumi config set certificateArn arn:aws:acm:...
pulumi config set domainName sorolens.example.com
pulumi config set route53ZoneId Z0000000000000000000
pulumi up
```

Steps 1, 4 and 5 are identical; outputs have the same names in camelCase
(`url`, `healthUrl`, `migrationsRunTaskCommand`, ...). Pulumi encrypts secret
values in its state.
