output "name" {
  description = "Validated name prefix for resources."
  value       = var.name
}

output "api_environment" {
  description = "Non-secret environment variables for the API container. DATABASE_URL, DIRECT_DATABASE_URL and REDIS_URL are secrets and are injected by the cloud module."
  value       = local.api_environment
}

output "indexer_environment" {
  description = "Non-secret environment variables for the indexer container. DATABASE_URL and REDIS_URL are injected by the cloud module as secrets."
  value       = local.indexer_environment
}

output "dashboard_environment" {
  description = "Environment variables for the dashboard container."
  value       = local.dashboard_environment
}

output "indexer_command" {
  description = "Arguments for the indexer binary: continuous mode with the configured poll interval."
  value       = local.indexer_command
}

output "api_port" {
  description = "API container port."
  value       = var.api_port
}

output "dashboard_port" {
  description = "Dashboard container port."
  value       = var.dashboard_port
}

output "indexer_metrics_port" {
  description = "Indexer metrics port."
  value       = var.indexer_metrics_port
}
