variable "region" {
  description = "AWS region to deploy into, e.g. us-east-1."
  type        = string

  validation {
    condition     = can(regex("^[a-z]{2}(-[a-z]+)+-[0-9]$", var.region))
    error_message = "region must be an AWS region code such as us-east-1."
  }
}

variable "name" {
  description = "Name prefix for every resource (at most 20 characters)."
  type        = string
  default     = "sorolens"
}

variable "api_image" {
  description = "API image with an explicit tag or digest."
  type        = string
}

variable "indexer_image" {
  description = "Indexer image with an explicit tag or digest."
  type        = string
}

variable "dashboard_image" {
  description = "Dashboard image with an explicit tag or digest, built with NEXT_PUBLIC_API_URL set to this deployment's URL."
  type        = string
}

variable "migrations_image" {
  description = "Image for the one-off migrations task (defaults to api_image). See step 4 of docs/self-hosting/aws.md."
  type        = string
  default     = null
}

variable "migrations_command" {
  description = "Command that applies migrations and exits in migrations_image. See step 4 of docs/self-hosting/aws.md."
  type        = list(string)
  default     = null
}

variable "certificate_arn" {
  description = "ACM certificate ARN for HTTPS (in the same region)."
  type        = string
  default     = null
}

variable "allow_http_only" {
  description = "Serve plain HTTP without a certificate. Evaluation stacks only."
  type        = bool
  default     = false
}

variable "domain_name" {
  description = "Public hostname, e.g. sorolens.example.com."
  type        = string
  default     = null
}

variable "route53_zone_id" {
  description = "Route 53 hosted zone to create the domain_name record in."
  type        = string
  default     = null
}

variable "stellar_network" {
  description = "Default Stellar network: testnet, mainnet or futurenet."
  type        = string
  default     = "testnet"
}

variable "soroban_rpc_urls" {
  description = "Soroban RPC endpoint per network (mainnet has no public default)."
  type        = map(string)
  default     = {}
}

variable "watchdog_contract_id" {
  description = "Deployed sorolens-watchdog contract ID, if any."
  type        = string
  default     = null
}

variable "db_deletion_protection" {
  description = "Protect the database from deletion. Set to false and apply before destroying."
  type        = bool
  default     = true
}

variable "db_skip_final_snapshot" {
  description = "Skip the final database snapshot on destroy (throwaway stacks only)."
  type        = bool
  default     = false
}
