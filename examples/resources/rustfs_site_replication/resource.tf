resource "rustfs_site_replication" "example" {
  replicate_ilm_expiry = true

  peer = [
    {
      name     = "site-a"
      endpoint = "https://site-a.example.com:9000"
    },
    {
      name     = "site-b"
      endpoint = "https://site-b.example.com:9000"
    },
  ]
}
