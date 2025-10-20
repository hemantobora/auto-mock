# terraform/modules/automock-ecs/ssl.tf
# SSL Certificate, Domain Configuration, and ALB Listeners

# ACM Certificate for custom domain
resource "aws_acm_certificate" "main" {
  count = var.custom_domain != "" ? 1 : 0

  domain_name       = var.custom_domain
  validation_method = "DNS"

  tags = merge(local.common_tags, {
    Name = "${local.name_prefix}-certificate"
  })

  lifecycle {
    create_before_destroy = true
  }
}

# Route53 record for ACM certificate validation
resource "aws_route53_record" "cert_validation" {
  for_each = var.custom_domain != "" ? {
    for dvo in aws_acm_certificate.main[0].domain_validation_options : dvo.domain_name => {
      name   = dvo.resource_record_name
      record = dvo.resource_record_value
      type   = dvo.resource_record_type
    }
  } : {}

  allow_overwrite = true
  name            = each.value.name
  records         = [each.value.record]
  type            = each.value.type
  zone_id         = data.aws_route53_zone.domain[0].zone_id
}

# ACM certificate validation
resource "aws_acm_certificate_validation" "main" {
  count = var.custom_domain != "" ? 1 : 0

  certificate_arn         = aws_acm_certificate.main[0].arn
  validation_record_fqdns = [for record in aws_route53_record.cert_validation : record.fqdn]

  timeouts {
    create = "5m"
  }
}

# Route53 A record for custom domain
resource "aws_route53_record" "main" {
  count = var.custom_domain != "" ? 1 : 0

  zone_id = data.aws_route53_zone.domain[0].zone_id
  name    = var.custom_domain
  type    = "A"

  alias {
    name                   = aws_lb.main.dns_name
    zone_id                = aws_lb.main.zone_id
    evaluate_target_health = true
  }
}

# Route53 AAAA record for IPv6 support
resource "aws_route53_record" "ipv6" {
  count = var.custom_domain != "" ? 1 : 0

  zone_id = data.aws_route53_zone.domain[0].zone_id
  name    = var.custom_domain
  type    = "AAAA"

  alias {
    name                   = aws_lb.main.dns_name
    zone_id                = aws_lb.main.zone_id
    evaluate_target_health = true
  }
}

# HTTP Listener (port 80)
resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.main.arn
  port              = "80"
  protocol          = "HTTP"

  # Redirect HTTP to HTTPS if custom domain is used
  dynamic "default_action" {
    for_each = var.custom_domain != "" ? [1] : []
    content {
      type = "redirect"
      redirect {
        port        = "443"
        protocol    = "HTTPS"
        status_code = "HTTP_301"
      }
    }
  }

  # Forward to MockServer API if no custom domain
  dynamic "default_action" {
    for_each = var.custom_domain == "" ? [1] : []
    content {
      type             = "forward"
      target_group_arn = aws_lb_target_group.mockserver_api.arn
    }
  }

  tags = local.common_tags
}

# HTTPS Listener for API (port 443) - Only if custom domain
resource "aws_lb_listener" "https_api" {
  count = var.custom_domain != "" ? 1 : 0

  load_balancer_arn = aws_lb.main.arn
  port              = "443"
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-TLS-1-2-2017-01"
  certificate_arn   = aws_acm_certificate_validation.main[0].certificate_arn

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.mockserver_api.arn
  }

  tags = local.common_tags
}