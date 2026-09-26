# Connection URLs live in Secrets Manager and reach the containers through
# ECS "secrets", never as plain environment variables. Note that the
# generated passwords are also in Terraform state: keep state in an
# encrypted, access-controlled backend.

resource "aws_secretsmanager_secret" "database_url" {
  name                    = "${local.name}/database-url"
  description             = "Sorolens ${local.name} PostgreSQL connection URL (DATABASE_URL / DIRECT_DATABASE_URL)"
  recovery_window_in_days = var.secret_recovery_window_days

  tags = local.tags
}

resource "aws_secretsmanager_secret_version" "database_url" {
  secret_id = aws_secretsmanager_secret.database_url.id
  secret_string = format(
    "postgres://%s:%s@%s:%d/%s?sslmode=require",
    aws_rds_cluster.this.master_username,
    random_password.db.result,
    aws_rds_cluster.this.endpoint,
    aws_rds_cluster.this.port,
    aws_rds_cluster.this.database_name,
  )
}

resource "aws_secretsmanager_secret" "redis_url" {
  name                    = "${local.name}/redis-url"
  description             = "Sorolens ${local.name} Redis connection URL (REDIS_URL, TLS)"
  recovery_window_in_days = var.secret_recovery_window_days

  tags = local.tags
}

resource "aws_secretsmanager_secret_version" "redis_url" {
  secret_id = aws_secretsmanager_secret.redis_url.id
  # rediss:// = Redis over TLS, which go-redis understands natively.
  secret_string = format(
    "rediss://:%s@%s:%d",
    random_password.redis.result,
    aws_elasticache_replication_group.this.primary_endpoint_address,
    aws_elasticache_replication_group.this.port,
  )
}
