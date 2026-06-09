// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccSiteReplicationResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		PreCheck:                 func() { testAccSiteReplicationResourcePreCheck(t) },
		Steps: []resource.TestStep{
			{
				Config: testAccSiteReplicationResourceConfig(false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("rustfs_site_replication.test", "id", siteReplicationResourceID),
					resource.TestCheckResourceAttr("rustfs_site_replication.test", "replicate_ilm_expiry", "false"),
					resource.TestCheckResourceAttr("rustfs_site_replication.test", "enabled", "true"),
					resource.TestCheckResourceAttr("rustfs_site_replication.test", "peer.0.name", envValue(envSiteReplicationPeerName)),
					resource.TestCheckResourceAttr("rustfs_site_replication.test", "peer.0.endpoint", envValue(envSiteReplicationPeerEndpoint)),
					resource.TestCheckResourceAttrSet("rustfs_site_replication.test", "service_account_access_key"),
				),
			},
			{
				Config: testAccSiteReplicationResourceConfig(true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("rustfs_site_replication.test", "id", siteReplicationResourceID),
					resource.TestCheckResourceAttr("rustfs_site_replication.test", "replicate_ilm_expiry", "true"),
					resource.TestCheckResourceAttr("rustfs_site_replication.test", "enabled", "true"),
					resource.TestCheckResourceAttrSet("rustfs_site_replication.test", "service_account_access_key"),
				),
			},
			{
				ResourceName:            "rustfs_site_replication.test",
				ImportState:             true,
				ImportStateId:           siteReplicationResourceID,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"peer", "replicate_ilm_expiry"},
			},
		},
	})
}
