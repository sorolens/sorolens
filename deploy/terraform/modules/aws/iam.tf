# Two roles:
# - execution role: used by ECS itself to pull images, write logs and read
#   exactly the secrets the task definitions reference;
# - task role: assumed by the containers. Sorolens calls no AWS APIs, so it
#   has no permissions at all.

locals {
  ecs_tasks_assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ecs-tasks.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })

  readable_secret_arns = distinct(compact(concat(
    [
      aws_secretsmanager_secret.database_url.arn,
      aws_secretsmanager_secret.redis_url.arn,
      var.image_pull_secret_arn,
    ],
    values(var.api_extra_secrets),
    values(var.indexer_extra_secrets),
  )))
}

resource "aws_iam_role" "execution" {
  name               = "${local.name}-ecs-execution"
  assume_role_policy = local.ecs_tasks_assume_role_policy

  tags = local.tags
}

resource "aws_iam_role_policy" "execution" {
  name = "pull-images-write-logs-read-secrets"
  role = aws_iam_role.execution.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "PullFromEcr"
        Effect = "Allow"
        Action = [
          "ecr:GetAuthorizationToken",
          "ecr:BatchCheckLayerAvailability",
          "ecr:GetDownloadUrlForLayer",
          "ecr:BatchGetImage",
        ]
        Resource = "*"
      },
      {
        Sid      = "WriteContainerLogs"
        Effect   = "Allow"
        Action   = ["logs:CreateLogStream", "logs:PutLogEvents"]
        Resource = [for g in aws_cloudwatch_log_group.this : "${g.arn}:*"]
      },
      {
        Sid      = "ReadReferencedSecrets"
        Effect   = "Allow"
        Action   = ["secretsmanager:GetSecretValue"]
        Resource = local.readable_secret_arns
      },
    ]
  })
}

resource "aws_iam_role" "task" {
  name               = "${local.name}-ecs-task"
  assume_role_policy = local.ecs_tasks_assume_role_policy

  tags = local.tags
}
