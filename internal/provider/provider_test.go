// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

const (
	envSiteReplicationPeers = "RUSTFS_SITE_REPLICATION_PEERS"
)

type testAccSiteReplicationPeer struct {
	Name      string `json:"name"`
	Endpoint  string `json:"endpoint"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
}

// testAccProtoV6ProviderFactories are used by acceptance tests.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	providerTypeName: providerserver.NewProtocol6WithError(New("test")()),
}

var requiredAcceptanceEnvVars = []string{
	rustFSEndpointEnv,
	rustFSAccessKeyEnv,
	rustFSSecretKeyEnv,
}

func TestRequiredAcceptanceEnvVars(t *testing.T) {
	expected := []string{rustFSEndpointEnv, rustFSAccessKeyEnv, rustFSSecretKeyEnv}
	if len(requiredAcceptanceEnvVars) != len(expected) {
		t.Fatalf("expected required acceptance environment variables %v, got %v", expected, requiredAcceptanceEnvVars)
	}
	for i, envVar := range expected {
		if requiredAcceptanceEnvVars[i] != envVar {
			t.Fatalf("expected required acceptance environment variable %d to be %s, got %s", i, envVar, requiredAcceptanceEnvVars[i])
		}
	}
}

func testAccPreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC must be set to run acceptance tests")
	}

	missing := missingAcceptanceEnvVars(os.Getenv)
	if len(missing) > 0 {
		t.Fatalf("acceptance tests require environment variables: %s", strings.Join(missing, ", "))
	}
}

func missingAcceptanceEnvVars(lookupEnv func(string) string) []string {
	missing := make([]string, 0, len(requiredAcceptanceEnvVars))
	for _, envVar := range requiredAcceptanceEnvVars {
		if lookupEnv(envVar) == "" {
			missing = append(missing, envVar)
		}
	}
	return missing
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

  peers = [
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
