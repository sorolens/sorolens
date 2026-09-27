# Public Application Load Balancer: /api/*, /health and /readyz go to the
# API, everything else to the dashboard. HTTPS unless allow_http_only is set
# explicitly.

resource "aws_lb" "this" {
  name               = "${local.name}-alb"
  load_balancer_type = "application"
  internal           = false
  subnets            = aws_subnet.public[*].id
  security_groups    = [aws_security_group.alb.id]

  drop_invalid_header_fields = true
  enable_deletion_protection = var.alb_deletion_protection

  tags = local.tags

  lifecycle {
    precondition {
      condition     = var.certificate_arn != null || var.allow_http_only
      error_message = "Set certificate_arn to serve HTTPS, or set allow_http_only = true to explicitly accept plain HTTP for an evaluation stack."
    }
    precondition {
      condition     = var.route53_zone_id == null || var.domain_name != null
      error_message = "route53_zone_id is set but domain_name is not: the alias record needs a name."
    }
  }
}

resource "aws_lb_target_group" "api" {
  name        = "${local.name}-api"
  port        = module.core.api_port
  protocol    = "HTTP"
  target_type = "ip"
  vpc_id      = aws_vpc.this.id

  deregistration_delay = 30

  health_check {
    path                = "/health"
    matcher             = "200"
    interval            = 15
    healthy_threshold   = 2
    unhealthy_threshold = 3
  }

  tags = local.tags
}

resource "aws_lb_target_group" "dashboard" {
  name        = "${local.name}-dashboard"
  port        = module.core.dashboard_port
  protocol    = "HTTP"
  target_type = "ip"
  vpc_id      = aws_vpc.this.id

  deregistration_delay = 30

  health_check {
    path                = "/"
    matcher             = "200-399"
    interval            = 15
    healthy_threshold   = 2
    unhealthy_threshold = 3
  }

  tags = local.tags
}

resource "aws_lb_listener" "https" {
  count = var.certificate_arn != null ? 1 : 0

  load_balancer_arn = aws_lb.this.arn
  port              = 443
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-TLS13-1-2-2021-06"
  certificate_arn   = var.certificate_arn

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.dashboard.arn
  }

  tags = local.tags
}

# With a certificate, port 80 only redirects to HTTPS.
resource "aws_lb_listener" "http_redirect" {
  count = var.certificate_arn != null ? 1 : 0

  load_balancer_arn = aws_lb.this.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type = "redirect"

    redirect {
      port        = "443"
      protocol    = "HTTPS"
      status_code = "HTTP_301"
    }
  }

  tags = local.tags
}

# Plain HTTP only when explicitly allowed and no certificate is given.
resource "aws_lb_listener" "http" {
  count = var.certificate_arn == null ? 1 : 0

  load_balancer_arn = aws_lb.this.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.dashboard.arn
  }

  tags = local.tags
}

resource "aws_lb_listener_rule" "api" {
  listener_arn = var.certificate_arn != null ? aws_lb_listener.https[0].arn : aws_lb_listener.http[0].arn
  priority     = 10

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.api.arn
  }

  condition {
    path_pattern {
      values = ["/api/*", "/health", "/readyz"]
    }
  }

  tags = local.tags
}

resource "aws_route53_record" "this" {
  count = var.route53_zone_id != null ? 1 : 0

  zone_id = var.route53_zone_id
  name    = var.domain_name
  type    = "A"

  alias {
    name                   = aws_lb.this.dns_name
    zone_id                = aws_lb.this.zone_id
    evaluate_target_health = true
  }
}
