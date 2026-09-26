# ECS on Fargate: API and dashboard behind the load balancer, one indexer,
# and a migrations task definition that is run on demand.

locals {
  services = toset(["api", "dashboard", "indexer", "migrations"])

  # Secrets every data-plane container gets.
  connection_secrets = {
    DATABASE_URL = aws_secretsmanager_secret.database_url.arn
    REDIS_URL    = aws_secretsmanager_secret.redis_url.arn
  }

  api_secrets = merge(
    local.connection_secrets,
    # Migrations bypass any pooler through DIRECT_DATABASE_URL; on Aurora it
    # is simply the same URL.
    { DIRECT_DATABASE_URL = aws_secretsmanager_secret.database_url.arn },
    var.api_extra_secrets,
  )

  indexer_secrets = merge(local.connection_secrets, var.indexer_extra_secrets)

  api_environment     = merge(module.core.api_environment, var.api_extra_environment)
  indexer_environment = merge(module.core.indexer_environment, var.indexer_extra_environment)

  repository_credentials = var.image_pull_secret_arn != null ? { credentialsParameter = var.image_pull_secret_arn } : null

  # One container definition per task; secrets are referenced by ARN only.
  container_definitions = {
    api = {
      image        = var.api_image
      command      = null
      environment  = local.api_environment
      secrets      = local.api_secrets
      portMappings = [{ containerPort = module.core.api_port, protocol = "tcp" }]
    }
    dashboard = {
      image        = var.dashboard_image
      command      = null
      environment  = module.core.dashboard_environment
      secrets      = {}
      portMappings = [{ containerPort = module.core.dashboard_port, protocol = "tcp" }]
    }
    indexer = {
      image        = var.indexer_image
      command      = module.core.indexer_command
      environment  = local.indexer_environment
      secrets      = local.indexer_secrets
      portMappings = []
    }
    migrations = {
      image        = coalesce(var.migrations_image, var.api_image)
      command      = var.migrations_command
      environment  = local.api_environment
      secrets      = local.api_secrets
      portMappings = []
    }
  }

  task_sizes = {
    api        = { cpu = var.api_cpu, memory = var.api_memory }
    dashboard  = { cpu = var.dashboard_cpu, memory = var.dashboard_memory }
    indexer    = { cpu = var.indexer_cpu, memory = var.indexer_memory }
    migrations = { cpu = var.api_cpu, memory = var.api_memory }
  }
}

resource "aws_cloudwatch_log_group" "this" {
  for_each = local.services

  name              = "/ecs/${local.name}/${each.key}"
  retention_in_days = var.log_retention_days

  tags = local.tags
}

resource "aws_ecs_cluster" "this" {
  name = local.name

  setting {
    name  = "containerInsights"
    value = var.enable_container_insights ? "enabled" : "disabled"
  }

  tags = local.tags
}

resource "aws_ecs_task_definition" "this" {
  for_each = local.container_definitions

  family                   = "${local.name}-${each.key}"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = tostring(local.task_sizes[each.key].cpu)
  memory                   = tostring(local.task_sizes[each.key].memory)
  execution_role_arn       = aws_iam_role.execution.arn
  task_role_arn            = aws_iam_role.task.arn

  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = var.cpu_architecture
  }

  container_definitions = jsonencode([
    merge(
      {
        name         = each.key
        image        = each.value.image
        essential    = true
        portMappings = each.value.portMappings
        environment  = [for k in sort(keys(each.value.environment)) : { name = k, value = each.value.environment[k] }]
        secrets      = [for k in sort(keys(each.value.secrets)) : { name = k, valueFrom = each.value.secrets[k] }]
        logConfiguration = {
          logDriver = "awslogs"
          options = {
            awslogs-group         = aws_cloudwatch_log_group.this[each.key].name
            awslogs-region        = data.aws_region.current.region
            awslogs-stream-prefix = each.key
          }
        }
      },
      each.value.command != null ? { command = each.value.command } : {},
      local.repository_credentials != null ? { repositoryCredentials = local.repository_credentials } : {},
    )
  ])

  tags = local.tags
}

data "aws_region" "current" {}

resource "aws_ecs_service" "api" {
  name            = "${local.name}-api"
  cluster         = aws_ecs_cluster.this.id
  task_definition = aws_ecs_task_definition.this["api"].arn
  desired_count   = var.api_desired_count
  launch_type     = "FARGATE"

  health_check_grace_period_seconds = 60
  enable_execute_command            = false

  network_configuration {
    subnets          = aws_subnet.private[*].id
    security_groups  = [aws_security_group.web.id]
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.api.arn
    container_name   = "api"
    container_port   = module.core.api_port
  }

  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }

  tags = local.tags

  depends_on = [aws_lb_listener_rule.api]
}

resource "aws_ecs_service" "dashboard" {
  name            = "${local.name}-dashboard"
  cluster         = aws_ecs_cluster.this.id
  task_definition = aws_ecs_task_definition.this["dashboard"].arn
  desired_count   = var.dashboard_desired_count
  launch_type     = "FARGATE"

  health_check_grace_period_seconds = 60
  enable_execute_command            = false

  network_configuration {
    subnets          = aws_subnet.private[*].id
    security_groups  = [aws_security_group.web.id]
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.dashboard.arn
    container_name   = "dashboard"
    container_port   = module.core.dashboard_port
  }

  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }

  tags = local.tags

  depends_on = [aws_lb_listener.https, aws_lb_listener.http]
}

# Exactly one indexer, and deployments stop the old task before starting the
# new one, so two indexers never run at the same time (the Redis lock is the
# second line of defence).
resource "aws_ecs_service" "indexer" {
  name            = "${local.name}-indexer"
  cluster         = aws_ecs_cluster.this.id
  task_definition = aws_ecs_task_definition.this["indexer"].arn
  desired_count   = 1
  launch_type     = "FARGATE"

  deployment_minimum_healthy_percent = 0
  deployment_maximum_percent         = 100
  enable_execute_command             = false

  network_configuration {
    subnets          = aws_subnet.private[*].id
    security_groups  = [aws_security_group.worker.id]
    assign_public_ip = false
  }

  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }

  tags = local.tags
}
