# Terraform Provider for RustFS

This provider manages RustFS administration APIs. The initial implementation focuses on RustFS site replication.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://go.dev/doc/install) >= 1.25.8

## Required Providers

```terraform
terraform {
  required_providers {
    rustfs = {
      source = "jkossis/rustfs"
    }
  }
}
```

## Install

Initialize Terraform to install the provider from the registry:

```shell
terraform init
```

## Provider Configuration

```terraform
provider "rustfs" {
  endpoint   = "https://rustfs.example.com:9000"
  access_key = var.rustfs_access_key
  secret_key = var.rustfs_secret_key
}
```

Configuration can also be supplied with environment variables:

- `RUSTFS_ENDPOINT`: RustFS endpoint, including `http://` or `https://`.
- `RUSTFS_ACCESS_KEY`: RustFS administrator access key.
- `RUSTFS_SECRET_KEY`: RustFS administrator secret key.
- `RUSTFS_INSECURE_SKIP_TLS_VERIFY`: optional boolean accepted by Go's standard boolean parser, such as `true`, `false`, `1`, or `0`.

Values set in the provider block take precedence over environment variables. `endpoint`, `access_key`, and `secret_key` must be provided either way. `insecure_skip_tls_verify` is optional and is not required for tests.

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

The `peers` list is the desired set of RustFS peer sites for replication. It can include every canonical site in the active-active topology. This supports configuring the provider with a VIP endpoint that may route to any site. Before configuring replication, the provider identifies the site currently serving the provider endpoint, omits that site from the RustFS add request, and sends the add request through that site's canonical endpoint when available.

By default, Terraform uses the provider `access_key` and `secret_key` for each peer. To use different credentials for a specific peer, set both `access_key` and `secret_key` on that peer.

Import uses the fixed singleton ID `site-replication`:

```shell
terraform import rustfs_site_replication.example site-replication
```

## Data Sources

- `rustfs_site_replication_info`
- `rustfs_site_replication_status`
- `rustfs_site_replication_metainfo`

The data sources expose typed top-level fields and a sensitive `raw_json` attribute for the full RustFS response. Terraform redacts this value because RustFS responses can contain service-account credentials.

## Build

Build the provider locally:

```shell
go build ./...
```

Run the fast test suite:

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

Acceptance tests are skipped unless `TF_ACC=1` is set. When `TF_ACC=1` is set, `RUSTFS_ENDPOINT`, `RUSTFS_ACCESS_KEY`, and `RUSTFS_SECRET_KEY` are required. `RUSTFS_INSECURE_SKIP_TLS_VERIFY` may be set when testing against a deployment with untrusted TLS certificates.

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
