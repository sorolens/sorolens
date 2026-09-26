terraform {
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

  # State contains the generated database and Redis passwords. For anything
  # beyond a throwaway test, keep it in an encrypted, access-controlled
  # backend instead of the local default, e.g.:
  #
  # backend "s3" {
  #   bucket       = "my-terraform-state"
  #   key          = "sorolens/terraform.tfstate"
  #   region       = "us-east-1"
  #   encrypt      = true
  #   use_lockfile = true
  # }
}

provider "aws" {
  region = var.region

  default_tags {
    tags = {
      "app"        = "sorolens"
      "managed-by" = "terraform"
    }
  }
}
