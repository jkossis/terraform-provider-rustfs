// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"strconv"
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
					append([]resource.TestCheckFunc{
						resource.TestCheckResourceAttr("rustfs_site_replication.test", "id", siteReplicationResourceID),
						resource.TestCheckResourceAttr("rustfs_site_replication.test", "replicate_ilm_expiry", "false"),
						resource.TestCheckResourceAttr("rustfs_site_replication.test", "enabled", "true"),
						resource.TestCheckResourceAttrSet("rustfs_site_replication.test", "service_account_access_key"),
					}, testAccSiteReplicationPeerChecks()...)...,
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
				ImportStateVerifyIgnore: []string{"peers", "replicate_ilm_expiry"},
			},
		},
	})
}

func testAccSiteReplicationPeerChecks() []resource.TestCheckFunc {
	peers, err := testAccSiteReplicationPeersFromEnv()
	if err != nil {
		return nil
	}

	checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttr("rustfs_site_replication.test", "peers.#", strconv.Itoa(len(peers))),
	}
	for i, peer := range peers {
		prefix := "peers." + strconv.Itoa(i)
		checks = append(checks,
			resource.TestCheckResourceAttr("rustfs_site_replication.test", prefix+".name", peer.Name),
			resource.TestCheckResourceAttr("rustfs_site_replication.test", prefix+".endpoint", peer.Endpoint),
		)
	}

	return checks
}
