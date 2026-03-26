# Terraform Provider: PortvMind

Terraform provider for PortvMind services. The current release includes VKE resources and obtains an OpenStack Keystone token, then sends it to the API as `X-Auth-Token`.

## Requirements

- Terraform 1.0+
- Go 1.22+ (for development)

## Usage (Terraform Registry)

After the provider is published:

```hcl
terraform {
  required_providers {
    portvmind = {
      source  = "vmindtech/portvmind"
      version = "~> 0.1"
    }
  }
}

provider "portvmind" {
  endpoint = "https://YOUR-VKE-API/api/v1"
  auth_url = "https://YOUR-KEYSTONE:5000/v3"

  # Option A: password (same variables as the OpenStack provider)
  user_name        = var.user_name
  password         = var.password
  user_domain_name = var.user_domain_name
  tenant_name      = var.project_name

  # Option B: application credential (do not set password fields)
  # application_credential_id     = var.app_cred_id
  # application_credential_secret = var.app_cred_secret
}
```

## Publishing and testing via the Registry

1. Host the repository at **`github.com/vmindtech/terraform-provider-portvmind`** (or keep `go.mod` consistent with your module path).
2. Sign in to the [Terraform Registry](https://registry.terraform.io/sign-in) → **Publish** → Provider → connect GitHub; namespace `vmindtech`, provider name `portvmind`.
3. **GPG signing:** [Signing keys](https://developer.hashicorp.com/terraform/registry/providers/publishing#preparing-and-gpg-signing) — add your GPG key in the Registry. For the first release you can try without signing; if the Registry requires it, uncomment the `signs` block in `.goreleaser.yml` and set `GPG_FINGERPRINT` locally.
4. Push a version tag; the GitHub Actions `release` workflow runs GoReleaser and publishes zips and `SHA256SUMS`:

```bash
git tag v0.1.0
git push origin v0.1.0
```

5. After a short delay, `terraform init` will download the provider from the Registry.

## Local build

```bash
go build -o terraform-provider-portvmind .
```

## Resources

- `portvmind_vke_cluster` — create/destroy cluster; `kubeconfig` after the cluster is Active
- `portvmind_vke_node_group` — worker node groups
- `portvmind_vke_cluster` (data source) — read existing cluster metadata

## Examples

- `examples/minimal/main.tf` — password auth (aligned with the OpenStack provider variables)
- `examples/minimal/application_credential/main.tf` — application credential auth

### Anonymized end-to-end example

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

  # Option A: password (same variables as the OpenStack provider)
  user_name        = "<user_name>"
  password         = "<password>"
  user_domain_name = "Default"
  tenant_name      = "<tenant_name>"
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

resource "local_file" "kubeconfig" {
  filename        = "${path.module}/kubeconfig.yaml"
  content         = portvmind_vke_cluster.main.kubeconfig
  file_permission = "0600"
}

output "kubeconfig_base64" {
  description = "Kubeconfig YAML (base64)"
  value       = base64encode(portvmind_vke_cluster.main.kubeconfig)
  sensitive   = true
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
```

## License

See `LICENSE`.
