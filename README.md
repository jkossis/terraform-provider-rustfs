# Terraform Provider for RustFS

This provider manages RustFS administration APIs. The initial implementation focuses on RustFS site replication.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://go.dev/doc/install) >= 1.24

## Provider Configuration

```terraform
provider "rustfs" {
  endpoint   = "https://rustfs.example.com:9000"
  access_key = var.rustfs_access_key
  secret_key = var.rustfs_secret_key
}
```

Configuration can also be supplied with environment variables:

- `RUSTFS_ENDPOINT`
- `RUSTFS_ACCESS_KEY`
- `RUSTFS_SECRET_KEY`
- `RUSTFS_INSECURE_SKIP_TLS_VERIFY=true`

## Site Replication

```terraform
resource "rustfs_site_replication" "example" {
  replicate_ilm_expiry = true

  peer = [
    {
      name       = "site-b"
      endpoint   = "https://site-b.example.com:9000"
      access_key = var.site_b_access_key
      secret_key = var.site_b_secret_key
    },
  ]
}
```

The configured provider endpoint is treated as the local RustFS site. Remote peer credentials are used by RustFS only while joining the peer sites.

Import uses the fixed singleton ID `site-replication`:

```shell
terraform import rustfs_site_replication.example site-replication
```

## Data Sources

- `rustfs_site_replication_info`
- `rustfs_site_replication_status`
- `rustfs_site_replication_metainfo`

The status and metainfo data sources expose typed top-level fields and a `raw_json` attribute for the full RustFS response.

## Development

Build and test the provider:

```shell
go test ./...
```

Run data source acceptance tests against a RustFS deployment:

```shell
export TF_ACC=1
export RUSTFS_ENDPOINT="https://rustfs.example.com:9000"
export RUSTFS_ACCESS_KEY="..."
export RUSTFS_SECRET_KEY="..."
go test ./internal/provider -run 'TestAccSiteReplication.*DataSource' -v
```

Run the site replication resource acceptance test only against disposable replication test sites. It creates site replication topology and removes all site replication state during destroy:

```shell
export TF_ACC=1
export RUSTFS_SITE_REPLICATION_PEER_NAME="site-b"
export RUSTFS_SITE_REPLICATION_PEER_ENDPOINT="https://site-b.example.com:9000"
export RUSTFS_SITE_REPLICATION_PEER_ACCESS_KEY="..."
export RUSTFS_SITE_REPLICATION_PEER_SECRET_KEY="..."
go test ./internal/provider -run TestAccSiteReplicationResource_basic -v
```

Generate documentation:

```shell
make generate
```
