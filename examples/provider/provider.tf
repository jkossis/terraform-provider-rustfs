terraform {
  required_providers {
    rustfs = {
      source = "jkossis/rustfs"
    }
  }
}

provider "rustfs" {
  endpoint   = "https://rustfs.example.com:9000"
  access_key = var.rustfs_access_key
  secret_key = var.rustfs_secret_key
}
