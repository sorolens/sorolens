# Sorolens on AWS (Terraform module)

Provisions a complete, self-hosted Sorolens stack in one AWS region:

| Piece | AWS service |
|---|---|
| API, dashboard | ECS Fargate services behind an Application Load Balancer (`/api/*`, `/health`, `/readyz` go to the API, everything else to the dashboard) |
| Indexer | ECS Fargate service, **exactly one task** (see below) |
| Migrations | ECS task definition, run on demand (`migrations_run_task_command` output) |
| PostgreSQL 16 | Aurora PostgreSQL Serverless v2 |
| Redis 7 | ElastiCache for Redis OSS |
| Secrets | Secrets Manager (`DATABASE_URL`, `REDIS_URL`) |
| Network | New VPC: 2 public subnets (load balancer, NAT), 2 private subnets (tasks, database, cache) |
| Logs | CloudWatch Logs, one group per container |

The install walkthrough is in [docs/self-hosting/aws.md](../../../../docs/self-hosting/aws.md).
A Pulumi (TypeScript) equivalent lives in [deploy/pulumi](../../../pulumi).

## Usage

```hcl
module "sorolens" {
  source = "github.com/sorolens/sorolens//deploy/terraform/modules/aws?ref=<commit or tag>"

  api_image       = "123456789012.dkr.ecr.us-east-1.amazonaws.com/sorolens-api:0.1.0"
  indexer_image   = "123456789012.dkr.ecr.us-east-1.amazonaws.com/sorolens-indexer:0.1.0"
  dashboard_image = "123456789012.dkr.ecr.us-east-1.amazonaws.com/sorolens-dashboard:0.1.0"

  certificate_arn = "arn:aws:acm:us-east-1:123456789012:certificate/..."
  domain_name     = "sorolens.example.com"
  route53_zone_id = "Z0000000000000000000"
}
```

The region comes from your `aws` provider configuration; see
[examples/aws](../../examples/aws) for a complete root module.

## Secure by default

- Aurora and ElastiCache sit in private subnets and accept connections only
  from the Sorolens task security groups. Aurora instances are never publicly
  accessible.
- Encryption at rest for Aurora and Redis; TLS in transit
  (`sslmode=require`, `rediss://` with an AUTH token).
- Database password and Redis token are generated (`random_password`) and
  stored in Secrets Manager. Containers receive `DATABASE_URL`,
  `DIRECT_DATABASE_URL` and `REDIS_URL` through ECS `secrets`, never as plain
  environment variables. **The generated values are also in Terraform state**:
  use an encrypted, access-controlled backend.
- HTTPS with a TLS 1.2/1.3 policy and an HTTP to HTTPS redirect. Plain HTTP
  needs an explicit `allow_http_only = true` (evaluation stacks only).
- Least privilege: the ECS execution role can read only the secrets the task
  definitions reference; the containers' task role has no permissions (Sorolens
  calls no AWS APIs). ECS Exec is disabled.
- The VPC's default security group is emptied; the load balancer drops invalid
  headers; Aurora is deletion-protected and keeps a final snapshot by default.

## Things to know before you deploy

- **Images are not published yet.** Sorolens has no official images (#96;
  #381 adds the API Dockerfile). `*_image` inputs are required and must carry
  an explicit tag or digest (`:latest` is rejected).
- **The indexer runs as exactly one task.** It holds a Redis single-writer lock
  and the only shard store on `main` is in-memory, so the service has
  `desired_count = 1` and stop-before-start deployments. Note that on `main` the
  indexer binary is still wired to stub dependencies, so it starts but does not
  index yet.
- **Migrations** run as a one-off task (`migrations_run_task_command`). What
  command applies them depends on how #381 packages migrations; set
  `migrations_command` accordingly. The migration files on `main` currently
  have duplicate version numbers, which #381 renumbers.
- **Dashboard URL is baked in at build time.** Next.js inlines
  `NEXT_PUBLIC_API_URL` when the image is built. Build the dashboard image with
  the `dashboard_build_api_url` output (the stack's own origin).

## Requirements

| Name | Version |
|---|---|
| Terraform | >= 1.9.0 (running `tests/` needs >= 1.11) |
| hashicorp/aws | ~> 6.0 |
| hashicorp/random | ~> 3.6 |

## Inputs

| Name | Type | Default | Description |
|---|---|---|---|
| `name` | `string` | `"sorolens"` | Name prefix for every resource. Lowercase letters, digits and hyphens, at most 20 characters. |
| `tags` | `map(string)` | `{}` | Tags added to every resource that supports them. |
| `api_image` | `string` | **required** | API container image with an explicit tag or digest, e.g. ghcr.io/org/sorolens-api:1.2.3. Sorolens does not publish images yet (#96, #381), so there is no default. |
| `indexer_image` | `string` | **required** | Indexer container image with an explicit tag or digest. Its entrypoint must be the indexer binary, which receives -mode=continuous. No default: see api_image. |
| `dashboard_image` | `string` | **required** | Dashboard (Next.js) container image with an explicit tag or digest. NEXT_PUBLIC_API_URL is inlined when the image is built, so build it for this deployment's public URL. No default: see api_image. |
| `migrations_image` | `string` | `null` | Image for the one-off migrations task. Defaults to api_image. |
| `migrations_command` | `list(string)` | `null` | Command override for the migrations task, i.e. whatever applies pending migrations and exits in your migrations_image. null keeps the image's default command. Depends on how #381 packages migrations. |
| `image_pull_secret_arn` | `string` | `null` | Optional Secrets Manager secret ARN with {"username","password"} for a private registry. Not needed for public images or ECR in the same account. |
| `cpu_architecture` | `string` | `"X86_64"` | CPU architecture of the images: X86_64 or ARM64. |
| `stellar_network` | `string` | `"testnet"` | Default Stellar network: testnet, mainnet or futurenet. |
| `soroban_rpc_urls` | `map(string)` | `{}` | Soroban RPC endpoint per network. testnet and futurenet default to the public SDF endpoints; mainnet must be set when used. |
| `watchdog_contract_id` | `string` | `null` | Deployed sorolens-watchdog contract ID; enables watchdog processing in the indexer. |
| `log_level` | `string` | `"info"` | API log level: debug, info, warn or error. |
| `allowed_origins` | `list(string)` | `[]` | Extra browser origins allowed to call the API. The dashboard is served from the same origin, so this is usually empty. |
| `indexer_poll_interval` | `string` | `"5m"` | Sleep between indexer passes (Go duration). |
| `api_extra_environment` | `map(string)` | `{}` | Additional plain environment variables for the API and migrations containers (e.g. SENTRY_ENVIRONMENT). Must not contain secrets. |
| `api_extra_secrets` | `map(string)` | `{}` | Additional secrets for the API and migrations containers: environment variable name => Secrets Manager secret ARN (e.g. SENTRY_DSN, SLACK_SIGNING_SECRET). |
| `indexer_extra_environment` | `map(string)` | `{}` | Additional plain environment variables for the indexer (e.g. INDEXER_ANOMALY_ENABLED). Must not contain secrets. |
| `indexer_extra_secrets` | `map(string)` | `{}` | Additional secrets for the indexer: environment variable name => Secrets Manager secret ARN. |
| `api_cpu` | `number` | `256` | Fargate CPU units for each API task (256 = 0.25 vCPU). |
| `api_memory` | `number` | `512` | Fargate memory (MiB) for each API task. |
| `api_desired_count` | `number` | `1` | Number of API tasks. The API is stateless and can scale horizontally. |
| `dashboard_cpu` | `number` | `256` | Fargate CPU units for each dashboard task. |
| `dashboard_memory` | `number` | `512` | Fargate memory (MiB) for each dashboard task. |
| `dashboard_desired_count` | `number` | `1` | Number of dashboard tasks. |
| `indexer_cpu` | `number` | `256` | Fargate CPU units for the indexer task. The indexer always runs as exactly one task. |
| `indexer_memory` | `number` | `512` | Fargate memory (MiB) for the indexer task. |
| `vpc_cidr` | `string` | `"10.40.0.0/16"` | CIDR block of the VPC. Split into two public and two private subnets. |
| `availability_zones` | `list(string)` | `[]` | Exactly two availability zones to use. Empty means the first two available zones in the region. |
| `single_nat_gateway` | `bool` | `true` | Use one NAT gateway for both private subnets (cheaper) instead of one per zone (survives a zone outage). |
| `certificate_arn` | `string` | `null` | ACM certificate ARN for HTTPS on the load balancer. Required unless allow_http_only is true. |
| `allow_http_only` | `bool` | `false` | Explicit opt-in to serve plain HTTP when no certificate_arn is given. Only for short-lived evaluation stacks. |
| `domain_name` | `string` | `null` | Public hostname for the stack, e.g. sorolens.example.com. Needs a certificate covering it. When route53_zone_id is also set, an alias record is created. |
| `route53_zone_id` | `string` | `null` | Route 53 hosted zone ID in which to create the domain_name alias record. |
| `alb_ingress_cidrs` | `list(string)` | `["0.0.0.0/0"]` | IPv4 CIDR blocks allowed to reach the load balancer. |
| `alb_deletion_protection` | `bool` | `false` | Enable deletion protection on the load balancer. |
| `db_engine_version` | `string` | `"16.6"` | Aurora PostgreSQL engine version (major 16, matching docker-compose). Must be available in the region: aws rds describe-db-engine-versions --engine aurora-postgresql. |
| `db_min_capacity` | `number` | `0.5` | Minimum Aurora Serverless v2 capacity (ACUs). |
| `db_max_capacity` | `number` | `4` | Maximum Aurora Serverless v2 capacity (ACUs). |
| `db_instance_count` | `number` | `1` | Aurora instances: 1 writer, plus readers for failover. 2 or more for high availability. |
| `db_backup_retention_days` | `number` | `7` | Days of automated Aurora backups to keep. |
| `db_deletion_protection` | `bool` | `true` | Protect the Aurora cluster from deletion. Set to false (and apply) before terraform destroy. |
| `db_skip_final_snapshot` | `bool` | `false` | Skip the final snapshot when the cluster is destroyed. Keep false for anything holding real data. |
| `redis_node_type` | `string` | `"cache.t4g.micro"` | ElastiCache node type. |
| `redis_engine_version` | `string` | `"7.1"` | ElastiCache Redis OSS engine version (major 7, matching docker-compose). |
| `redis_num_cache_clusters` | `number` | `1` | Redis nodes: 1 primary plus replicas. 2 or more enables automatic failover. |
| `log_retention_days` | `number` | `30` | CloudWatch Logs retention for the container logs. |
| `enable_container_insights` | `bool` | `false` | Enable CloudWatch Container Insights on the ECS cluster (extra cost). |
| `secret_recovery_window_days` | `number` | `7` | Days Secrets Manager keeps deleted secrets recoverable (0 deletes immediately, 7-30 otherwise). |

App settings (`stellar_network`, `soroban_rpc_urls`, `watchdog_contract_id`,
`log_level`, `allowed_origins`, `indexer_poll_interval`, `name`) are validated
by [modules/core](../core).

## Outputs

| Name | Description |
|---|---|
| `url` | Public URL of the dashboard. The API is served under the same origin at /api/v1. |
| `health_url` | API health check URL. |
| `load_balancer_dns_name` | DNS name of the load balancer (point your own DNS here if you did not set route53_zone_id). |
| `load_balancer_zone_id` | Hosted zone ID of the load balancer, for alias records managed elsewhere. |
| `dashboard_build_api_url` | Value to build the dashboard image with (NEXT_PUBLIC_API_URL is inlined at build time). |
| `cluster_name` | ECS cluster name. |
| `service_names` | ECS service names. |
| `migrations_task_definition_arn` | Task definition that applies database migrations. Run it with migrations_run_task_command. |
| `migrations_run_task_command` | AWS CLI command that runs the migrations task once in the private subnets. |
| `database_endpoint` | Aurora writer endpoint (reachable only from inside the VPC). |
| `database_url_secret_arn` | Secrets Manager ARN holding DATABASE_URL. |
| `redis_url_secret_arn` | Secrets Manager ARN holding REDIS_URL. |
| `log_group_names` | CloudWatch log group per container. |
| `vpc_id` | ID of the VPC created for the stack. |
| `private_subnet_ids` | Private subnet IDs (tasks, database, cache). |

## Tests

```sh
terraform init -backend=false
terraform test
```

The tests plan against mocked `aws` and `random` providers (shared defaults in
`tests/mocks/`): no credentials are needed and nothing is created.
