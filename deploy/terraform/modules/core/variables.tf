variable "name" {
  description = "Name prefix for every resource of this deployment. Lowercase letters, digits and hyphens; starts with a letter; at most 20 characters so derived names (load balancer, target groups) stay within AWS limits."
  type        = string
  default     = "sorolens"

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{0,19}$", var.name)) && !endswith(var.name, "-")
    error_message = "name must match ^[a-z][a-z0-9-]{0,19}$ and must not end with a hyphen."
  }
}

variable "stellar_network" {
  description = "Default Stellar network for the API (STELLAR_NETWORK): testnet, mainnet or futurenet."
  type        = string
  default     = "testnet"

  validation {
    condition     = contains(["testnet", "mainnet", "futurenet"], var.stellar_network)
    error_message = "stellar_network must be one of: testnet, mainnet, futurenet."
  }
}

variable "soroban_rpc_urls" {
  description = "Soroban RPC endpoint per network, e.g. { testnet = \"https://...\" }. testnet and futurenet fall back to the public SDF endpoints; mainnet has no default because it should use a commercial provider. The indexer indexes every network listed here plus stellar_network."
  type        = map(string)
  default     = {}

  validation {
    condition     = alltrue([for k in keys(var.soroban_rpc_urls) : contains(["testnet", "mainnet", "futurenet"], k)])
    error_message = "soroban_rpc_urls keys must be testnet, mainnet or futurenet."
  }

  validation {
    condition     = alltrue([for v in values(var.soroban_rpc_urls) : startswith(v, "https://")])
    error_message = "soroban_rpc_urls values must be https:// URLs."
  }

  validation {
    condition     = var.stellar_network != "mainnet" || contains(keys(var.soroban_rpc_urls), "mainnet")
    error_message = "stellar_network is mainnet, so soroban_rpc_urls must include a mainnet endpoint (there is no public default)."
  }
}

variable "watchdog_contract_id" {
  description = "Deployed sorolens-watchdog contract ID (56-character C... strkey). When set, the indexer treats that contract's events as watchdog telemetry (WATCHDOG_ENABLED=true)."
  type        = string
  default     = null

  validation {
    condition     = var.watchdog_contract_id == null || can(regex("^C[A-Z2-7]{55}$", var.watchdog_contract_id))
    error_message = "watchdog_contract_id must be a 56-character contract strkey starting with C."
  }
}

variable "log_level" {
  description = "API log level (LOG_LEVEL): debug, info, warn or error."
  type        = string
  default     = "info"

  validation {
    condition     = contains(["debug", "info", "warn", "error"], var.log_level)
    error_message = "log_level must be one of: debug, info, warn, error."
  }
}

variable "allowed_origins" {
  description = "Browser origins allowed to call the API (ALLOWED_ORIGINS, read by the API once #381 lands). Leave empty when the dashboard is served from the same origin as the API."
  type        = list(string)
  default     = []

  validation {
    condition     = alltrue([for o in var.allowed_origins : can(regex("^https?://[^/]+$", o))])
    error_message = "allowed_origins entries must be origins such as https://sorolens.example.com (scheme and host, no path)."
  }
}

variable "public_url" {
  description = "Public origin the dashboard and API are served from, e.g. https://sorolens.example.com. Used for the dashboard's NEXT_PUBLIC_API_URL."
  type        = string

  validation {
    condition     = can(regex("^https?://[^/]+$", var.public_url))
    error_message = "public_url must be an origin such as https://sorolens.example.com (scheme and host, no path)."
  }
}

variable "indexer_poll_interval" {
  description = "Sleep between indexer passes in continuous mode (Go duration, e.g. 5m)."
  type        = string
  default     = "5m"

  validation {
    condition     = can(regex("^([0-9]+(ns|us|ms|s|m|h))+$", var.indexer_poll_interval))
    error_message = "indexer_poll_interval must be a Go duration such as 30s, 5m or 1h."
  }
}

variable "api_port" {
  description = "Port the API container listens on (PORT)."
  type        = number
  default     = 8080
}

variable "dashboard_port" {
  description = "Port the dashboard container listens on."
  type        = number
  default     = 3000
}

variable "indexer_metrics_port" {
  description = "Port of the indexer's Prometheus /metrics endpoint (INDEXER_METRICS_ADDR). Not exposed outside the private network."
  type        = number
  default     = 9100
}
