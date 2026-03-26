# Example: VKE provider with OpenStack application credentials (no user/password).
# Do not set user_name, password, user_domain_name, or tenant_name when using this mode.
# Pass values via TF_VAR_* or a gitignored *.tfvars file.

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

variable "portvmind_application_credential_id" { type = string }
variable "portvmind_application_credential_secret" {
  type      = string
  sensitive = true
}

provider "portvmind" {
  endpoint   = var.portvmind_endpoint
  auth_url   = var.portvmind_auth_url

  application_credential_id     = var.portvmind_application_credential_id
  application_credential_secret = var.portvmind_application_credential_secret
}

# data "portvmind_cluster" "example" {
#   cluster_id = "..."
# }
