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

# Kubernetes provider connects to the AKS cluster provisioned by the locust module.
provider "kubernetes" {
  host                   = module.azure_locust.kube_config.host
  client_certificate     = base64decode(module.azure_locust.kube_config.client_certificate)
  client_key             = base64decode(module.azure_locust.kube_config.client_key)
  cluster_ca_certificate = base64decode(module.azure_locust.kube_config.cluster_ca_certificate)
}

locals {
  common_tags = {
    ManagedBy   = "AutoMock-Terraform"
    Project     = "AutoMock"
    ProjectName = var.project_name
    Component   = "LoadTest"
    Location    = var.location
  }
}

# ── Locust on AKS ─────────────────────────────────────────────────────────────
# Provisions a dedicated AKS cluster and deploys Locust master + workers as
# Kubernetes Deployments.  Master is exposed via a LoadBalancer Service so the
# Locust UI is reachable from outside the cluster.
module "azure_locust" {
  source = "git::https://github.com/hemantobora/automock-terraform.git//modules/azure/locust"

  project_name         = var.project_name
  location             = var.location
  resource_group       = var.resource_group
  subscription_id      = var.subscription_id
  storage_account_name = var.storage_account_name
  container_name       = var.container_name

  # AKS node pool
  node_vm_size = var.node_vm_size
  node_count   = var.node_count

  # Locust workload
  locust_container_image = var.locust_container_image
  worker_desired_count   = var.worker_desired_count
  master_port            = var.master_port

  extra_environment = var.extra_environment

  tags = local.common_tags
}
