// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

const (
	envRustFSEndpoint  = "RUSTFS_ENDPOINT"
	envRustFSAccessKey = "RUSTFS_ACCESS_KEY"
	envRustFSSecretKey = "RUSTFS_SECRET_KEY"

	envSiteReplicationPeerName      = "RUSTFS_SITE_REPLICATION_PEER_NAME"
	envSiteReplicationPeerEndpoint  = "RUSTFS_SITE_REPLICATION_PEER_ENDPOINT"
	envSiteReplicationPeerAccessKey = "RUSTFS_SITE_REPLICATION_PEER_ACCESS_KEY"
	envSiteReplicationPeerSecretKey = "RUSTFS_SITE_REPLICATION_PEER_SECRET_KEY"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"rustfs": providerserver.NewProtocol6WithError(New("test")()),
}

func TestProviderMetadata(t *testing.T) {
	t.Parallel()

	providerUnderTest := &RustFSProvider{version: "test"}
	resp := &provider.MetadataResponse{}

	providerUnderTest.Metadata(context.Background(), provider.MetadataRequest{}, resp)

	if resp.TypeName != "rustfs" {
		t.Fatalf("expected provider type name rustfs, got %q", resp.TypeName)
	}

	if resp.Version != "test" {
		t.Fatalf("expected provider version test, got %q", resp.Version)
	}
}

func testAccPreCheck(t *testing.T) {
	t.Helper()

	for _, envName := range []string{envRustFSEndpoint, envRustFSAccessKey, envRustFSSecretKey} {
		if os.Getenv(envName) == "" {
			t.Fatalf("%s must be set for acceptance tests", envName)
		}
	}
}

func testAccSiteReplicationResourcePreCheck(t *testing.T) {
	t.Helper()
	testAccPreCheck(t)

	for _, envName := range []string{
		envSiteReplicationPeerName,
		envSiteReplicationPeerEndpoint,
		envSiteReplicationPeerAccessKey,
		envSiteReplicationPeerSecretKey,
	} {
		if os.Getenv(envName) == "" {
			t.Fatalf("%s must be set for site replication resource acceptance tests", envName)
		}
	}
}

func testAccProviderConfig() string {
	return `
provider "rustfs" {}
`
}

func envValue(envName string) string {
	return os.Getenv(envName)
}

func testAccSiteReplicationResourceConfig(replicateILMExpiry bool) string {
	return fmt.Sprintf(`
provider "rustfs" {}

resource "rustfs_site_replication" "test" {
  replicate_ilm_expiry = %[1]t

  peer = [
    {
      name       = %[2]q
      endpoint   = %[3]q
      access_key = %[4]q
      secret_key = %[5]q
    },
  ]
}
`,
		replicateILMExpiry,
		os.Getenv(envSiteReplicationPeerName),
		os.Getenv(envSiteReplicationPeerEndpoint),
		os.Getenv(envSiteReplicationPeerAccessKey),
		os.Getenv(envSiteReplicationPeerSecretKey),
	)
}
