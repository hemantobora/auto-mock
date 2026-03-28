terraform {
  required_version = ">= 1.5"

  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 3.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.0"
    }
  }

  # Backend configuration is injected dynamically by the Go CLI.
  # backend "azurerm" {}
}

provider "azurerm" {
  features {}
  subscription_id = var.subscription_id
}

# ── Kubernetes provider — connects to the AKS cluster created by the aks module ──
provider "kubernetes" {
  host                   = module.aks.kube_config.host
  client_certificate     = base64decode(module.aks.kube_config.client_certificate)
  client_key             = base64decode(module.aks.kube_config.client_key)
  cluster_ca_certificate = base64decode(module.aks.kube_config.cluster_ca_certificate)
}

locals {
  name_prefix = "automock-${var.project_name}"

  common_tags = {
    ManagedBy = "AutoMock-Terraform"
    Project   = "AutoMock"
    ProjectName = var.project_name
    Location    = var.location
  }
}

# ── State Backend ────────────────────────────────────────────────────────────
# Ensures the Blob container used for Terraform state exists.
module "state_backend" {
  count  = var.create_state_backend ? 1 : 0
  source = "git::https://github.com/hemantobora/automock-terraform.git//modules/azure/state-backend"

  resource_group_name  = var.resource_group
  storage_account_name = var.storage_account_name
  container_name       = var.container_name
  location             = var.location
  tags                 = local.common_tags
}

# ── AKS Cluster ──────────────────────────────────────────────────────────────
# Provisions a managed Kubernetes cluster (AKS) in the given resource group.
# The MockServer Deployment and Service are applied inside this cluster via
# the kubernetes provider above.
module "aks" {
  source = "git::https://github.com/hemantobora/automock-terraform.git//modules/azure/aks"

  project_name         = var.project_name
  location             = var.location
  resource_group       = var.resource_group
  subscription_id      = var.subscription_id
  node_vm_size         = var.node_vm_size
  node_count           = var.node_count
  storage_account_name = var.storage_account_name
  container_name       = var.container_name
  tags                 = local.common_tags
}
