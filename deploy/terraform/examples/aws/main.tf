module "sorolens" {
  source = "../../modules/aws"

  name = var.name

  api_image          = var.api_image
  indexer_image      = var.indexer_image
  dashboard_image    = var.dashboard_image
  migrations_image   = var.migrations_image
  migrations_command = var.migrations_command

  certificate_arn = var.certificate_arn
  allow_http_only = var.allow_http_only
  domain_name     = var.domain_name
  route53_zone_id = var.route53_zone_id

  stellar_network      = var.stellar_network
  soroban_rpc_urls     = var.soroban_rpc_urls
  watchdog_contract_id = var.watchdog_contract_id

  db_deletion_protection = var.db_deletion_protection
  db_skip_final_snapshot = var.db_skip_final_snapshot
}
