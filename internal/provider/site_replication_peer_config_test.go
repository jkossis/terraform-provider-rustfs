// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"strings"
	"testing"
)

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
