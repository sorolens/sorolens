# Plans the example root against mocked providers (no credentials, no
# resources) to prove its variables reach the module.

mock_provider "aws" {
  override_during = plan
  source          = "../../modules/aws/tests/mocks"
}

mock_provider "random" {
  override_during = plan

  mock_resource "random_password" {
    defaults = {
      result = "MockGeneratedPassword0123456789ab"
    }
  }
}

variables {
  region          = "us-east-1"
  api_image       = "ghcr.io/example/sorolens-api:0.1.0"
  indexer_image   = "ghcr.io/example/sorolens-indexer:0.1.0"
  dashboard_image = "ghcr.io/example/sorolens-dashboard:0.1.0"
}

run "https_install" {
  command = plan

  variables {
    certificate_arn      = "arn:aws:acm:us-east-1:123456789012:certificate/00000000-0000-0000-0000-000000000000"
    domain_name          = "sorolens.example.com"
    route53_zone_id      = "Z0000000000000000000"
    watchdog_contract_id = "CACXRL67WL5KRD6HKWGYADHEUF6RQOCODUN26UQE7MGFZEMIR7PAX6R7"
  }

  assert {
    condition     = output.url == "https://sorolens.example.com"
    error_message = "the example should expose the HTTPS URL"
  }

  assert {
    condition     = output.health_url == "https://sorolens.example.com/health"
    error_message = "the health URL should be on the same origin"
  }

  assert {
    condition     = strcontains(output.migrations_run_task_command, "aws ecs run-task") && strcontains(output.migrations_run_task_command, "assignPublicIp=DISABLED")
    error_message = "the migrations command should run the task privately"
  }
}

run "evaluation_http_only" {
  command = plan

  variables {
    allow_http_only        = true
    db_deletion_protection = false
    db_skip_final_snapshot = true
  }

  assert {
    condition     = startswith(output.url, "http://")
    error_message = "without a certificate the example should serve plain HTTP"
  }
}

run "rejects_bad_region" {
  command = plan

  variables {
    region          = "US East"
    allow_http_only = true
  }

  expect_failures = [var.region]
}
