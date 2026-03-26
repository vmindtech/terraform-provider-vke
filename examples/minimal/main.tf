# Example: password authentication (same inputs as the OpenStack provider).
# For application credentials, see: examples/minimal/application_credential/
# Pass credentials via TF_VAR_* or a gitignored *.tfvars file.

terraform {
  required_providers {
    portvmind = {
      source  = "vmindtech/portvmind"
      version = "~> 0.1"
    }
  }
}

variable "portvmind_endpoint" { type = string }
variable "portvmind_auth_url" { type = string }

variable "portvmind_user_name" { type = string }
variable "portvmind_password" { type = string sensitive = true }
variable "portvmind_user_domain_name" { type = string }
variable "portvmind_tenant_name" { type = string }

provider "portvmind" {
  endpoint         = var.portvmind_endpoint
  auth_url         = var.portvmind_auth_url
  user_name        = var.portvmind_user_name
  password         = var.portvmind_password
  user_domain_name = var.portvmind_user_domain_name
  tenant_name      = var.portvmind_tenant_name
}

# data "portvmind_cluster" "example" {
#   cluster_id = "..."
# }
