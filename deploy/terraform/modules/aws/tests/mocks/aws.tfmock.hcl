# Shared AWS mock defaults for terraform test (modules/aws and examples/aws).
# Values look like real ARNs/endpoints so rendered JSON can be asserted on.

mock_data "aws_availability_zones" {
  defaults = {
    names = ["us-east-1a", "us-east-1b", "us-east-1c"]
  }
}

mock_data "aws_region" {
  defaults = {
    region = "us-east-1"
  }
}

mock_resource "aws_lb" {
  defaults = {
    arn      = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/sorolens-alb/0123456789abcdef"
    dns_name = "sorolens-alb-123456.us-east-1.elb.amazonaws.com"
    zone_id  = "Z35SXDOTRQ7X7K"
  }
}

mock_resource "aws_secretsmanager_secret" {
  defaults = {
    arn = "arn:aws:secretsmanager:us-east-1:123456789012:secret:sorolens-mock"
  }
}

mock_resource "aws_iam_role" {
  defaults = {
    arn = "arn:aws:iam::123456789012:role/sorolens-mock"
  }
}

mock_resource "aws_cloudwatch_log_group" {
  defaults = {
    arn = "arn:aws:logs:us-east-1:123456789012:log-group:sorolens-mock"
  }
}

mock_resource "aws_rds_cluster" {
  defaults = {
    endpoint = "sorolens-db.cluster-mock.us-east-1.rds.amazonaws.com"
    port     = 5432
  }
}

mock_resource "aws_elasticache_replication_group" {
  defaults = {
    primary_endpoint_address = "master.sorolens-redis.mock.use1.cache.amazonaws.com"
  }
}

mock_resource "aws_subnet" {
  defaults = {
    id = "subnet-0123456789abcdef0"
  }
}

mock_resource "aws_security_group" {
  defaults = {
    id = "sg-0123456789abcdef0"
  }
}

mock_resource "aws_ecs_task_definition" {
  defaults = {
    arn = "arn:aws:ecs:us-east-1:123456789012:task-definition/sorolens-mock:1"
  }
}
