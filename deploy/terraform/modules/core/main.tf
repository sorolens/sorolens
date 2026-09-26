# Cloud-agnostic Sorolens configuration: validates the app settings and
# turns them into the environment each service reads. Creates no resources,
# so every cloud module can share it.

locals {
  # Public SDF endpoints; mainnet deliberately has none (see soroban_rpc_urls).
  default_rpc_urls = {
    testnet   = "https://soroban-testnet.stellar.org:443"
    futurenet = "https://soroban-futurenet.stellar.org:443"
  }

  rpc_urls = merge(local.default_rpc_urls, var.soroban_rpc_urls)

  # Networks the indexer polls: the default network plus any given explicitly.
  indexed_networks = distinct(concat([var.stellar_network], sort(keys(var.soroban_rpc_urls))))

  per_network_rpc_env = {
    for n in local.indexed_networks : "SOROBAN_RPC_URL_${upper(n)}" => local.rpc_urls[n]
  }

  api_environment = merge(
    {
      PORT            = tostring(var.api_port)
      STELLAR_NETWORK = var.stellar_network
      LOG_LEVEL       = var.log_level
      SOROBAN_RPC_URL = local.rpc_urls[var.stellar_network]
    },
    local.per_network_rpc_env,
    length(var.allowed_origins) > 0 ? { ALLOWED_ORIGINS = join(",", var.allowed_origins) } : {},
  )

  indexer_environment = merge(
    local.per_network_rpc_env,
    {
      # Single process owning every shard; the only shard store on main is
      # in-memory, so the indexer must run as exactly one instance.
      INDEXER_ROLE         = "all"
      INDEXER_METRICS_ADDR = ":${var.indexer_metrics_port}"
      WATCHDOG_ENABLED     = var.watchdog_contract_id != null ? "true" : "false"
    },
    var.watchdog_contract_id != null ? { WATCHDOG_CONTRACT_ID = var.watchdog_contract_id } : {},
  )

  dashboard_environment = {
    PORT     = tostring(var.dashboard_port)
    HOSTNAME = "0.0.0.0"
    # Next.js inlines NEXT_PUBLIC_* at build time, so this only takes effect
    # if the image was built with the same value. See the install guide.
    NEXT_PUBLIC_API_URL = var.public_url
  }

  indexer_command = ["-mode=continuous", "-poll-interval=${var.indexer_poll_interval}"]
}
