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
	addSites []peerSite
	addOpts  srAddOptions
	info     siteReplicationInfo
	edits    []srEditOptions
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
	return srInfo{}, nil
}

func (f *fakeSiteReplicationClient) SRStatusInfo(context.Context, srStatusOptions) (srStatusInfo, error) {
	return srStatusInfo{}, nil
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

func testPeerListValue(t *testing.T) types.List {
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
			AccessKey: types.StringValue("access"),
			SecretKey: types.StringValue("secret"),
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
