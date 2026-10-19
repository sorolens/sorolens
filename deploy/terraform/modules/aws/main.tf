locals {
  # Validated by modules/core; used directly so naming never waits on core.
  name = var.name

  public_host = var.domain_name != null ? var.domain_name : aws_lb.this.dns_name
  scheme      = var.certificate_arn != null ? "https" : "http"
  # The dashboard and the API share this origin: /api/* is routed to the API.
  public_url = "${local.scheme}://${local.public_host}"

  tags = merge({ "sorolens:stack" = var.name }, var.tags)
}

module "core" {
  source = "../core"

  name                  = var.name
  stellar_network       = var.stellar_network
  soroban_rpc_urls      = var.soroban_rpc_urls
  watchdog_contract_id  = var.watchdog_contract_id
  log_level             = var.log_level
  allowed_origins       = var.allowed_origins
  indexer_poll_interval = var.indexer_poll_interval
  public_url            = local.public_url
}
