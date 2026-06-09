// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

const (
	envRustFSEndpoint  = "RUSTFS_ENDPOINT"
	envRustFSAccessKey = "RUSTFS_ACCESS_KEY"
	envRustFSSecretKey = "RUSTFS_SECRET_KEY"

	envSiteReplicationPeers = "RUSTFS_SITE_REPLICATION_PEERS"
)

type testAccSiteReplicationPeer struct {
	Name      string `json:"name"`
	Endpoint  string `json:"endpoint"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
}

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

func TestSiteReplicationAcceptancePeersFromEnv(t *testing.T) {
	t.Setenv(envSiteReplicationPeers, `[
	  {"name":"site-a","endpoint":"https://site-a.example.com:9000"},
	  {"name":"site-b","endpoint":"https://site-b.example.com:9000"}
	]`)

	peers, err := testAccSiteReplicationPeersFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if len(peers) != 2 {
		t.Fatalf("expected two peers, got %d", len(peers))
	}

	config := testAccSiteReplicationResourceConfig(false)
	for _, expected := range []string{`name       = "site-a"`, `name       = "site-b"`} {
		if !strings.Contains(config, expected) {
			t.Fatalf("expected config to contain %q:\n%s", expected, config)
		}
	}

	if strings.Contains(config, "access_key") || strings.Contains(config, "secret_key") {
		t.Fatalf("expected config to omit peer credentials:\n%s", config)
	}
}

func TestSiteReplicationAcceptancePeersFromEnvRejectsPartialCredentials(t *testing.T) {
	t.Setenv(envSiteReplicationPeers, `[
	  {"name":"site-a","endpoint":"https://site-a.example.com:9000","access_key":"access-a"}
	]`)

	_, err := testAccSiteReplicationPeersFromEnv()
	if err == nil {
		t.Fatalf("expected error")
	}

	if !strings.Contains(err.Error(), "must set both access_key and secret_key") {
		t.Fatalf("expected partial credentials error, got %s", err)
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

	_, err := testAccSiteReplicationPeersFromEnv()
	if err != nil {
		t.Fatal(err)
	}
}

func testAccProviderConfig() string {
	return `
provider "rustfs" {}
`
}

func testAccSiteReplicationResourceConfig(replicateILMExpiry bool) string {
	return fmt.Sprintf(`
provider "rustfs" {}

resource "rustfs_site_replication" "test" {
  replicate_ilm_expiry = %[1]t

  peer = [
%[2]s
  ]
}
`,
		replicateILMExpiry,
		testAccSiteReplicationPeerConfig(),
	)
}

func testAccSiteReplicationPeerConfig() string {
	peers, err := testAccSiteReplicationPeersFromEnv()
	if err != nil {
		return ""
	}

	var builder strings.Builder
	for _, peer := range peers {
		fmt.Fprintf(&builder, `    {
      name       = %q
      endpoint   = %q
`, peer.Name, peer.Endpoint)
		if peer.AccessKey != "" && peer.SecretKey != "" {
			fmt.Fprintf(&builder, `      access_key = %q
      secret_key = %q
`, peer.AccessKey, peer.SecretKey)
		}
		builder.WriteString(`    },
`)
	}

	return builder.String()
}

func testAccSiteReplicationPeersFromEnv() ([]testAccSiteReplicationPeer, error) {
	peersJSON := strings.TrimSpace(os.Getenv(envSiteReplicationPeers))
	if peersJSON == "" {
		return nil, fmt.Errorf("%s must be set for site replication resource acceptance tests", envSiteReplicationPeers)
	}

	var peers []testAccSiteReplicationPeer
	if err := json.Unmarshal([]byte(peersJSON), &peers); err != nil {
		return nil, fmt.Errorf("%s must be a JSON array of peer objects: %w", envSiteReplicationPeers, err)
	}
	if len(peers) == 0 {
		return nil, fmt.Errorf("%s must contain at least one peer", envSiteReplicationPeers)
	}

	for i, peer := range peers {
		if peer.Name == "" {
			return nil, fmt.Errorf("%s[%d].name must be set", envSiteReplicationPeers, i)
		}
		if peer.Endpoint == "" {
			return nil, fmt.Errorf("%s[%d].endpoint must be set", envSiteReplicationPeers, i)
		}
		if (peer.AccessKey == "") != (peer.SecretKey == "") {
			return nil, fmt.Errorf("%s[%d] must set both access_key and secret_key, or omit both to use provider credentials", envSiteReplicationPeers, i)
		}
	}

	return peers, nil
}
