// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestSiteReplicationOperationStatus(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, operation, body, wantError string }{
		{"add rejected", "add", `{"success":false,"status":"failed","errorDetail":"peer rejected join"}`, "peer rejected join"},
		{"add initial sync failed", "add", `{"success":true,"status":"ok","initialSyncErrorMessage":"bucket bootstrap failed"}`, "bucket bootstrap failed"},
		{"add error despite success", "add", `{"success":true,"errorDetail":"incomplete join"}`, "incomplete join"},
		{"add missing success", "add", `{}`, "success"},
		{"add success", "add", `{"success":true,"status":"Requested sites were configured for replication successfully."}`, ""},
		{"edit rejected", "edit", `{"success":false,"status":"Requested site was updated successfully.","errorDetail":"peer edit rejected"}`, "peer edit rejected"},
		{"edit error despite success", "edit", `{"success":true,"errorDetail":"incomplete edit"}`, "incomplete edit"},
		{"edit missing success", "edit", `{}`, "success"},
		{"edit success", "edit", `{"success":true,"status":"Requested site was updated successfully."}`, ""},
		{"remove partial", "remove", `{"status":"Partial","errorDetail":"failed to notify peer"}`, "failed to notify peer"},
		{"remove partial without detail", "remove", `{"status":"Partial"}`, "Partial"},
		{"remove unknown status", "remove", `{"status":"queued"}`, "queued"},
		{"remove missing status", "remove", `{}`, "status"},
		{"remove success", "remove", `{"status":"Requested site(s) were removed from cluster replication successfully."}`, ""},
		{"remove success with error", "remove", `{"status":"Requested site(s) were removed from cluster replication successfully.","errorDetail":"remote failed"}`, "remote failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != http.MethodPut || req.URL.Path != rustfsAdminV3Prefix+"/site-replication/"+tc.operation {
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			client, err := newRustFSClient(server.URL, "test-access", "test-secret", false)
			if err != nil {
				t.Fatal(err)
			}
			switch tc.operation {
			case "add":
				_, err = client.SiteReplicationAdd(t.Context(), nil, srAddOptions{})
			case "edit":
				_, err = client.SiteReplicationEdit(t.Context(), peerInfo{}, srEditOptions{})
			case "remove":
				_, err = client.SiteReplicationRemove(t.Context(), srRemoveReq{RemoveAll: true})
			}
			if tc.wantError == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("expected error containing %q, got %v", tc.wantError, err)
			}
		})
	}
}

type failingReplicationClient struct {
	fakeSiteReplicationClient
	addError  error
	infoError error
}

func (f *failingReplicationClient) SiteReplicationAdd(ctx context.Context, peers []peerSite, opts srAddOptions) (replicateAddStatus, error) {
	status, _ := f.fakeSiteReplicationClient.SiteReplicationAdd(ctx, peers, opts)
	return status, f.addError
}
func (f *failingReplicationClient) SiteReplicationAddFromPeer(ctx context.Context, _ peerSite, peers []peerSite, opts srAddOptions) (replicateAddStatus, error) {
	return f.SiteReplicationAdd(ctx, peers, opts)
}

func (f *failingReplicationClient) SiteReplicationInfo(context.Context) (siteReplicationInfo, error) {
	return f.info, f.infoError
}

func testReplicationState(t *testing.T, data siteReplicationResourceModel) tfsdk.State {
	t.Helper()
	var schemaResp resource.SchemaResponse
	(&SiteReplicationResource{}).Schema(t.Context(), resource.SchemaRequest{}, &schemaResp)
	state := tfsdk.State{Schema: schemaResp.Schema}
	if diags := state.Set(t.Context(), &data); diags.HasError() {
		t.Fatal(diags)
	}
	return state
}

func testReplicationPlan(t *testing.T) tfsdk.Plan {
	t.Helper()
	state := testReplicationState(t, siteReplicationResourceModel{
		ID: types.StringUnknown(), ReplicateILMExpiry: types.BoolValue(false), Peers: testPeerListValue(t),
		Sites: types.ListUnknown(siteReplicationSiteObjectType), Enabled: types.BoolUnknown(),
		ServiceAccountAccessKey: types.StringUnknown(), APIVersion: types.StringUnknown(),
	})
	return tfsdk.Plan(state)
}

func TestSiteReplicationCreateRetainsPartialState(t *testing.T) {
	t.Parallel()
	for _, readFails := range []bool{false, true} {
		name := "read succeeds"
		if readFails {
			name = "read fails"
		}
		t.Run(name, func(t *testing.T) {
			client := &failingReplicationClient{addError: errors.New("join partially applied")}
			client.info = siteReplicationInfo{Enabled: true, Sites: []peerInfo{{Name: "site-b", DeploymentID: "actual-b"}}}
			if readFails {
				client.infoError = errors.New("refresh unavailable")
			}
			r := &SiteReplicationResource{client: client}
			plan := testReplicationPlan(t)
			resp := resource.CreateResponse{State: tfsdk.State{Schema: plan.Schema, Raw: tftypes.NewValue(plan.Raw.Type(), nil)}}
			r.Create(t.Context(), resource.CreateRequest{Plan: plan}, &resp)
			if !resp.Diagnostics.HasError() {
				t.Fatal("expected incomplete operation diagnostic")
			}
			if resp.State.Raw.IsNull() {
				t.Fatal("partially created topology was lost from state")
			}
			if !resp.State.Raw.IsFullyKnown() {
				t.Fatal("saved partial state contains unknown values")
			}
			var data siteReplicationResourceModel
			if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
				t.Fatal(diags)
			}
			if data.ID.ValueString() != siteReplicationResourceID {
				t.Fatalf("missing ownership ID: %s", data.ID)
			}
		})
	}
}

func TestSiteReplicationReadErrorRetainsState(t *testing.T) {
	t.Parallel()
	plan := testReplicationPlan(t)
	state := tfsdk.State(plan)
	if diags := state.SetAttribute(t.Context(), path.Root("id"), siteReplicationResourceID); diags.HasError() {
		t.Fatal(diags)
	}
	client := &failingReplicationClient{infoError: errors.New("temporarily unavailable")}
	r := &SiteReplicationResource{client: client}
	resp := resource.ReadResponse{State: state}
	r.Read(t.Context(), resource.ReadRequest{State: state}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected read diagnostic")
	}
	if !resp.State.Raw.Equal(state.Raw) {
		t.Fatal("read failure changed or removed owned state")
	}
}
