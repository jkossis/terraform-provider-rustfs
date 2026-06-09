// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type fakeSiteReplicationClient struct {
	addSites          []peerSite
	addOpts           srAddOptions
	info              siteReplicationInfo
	metaInfo          srInfo
	peerDeploymentIDs map[string]string
	defaultAccessKey  string
	defaultSecretKey  string
	edits             []srEditOptions
}

func (f *fakeSiteReplicationClient) SiteReplicationAdd(_ context.Context, sites []peerSite, opts srAddOptions) (replicateAddStatus, error) {
	f.addSites = sites
	f.addOpts = opts
	return replicateAddStatus{}, nil
}

func (f *fakeSiteReplicationClient) SiteReplicationEdit(_ context.Context, _ peerInfo, opts srEditOptions) (replicateEditStatus, error) {
	f.edits = append(f.edits, opts)
	return replicateEditStatus{}, nil
}

func (f *fakeSiteReplicationClient) SiteReplicationInfo(_ context.Context) (siteReplicationInfo, error) {
	return f.info, nil
}

func (f *fakeSiteReplicationClient) SiteReplicationRemove(context.Context, srRemoveReq) (replicateRemoveStatus, error) {
	return replicateRemoveStatus{}, nil
}

func (f *fakeSiteReplicationClient) SRMetaInfo(context.Context, srStatusOptions) (srInfo, error) {
	return f.metaInfo, nil
}

func (f *fakeSiteReplicationClient) SRStatusInfo(context.Context, srStatusOptions) (srStatusInfo, error) {
	return srStatusInfo{}, nil
}

func (f *fakeSiteReplicationClient) PeerDeploymentID(_ context.Context, peer peerSite) (string, error) {
	return f.peerDeploymentIDs[peer.Endpoint], nil
}

func (f *fakeSiteReplicationClient) SiteReplicationPeerCredentials() (string, string) {
	return f.defaultAccessKey, f.defaultSecretKey
}

func TestSiteReplicationConfigureDisablesStickyILMExpiry(t *testing.T) {
	t.Parallel()

	client := &fakeSiteReplicationClient{
		info: siteReplicationInfo{
			Enabled: true,
			Sites: []peerInfo{
				{Name: "site-a", DeploymentID: "site-a", ReplicateILMExpiry: true},
			},
		},
	}
	resource := &SiteReplicationResource{client: client}
	data := siteReplicationResourceModel{
		ReplicateILMExpiry: types.BoolValue(false),
		Peer:               testPeerListValue(t),
	}

	ok := resource.configureReplication(context.Background(), &data, failAttributeError(t), failError(t))
	if !ok {
		t.Fatalf("expected configureReplication to succeed")
	}

	if client.addOpts.ReplicateILMExpiry {
		t.Fatalf("expected add request to leave ILM expiry replication disabled")
	}

	if len(client.edits) != 1 {
		t.Fatalf("expected one edit request, got %d", len(client.edits))
	}

	if !client.edits[0].DisableILMExpiryReplication {
		t.Fatalf("expected edit request to disable ILM expiry replication")
	}
}

func TestSiteReplicationConfigureEnablesILMExpiry(t *testing.T) {
	t.Parallel()

	client := &fakeSiteReplicationClient{}
	resource := &SiteReplicationResource{client: client}
	data := siteReplicationResourceModel{
		ReplicateILMExpiry: types.BoolValue(true),
		Peer:               testPeerListValue(t),
	}

	ok := resource.configureReplication(context.Background(), &data, failAttributeError(t), failError(t))
	if !ok {
		t.Fatalf("expected configureReplication to succeed")
	}

	if !client.addOpts.ReplicateILMExpiry {
		t.Fatalf("expected add request to enable ILM expiry replication")
	}

	if len(client.edits) != 0 {
		t.Fatalf("expected no edit request, got %d", len(client.edits))
	}
}

func TestSiteReplicationDefaultsPeerCredentialsFromProvider(t *testing.T) {
	t.Parallel()

	client := &fakeSiteReplicationClient{
		defaultAccessKey: "provider-access",
		defaultSecretKey: "provider-secret",
	}
	resource := &SiteReplicationResource{client: client}
	data := siteReplicationResourceModel{
		ReplicateILMExpiry: types.BoolValue(true),
		Peer:               testPeerListValueWithCredentials(t, types.StringNull(), types.StringNull()),
	}

	ok := resource.configureReplication(context.Background(), &data, failAttributeError(t), failError(t))
	if !ok {
		t.Fatalf("expected configureReplication to succeed")
	}

	if len(client.addSites) != 1 {
		t.Fatalf("expected one add site, got %d", len(client.addSites))
	}

	if client.addSites[0].AccessKey != "provider-access" {
		t.Fatalf("expected provider access key, got %q", client.addSites[0].AccessKey)
	}

	if client.addSites[0].SecretKey != "provider-secret" {
		t.Fatalf("expected provider secret key, got %q", client.addSites[0].SecretKey)
	}
}

func TestSiteReplicationRejectsPartialPeerCredentials(t *testing.T) {
	t.Parallel()

	client := &fakeSiteReplicationClient{
		defaultAccessKey: "provider-access",
		defaultSecretKey: "provider-secret",
	}
	resource := &SiteReplicationResource{client: client}
	data := siteReplicationResourceModel{
		ReplicateILMExpiry: types.BoolValue(true),
		Peer:               testPeerListValueWithCredentials(t, types.StringValue("peer-access"), types.StringNull()),
	}
	var errorSummary string

	ok := resource.configureReplication(context.Background(), &data, func(_ path.Path, summary, _ string) {
		errorSummary = summary
	}, failError(t))
	if ok {
		t.Fatalf("expected configureReplication to fail")
	}

	if errorSummary != "Incomplete Site Replication Peer Credentials" {
		t.Fatalf("expected incomplete credentials error, got %q", errorSummary)
	}
}

func TestSiteReplicationFiltersCurrentBackendFromAllPeers(t *testing.T) {
	t.Parallel()

	client := &fakeSiteReplicationClient{
		metaInfo: srInfo{DeploymentID: "site-a-deployment"},
		peerDeploymentIDs: map[string]string{
			"https://site-a.example.com:9000": "site-a-deployment",
			"https://site-b.example.com:9000": "site-b-deployment",
		},
	}
	resource := &SiteReplicationResource{client: client}

	peers, ok := resource.peersForAdd(context.Background(), []peerSite{
		{Name: "site-a", Endpoint: "https://site-a.example.com:9000"},
		{Name: "site-b", Endpoint: "https://site-b.example.com:9000"},
	}, failError(t))
	if !ok {
		t.Fatalf("expected peersForAdd to succeed")
	}

	if len(peers) != 1 {
		t.Fatalf("expected one non-local peer, got %d", len(peers))
	}

	if peers[0].Name != "site-b" {
		t.Fatalf("expected site-b to remain, got %q", peers[0].Name)
	}
}

func TestSiteReplicationFailsWhenPeerDeploymentIDIsMissing(t *testing.T) {
	t.Parallel()

	client := &fakeSiteReplicationClient{
		metaInfo: srInfo{DeploymentID: "site-a-deployment"},
		peerDeploymentIDs: map[string]string{
			"https://site-b.example.com:9000": "",
		},
	}
	resource := &SiteReplicationResource{client: client}
	var errorSummary string

	peers, ok := resource.peersForAdd(context.Background(), []peerSite{
		{Name: "site-b", Endpoint: "https://site-b.example.com:9000"},
	}, func(summary, _ string) {
		errorSummary = summary
	})
	if ok {
		t.Fatalf("expected peersForAdd to fail")
	}

	if peers != nil {
		t.Fatalf("expected no peers, got %#v", peers)
	}

	if errorSummary != "Unable to Identify Site Replication Peer" {
		t.Fatalf("expected peer identification error, got %q", errorSummary)
	}
}

func testPeerListValue(t *testing.T) types.List {
	t.Helper()
	return testPeerListValueWithCredentials(t, types.StringValue("access"), types.StringValue("secret"))
}

func testPeerListValueWithCredentials(t *testing.T, accessKey, secretKey types.String) types.List {
	t.Helper()

	peerType := types.ObjectType{AttrTypes: map[string]attr.Type{
		"name":       types.StringType,
		"endpoint":   types.StringType,
		"access_key": types.StringType,
		"secret_key": types.StringType,
	}}

	value, diags := types.ListValueFrom(context.Background(), peerType, []siteReplicationPeerConfigModel{
		{
			Name:      types.StringValue("site-b"),
			Endpoint:  types.StringValue("https://site-b.example.com:9000"),
			AccessKey: accessKey,
			SecretKey: secretKey,
		},
	})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	return value
}

func failAttributeError(t *testing.T) func(path.Path, string, string) {
	t.Helper()

	return func(_ path.Path, summary, detail string) {
		t.Fatalf("unexpected attribute error: %s: %s", summary, detail)
	}
}

func failError(t *testing.T) func(string, string) {
	t.Helper()

	return func(summary, detail string) {
		t.Fatalf("unexpected error: %s: %s", summary, detail)
	}
}
