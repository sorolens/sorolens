output "url" {
  description = "Dashboard URL (the API is under /api/v1 on the same origin)."
  value       = module.sorolens.url
}

output "health_url" {
  description = "API health check URL."
  value       = module.sorolens.health_url
}

output "load_balancer_dns_name" {
  description = "Load balancer DNS name."
  value       = module.sorolens.load_balancer_dns_name
}

output "dashboard_build_api_url" {
  description = "NEXT_PUBLIC_API_URL to build the dashboard image with."
  value       = module.sorolens.dashboard_build_api_url
}

output "cluster_name" {
  description = "ECS cluster name."
  value       = module.sorolens.cluster_name
}

output "service_names" {
  description = "ECS service names."
  value       = module.sorolens.service_names
}

output "migrations_run_task_command" {
  description = "Runs the migrations task once."
  value       = module.sorolens.migrations_run_task_command
}

output "log_group_names" {
  description = "CloudWatch log groups."
  value       = module.sorolens.log_group_names
}
