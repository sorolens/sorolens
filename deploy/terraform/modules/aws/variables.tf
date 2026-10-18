# ---- general ---------------------------------------------------------------

variable "name" {
  description = "Name prefix for every resource. Lowercase letters, digits and hyphens, at most 20 characters."
  type        = string
  default     = "sorolens"
}

variable "tags" {
  description = "Tags added to every resource that supports them."
  type        = map(string)
  default     = {}
}

# ---- images (#96 / #381) ------------------------------------------------------

variable "api_image" {
  description = "API container image with an explicit tag or digest, e.g. ghcr.io/org/sorolens-api:1.2.3. Sorolens does not publish images yet (#96, #381), so there is no default."
  type        = string

  validation {
    condition     = can(regex("^[^\\s]+(:[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}|@sha256:[a-f0-9]{64})$", var.api_image)) && !endswith(var.api_image, ":latest")
    error_message = "api_image must include an explicit tag (not :latest) or an @sha256 digest."
  }
}

variable "indexer_image" {
  description = "Indexer container image with an explicit tag or digest. Its entrypoint must be the indexer binary, which receives -mode=continuous. No default: see api_image."
  type        = string

  validation {
    condition     = can(regex("^[^\\s]+(:[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}|@sha256:[a-f0-9]{64})$", var.indexer_image)) && !endswith(var.indexer_image, ":latest")
    error_message = "indexer_image must include an explicit tag (not :latest) or an @sha256 digest."
  }
}

variable "dashboard_image" {
  description = "Dashboard (Next.js) container image with an explicit tag or digest. NEXT_PUBLIC_API_URL is inlined when the image is built, so build it for this deployment's public URL. No default: see api_image."
  type        = string

  validation {
    condition     = can(regex("^[^\\s]+(:[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}|@sha256:[a-f0-9]{64})$", var.dashboard_image)) && !endswith(var.dashboard_image, ":latest")
    error_message = "dashboard_image must include an explicit tag (not :latest) or an @sha256 digest."
  }
}

variable "migrations_image" {
  description = "Image for the one-off migrations task. Defaults to api_image."
  type        = string
  default     = null
}

variable "migrations_command" {
  description = "Command override for the migrations task, i.e. whatever applies pending migrations and exits in your migrations_image. null keeps the image's default command. Depends on how #381 packages migrations."
  type        = list(string)
  default     = null
}

variable "image_pull_secret_arn" {
  description = "Optional Secrets Manager secret ARN with {\"username\",\"password\"} for a private registry. Not needed for public images or ECR in the same account."
  type        = string
  default     = null
}

variable "cpu_architecture" {
  description = "CPU architecture of the images: X86_64 or ARM64."
  type        = string
  default     = "X86_64"

  validation {
    condition     = contains(["X86_64", "ARM64"], var.cpu_architecture)
    error_message = "cpu_architecture must be X86_64 or ARM64."
  }
}

# ---- app configuration (passed to modules/core) ----------------------------

variable "stellar_network" {
  description = "Default Stellar network: testnet, mainnet or futurenet."
  type        = string
  default     = "testnet"
}

variable "soroban_rpc_urls" {
  description = "Soroban RPC endpoint per network. testnet and futurenet default to the public SDF endpoints; mainnet must be set when used."
  type        = map(string)
  default     = {}
}

variable "watchdog_contract_id" {
  description = "Deployed sorolens-watchdog contract ID; enables watchdog processing in the indexer."
  type        = string
  default     = null
}

variable "log_level" {
  description = "API log level: debug, info, warn or error."
  type        = string
  default     = "info"
}

variable "allowed_origins" {
  description = "Extra browser origins allowed to call the API. The dashboard is served from the same origin, so this is usually empty."
  type        = list(string)
  default     = []
}

variable "indexer_poll_interval" {
  description = "Sleep between indexer passes (Go duration)."
  type        = string
  default     = "5m"
}

variable "api_extra_environment" {
  description = "Additional plain environment variables for the API and migrations containers (e.g. SENTRY_ENVIRONMENT). Must not contain secrets."
  type        = map(string)
  default     = {}
}

variable "api_extra_secrets" {
  description = "Additional secrets for the API and migrations containers: environment variable name => Secrets Manager secret ARN (e.g. SENTRY_DSN, SLACK_SIGNING_SECRET)."
  type        = map(string)
  default     = {}
}

variable "indexer_extra_environment" {
  description = "Additional plain environment variables for the indexer (e.g. INDEXER_ANOMALY_ENABLED). Must not contain secrets."
  type        = map(string)
  default     = {}
}

variable "indexer_extra_secrets" {
  description = "Additional secrets for the indexer: environment variable name => Secrets Manager secret ARN."
  type        = map(string)
  default     = {}
}

# ---- compute sizing ----------------------------------------------------------

variable "api_cpu" {
  description = "Fargate CPU units for each API task (256 = 0.25 vCPU)."
  type        = number
  default     = 256
}

variable "api_memory" {
  description = "Fargate memory (MiB) for each API task."
  type        = number
  default     = 512
}

variable "api_desired_count" {
  description = "Number of API tasks. The API is stateless and can scale horizontally."
  type        = number
  default     = 1

  validation {
    condition     = var.api_desired_count >= 1 && var.api_desired_count <= 20 && floor(var.api_desired_count) == var.api_desired_count
    error_message = "api_desired_count must be a whole number between 1 and 20."
  }
}

variable "dashboard_cpu" {
  description = "Fargate CPU units for each dashboard task."
  type        = number
  default     = 256
}

variable "dashboard_memory" {
  description = "Fargate memory (MiB) for each dashboard task."
  type        = number
  default     = 512
}

variable "dashboard_desired_count" {
  description = "Number of dashboard tasks."
  type        = number
  default     = 1

  validation {
    condition     = var.dashboard_desired_count >= 1 && var.dashboard_desired_count <= 20 && floor(var.dashboard_desired_count) == var.dashboard_desired_count
    error_message = "dashboard_desired_count must be a whole number between 1 and 20."
  }
}

variable "indexer_cpu" {
  description = "Fargate CPU units for the indexer task. The indexer always runs as exactly one task."
  type        = number
  default     = 256
}

variable "indexer_memory" {
  description = "Fargate memory (MiB) for the indexer task."
  type        = number
  default     = 512
}

# ---- network -------------------------------------------------------------------

variable "vpc_cidr" {
  description = "CIDR block of the VPC. Split into two public and two private subnets."
  type        = string
  default     = "10.40.0.0/16"

  validation {
    condition     = can(cidrnetmask(var.vpc_cidr)) && tonumber(split("/", var.vpc_cidr)[1]) <= 20
    error_message = "vpc_cidr must be a valid IPv4 CIDR of /20 or larger."
  }
}

variable "availability_zones" {
  description = "Exactly two availability zones to use. Empty means the first two available zones in the region."
  type        = list(string)
  default     = []

  validation {
    condition     = length(var.availability_zones) == 0 || length(var.availability_zones) == 2
    error_message = "availability_zones must be empty or list exactly two zones."
  }
}

variable "single_nat_gateway" {
  description = "Use one NAT gateway for both private subnets (cheaper) instead of one per zone (survives a zone outage)."
  type        = bool
  default     = true
}

# ---- load balancer / TLS -------------------------------------------------------

variable "certificate_arn" {
  description = "ACM certificate ARN for HTTPS on the load balancer. Required unless allow_http_only is true."
  type        = string
  default     = null
}

variable "allow_http_only" {
  description = "Explicit opt-in to serve plain HTTP when no certificate_arn is given. Only for short-lived evaluation stacks."
  type        = bool
  default     = false
}

variable "domain_name" {
  description = "Public hostname for the stack, e.g. sorolens.example.com. Needs a certificate covering it. When route53_zone_id is also set, an alias record is created."
  type        = string
  default     = null
}

variable "route53_zone_id" {
  description = "Route 53 hosted zone ID in which to create the domain_name alias record."
  type        = string
  default     = null
}

variable "alb_ingress_cidrs" {
  description = "IPv4 CIDR blocks allowed to reach the load balancer."
  type        = list(string)
  default     = ["0.0.0.0/0"]

  validation {
    condition     = alltrue([for c in var.alb_ingress_cidrs : can(cidrnetmask(c))])
    error_message = "alb_ingress_cidrs must contain IPv4 CIDR blocks."
  }
}

variable "alb_deletion_protection" {
  description = "Enable deletion protection on the load balancer."
  type        = bool
  default     = false
}

# ---- database ------------------------------------------------------------------

variable "db_engine_version" {
  description = "Aurora PostgreSQL engine version (major 16, matching docker-compose). Must be available in the region: aws rds describe-db-engine-versions --engine aurora-postgresql."
  type        = string
  default     = "16.6"

  validation {
    condition     = can(regex("^16(\\.[0-9]+)?$", var.db_engine_version))
    error_message = "db_engine_version must be an Aurora PostgreSQL 16.x version."
  }
}

variable "db_min_capacity" {
  description = "Minimum Aurora Serverless v2 capacity (ACUs)."
  type        = number
  default     = 0.5
}

variable "db_max_capacity" {
  description = "Maximum Aurora Serverless v2 capacity (ACUs)."
  type        = number
  default     = 4

  validation {
    condition     = var.db_max_capacity >= 1 && var.db_max_capacity <= 256
    error_message = "db_max_capacity must be between 1 and 256 ACUs."
  }
}

variable "db_instance_count" {
  description = "Aurora instances: 1 writer, plus readers for failover. 2 or more for high availability."
  type        = number
  default     = 1

  validation {
    condition     = var.db_instance_count >= 1 && var.db_instance_count <= 3
    error_message = "db_instance_count must be between 1 and 3."
  }
}

variable "db_backup_retention_days" {
  description = "Days of automated Aurora backups to keep."
  type        = number
  default     = 7

  validation {
    condition     = var.db_backup_retention_days >= 1 && var.db_backup_retention_days <= 35
    error_message = "db_backup_retention_days must be between 1 and 35."
  }
}

variable "db_deletion_protection" {
  description = "Protect the Aurora cluster from deletion. Set to false (and apply) before terraform destroy."
  type        = bool
  default     = true
}

variable "db_skip_final_snapshot" {
  description = "Skip the final snapshot when the cluster is destroyed. Keep false for anything holding real data."
  type        = bool
  default     = false
}

# ---- redis ---------------------------------------------------------------------

variable "redis_node_type" {
  description = "ElastiCache node type."
  type        = string
  default     = "cache.t4g.micro"
}

variable "redis_engine_version" {
  description = "ElastiCache Redis OSS engine version (major 7, matching docker-compose)."
  type        = string
  default     = "7.1"
}

variable "redis_num_cache_clusters" {
  description = "Redis nodes: 1 primary plus replicas. 2 or more enables automatic failover."
  type        = number
  default     = 1

  validation {
    condition     = var.redis_num_cache_clusters >= 1 && var.redis_num_cache_clusters <= 3
    error_message = "redis_num_cache_clusters must be between 1 and 3."
  }
}

# ---- observability / housekeeping --------------------------------------------

variable "log_retention_days" {
  description = "CloudWatch Logs retention for the container logs."
  type        = number
  default     = 30
}

variable "enable_container_insights" {
  description = "Enable CloudWatch Container Insights on the ECS cluster (extra cost)."
  type        = bool
  default     = false
}

variable "secret_recovery_window_days" {
  description = "Days Secrets Manager keeps deleted secrets recoverable (0 deletes immediately, 7-30 otherwise)."
  type        = number
  default     = 7

  validation {
    condition     = var.secret_recovery_window_days == 0 || (var.secret_recovery_window_days >= 7 && var.secret_recovery_window_days <= 30)
    error_message = "secret_recovery_window_days must be 0 or between 7 and 30."
  }
}
