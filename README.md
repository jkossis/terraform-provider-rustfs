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

  peers = [
    {
      name       = "site-a"
      endpoint   = "https://site-a.example.com:9000"
    },
    {
      name       = "site-b"
      endpoint   = "https://site-b.example.com:9000"
    },
  ]
}
```

The `peers` list is the desired set of RustFS peer sites for replication. It can include every canonical site in the active-active topology. This supports configuring the provider with a VIP endpoint that may route to any site. Before configuring replication, the provider identifies the site currently serving the provider endpoint and omits that site from the RustFS add request.

By default, Terraform uses the provider `access_key` and `secret_key` for each peer. To use different credentials for a specific peer, set both `access_key` and `secret_key` on that peer.

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
export RUSTFS_ENDPOINT="https://rustfs.example.com:9000"
export RUSTFS_ACCESS_KEY="..."
export RUSTFS_SECRET_KEY="..."
export RUSTFS_SITE_REPLICATION_PEERS='[
  {"name":"site-a","endpoint":"https://site-a.example.com:9000"},
  {"name":"site-b","endpoint":"https://site-b.example.com:9000"}
]'
go test ./internal/provider -run TestAccSiteReplicationResource_basic -v
```

Generate documentation:

```shell
make generate
```
