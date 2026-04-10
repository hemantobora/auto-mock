##############################################
# AutoMock Root Variables (Azure / AKS)
##############################################

# ── General ──────────────────────────────────────────────────────────────────

variable "project_name" {
  description = "Unique name of the AutoMock project; used for tagging and naming Azure resources."
  type        = string

  validation {
    condition     = can(regex("^[a-z0-9-]+$", var.project_name))
    error_message = "Project name must contain only lowercase letters, numbers, and hyphens."
  }
}

variable "location" {
  description = "Azure region in which to deploy infrastructure (e.g., eastus)."
  type        = string
  default     = "eastus"
}

variable "subscription_id" {
  description = "Azure subscription ID."
  type        = string
}

variable "resource_group" {
  description = "Name of the Azure resource group to deploy into."
  type        = string
  default     = "auto-mock-rg"
}

# ── Storage (reuse the account created by `automock init`) ───────────────────

variable "storage_account_name" {
  description = "Name of the existing Azure Storage Account (created by automock init)."
  type        = string
}

variable "container_name" {
  description = "Name of the Blob container for this project (e.g., auto-mock-<project>)."
  type        = string
}

variable "create_state_backend" {
  description = "Whether to run the state-backend module (ensures the Blob container exists)."
  type        = bool
  default     = false
}

# ── AKS Node Pool ────────────────────────────────────────────────────────────

variable "node_vm_size" {
  description = "VM size for AKS worker nodes (e.g., Standard_B2s for dev, Standard_D2s_v3 for prod)."
  type        = string
  default     = "Standard_B2s"
}

variable "node_count" {
  description = "Initial number of nodes in the AKS default node pool."
  type        = number
  default     = 1
}
