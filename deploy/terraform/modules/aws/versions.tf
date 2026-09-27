terraform {
  # 1.9 for cross-variable validation (modules/core). Running the tests in
  # tests/ needs Terraform 1.11+ (mock override_during).
  required_version = ">= 1.9.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }
}
