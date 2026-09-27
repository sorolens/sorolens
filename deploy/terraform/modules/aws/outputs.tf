output "url" {
  description = "Public URL of the dashboard. The API is served under the same origin at /api/v1."
  value       = local.public_url
}

output "health_url" {
  description = "API health check URL."
  value       = "${local.public_url}/health"
}

output "load_balancer_dns_name" {
  description = "DNS name of the load balancer (point your own DNS here if you did not set route53_zone_id)."
  value       = aws_lb.this.dns_name
}

output "load_balancer_zone_id" {
  description = "Hosted zone ID of the load balancer, for alias records managed elsewhere."
  value       = aws_lb.this.zone_id
}

output "dashboard_build_api_url" {
  description = "Value to build the dashboard image with (NEXT_PUBLIC_API_URL is inlined at build time)."
  value       = local.public_url
}

output "cluster_name" {
  description = "ECS cluster name."
  value       = aws_ecs_cluster.this.name
}

output "service_names" {
  description = "ECS service names."
  value = {
    api       = aws_ecs_service.api.name
    dashboard = aws_ecs_service.dashboard.name
    indexer   = aws_ecs_service.indexer.name
  }
}

output "migrations_task_definition_arn" {
  description = "Task definition that applies database migrations. Run it with migrations_run_task_command."
  value       = aws_ecs_task_definition.this["migrations"].arn
}

output "migrations_run_task_command" {
  description = "AWS CLI command that runs the migrations task once in the private subnets."
  value = join(" ", [
    "aws ecs run-task",
    "--cluster ${aws_ecs_cluster.this.name}",
    "--task-definition ${aws_ecs_task_definition.this["migrations"].arn}",
    "--launch-type FARGATE",
    "--network-configuration 'awsvpcConfiguration={subnets=[${join(",", aws_subnet.private[*].id)}],securityGroups=[${aws_security_group.worker.id}],assignPublicIp=DISABLED}'",
  ])
}

output "database_endpoint" {
  description = "Aurora writer endpoint (reachable only from inside the VPC)."
  value       = aws_rds_cluster.this.endpoint
}

output "database_url_secret_arn" {
  description = "Secrets Manager ARN holding DATABASE_URL."
  value       = aws_secretsmanager_secret.database_url.arn
}

output "redis_url_secret_arn" {
  description = "Secrets Manager ARN holding REDIS_URL."
  value       = aws_secretsmanager_secret.redis_url.arn
}

output "log_group_names" {
  description = "CloudWatch log group per container."
  value       = { for k, g in aws_cloudwatch_log_group.this : k => g.name }
}

output "vpc_id" {
  description = "ID of the VPC created for the stack."
  value       = aws_vpc.this.id
}

output "private_subnet_ids" {
  description = "Private subnet IDs (tasks, database, cache)."
  value       = aws_subnet.private[*].id
}
