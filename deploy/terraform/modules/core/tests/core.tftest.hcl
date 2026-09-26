# Pure configuration module: no providers, so plan runs need no mocks.

variables {
  public_url = "https://sorolens.example.com"
}

run "defaults" {
  command = plan

  assert {
    condition     = output.api_environment["STELLAR_NETWORK"] == "testnet"
    error_message = "stellar_network should default to testnet"
  }

  assert {
    condition     = output.api_environment["SOROBAN_RPC_URL"] == "https://soroban-testnet.stellar.org:443"
    error_message = "testnet should fall back to the public SDF RPC endpoint"
  }

  assert {
    condition     = output.indexer_environment["SOROBAN_RPC_URL_TESTNET"] == "https://soroban-testnet.stellar.org:443"
    error_message = "the indexer should poll the default network"
  }

  assert {
    condition     = !contains(keys(output.indexer_environment), "SOROBAN_RPC_URL_MAINNET")
    error_message = "the indexer should only poll networks that were configured"
  }

  assert {
    condition     = output.indexer_environment["INDEXER_ROLE"] == "all"
    error_message = "the single indexer instance must own every shard"
  }

  assert {
    condition     = output.indexer_environment["WATCHDOG_ENABLED"] == "false" && !contains(keys(output.indexer_environment), "WATCHDOG_CONTRACT_ID")
    error_message = "watchdog processing should be off without a contract id"
  }

  assert {
    condition     = !contains(keys(output.api_environment), "ALLOWED_ORIGINS")
    error_message = "ALLOWED_ORIGINS should be omitted when no origins are given"
  }

  assert {
    condition     = output.dashboard_environment["NEXT_PUBLIC_API_URL"] == "https://sorolens.example.com"
    error_message = "the dashboard should point at the public URL"
  }

  assert {
    condition     = output.indexer_command == ["-mode=continuous", "-poll-interval=5m"]
    error_message = "the indexer should run continuously with the default poll interval"
  }

  # No secret may ever appear in the plain environment.
  assert {
    condition = alltrue([
      for env in [output.api_environment, output.indexer_environment, output.dashboard_environment] :
      length(setintersection(keys(env), ["DATABASE_URL", "DIRECT_DATABASE_URL", "REDIS_URL", "SENTRY_DSN"])) == 0
    ])
    error_message = "secrets must not be part of the plain environment"
  }
}

run "mainnet_with_watchdog_and_origins" {
  command = plan

  variables {
    stellar_network       = "mainnet"
    soroban_rpc_urls      = { mainnet = "https://rpc.example.com", testnet = "https://testnet-rpc.example.com" }
    watchdog_contract_id  = "CACXRL67WL5KRD6HKWGYADHEUF6RQOCODUN26UQE7MGFZEMIR7PAX6R7"
    allowed_origins       = ["https://a.example.com", "https://b.example.com"]
    indexer_poll_interval = "30s"
  }

  assert {
    condition     = output.api_environment["SOROBAN_RPC_URL"] == "https://rpc.example.com"
    error_message = "the API should use the mainnet endpoint"
  }

  assert {
    condition     = output.indexer_environment["SOROBAN_RPC_URL_MAINNET"] == "https://rpc.example.com" && output.indexer_environment["SOROBAN_RPC_URL_TESTNET"] == "https://testnet-rpc.example.com"
    error_message = "the indexer should poll every configured network, with overrides applied"
  }

  assert {
    condition     = output.indexer_environment["WATCHDOG_ENABLED"] == "true" && output.indexer_environment["WATCHDOG_CONTRACT_ID"] == "CACXRL67WL5KRD6HKWGYADHEUF6RQOCODUN26UQE7MGFZEMIR7PAX6R7"
    error_message = "watchdog processing should be on with a contract id"
  }

  assert {
    condition     = output.api_environment["ALLOWED_ORIGINS"] == "https://a.example.com,https://b.example.com"
    error_message = "ALLOWED_ORIGINS should be a comma-separated list"
  }

  assert {
    condition     = output.indexer_command[1] == "-poll-interval=30s"
    error_message = "the poll interval should be passed to the indexer"
  }
}

run "rejects_mainnet_without_rpc" {
  command = plan

  variables {
    stellar_network = "mainnet"
  }

  expect_failures = [var.soroban_rpc_urls]
}

run "rejects_unknown_network" {
  command = plan

  variables {
    stellar_network = "pubnet"
  }

  expect_failures = [var.stellar_network]
}

run "rejects_http_rpc_url" {
  command = plan

  variables {
    soroban_rpc_urls = { testnet = "http://insecure.example.com" }
  }

  expect_failures = [var.soroban_rpc_urls]
}

run "rejects_bad_watchdog_id" {
  command = plan

  variables {
    watchdog_contract_id = "not-a-contract"
  }

  expect_failures = [var.watchdog_contract_id]
}

run "rejects_long_name" {
  command = plan

  variables {
    name = "this-name-is-far-too-long-for-aws"
  }

  expect_failures = [var.name]
}

run "rejects_origin_with_path" {
  command = plan

  variables {
    allowed_origins = ["https://example.com/app"]
  }

  expect_failures = [var.allowed_origins]
}
