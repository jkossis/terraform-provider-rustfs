// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &SiteReplicationResource{}
var _ resource.ResourceWithImportState = &SiteReplicationResource{}

func NewSiteReplicationResource() resource.Resource {
	return &SiteReplicationResource{}
}

type SiteReplicationResource struct {
	client siteReplicationAdminClient
}

const siteReplicationResourceID = "site-replication"

type siteReplicationAddRequest struct {
	TargetSite      *peerSite
	Peers           []peerSite
	RemotePeerCount int
}

type siteReplicationResourceModel struct {
	ID                      types.String `tfsdk:"id"`
	ReplicateILMExpiry      types.Bool   `tfsdk:"replicate_ilm_expiry"`
	Peers                   types.List   `tfsdk:"peers"`
	Sites                   types.List   `tfsdk:"sites"`
	Enabled                 types.Bool   `tfsdk:"enabled"`
	ServiceAccountAccessKey types.String `tfsdk:"service_account_access_key"`
	APIVersion              types.String `tfsdk:"api_version"`
}

func (r *SiteReplicationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = typeNamePrefix + "_site_replication"
}

func (r *SiteReplicationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages RustFS site replication topology. Configure `peers` with every RustFS site in the topology, including the deployment addressed by the provider endpoint; read `sites` for the topology RustFS reports after configuration. Creation and updates verify membership and ILM expiry settings directly on every configured site. Incomplete operations return errors and retain resource ownership after a mutating request; failed creations are tainted, so review the replacement plan before retrying.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Resource identifier. Site replication is global to the configured RustFS deployment, so this value is always `site-replication` and is the import ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"replicate_ilm_expiry": schema.BoolAttribute{
				MarkdownDescription: "Enable replication for ILM expiry rules across all configured sites.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"peers":                      siteReplicationPeersResourceAttribute(),
			"sites":                      siteReplicationSitesResourceAttribute(),
			"enabled":                    schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether RustFS reports site replication as enabled."},
			"service_account_access_key": schema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "RustFS site replication service account access key."},
			"api_version":                schema.StringAttribute{Computed: true, MarkdownDescription: "Site replication API version reported by RustFS."},
		},
	}
}

func (r *SiteReplicationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(siteReplicationAdminClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected siteReplicationAdminClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *SiteReplicationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data siteReplicationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Computed values must be known (or null) even when the server mutates
	// topology and then returns an error, or the follow-up read fails.
	data.ID = types.StringNull()
	data.Sites = types.ListNull(siteReplicationSiteObjectType)
	data.Enabled = types.BoolNull()
	data.ServiceAccountAccessKey = types.StringNull()
	data.APIVersion = types.StringNull()
	configured := r.configureReplication(ctx, &data, resp.Diagnostics.AddAttributeError, resp.Diagnostics.AddError)
	if data.ID.IsNull() {
		return // Validation/preflight failed before a mutating request.
	}

	refreshed := r.refresh(ctx, &data, false, resp.Diagnostics.AddError)
	if configured && refreshed {
		r.verifyMembership(ctx, &data, resp.Diagnostics.AddAttributeError, resp.Diagnostics.AddError)
	}
	// An add error can follow a committed join. Return ownership alongside
	// the diagnostic so the next operation cannot forget the partial topology.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SiteReplicationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data siteReplicationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.refresh(ctx, &data, true, resp.Diagnostics.AddError) {
		if resp.Diagnostics.HasError() {
			return
		}
		absent := r.replicationAbsent(ctx, &data, resp.Diagnostics.AddAttributeError, resp.Diagnostics.AddError)
		if resp.Diagnostics.HasError() {
			return
		}
		if absent {
			resp.State.RemoveResource(ctx)
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SiteReplicationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.State = req.State
	var data siteReplicationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.setILMExpiryReplication(ctx, data.ReplicateILMExpiry.ValueBool(), resp.Diagnostics.AddError) {
		return
	}

	data.ID = types.StringValue(siteReplicationResourceID)
	if !r.refresh(ctx, &data, false, resp.Diagnostics.AddError) {
		return
	}

	if !r.verifyMembership(ctx, &data, resp.Diagnostics.AddAttributeError, resp.Diagnostics.AddError) {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SiteReplicationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data siteReplicationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.recoverImportedPeers(ctx, &data, data.Sites, resp.Diagnostics.AddError) {
		return
	}
	peers, ok := r.configuredPeers(ctx, &data, resp.Diagnostics.AddAttributeError, resp.Diagnostics.AddError)
	if !ok {
		return
	}
	if len(peers) == 0 {
		resp.Diagnostics.AddError("Missing Site Replication Peers", "Cannot remove replication without peer endpoints to verify the result. Refresh or re-import the existing topology first.")
		return
	}
	// Persist recovered import endpoints before a removal can clear the
	// coordinator's membership map, including when removal only partly succeeds.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// A partial removal can clear the coordinator's membership map first.
	// Retry through a retained peer that still owns replication state.
	for _, peer := range peers {
		info, err := r.client.SiteReplicationInfoFromPeer(ctx, peer)
		if err != nil {
			resp.Diagnostics.AddError("Unable to Read Site Replication Peer", fmt.Sprintf("Cannot check peer %q before removing replication: %s", peer.Name, err))
			return
		}
		if !replicationConfigured(info) {
			continue
		}
		if _, err := r.client.SiteReplicationRemoveFromPeer(ctx, peer, srRemoveReq{RemoveAll: true}); err != nil {
			resp.Diagnostics.AddError("Unable to Remove Site Replication", fmt.Sprintf("RustFS returned an error while removing site replication through peer %q: %s", peer.Name, err))
			return
		}
		break
	}
	if !r.replicationAbsent(ctx, &data, resp.Diagnostics.AddAttributeError, resp.Diagnostics.AddError) && !resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError("Incomplete Site Replication Removal", "RustFS still reports replication configuration on at least one site. Resource ownership is retained; retry removal after resolving the pending operation.")
	}
}

func (r *SiteReplicationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID != siteReplicationResourceID {
		resp.Diagnostics.AddError(
			"Invalid Site Replication Import ID",
			fmt.Sprintf("The rustfs_site_replication resource is a singleton and must be imported with the fixed ID %q.", siteReplicationResourceID),
		)
		return
	}

	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *SiteReplicationResource) configureReplication(
	ctx context.Context,
	data *siteReplicationResourceModel,
	addAttributeError func(path.Path, string, string),
	addError func(string, string),
) bool {
	configuredPeers, ok := r.configuredPeers(ctx, data, addAttributeError, addError)
	if !ok {
		return false
	}
	if len(configuredPeers) == 0 {
		addAttributeError(path.Root("peers"), "Missing Site Replication Peers", "Configure at least one desired peer site.")
		return false
	}

	addRequest, ok := r.addRequest(ctx, configuredPeers, addError)
	if !ok {
		return false
	}

	if addRequest.RemotePeerCount == 0 {
		addAttributeError(
			path.Root("peers"),
			"Missing Remote Site Replication Peers",
			"After filtering the current backend site, no remote peer sites remain. Configure at least one additional site.",
		)
		return false
	}

	// From this point even a transport failure may follow a server-side commit.
	data.ID = types.StringValue(siteReplicationResourceID)
	desiredILMExpiry := data.ReplicateILMExpiry.ValueBool()
	err := r.siteReplicationAdd(ctx, addRequest, srAddOptions{ReplicateILMExpiry: desiredILMExpiry})
	if err != nil {
		addError("Unable to Configure Site Replication", fmt.Sprintf("RustFS returned an error while configuring site replication: %s", err))
		return false
	}

	return r.setILMExpiryReplication(ctx, desiredILMExpiry, addError)
}

func (r *SiteReplicationResource) setILMExpiryReplication(ctx context.Context, enabled bool, addError func(string, string)) bool {
	info, err := r.client.SiteReplicationInfo(ctx)
	if err != nil {
		addError("Unable to Read Site Replication", fmt.Sprintf("RustFS returned an error while reading site replication: %s", err))
		return false
	}

	if len(info.Sites) > 0 {
		// This is a group-wide operation. Replay it even if this endpoint
		// already agrees: a previous attempt may have missed a remote site.
		opts := srEditOptions{DisableILMExpiryReplication: !enabled, EnableILMExpiryReplication: enabled}
		_, err := r.client.SiteReplicationEdit(ctx, info.Sites[0], opts)
		if err != nil {
			addError("Unable to Update ILM Expiry Replication", fmt.Sprintf("RustFS returned an error while updating ILM expiry replication: %s", err))
			return false
		}
	}

	return true
}

func (r *SiteReplicationResource) siteReplicationAdd(ctx context.Context, addRequest siteReplicationAddRequest, opts srAddOptions) error {
	if addRequest.TargetSite == nil {
		_, err := r.client.SiteReplicationAdd(ctx, addRequest.Peers, opts)
		return err
	}

	adder, ok := r.client.(peerSiteReplicationAdder)
	if !ok {
		return fmt.Errorf("client cannot configure site replication through canonical site %q", addRequest.TargetSite.Endpoint)
	}

	_, err := adder.SiteReplicationAddFromPeer(ctx, *addRequest.TargetSite, addRequest.Peers, opts)
	return err
}

func (r *SiteReplicationResource) peersWithCredentials(
	peers []peerSite,
	addAttributeError func(path.Path, string, string),
	addError func(string, string),
) ([]peerSite, bool) {
	credentialProvider, ok := r.client.(siteReplicationPeerCredentialProvider)
	if !ok {
		for i, peer := range peers {
			if peer.AccessKey == "" || peer.SecretKey == "" {
				addAttributeError(
					path.Root("peers").AtListIndex(i),
					"Missing Site Replication Peer Credentials",
					"Configure both access_key and secret_key for this peer.",
				)
				return nil, false
			}
		}

		return peers, true
	}

	resolvedPeers := make([]peerSite, 0, len(peers))
	for i, peer := range peers {
		missingAccessKey := peer.AccessKey == ""
		missingSecretKey := peer.SecretKey == ""
		if missingAccessKey != missingSecretKey {
			addAttributeError(
				path.Root("peers").AtListIndex(i),
				"Incomplete Site Replication Peer Credentials",
				"Configure both access_key and secret_key for this peer, or omit both to use the provider credentials.",
			)
			return nil, false
		}

		if missingAccessKey {
			defaultAccessKey, defaultSecretKey := credentialProvider.SiteReplicationPeerCredentials()
			if defaultAccessKey == "" || defaultSecretKey == "" {
				addError("Missing RustFS Peer Credentials", "The provider credentials are unavailable, so peer credentials cannot be defaulted.")
				return nil, false
			}

			peer.AccessKey = defaultAccessKey
			peer.SecretKey = defaultSecretKey
		}

		resolvedPeers = append(resolvedPeers, peer)
	}

	return resolvedPeers, true
}

func (r *SiteReplicationResource) addRequest(ctx context.Context, peers []peerSite, addError func(string, string)) (siteReplicationAddRequest, bool) {
	resolver, ok := r.client.(peerDeploymentIDResolver)
	if !ok {
		addError("Unable to Identify Site Replication Peers", "The client cannot resolve peer deployment IDs. No replication changes were requested.")
		return siteReplicationAddRequest{}, false
	}

	localInfo, err := r.client.SRMetaInfo(ctx, srStatusOptions{})
	if err != nil {
		addError("Unable to Identify Local Site", fmt.Sprintf("RustFS returned an error while identifying the local site behind the provider endpoint: %s", err))
		return siteReplicationAddRequest{}, false
	}

	if localInfo.Enabled || len(localInfo.State.Peers) != 0 {
		addError("Site Replication Already Configured", "The provider endpoint already has site replication state. Import the existing topology with ID site-replication before managing it.")
		return siteReplicationAddRequest{}, false
	}

	if localInfo.DeploymentID == "" {
		addError("Unable to Identify Local Site", "RustFS did not return a deployment ID for the provider endpoint. No replication changes were requested.")
		return siteReplicationAddRequest{}, false
	}

	filtered := make([]peerSite, 0, len(peers))
	peerByDeploymentID := make(map[string]peerSite, len(peers))
	var targetSite *peerSite
	for _, peer := range peers {
		peerDeploymentID, err := resolver.PeerDeploymentID(ctx, peer)
		if err != nil {
			addError(
				"Unable to Identify Site Replication Peer",
				fmt.Sprintf("RustFS returned an error while identifying peer %q at %q: %s", peer.Name, peer.Endpoint, err),
			)
			return siteReplicationAddRequest{}, false
		}
		if peerDeploymentID == "" {
			addError(
				"Unable to Identify Site Replication Peer",
				fmt.Sprintf("RustFS did not return a deployment ID for peer %q at %q.", peer.Name, peer.Endpoint),
			)
			return siteReplicationAddRequest{}, false
		}
		if existingPeer, exists := peerByDeploymentID[peerDeploymentID]; exists {
			addError(
				"Duplicate Site Replication Deployment",
				fmt.Sprintf("Peers %q at %q and %q at %q resolve to the same RustFS deployment. Configure each deployment only once.", existingPeer.Name, existingPeer.Endpoint, peer.Name, peer.Endpoint),
			)
			return siteReplicationAddRequest{}, false
		}
		peerByDeploymentID[peerDeploymentID] = peer

		info, err := r.client.SiteReplicationInfoFromPeer(ctx, peer)
		if err != nil {
			addError("Unable to Read Site Replication Peer", fmt.Sprintf("Cannot check peer %q before creating replication: %s", peer.Name, err))
			return siteReplicationAddRequest{}, false
		}
		if replicationConfigured(info) {
			addError("Site Replication Already Configured", fmt.Sprintf("Peer %q already has replication state or a pending operation. Import the existing topology with ID site-replication before managing it.", peer.Name))
			return siteReplicationAddRequest{}, false
		}

		if peerDeploymentID == localInfo.DeploymentID {
			currentBackend := peer
			targetSite = &currentBackend
			continue
		}

		filtered = append(filtered, peer)
	}

	if targetSite == nil {
		addError(
			"Missing Local Site Replication Peer",
			"Configure `peers` with the RustFS deployment addressed by the provider `endpoint`, as well as at least one remote site.",
		)
		return siteReplicationAddRequest{}, false
	}

	return siteReplicationAddRequest{TargetSite: targetSite, Peers: peers, RemotePeerCount: len(filtered)}, true
}

func (r *SiteReplicationResource) refresh(ctx context.Context, data *siteReplicationResourceModel, removeWhenDisabled bool, addError func(string, string)) bool {
	info, err := r.client.SiteReplicationInfo(ctx)
	if err != nil {
		addError("Unable to Read Site Replication", fmt.Sprintf("RustFS returned an error while reading site replication: %s", err))
		return false
	}

	sites, diags := peerInfoListValue(ctx, info.Sites)
	if diags.HasError() {
		for _, diagnostic := range diags {
			addError(diagnostic.Summary(), diagnostic.Detail())
		}
		return false
	}

	if !r.recoverImportedPeers(ctx, data, sites, addError) {
		return false
	}

	data.ID = types.StringValue(siteReplicationResourceID)
	data.Enabled = types.BoolValue(info.Enabled)
	data.ServiceAccountAccessKey = nullableString(info.ServiceAccountAccessKey)
	data.APIVersion = nullableString(info.APIVersion)
	data.Sites = sites

	return !removeWhenDisabled || replicationConfigured(info)
}

// Imports have no configured peers. Recover them from the saved topology
// before refresh overwrites it, or from the first successful import read.
func (r *SiteReplicationResource) recoverImportedPeers(ctx context.Context, data *siteReplicationResourceModel, currentSites types.List, addError func(string, string)) bool {
	if !data.Peers.IsNull() {
		return true
	}
	knownSites := data.Sites
	if knownSites.IsNull() || knownSites.IsUnknown() || len(knownSites.Elements()) == 0 {
		knownSites = currentSites
	}
	var sites []siteReplicationSiteModel
	diags := knownSites.ElementsAs(ctx, &sites, false)
	if diags.HasError() {
		for _, diagnostic := range diags {
			addError(diagnostic.Summary(), diagnostic.Detail())
		}
		return false
	}
	if len(sites) == 0 {
		addError("Missing Imported Site Replication Peers", "No peer endpoints are available from the imported topology. Restore access to a configured site and refresh or re-import before removing replication.")
		return false
	}
	peers := make([]siteReplicationPeerConfigModel, 0, len(sites))
	for _, site := range sites {
		if _, _, err := normalizeEndpoint(site.Endpoint.ValueString()); err != nil {
			addError("Invalid Imported Site Replication Endpoint", fmt.Sprintf("Cannot recover peer %q: %s", site.Name.ValueString(), err))
			return false
		}
		peers = append(peers, siteReplicationPeerConfigModel{
			Name: site.Name, Endpoint: site.Endpoint,
			AccessKey: types.StringNull(), SecretKey: types.StringNull(),
		})
	}
	value, diags := types.ListValueFrom(ctx, data.Peers.ElementType(ctx), peers)
	if diags.HasError() {
		for _, diagnostic := range diags {
			addError(diagnostic.Summary(), diagnostic.Detail())
		}
		return false
	}
	data.Peers = value
	return true
}

func replicationConfigured(info siteReplicationInfo) bool {
	return info.Enabled || len(info.Sites) != 0 || info.PendingOperation != nil
}

func (r *SiteReplicationResource) configuredPeers(ctx context.Context, data *siteReplicationResourceModel, addAttributeError func(path.Path, string, string), addError func(string, string)) ([]peerSite, bool) {
	peers, diags := peerSitesFromList(ctx, data.Peers)
	if diags.HasError() {
		for _, diagnostic := range diags {
			addError(diagnostic.Summary(), diagnostic.Detail())
		}
		return nil, false
	}
	return r.peersWithCredentials(peers, addAttributeError, addError)
}

func (r *SiteReplicationResource) replicationAbsent(ctx context.Context, data *siteReplicationResourceModel, addAttributeError func(path.Path, string, string), addError func(string, string)) bool {
	info, err := r.client.SiteReplicationInfo(ctx)
	if err != nil {
		addError("Unable to Read Site Replication", fmt.Sprintf("Cannot confirm replication removal: %s", err))
		return false
	}
	if replicationConfigured(info) {
		return false
	}
	peers, ok := r.configuredPeers(ctx, data, addAttributeError, addError)
	if !ok {
		return false
	}
	for _, peer := range peers {
		info, err := r.client.SiteReplicationInfoFromPeer(ctx, peer)
		if err != nil {
			addError("Unable to Read Site Replication Peer", fmt.Sprintf("Cannot confirm replication removal on peer %q: %s", peer.Name, err))
			return false
		}
		if replicationConfigured(info) {
			return false
		}
	}
	return true
}

func (r *SiteReplicationResource) verifyMembership(ctx context.Context, data *siteReplicationResourceModel, addAttributeError func(path.Path, string, string), addError func(string, string)) bool {
	peers, ok := r.configuredPeers(ctx, data, addAttributeError, addError)
	if !ok {
		return false
	}
	if len(peers) < 2 {
		addError("Unable to Verify Site Replication", "Configure at least two canonical peer sites to verify replication membership.")
		return false
	}

	resolver, ok := r.client.(peerDeploymentIDResolver)
	if !ok {
		addError("Unable to Verify Site Replication", "The client cannot resolve peer deployment IDs.")
		return false
	}
	expected := make(map[string]peerSite, len(peers))
	for _, peer := range peers {
		id, err := resolver.PeerDeploymentID(ctx, peer)
		if err != nil || id == "" {
			addError("Unable to Verify Site Replication", fmt.Sprintf("Cannot identify peer %q: deployment ID %q, error: %v", peer.Name, id, err))
			return false
		}
		if _, duplicate := expected[id]; duplicate {
			addError("Unable to Verify Site Replication", "Configured peers resolve to duplicate deployment IDs.")
			return false
		}
		expected[id] = peer
	}
	for _, observer := range peers {
		info, err := r.client.SiteReplicationInfoFromPeer(ctx, observer)
		if err != nil {
			addError("Unable to Verify Site Replication", fmt.Sprintf("Cannot read peer %q directly: %s", observer.Name, err))
			return false
		}
		if !info.Enabled || info.PendingOperation != nil || len(info.Sites) != len(expected) {
			addError("Incomplete Site Replication", fmt.Sprintf("Peer %q reports disabled replication, a pending operation, or an incomplete membership list.", observer.Name))
			return false
		}
		seen := make(map[string]bool, len(expected))
		for _, stored := range info.Sites {
			want, exists := expected[stored.DeploymentID]
			host, secure, endpointErr := normalizeEndpoint(stored.Endpoint)
			wantHost, wantSecure, wantErr := normalizeEndpoint(want.Endpoint)
			if !exists || seen[stored.DeploymentID] || stored.Name != want.Name || endpointErr != nil || wantErr != nil || !strings.EqualFold(host, wantHost) || secure != wantSecure {
				addError("Inconsistent Site Replication Membership", fmt.Sprintf("Peer %q reports unexpected identity or endpoint for site %q (deployment ID %q). Verify each site's actual deployment ID and repair the server topology before retrying.", observer.Name, stored.Name, stored.DeploymentID))
				return false
			}
			if stored.ReplicateILMExpiry != data.ReplicateILMExpiry.ValueBool() {
				addError("Incomplete ILM Expiry Replication Update", fmt.Sprintf("Peer %q still reports a different ILM expiry setting for site %q.", observer.Name, stored.Name))
				return false
			}
			seen[stored.DeploymentID] = true
		}
	}
	return true
}
