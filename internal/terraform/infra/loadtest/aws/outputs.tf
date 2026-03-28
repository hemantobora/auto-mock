output "cluster_name" {
  description = "ECS cluster name for Locust"
  value       = module.aws_locust.cluster_name
}

output "master_service_name" {
  description = "Master service name"
  value       = module.aws_locust.master_service_name
}

output "worker_service_name" {
  description = "Worker service name"
  value       = module.aws_locust.worker_service_name
}

output "worker_desired_count" {
  description = "Desired worker count"
  value       = var.worker_desired_count
}

output "alb_dns_name" {
  description = "ALB DNS name"
  value       = module.aws_locust.alb_dns_name
}

output "cloud_map_master_fqdn" {
  description = "Cloud Map FQDN for master"
  value       = module.aws_locust.cloud_map_master_fqdn
}
