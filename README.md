# Terraform Provider: PortvMind

Use the PortvMind Terraform Provider to manage PortvMind cloud services with Terraform.

## Requirements

- Terraform `>= 1.0`

## Supported resources

- `portvmind_vke_cluster`: Create/delete VKE clusters and generate `kubeconfig`
- `portvmind_vke_node_group`: Create/update/delete worker node groups for a cluster
- `data.portvmind_vke_cluster`: Read metadata for an existing cluster

## Quick start

The following example is an end-to-end single-file configuration.

```hcl
terraform {
  required_providers {
    portvmind = {
      source  = "vmindtech/portvmind"
      version = "~> 0.1.1"
    }
  }
}

provider "portvmind" {
  endpoint = "https://<region>-apigw.portvmind.com/vke/api/v1"
  auth_url = "https://<region>-apigw.portvmind.com"

  # Option A: password authentication
  user_name        = "<user_name>"
  password         = "<password>"
  user_domain_name = "Default"
  tenant_name      = "<tenant_name>"

  # Option B: application credential (do not use with Option A)
  # application_credential_id     = "<app_cred_id>"
  # application_credential_secret = "<app_cred_secret>"
}

resource "portvmind_vke_cluster" "main" {
  project_id         = "<project_uuid>"
  name               = "terraform-cluster"
  kubernetes_version = "<supported_k8s_version>"

  node_key_pair_name = "<keypair_name>"
  cluster_api_access = "public"
  subnet_ids = [
    "<subnet_uuid>",
  ]

  worker_node_group_min_size   = 2
  worker_node_group_max_size   = 3
  worker_instance_flavor_uuid  = "<worker_flavor_uuid>"
  master_instance_flavor_uuid  = "<master_flavor_uuid>"
  worker_disk_size_gb          = 50
  allowed_cidrs                = ["0.0.0.0/0"] # Restrict in production.
}

resource "portvmind_vke_node_group" "workers_2" {
  cluster_id       = portvmind_vke_cluster.main.id
  name             = "workers-2"
  node_flavor_uuid = "<node_flavor_uuid>"
  node_disk_size   = 50
  min_size         = 1
  max_size         = 4

  # optional
  # node_group_labels = [
  #   "workload=gpu",
  #   "team=ml",
  # ]
  # node_group_taints = [
  #   "dedicated=worker2:NoSchedule",
  # ]
}

output "cluster_id" {
  value = portvmind_vke_cluster.main.id
}

output "cluster_status" {
  value = portvmind_vke_cluster.main.status
}

output "kubeconfig" {
  value     = portvmind_vke_cluster.main.kubeconfig
  sensitive = true
}
```

## Run

```bash
terraform init
terraform plan
terraform apply
```

## Notes

- The `kubeconfig` attribute is returned as a sensitive value.
- Cluster provisioning can take time depending on infrastructure state.
- Restrict `allowed_cidrs` according to your security requirements.

## License

See `LICENSE` for details.
