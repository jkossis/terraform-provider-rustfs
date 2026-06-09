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
