data "aws_vpc" "this" {
  id      = var.vpc_id
  default = var.vpc_id == null
}

data "aws_subnets" "this" {
  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.this.id]
  }
}

locals {
  subnet_ids = coalesce(var.subnet_ids, data.aws_subnets.this.ids)
  https      = var.certificate_arn != null
  url        = "${local.https ? "https" : "http"}://${coalesce(var.domain, aws_lb.this.dns_name)}"
}

resource "random_password" "db" {
  length  = 32
  special = false
}

resource "random_id" "secret_key" {
  byte_length = 32
}

resource "aws_security_group" "lb" {
  name   = "${var.name}-lb"
  vpc_id = data.aws_vpc.this.id

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "app" {
  name   = "${var.name}-app"
  vpc_id = data.aws_vpc.this.id

  ingress {
    from_port       = 8080
    to_port         = 8080
    protocol        = "tcp"
    security_groups = [aws_security_group.lb.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "db" {
  name   = "${var.name}-db"
  vpc_id = data.aws_vpc.this.id

  ingress {
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.app.id]
  }
}

resource "aws_db_subnet_group" "this" {
  name       = var.name
  subnet_ids = local.subnet_ids
}

resource "aws_db_instance" "this" {
  identifier              = var.name
  engine                  = "postgres"
  engine_version          = "18"
  instance_class          = var.db_instance_class
  allocated_storage       = var.db_storage
  storage_encrypted       = true
  db_name                 = "archive"
  username                = "archive"
  password                = random_password.db.result
  db_subnet_group_name    = aws_db_subnet_group.this.name
  vpc_security_group_ids  = [aws_security_group.db.id]
  backup_retention_period = 7
  deletion_protection     = var.deletion_protection
  skip_final_snapshot     = true
}

resource "aws_s3_bucket" "this" {
  bucket_prefix = "${var.name}-"
}

resource "aws_iam_user" "blob" {
  name = "${var.name}-blob"
}

resource "aws_iam_user_policy" "blob" {
  user = aws_iam_user.blob.name
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["s3:GetObject", "s3:PutObject", "s3:DeleteObject", "s3:ListBucket"]
      Resource = [aws_s3_bucket.this.arn, "${aws_s3_bucket.this.arn}/*"]
    }]
  })
}

resource "aws_iam_access_key" "blob" {
  user = aws_iam_user.blob.name
}

resource "aws_secretsmanager_secret" "this" {
  name_prefix = "${var.name}-"
}

resource "aws_secretsmanager_secret_version" "this" {
  secret_id = aws_secretsmanager_secret.this.id
  secret_string = jsonencode({
    DATABASE_URL        = "postgres://archive:${random_password.db.result}@${aws_db_instance.this.address}:${aws_db_instance.this.port}/archive?sslmode=require"
    ARCHIVE_SECRET_KEY  = random_id.secret_key.hex
    ARCHIVE_BLOB_SECRET = aws_iam_access_key.blob.secret
  })
}

resource "aws_lb" "this" {
  name            = var.name
  subnets         = local.subnet_ids
  security_groups = [aws_security_group.lb.id]
}

resource "aws_lb_target_group" "this" {
  name        = var.name
  port        = 8080
  protocol    = "HTTP"
  target_type = "ip"
  vpc_id      = data.aws_vpc.this.id

  health_check {
    path    = "/healthz"
    matcher = "204"
  }
}

resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.this.arn
  port              = 80

  default_action {
    type             = local.https ? "redirect" : "forward"
    target_group_arn = local.https ? null : aws_lb_target_group.this.arn

    dynamic "redirect" {
      for_each = local.https ? [1] : []
      content {
        port        = "443"
        protocol    = "HTTPS"
        status_code = "HTTP_301"
      }
    }
  }
}

resource "aws_lb_listener" "https" {
  count             = local.https ? 1 : 0
  load_balancer_arn = aws_lb.this.arn
  port              = 443
  protocol          = "HTTPS"
  certificate_arn   = var.certificate_arn

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.this.arn
  }
}

resource "aws_cloudwatch_log_group" "this" {
  name              = "/ecs/${var.name}"
  retention_in_days = 30
}

resource "aws_iam_role" "execution" {
  name = "${var.name}-execution"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Action    = "sts:AssumeRole"
      Principal = { Service = "ecs-tasks.amazonaws.com" }
    }]
  })
}

resource "aws_iam_role_policy_attachment" "execution" {
  role       = aws_iam_role.execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

resource "aws_iam_role_policy" "execution" {
  role = aws_iam_role.execution.name
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = "secretsmanager:GetSecretValue"
      Resource = aws_secretsmanager_secret.this.arn
    }]
  })
}

resource "aws_ecs_cluster" "this" {
  name = var.name
}

resource "aws_ecs_task_definition" "this" {
  family                   = var.name
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.cpu
  memory                   = var.memory
  execution_role_arn       = aws_iam_role.execution.arn

  container_definitions = jsonencode([{
    name         = "archive"
    image        = "ghcr.io/mulholland-inc/archive:${var.image_tag}"
    essential    = true
    portMappings = [{ containerPort = 8080 }]
    environment = [for k, v in merge(var.env, {
      ARCHIVE_URL         = local.url
      ARCHIVE_ADMIN_EMAIL = var.admin_email
      ARCHIVE_INSTANCE    = var.instance
      ARCHIVE_BLOB_URL    = "s3://${aws_s3_bucket.this.bucket}?region=${var.region}"
      ARCHIVE_BLOB_KEY    = aws_iam_access_key.blob.id
    }) : { name = k, value = v }]
    secrets = [for k in ["DATABASE_URL", "ARCHIVE_SECRET_KEY", "ARCHIVE_BLOB_SECRET"] : {
      name      = k
      valueFrom = "${aws_secretsmanager_secret.this.arn}:${k}::"
    }]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        awslogs-group         = aws_cloudwatch_log_group.this.name
        awslogs-region        = var.region
        awslogs-stream-prefix = "archive"
      }
    }
  }])

  depends_on = [aws_secretsmanager_secret_version.this]
}

resource "aws_ecs_service" "this" {
  name                               = var.name
  cluster                            = aws_ecs_cluster.this.id
  task_definition                    = aws_ecs_task_definition.this.arn
  launch_type                        = "FARGATE"
  desired_count                      = 1
  deployment_minimum_healthy_percent = 0
  deployment_maximum_percent         = 100
  health_check_grace_period_seconds  = 300

  network_configuration {
    subnets          = local.subnet_ids
    security_groups  = [aws_security_group.app.id]
    assign_public_ip = true
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.this.arn
    container_name   = "archive"
    container_port   = 8080
  }

  depends_on = [aws_lb_listener.http]
}
