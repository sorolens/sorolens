# ElastiCache Redis OSS for the indexer's advisory locks and the API cache.
# Encrypted at rest and in transit, AUTH token required, private subnets only.

resource "random_password" "redis" {
  length = 32
  # ElastiCache AUTH tokens reject several symbols; alphanumeric is always valid.
  special = false
}

resource "aws_elasticache_subnet_group" "this" {
  name       = "${local.name}-redis"
  subnet_ids = aws_subnet.private[*].id

  tags = local.tags
}

# Own parameter group (the AWS default one cannot be edited) so settings can
# be tuned later without replacing the cluster.
resource "aws_elasticache_parameter_group" "this" {
  name   = "${local.name}-redis7"
  family = "redis7"

  tags = local.tags
}

resource "aws_elasticache_replication_group" "this" {
  replication_group_id = "${local.name}-redis"
  description          = "Sorolens ${local.name} Redis"

  engine               = "redis"
  engine_version       = var.redis_engine_version
  node_type            = var.redis_node_type
  num_cache_clusters   = var.redis_num_cache_clusters
  parameter_group_name = aws_elasticache_parameter_group.this.name
  port                 = 6379

  subnet_group_name  = aws_elasticache_subnet_group.this.name
  security_group_ids = [aws_security_group.redis.id]

  at_rest_encryption_enabled = true
  transit_encryption_enabled = true
  auth_token                 = random_password.redis.result

  automatic_failover_enabled = var.redis_num_cache_clusters > 1
  multi_az_enabled           = var.redis_num_cache_clusters > 1

  tags = local.tags
}
