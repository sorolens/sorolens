# Aurora PostgreSQL Serverless v2 in the private subnets, encrypted at rest,
# never publicly reachable. The app uses plain pgx over TCP (nothing
# Neon-specific), so DATABASE_URL and DIRECT_DATABASE_URL are the same URL.

resource "random_password" "db" {
  length = 32
  # Alphanumeric only, so the password is safe inside a connection URL.
  special = false
}

resource "aws_db_subnet_group" "this" {
  name       = "${local.name}-db"
  subnet_ids = aws_subnet.private[*].id

  tags = local.tags
}

resource "aws_rds_cluster" "this" {
  cluster_identifier = "${local.name}-db"
  engine             = "aurora-postgresql"
  engine_mode        = "provisioned"
  engine_version     = var.db_engine_version

  database_name   = "sorolens"
  master_username = "sorolens"
  master_password = random_password.db.result

  db_subnet_group_name   = aws_db_subnet_group.this.name
  vpc_security_group_ids = [aws_security_group.db.id]

  storage_encrypted                   = true
  iam_database_authentication_enabled = false
  enabled_cloudwatch_logs_exports     = ["postgresql"]

  backup_retention_period   = var.db_backup_retention_days
  copy_tags_to_snapshot     = true
  deletion_protection       = var.db_deletion_protection
  skip_final_snapshot       = var.db_skip_final_snapshot
  final_snapshot_identifier = var.db_skip_final_snapshot ? null : "${local.name}-db-final"

  serverlessv2_scaling_configuration {
    min_capacity = var.db_min_capacity
    max_capacity = var.db_max_capacity
  }

  tags = local.tags

  lifecycle {
    precondition {
      condition     = var.db_min_capacity >= 0 && var.db_min_capacity <= var.db_max_capacity
      error_message = "db_min_capacity must be between 0 and db_max_capacity."
    }
  }
}

resource "aws_rds_cluster_instance" "this" {
  count = var.db_instance_count

  identifier         = "${local.name}-db-${count.index}"
  cluster_identifier = aws_rds_cluster.this.id
  instance_class     = "db.serverless"
  engine             = aws_rds_cluster.this.engine
  engine_version     = aws_rds_cluster.this.engine_version

  db_subnet_group_name       = aws_db_subnet_group.this.name
  publicly_accessible        = false
  auto_minor_version_upgrade = true

  tags = local.tags
}
