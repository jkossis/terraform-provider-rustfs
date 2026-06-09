// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccSiteReplicationDataSources_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		PreCheck:                 func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
data "rustfs_site_replication_info" "test" {}

data "rustfs_site_replication_status" "test" {}

data "rustfs_site_replication_metainfo" "test" {}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.rustfs_site_replication_info.test", "id", "site-replication-info"),
					resource.TestCheckResourceAttrSet("data.rustfs_site_replication_info.test", "raw_json"),
					resource.TestCheckResourceAttr("data.rustfs_site_replication_status.test", "id", "site-replication-status"),
					resource.TestCheckResourceAttrSet("data.rustfs_site_replication_status.test", "raw_json"),
					resource.TestCheckResourceAttr("data.rustfs_site_replication_metainfo.test", "id", "site-replication-metainfo"),
					resource.TestCheckResourceAttrSet("data.rustfs_site_replication_metainfo.test", "raw_json"),
				),
			},
		},
	})
}

func TestAccSiteReplicationStatusDataSource_withFilters(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		PreCheck:                 func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
data "rustfs_site_replication_status" "test" {
  buckets = true
  metrics = true
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.rustfs_site_replication_status.test", "id", "site-replication-status"),
					resource.TestCheckResourceAttr("data.rustfs_site_replication_status.test", "buckets", "true"),
					resource.TestCheckResourceAttr("data.rustfs_site_replication_status.test", "metrics", "true"),
					resource.TestCheckResourceAttrSet("data.rustfs_site_replication_status.test", "raw_json"),
				),
			},
		},
	})
}

func TestAccSiteReplicationMetaInfoDataSource_withFilters(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		PreCheck:                 func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
data "rustfs_site_replication_metainfo" "test" {
  peer_state = true
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.rustfs_site_replication_metainfo.test", "id", "site-replication-metainfo"),
					resource.TestCheckResourceAttr("data.rustfs_site_replication_metainfo.test", "peer_state", "true"),
					resource.TestCheckResourceAttrSet("data.rustfs_site_replication_metainfo.test", "raw_json"),
				),
			},
		},
	})
}
