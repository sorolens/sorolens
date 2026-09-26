# VPC with two public subnets (load balancer, NAT) and two private subnets
# (tasks, Aurora, ElastiCache). Nothing but the load balancer is reachable
# from the internet.

data "aws_availability_zones" "available" {
  count = length(var.availability_zones) == 0 ? 1 : 0
  state = "available"
}

locals {
  azs = length(var.availability_zones) > 0 ? var.availability_zones : slice(data.aws_availability_zones.available[0].names, 0, 2)

  nat_gateway_count = var.single_nat_gateway ? 1 : 2
}

resource "aws_vpc" "this" {
  cidr_block           = var.vpc_cidr
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = merge(local.tags, { Name = local.name })
}

# Lock down the VPC's default security group so nothing can use it by accident.
resource "aws_default_security_group" "this" {
  vpc_id = aws_vpc.this.id

  tags = merge(local.tags, { Name = "${local.name}-default-deny" })
}

resource "aws_internet_gateway" "this" {
  vpc_id = aws_vpc.this.id

  tags = merge(local.tags, { Name = local.name })
}

resource "aws_subnet" "public" {
  count = 2

  vpc_id            = aws_vpc.this.id
  availability_zone = local.azs[count.index]
  cidr_block        = cidrsubnet(var.vpc_cidr, 4, count.index)

  tags = merge(local.tags, { Name = "${local.name}-public-${local.azs[count.index]}" })
}

resource "aws_subnet" "private" {
  count = 2

  vpc_id            = aws_vpc.this.id
  availability_zone = local.azs[count.index]
  cidr_block        = cidrsubnet(var.vpc_cidr, 4, count.index + 8)

  tags = merge(local.tags, { Name = "${local.name}-private-${local.azs[count.index]}" })
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.this.id

  tags = merge(local.tags, { Name = "${local.name}-public" })
}

resource "aws_route" "public_internet" {
  route_table_id         = aws_route_table.public.id
  destination_cidr_block = "0.0.0.0/0"
  gateway_id             = aws_internet_gateway.this.id
}

resource "aws_route_table_association" "public" {
  count = 2

  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

resource "aws_eip" "nat" {
  count = local.nat_gateway_count

  domain = "vpc"

  tags = merge(local.tags, { Name = "${local.name}-nat-${count.index}" })
}

resource "aws_nat_gateway" "this" {
  count = local.nat_gateway_count

  allocation_id = aws_eip.nat[count.index].id
  subnet_id     = aws_subnet.public[count.index].id

  tags = merge(local.tags, { Name = "${local.name}-${count.index}" })

  depends_on = [aws_internet_gateway.this]
}

resource "aws_route_table" "private" {
  count = 2

  vpc_id = aws_vpc.this.id

  tags = merge(local.tags, { Name = "${local.name}-private-${local.azs[count.index]}" })
}

# Private subnets reach the internet (Soroban RPC, image registry, AWS APIs)
# only through NAT.
resource "aws_route" "private_nat" {
  count = 2

  route_table_id         = aws_route_table.private[count.index].id
  destination_cidr_block = "0.0.0.0/0"
  nat_gateway_id         = aws_nat_gateway.this[var.single_nat_gateway ? 0 : count.index].id
}

resource "aws_route_table_association" "private" {
  count = 2

  subnet_id      = aws_subnet.private[count.index].id
  route_table_id = aws_route_table.private[count.index].id
}

# ---- security groups -------------------------------------------------------------

resource "aws_security_group" "alb" {
  name        = "${local.name}-alb"
  description = "Sorolens load balancer: HTTP/HTTPS from alb_ingress_cidrs"
  vpc_id      = aws_vpc.this.id

  tags = merge(local.tags, { Name = "${local.name}-alb" })
}

resource "aws_security_group" "web" {
  name        = "${local.name}-web"
  description = "Sorolens API and dashboard tasks: traffic from the load balancer only"
  vpc_id      = aws_vpc.this.id

  tags = merge(local.tags, { Name = "${local.name}-web" })
}

resource "aws_security_group" "worker" {
  name        = "${local.name}-worker"
  description = "Sorolens indexer and migrations tasks: no inbound traffic"
  vpc_id      = aws_vpc.this.id

  tags = merge(local.tags, { Name = "${local.name}-worker" })
}

resource "aws_security_group" "db" {
  name        = "${local.name}-db"
  description = "Sorolens Aurora PostgreSQL: 5432 from Sorolens tasks only"
  vpc_id      = aws_vpc.this.id

  tags = merge(local.tags, { Name = "${local.name}-db" })
}

resource "aws_security_group" "redis" {
  name        = "${local.name}-redis"
  description = "Sorolens ElastiCache Redis: 6379 from Sorolens tasks only"
  vpc_id      = aws_vpc.this.id

  tags = merge(local.tags, { Name = "${local.name}-redis" })
}

locals {
  alb_listener_ports = var.certificate_arn != null ? [80, 443] : [80]

  alb_ingress = {
    for pair in setproduct(var.alb_ingress_cidrs, local.alb_listener_ports) :
    "${pair[0]}-${pair[1]}" => { cidr = pair[0], port = pair[1] }
  }

  # Tasks that talk to Postgres and Redis.
  data_client_security_groups = {
    web    = aws_security_group.web.id
    worker = aws_security_group.worker.id
  }
}

resource "aws_vpc_security_group_ingress_rule" "alb" {
  for_each = local.alb_ingress

  security_group_id = aws_security_group.alb.id
  description       = "HTTP(S) from ${each.value.cidr}"
  cidr_ipv4         = each.value.cidr
  ip_protocol       = "tcp"
  from_port         = each.value.port
  to_port           = each.value.port
}

resource "aws_vpc_security_group_egress_rule" "alb_to_web" {
  for_each = toset([tostring(module.core.api_port), tostring(module.core.dashboard_port)])

  security_group_id            = aws_security_group.alb.id
  description                  = "To Sorolens tasks on ${each.value}"
  referenced_security_group_id = aws_security_group.web.id
  ip_protocol                  = "tcp"
  from_port                    = tonumber(each.value)
  to_port                      = tonumber(each.value)
}

resource "aws_vpc_security_group_ingress_rule" "web_from_alb" {
  for_each = toset([tostring(module.core.api_port), tostring(module.core.dashboard_port)])

  security_group_id            = aws_security_group.web.id
  description                  = "From the load balancer on ${each.value}"
  referenced_security_group_id = aws_security_group.alb.id
  ip_protocol                  = "tcp"
  from_port                    = tonumber(each.value)
  to_port                      = tonumber(each.value)
}

# Tasks need outbound access to Soroban RPC, the image registry and AWS APIs.
resource "aws_vpc_security_group_egress_rule" "tasks_outbound" {
  for_each = local.data_client_security_groups

  security_group_id = each.value
  description       = "Outbound for RPC, registry and AWS APIs"
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "-1"
}

resource "aws_vpc_security_group_ingress_rule" "db_from_tasks" {
  for_each = local.data_client_security_groups

  security_group_id            = aws_security_group.db.id
  description                  = "PostgreSQL from ${each.key} tasks"
  referenced_security_group_id = each.value
  ip_protocol                  = "tcp"
  from_port                    = 5432
  to_port                      = 5432
}

resource "aws_vpc_security_group_ingress_rule" "redis_from_tasks" {
  for_each = local.data_client_security_groups

  security_group_id            = aws_security_group.redis.id
  description                  = "Redis from ${each.key} tasks"
  referenced_security_group_id = each.value
  ip_protocol                  = "tcp"
  from_port                    = 6379
  to_port                      = 6379
}
