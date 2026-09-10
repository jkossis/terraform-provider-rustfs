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

The `peers` list must include every canonical site in the topology, including the deployment behind the provider endpoint. The provider resolves deployment IDs, sends the full topology in the add request, and uses that backend site's canonical endpoint. After creation and updates, it reads every canonical site directly and checks membership, actual deployment IDs, endpoints, names, and ILM expiry settings. A correct response from a VIP or the coordinator alone does not prove convergence.

By default, Terraform uses the provider `access_key` and `secret_key` for each peer. To use different credentials for a specific peer, set both `access_key` and `secret_key` on that peer.

Import uses the fixed singleton ID `site-replication`:

```shell
terraform import rustfs_site_replication.example site-replication
```

Import recovers and retains peer names and endpoints from the reported topology, using the provider credentials for direct checks. A refresh also recovers endpoints from older imported state before replacing its reported `sites`. Review the recovered peer order and any per-site credentials against your configuration before applying; changes to `peers` require replacement. Missing endpoint information prevents removal before any mutation.

### Incomplete operations and recovery

An HTTP 200 response is not sufficient for success. The provider checks add/edit success flags and error details, reports `initialSyncErrorMessage` as an error even when add reports `success: true`, and requires the documented successful remove status. Unknown or partial remove results remain errors. Creation refuses already configured sites or pending operations; import an existing topology instead of creating a second owner.

Once an add request may have changed a site, an error retains the singleton ID and peer configuration in state, even if the follow-up read fails. OpenTofu/Terraform marks failed creations as **tainted** and normally proposes destroying and recreating the replication topology on the next apply. Review that plan before retrying. To preserve an existing topology, repair and verify it directly on every site first, then explicitly clear the taint only after confirming the configuration is complete; removing state would lose ownership. A failed ILM update retains the previous state so the next apply retries the group-wide change.

Reads do not remove state on connection errors, a locally disabled site with a pending operation, or while another configured peer still has replication state. Removal must be confirmed on every configured peer. An unavailable peer blocks that confirmation; resolve connectivity or the pending server operation and retry. These checks establish configuration agreement, not object or metadata parity. An initial-sync error still requires investigation even when membership agrees.

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

Run the full acceptance suite with `mise run testacc`; it sets `TF_ACC=1`. `RUSTFS_ENDPOINT`, `RUSTFS_ACCESS_KEY`, and `RUSTFS_SECRET_KEY` are required. `RUSTFS_INSECURE_SKIP_TLS_VERIFY` may be set when testing against a deployment with untrusted TLS certificates. Put local credentials in ignored `mise.local.toml` or export them in your shell.

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
