##############################################
# AutoMock LoadTest Variables (Azure / AKS)
##############################################

# ── General ──────────────────────────────────────────────────────────────────

variable "project_name" {
  type        = string
  description = "Project name (base for Kubernetes/Azure resource names)"

  validation {
    condition     = can(regex("^[a-z0-9-]+$", var.project_name))
    error_message = "Project name must contain only lowercase letters, numbers, and hyphens."
  }
}

variable "location" {
  type        = string
  description = "Azure region in which to deploy the AKS cluster (e.g. eastus)"
  default     = "eastus"
}

variable "subscription_id" {
  type        = string
  description = "Azure subscription ID"
}

variable "resource_group" {
  type        = string
  description = "Existing resource group to deploy the AKS loadtest cluster into"
  default     = "auto-mock-rg"
}

variable "cloud_provider" {
  type        = string
  description = "Cloud provider identifier (azure)"
  default     = "azure"
}

# ── Storage (reuse account created by `automock init`) ────────────────────────

variable "storage_account_name" {
  type        = string
  description = "Existing Azure Storage Account name — used to fetch Locust test scripts"
}

variable "container_name" {
  type        = string
  description = "Blob container name (e.g. auto-mock-<project>)"
}

# ── AKS node pool ─────────────────────────────────────────────────────────────

variable "node_vm_size" {
  type        = string
  description = "VM size for AKS worker nodes (e.g. Standard_B2s for dev, Standard_D2s_v3 for load)"
  default     = "Standard_B2s"
}

variable "node_count" {
  type        = number
  description = "Number of nodes in the AKS default node pool"
  default     = 1
}

# ── Locust workload ───────────────────────────────────────────────────────────

variable "locust_container_image" {
  type        = string
  description = "Locust container image to run in AKS pods"
  default     = "locustio/locust:2.31.2"
}

variable "worker_desired_count" {
  type        = number
  description = "Desired number of Locust worker pods (0 = master-only / idle)"
  default     = 0
}

variable "master_port" {
  type        = number
  description = "Port exposed by the Locust master UI (LoadBalancer Service)"
  default     = 8089
}

# ── Misc ──────────────────────────────────────────────────────────────────────

variable "extra_environment" {
  type        = map(string)
  description = "Arbitrary KEY=VALUE environment variables injected into master and worker pods. Avoid secrets — values are stored in Kubernetes ConfigMaps."
  default     = {}
}
