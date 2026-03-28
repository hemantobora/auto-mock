# Output names intentionally mirror the AWS loadtest outputs so the Go CLI can
# read them with a single provider-agnostic terraform output -json call.

output "cluster_name" {
  description = "AKS cluster name for the Locust loadtest stack"
  value       = module.azure_locust.cluster_name
}

output "master_service_name" {
  description = "Kubernetes Service name for the Locust master"
  value       = module.azure_locust.master_service_name
}

output "worker_service_name" {
  description = "Kubernetes Deployment name for Locust workers"
  value       = module.azure_locust.worker_deployment_name
}

output "worker_desired_count" {
  description = "Desired Locust worker pod count"
  value       = var.worker_desired_count
}

# alb_dns_name → external IP / hostname of the Kubernetes LoadBalancer Service
# (equivalent to the AWS ALB DNS name — the address users browse to for the Locust UI)
output "alb_dns_name" {
  description = "External IP or hostname of the Locust master LoadBalancer Service"
  value       = module.azure_locust.master_external_ip
}

# cloud_map_master_fqdn → Kubernetes cluster-internal DNS for master-worker comms
# (equivalent to the AWS Cloud Map FQDN used by worker tasks to reach the master)
output "cloud_map_master_fqdn" {
  description = "Kubernetes internal DNS FQDN for the Locust master service (used by workers)"
  value       = module.azure_locust.master_internal_fqdn
}
