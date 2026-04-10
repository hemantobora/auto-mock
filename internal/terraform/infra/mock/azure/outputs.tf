# outputs.tf — Azure / AKS MockServer outputs
# These mirror the names used by the AWS stack so manager.go's getOutputs()
# can parse them identically regardless of provider.

output "mockserver_url" {
  description = "MockServer API endpoint (public IP assigned by Azure Load Balancer)."
  value       = module.aks.mockserver_url
}

output "dashboard_url" {
  description = "MockServer dashboard URL."
  value       = module.aks.dashboard_url
}

output "config_bucket" {
  description = "Azure Blob container name used for project storage (mirrors S3 bucket output)."
  value       = var.container_name
}

output "integration_summary" {
  description = "Integration summary for the CLI."
  value = {
    project_name   = var.project_name
    mockserver_url = module.aks.mockserver_url
    dashboard_url  = module.aks.dashboard_url
    location       = var.location
  }
}

output "cli_integration_commands" {
  description = "CLI commands for interacting with the deployed infrastructure."
  value       = module.aks.cli_integration_commands
}

output "infrastructure_summary" {
  description = "Complete infrastructure summary."
  value       = module.aks.infrastructure_summary
}
