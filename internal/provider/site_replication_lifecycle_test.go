// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Three independently queried HTTP sites exercise the real signing/JSON client
// and resource lifecycle without any access to a live RustFS deployment.
type replicationTestCluster struct {
	mu                               sync.Mutex
	servers                          []*httptest.Server
	infos                            []siteReplicationInfo
	ids                              []string
	peers                            []siteReplicationPeerConfigModel
	addReply, editReply, removeReply string
	afterAdd, afterEdit, afterRemove func()
	infoUnavailable                  int
	adds                             int
}

func newReplicationTestCluster(t *testing.T) (*replicationTestCluster, *SiteReplicationResource, tfsdk.Plan) {
	t.Helper()
	f := &replicationTestCluster{
		infos: make([]siteReplicationInfo, 3), ids: []string{"actual-a", "actual-b", "actual-c"}, infoUnavailable: -1,
		addReply: `{"success":true,"status":"ok"}`, editReply: `{"success":true,"status":"ok"}`,
		removeReply: `{"status":"Requested site(s) were removed from cluster replication successfully."}`,
	}
	for i := range 3 {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			if req.Header.Get("Authorization") == "" {
				t.Error("missing signature")
			}
			w.Header().Set("Content-Type", "application/json")
			switch strings.TrimPrefix(req.URL.Path, rustfsAdminV3Prefix+"/site-replication") {
			case "/metainfo":
				peers := make(map[string]peerInfo)
				for _, peer := range f.infos[i].Sites {
					peers[peer.DeploymentID] = peer
				}
				_ = json.NewEncoder(w).Encode(srInfo{DeploymentID: f.ids[i], Enabled: f.infos[i].Enabled, State: srStateInfo{Peers: peers}})
			case "/info":
				if f.infoUnavailable == i {
					http.Error(w, "site unavailable", http.StatusServiceUnavailable)
					return
				}
				_ = json.NewEncoder(w).Encode(f.infos[i])
			case "/add":
				f.adds++
				f.enable(req.URL.Query().Get("replicateILMExpiry") == "true")
				if f.afterAdd != nil {
					f.afterAdd()
				}
				_, _ = w.Write([]byte(f.addReply))
			case "/edit":
				enabled := req.URL.Query().Get("enableILMExpiryReplication") == "true"
				for observer := range f.infos {
					for peer := range f.infos[observer].Sites {
						f.infos[observer].Sites[peer].ReplicateILMExpiry = enabled
					}
				}
				if f.afterEdit != nil {
					f.afterEdit()
				}
				_, _ = w.Write([]byte(f.editReply))
			case "/remove":
				for observer := range f.infos {
					f.infos[observer] = siteReplicationInfo{}
				}
				if f.afterRemove != nil {
					f.afterRemove()
				}
				_, _ = w.Write([]byte(f.removeReply))
			default:
				t.Errorf("unexpected request: %s", req.URL.Path)
				http.NotFound(w, req)
			}
		}))
		f.servers = append(f.servers, server)
		t.Cleanup(server.Close)
		f.peers = append(f.peers, siteReplicationPeerConfigModel{Name: types.StringValue(fmt.Sprintf("site-%c", 'a'+i)), Endpoint: types.StringValue(server.URL), AccessKey: types.StringNull(), SecretKey: types.StringNull()})
	}
	client, err := newRustFSClient(f.servers[0].URL, "test-access", "test-secret", false)
	if err != nil {
		t.Fatal(err)
	}
	plan := testReplicationPlan(t)
	state := tfsdk.State(plan)
	if diags := state.SetAttribute(t.Context(), path.Root("peers"), testPeerListValueFromModels(t, f.peers)); diags.HasError() {
		t.Fatal(diags)
	}
	return f, &SiteReplicationResource{client: client}, tfsdk.Plan(state)
}

// The caller holds mu.
func (f *replicationTestCluster) enable(ilm bool) {
	for i := range f.infos {
		sites := make([]peerInfo, 0, len(f.ids))
		for j, id := range f.ids {
			sites = append(sites, peerInfo{Name: f.peers[j].Name.ValueString(), Endpoint: f.servers[j].URL, DeploymentID: id, ReplicateILMExpiry: ilm})
		}
		f.infos[i] = siteReplicationInfo{Enabled: true, Sites: sites, ServiceAccountAccessKey: "test-replicator", APIVersion: "1"}
	}
}
func (f *replicationTestCluster) change(fn func()) { f.mu.Lock(); defer f.mu.Unlock(); fn() }

func createTestReplication(t *testing.T, r *SiteReplicationResource, plan tfsdk.Plan) resource.CreateResponse {
	t.Helper()
	resp := resource.CreateResponse{State: tfsdk.State{Schema: plan.Schema, Raw: tftypes.NewValue(plan.Raw.Type(), nil)}}
	r.Create(t.Context(), resource.CreateRequest{Plan: plan}, &resp)
	return resp
}

func TestSiteReplicationCreateVerifiesAllSites(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		change    func(*replicationTestCluster)
		wantError string
	}{
		{"converged", func(f *replicationTestCluster) {}, ""},
		{"remote placeholder", func(f *replicationTestCluster) { f.infos[1].Sites[2].DeploymentID = "endpoint-placeholder" }, "Inconsistent Site Replication Membership"},
		{"remote missing member", func(f *replicationTestCluster) { f.infos[2].Sites = f.infos[2].Sites[:2] }, "Incomplete Site Replication"},
		{"remote duplicate member", func(f *replicationTestCluster) { f.infos[1].Sites[2] = f.infos[1].Sites[1] }, "Inconsistent Site Replication Membership"},
		{"remote wrong endpoint", func(f *replicationTestCluster) { f.infos[2].Sites[1].Endpoint = "http://wrong.example:9000" }, "Inconsistent Site Replication Membership"},
		{"remote disabled", func(f *replicationTestCluster) { f.infos[1].Enabled = false }, "Incomplete Site Replication"},
		{"remote pending", func(f *replicationTestCluster) {
			f.infos[2].PendingOperation = &srPendingOperation{Operation: "remove"}
		}, "Incomplete Site Replication"},
		{"remote unavailable", func(f *replicationTestCluster) { f.infoUnavailable = 1 }, "Unable to Verify Site Replication"},
		{"initial sync error", func(f *replicationTestCluster) {
			f.addReply = `{"success":true,"initialSyncErrorMessage":"initial sync failed"}`
		}, "initial sync failed"},
		{"refresh failed after add", func(f *replicationTestCluster) { f.infoUnavailable = 0 }, "Unable to Read Site Replication"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f, r, plan := newReplicationTestCluster(t)
			f.change(func() { f.afterAdd = func() { tc.change(f) } })
			resp := createTestReplication(t, r, plan)
			if tc.wantError == "" {
				if resp.Diagnostics.HasError() {
					t.Fatal(resp.Diagnostics)
				}
			} else if !strings.Contains(fmt.Sprint(resp.Diagnostics), tc.wantError) {
				t.Fatalf("expected %q, got %v", tc.wantError, resp.Diagnostics)
			}
			if resp.State.Raw.IsNull() || !resp.State.Raw.IsFullyKnown() {
				t.Fatalf("missing or unknown state: %s", resp.State.Raw)
			}
			var data siteReplicationResourceModel
			if diags := resp.State.Get(t.Context(), &data); diags.HasError() {
				t.Fatal(diags)
			}
			if data.ID.ValueString() != siteReplicationResourceID {
				t.Fatalf("missing resource ID: %s", data.ID)
			}
		})
	}
}

func TestSiteReplicationCreateDoesNotClaimExistingTopology(t *testing.T) {
	t.Parallel()
	for _, site := range []int{0, 1, 2} {
		t.Run(fmt.Sprint(site), func(t *testing.T) {
			f, r, plan := newReplicationTestCluster(t)
			f.change(func() {
				f.infos[site] = siteReplicationInfo{PendingOperation: &srPendingOperation{Operation: "remove"}}
			})
			resp := createTestReplication(t, r, plan)
			if !strings.Contains(fmt.Sprint(resp.Diagnostics), "Already Configured") {
				t.Fatal(resp.Diagnostics)
			}
			if !resp.State.Raw.IsNull() {
				t.Fatal("claimed an existing topology")
			}
			f.change(func() {
				if f.adds != 0 {
					t.Fatal("mutated an existing topology")
				}
			})
		})
	}
}

func TestSiteReplicationUpdateFailureKeepsPriorState(t *testing.T) {
	t.Parallel()
	for _, rejected := range []bool{true, false} {
		t.Run(fmt.Sprint(rejected), func(t *testing.T) {
			f, r, plan := newReplicationTestCluster(t)
			created := createTestReplication(t, r, plan)
			if created.Diagnostics.HasError() {
				t.Fatal(created.Diagnostics)
			}
			planned := created.State
			if diags := planned.SetAttribute(t.Context(), path.Root("replicate_ilm_expiry"), true); diags.HasError() {
				t.Fatal(diags)
			}
			f.change(func() {
				if rejected {
					f.editReply = `{"success":false,"errorDetail":"remote edit failed"}`
				} else {
					f.afterEdit = func() { f.infos[2].Sites[0].ReplicateILMExpiry = false }
				}
			})
			resp := resource.UpdateResponse{State: planned}
			req := resource.UpdateRequest{State: created.State, Plan: tfsdk.Plan(planned)}
			r.Update(t.Context(), req, &resp)
			if !resp.Diagnostics.HasError() {
				t.Fatal("incomplete update succeeded")
			}
			if !resp.State.Raw.Equal(created.State.Raw) {
				t.Fatal("failed update changed prior state")
			}
			f.change(func() { f.editReply = `{"success":true}`; f.afterEdit = nil })
			success := resource.UpdateResponse{State: planned}
			r.Update(t.Context(), req, &success)
			if success.Diagnostics.HasError() {
				t.Fatal(success.Diagnostics)
			}
		})
	}
}

func TestSiteReplicationPartialRemovalRetainsOwnership(t *testing.T) {
	t.Parallel()
	f, r, plan := newReplicationTestCluster(t)
	created := createTestReplication(t, r, plan)
	if created.Diagnostics.HasError() {
		t.Fatal(created.Diagnostics)
	}
	f.change(func() {
		f.removeReply = `{"status":"Partial","errorDetail":"remote still configured"}`
		f.afterRemove = func() { f.infos[2] = siteReplicationInfo{Enabled: true} }
	})
	deleted := resource.DeleteResponse{State: created.State}
	r.Delete(t.Context(), resource.DeleteRequest{State: created.State}, &deleted)
	if !deleted.Diagnostics.HasError() || !deleted.State.Raw.Equal(created.State.Raw) {
		t.Fatal("partial remove lost resource ownership")
	}
	read := resource.ReadResponse{State: deleted.State}
	r.Read(t.Context(), resource.ReadRequest{State: deleted.State}, &read)
	if read.Diagnostics.HasError() || read.State.Raw.IsNull() {
		t.Fatalf("refresh forgot remote topology: %v", read.Diagnostics)
	}
	f.change(func() {
		f.removeReply = `{"status":"Requested site(s) were removed from cluster replication successfully."}`
	})
	stillPresent := resource.DeleteResponse{State: read.State}
	r.Delete(t.Context(), resource.DeleteRequest{State: read.State}, &stillPresent)
	if !stillPresent.Diagnostics.HasError() {
		t.Fatal("remove succeeded while remote still configured")
	}
	f.change(func() { f.afterRemove = nil })
	success := resource.DeleteResponse{State: read.State}
	r.Delete(t.Context(), resource.DeleteRequest{State: read.State}, &success)
	if success.Diagnostics.HasError() {
		t.Fatal(success.Diagnostics)
	}
	absent := resource.ReadResponse{State: read.State}
	r.Read(t.Context(), resource.ReadRequest{State: read.State}, &absent)
	if absent.Diagnostics.HasError() || !absent.State.Raw.IsNull() {
		t.Fatalf("confirmed absence not removed: %v", absent.Diagnostics)
	}
}

func TestSiteReplicationCreateRejectsMissingLocalIdentity(t *testing.T) {
	f, r, plan := newReplicationTestCluster(t)
	f.change(func() { f.ids[0] = "" })
	resp := createTestReplication(t, r, plan)
	if !resp.Diagnostics.HasError() || !resp.State.Raw.IsNull() {
		t.Fatalf("invalid preflight identity must fail without claiming state: %v", resp.Diagnostics)
	}
	f.change(func() {
		if f.adds != 0 {
			t.Fatal("add issued without a verified local identity")
		}
	})
}
