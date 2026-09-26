# Runs entirely against mocked providers: no AWS credentials, no resources.
# override_during = plan makes computed values (ARNs, endpoints) known at plan
# time so the rendered task definitions and IAM policy can be inspected.

mock_provider "aws" {
  override_during = plan
  source          = "./tests/mocks"
}

mock_provider "random" {
  override_during = plan

  mock_resource "random_password" {
    defaults = {
      result = "MockGeneratedPassword0123456789ab"
    }
  }
}

variables {
  api_image       = "ghcr.io/example/sorolens-api:0.1.0"
  indexer_image   = "ghcr.io/example/sorolens-indexer:0.1.0"
  dashboard_image = "ghcr.io/example/sorolens-dashboard:0.1.0"
}

run "requires_tls_or_explicit_http_opt_in" {
  command = plan

  expect_failures = [aws_lb.this]
}

run "secure_defaults" {
  command = plan

  variables {
    allow_http_only = true
  }

  # Database: encrypted, private, protected.
  assert {
    condition     = aws_rds_cluster.this.storage_encrypted && aws_rds_cluster.this.deletion_protection && !aws_rds_cluster.this.skip_final_snapshot
    error_message = "Aurora must be encrypted, deletion-protected and keep a final snapshot by default"
  }

  assert {
    condition     = alltrue([for i in aws_rds_cluster_instance.this : !i.publicly_accessible])
    error_message = "Aurora instances must not be publicly accessible"
  }

  assert {
    condition     = aws_rds_cluster.this.engine == "aurora-postgresql" && startswith(aws_rds_cluster.this.engine_version, "16")
    error_message = "the database must be Aurora PostgreSQL 16"
  }

  # Redis: encrypted at rest and in transit, with AUTH.
  assert {
    condition     = aws_elasticache_replication_group.this.at_rest_encryption_enabled && aws_elasticache_replication_group.this.transit_encryption_enabled
    error_message = "Redis must be encrypted at rest and in transit"
  }

  # Database and Redis accept traffic only from the Sorolens task groups.
  assert {
    condition     = keys(aws_vpc_security_group_ingress_rule.db_from_tasks) == ["web", "worker"] && alltrue([for r in aws_vpc_security_group_ingress_rule.db_from_tasks : r.cidr_ipv4 == null && r.from_port == 5432])
    error_message = "PostgreSQL must only be reachable from the task security groups"
  }

  assert {
    condition     = keys(aws_vpc_security_group_ingress_rule.redis_from_tasks) == ["web", "worker"] && alltrue([for r in aws_vpc_security_group_ingress_rule.redis_from_tasks : r.cidr_ipv4 == null && r.from_port == 6379])
    error_message = "Redis must only be reachable from the task security groups"
  }

  # Tasks run in private subnets without public IPs.
  assert {
    condition = alltrue([
      for s in [aws_ecs_service.api, aws_ecs_service.dashboard, aws_ecs_service.indexer] :
      !s.network_configuration[0].assign_public_ip
    ])
    error_message = "ECS tasks must not get public IPs"
  }

  # Exactly one indexer, never two at once during a deployment.
  assert {
    condition     = aws_ecs_service.indexer.desired_count == 1 && aws_ecs_service.indexer.deployment_maximum_percent == 100 && aws_ecs_service.indexer.deployment_minimum_healthy_percent == 0
    error_message = "the indexer must run as a single instance with stop-before-start deployments"
  }

  assert {
    condition     = jsondecode(aws_ecs_task_definition.this["indexer"].container_definitions)[0].command == ["-mode=continuous", "-poll-interval=5m"]
    error_message = "the indexer must run in continuous mode"
  }

  # Connection URLs are injected as secrets, never as plain environment.
  assert {
    condition = alltrue([
      for svc in ["api", "indexer", "migrations"] :
      contains([for s in jsondecode(aws_ecs_task_definition.this[svc].container_definitions)[0].secrets : s.name], "DATABASE_URL") &&
      contains([for s in jsondecode(aws_ecs_task_definition.this[svc].container_definitions)[0].secrets : s.name], "REDIS_URL")
    ])
    error_message = "API, indexer and migrations must receive DATABASE_URL and REDIS_URL as secrets"
  }

  assert {
    condition = alltrue([
      for svc in ["api", "dashboard", "indexer", "migrations"] :
      length(setintersection(
        [for e in jsondecode(aws_ecs_task_definition.this[svc].container_definitions)[0].environment : e.name],
        ["DATABASE_URL", "DIRECT_DATABASE_URL", "REDIS_URL"],
      )) == 0
    ])
    error_message = "connection URLs must not appear in any plain environment"
  }

  assert {
    condition     = !strcontains(aws_ecs_task_definition.this["api"].container_definitions, random_password.db.result) && !strcontains(aws_ecs_task_definition.this["indexer"].container_definitions, random_password.redis.result)
    error_message = "generated passwords must not be rendered into task definitions"
  }

  assert {
    condition     = length(jsondecode(aws_ecs_task_definition.this["dashboard"].container_definitions)[0].secrets) == 0
    error_message = "the dashboard needs no secrets"
  }

  # The connection URLs themselves require TLS.
  assert {
    condition     = endswith(aws_secretsmanager_secret_version.database_url.secret_string, "?sslmode=require") && startswith(aws_secretsmanager_secret_version.redis_url.secret_string, "rediss://")
    error_message = "database and Redis URLs must use TLS"
  }

  # Least privilege: the execution role reads only the referenced secrets and
  # the containers' task role has no policies.
  assert {
    condition     = jsondecode(aws_iam_role_policy.execution.policy).Statement[2].Resource == [aws_secretsmanager_secret.database_url.arn]
    error_message = "the execution role must only read the stack's own secrets"
  }

  assert {
    condition     = !contains(jsondecode(aws_iam_role_policy.execution.policy).Statement[2].Resource, "*")
    error_message = "secret access must never be granted on *"
  }

  # No ECS Exec shell into production tasks.
  assert {
    condition     = !aws_ecs_service.api.enable_execute_command && !aws_ecs_service.indexer.enable_execute_command && !aws_ecs_service.dashboard.enable_execute_command
    error_message = "ECS Exec must be disabled"
  }

  # HTTP-only evaluation mode: one plain listener, no HTTPS.
  assert {
    condition     = length(aws_lb_listener.https) == 0 && length(aws_lb_listener.http_redirect) == 0 && length(aws_lb_listener.http) == 1
    error_message = "without a certificate only the explicit HTTP listener should exist"
  }

  assert {
    condition     = output.url == "http://sorolens-alb-123456.us-east-1.elb.amazonaws.com"
    error_message = "the URL should be the load balancer over HTTP"
  }

  assert {
    condition     = aws_lb.this.drop_invalid_header_fields
    error_message = "the load balancer must drop invalid headers"
  }

  # Networking defaults: first two zones and one NAT gateway.
  assert {
    condition     = [for s in aws_subnet.private : s.availability_zone] == ["us-east-1a", "us-east-1b"] && length(aws_nat_gateway.this) == 1
    error_message = "defaults should use the first two zones and a single NAT gateway"
  }
}

run "https_with_domain" {
  command = plan

  variables {
    certificate_arn = "arn:aws:acm:us-east-1:123456789012:certificate/00000000-0000-0000-0000-000000000000"
    domain_name     = "sorolens.example.com"
    route53_zone_id = "Z0000000000000000000"
  }

  assert {
    condition     = length(aws_lb_listener.https) == 1 && length(aws_lb_listener.http_redirect) == 1 && length(aws_lb_listener.http) == 0
    error_message = "with a certificate there should be an HTTPS listener and an HTTP->HTTPS redirect only"
  }

  assert {
    condition     = aws_lb_listener.https[0].ssl_policy == "ELBSecurityPolicy-TLS13-1-2-2021-06"
    error_message = "HTTPS must use a TLS 1.2+/1.3 policy"
  }

  assert {
    condition     = aws_lb_listener.http_redirect[0].default_action[0].redirect[0].protocol == "HTTPS"
    error_message = "port 80 must redirect to HTTPS"
  }

  assert {
    condition     = length(aws_route53_record.this) == 1 && aws_route53_record.this[0].name == "sorolens.example.com"
    error_message = "an alias record for the domain should be created"
  }

  assert {
    condition     = output.url == "https://sorolens.example.com" && output.dashboard_build_api_url == "https://sorolens.example.com"
    error_message = "the public URL should be the HTTPS domain"
  }

  assert {
    condition     = contains([for e in jsondecode(aws_ecs_task_definition.this["dashboard"].container_definitions)[0].environment : e.value if e.name == "NEXT_PUBLIC_API_URL"], "https://sorolens.example.com")
    error_message = "the dashboard should target the public HTTPS origin"
  }

  assert {
    condition     = length([for r in aws_vpc_security_group_ingress_rule.alb : r if r.from_port == 443]) == 1
    error_message = "the load balancer should accept 443"
  }
}

run "extra_secrets_migrations_and_topology" {
  command = plan

  variables {
    allow_http_only    = true
    api_extra_secrets  = { SENTRY_DSN = "arn:aws:secretsmanager:us-east-1:123456789012:secret:sentry-dsn" }
    migrations_command = ["/app/migrate", "up"]
    availability_zones = ["eu-west-1b", "eu-west-1c"]
    single_nat_gateway = false
  }

  override_resource {
    target          = aws_nat_gateway.this[0]
    override_during = plan
    values          = { id = "nat-zone-b" }
  }

  override_resource {
    target          = aws_nat_gateway.this[1]
    override_during = plan
    values          = { id = "nat-zone-c" }
  }

  assert {
    condition     = contains([for s in jsondecode(aws_ecs_task_definition.this["api"].container_definitions)[0].secrets : s.name], "SENTRY_DSN")
    error_message = "extra API secrets should be injected"
  }

  assert {
    condition     = contains(jsondecode(aws_iam_role_policy.execution.policy).Statement[2].Resource, "arn:aws:secretsmanager:us-east-1:123456789012:secret:sentry-dsn")
    error_message = "the execution role should be allowed to read extra secrets"
  }

  assert {
    condition     = !contains([for s in jsondecode(aws_ecs_task_definition.this["indexer"].container_definitions)[0].secrets : s.name], "SENTRY_DSN")
    error_message = "API extra secrets must not leak into the indexer"
  }

  assert {
    condition     = jsondecode(aws_ecs_task_definition.this["migrations"].container_definitions)[0].command == ["/app/migrate", "up"]
    error_message = "the migrations command override should be applied"
  }

  assert {
    condition     = jsondecode(aws_ecs_task_definition.this["migrations"].container_definitions)[0].image == "ghcr.io/example/sorolens-api:0.1.0"
    error_message = "migrations should default to the API image"
  }

  assert {
    condition     = length(data.aws_availability_zones.available) == 0 && [for s in aws_subnet.private : s.availability_zone] == ["eu-west-1b", "eu-west-1c"]
    error_message = "explicit zones should be used without looking them up"
  }

  assert {
    condition     = length(aws_nat_gateway.this) == 2 && aws_route.private_nat[0].nat_gateway_id == "nat-zone-b" && aws_route.private_nat[1].nat_gateway_id == "nat-zone-c"
    error_message = "single_nat_gateway = false should give each private subnet its own NAT gateway"
  }
}

run "defaults_have_no_migrations_command" {
  command = plan

  variables {
    allow_http_only = true
  }

  assert {
    condition     = !contains(keys(jsondecode(aws_ecs_task_definition.this["migrations"].container_definitions)[0]), "command")
    error_message = "without migrations_command the image's default command should be kept"
  }
}

run "rejects_latest_tag" {
  command = plan

  variables {
    allow_http_only = true
    api_image       = "ghcr.io/example/sorolens-api:latest"
  }

  expect_failures = [var.api_image]
}

run "rejects_untagged_image" {
  command = plan

  variables {
    allow_http_only = true
    indexer_image   = "ghcr.io/example/sorolens-indexer"
  }

  expect_failures = [var.indexer_image]
}

run "rejects_zone_without_domain" {
  command = plan

  variables {
    allow_http_only = true
    route53_zone_id = "Z0000000000000000000"
  }

  expect_failures = [aws_lb.this]
}

run "rejects_three_zones" {
  command = plan

  variables {
    allow_http_only    = true
    availability_zones = ["us-east-1a", "us-east-1b", "us-east-1c"]
  }

  expect_failures = [var.availability_zones]
}
