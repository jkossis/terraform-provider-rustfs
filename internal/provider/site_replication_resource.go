// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

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

type siteReplicationResourceModel struct {
	ID                      types.String `tfsdk:"id"`
	ReplicateILMExpiry      types.Bool   `tfsdk:"replicate_ilm_expiry"`
	Peer                    types.List   `tfsdk:"peer"`
	Site                    types.List   `tfsdk:"site"`
	Enabled                 types.Bool   `tfsdk:"enabled"`
	ServiceAccountAccessKey types.String `tfsdk:"service_account_access_key"`
	APIVersion              types.String `tfsdk:"api_version"`
}

func (r *SiteReplicationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_replication"
}

func (r *SiteReplicationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages RustFS site replication topology for the configured local RustFS site.",
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
			"peer":                       siteReplicationPeerResourceAttribute(),
			"site":                       siteReplicationSiteResourceAttribute(),
			"enabled":                    schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether RustFS reports site replication as enabled."},
			"service_account_access_key": schema.StringAttribute{Computed: true, MarkdownDescription: "RustFS site replication service account access key."},
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

	if !r.configureReplication(ctx, &data, resp.Diagnostics.AddAttributeError, resp.Diagnostics.AddError) {
		return
	}

	data.ID = types.StringValue(siteReplicationResourceID)
	if !r.refresh(ctx, &data, false, resp.Diagnostics.AddError) {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SiteReplicationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data siteReplicationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.refresh(ctx, &data, true, resp.Diagnostics.AddError) {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SiteReplicationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data siteReplicationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.configureReplication(ctx, &data, resp.Diagnostics.AddAttributeError, resp.Diagnostics.AddError) {
		return
	}

	data.ID = types.StringValue(siteReplicationResourceID)
	if !r.refresh(ctx, &data, false, resp.Diagnostics.AddError) {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SiteReplicationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	_, err := r.client.SiteReplicationRemove(ctx, srRemoveReq{RemoveAll: true})
	if err != nil {
		resp.Diagnostics.AddError("Unable to Remove Site Replication", fmt.Sprintf("RustFS returned an error while removing site replication: %s", err))
		return
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
	configuredPeers, diags := peerSitesFromList(ctx, data.Peer)
	if diags.HasError() {
		for _, diagnostic := range diags {
			addError(diagnostic.Summary(), diagnostic.Detail())
		}
		return false
	}

	if len(configuredPeers) == 0 {
		addAttributeError(path.Root("peer"), "Missing Site Replication Peers", "Configure at least one remote peer site.")
		return false
	}

	configuredPeers, ok := r.peersWithCredentials(configuredPeers, addAttributeError, addError)
	if !ok {
		return false
	}

	peers, ok := r.peersForAdd(ctx, configuredPeers, addError)
	if !ok {
		return false
	}

	if len(peers) == 0 {
		addAttributeError(
			path.Root("peer"),
			"Missing Remote Site Replication Peers",
			"After filtering the current local site, no remote peer sites remain. Configure at least one additional site.",
		)
		return false
	}

	desiredILMExpiry := data.ReplicateILMExpiry.ValueBool()
	_, err := r.client.SiteReplicationAdd(ctx, peers, srAddOptions{ReplicateILMExpiry: desiredILMExpiry})
	if err != nil {
		addError("Unable to Configure Site Replication", fmt.Sprintf("RustFS returned an error while configuring site replication: %s", err))
		return false
	}

	if desiredILMExpiry {
		return true
	}

	info, err := r.client.SiteReplicationInfo(ctx)
	if err != nil {
		addError("Unable to Read Site Replication", fmt.Sprintf("RustFS returned an error while reading site replication after configuration: %s", err))
		return false
	}

	for _, site := range info.Sites {
		if site.ReplicateILMExpiry {
			_, err := r.client.SiteReplicationEdit(ctx, site, srEditOptions{DisableILMExpiryReplication: true})
			if err != nil {
				addError("Unable to Disable ILM Expiry Replication", fmt.Sprintf("RustFS returned an error while disabling ILM expiry replication: %s", err))
				return false
			}
			break
		}
	}

	return true
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
					path.Root("peer").AtListIndex(i),
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
				path.Root("peer").AtListIndex(i),
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

func (r *SiteReplicationResource) peersForAdd(ctx context.Context, peers []peerSite, addError func(string, string)) ([]peerSite, bool) {
	resolver, ok := r.client.(peerDeploymentIDResolver)
	if !ok {
		return peers, true
	}

	localInfo, err := r.client.SRMetaInfo(ctx, srStatusOptions{})
	if err != nil {
		addError("Unable to Identify Local Site", fmt.Sprintf("RustFS returned an error while identifying the local site behind the provider endpoint: %s", err))
		return nil, false
	}

	if localInfo.DeploymentID == "" {
		return peers, true
	}

	filtered := make([]peerSite, 0, len(peers))
	for _, peer := range peers {
		peerDeploymentID, err := resolver.PeerDeploymentID(ctx, peer)
		if err != nil {
			addError(
				"Unable to Identify Site Replication Peer",
				fmt.Sprintf("RustFS returned an error while identifying peer %q at %q: %s", peer.Name, peer.Endpoint, err),
			)
			return nil, false
		}
		if peerDeploymentID == "" {
			addError(
				"Unable to Identify Site Replication Peer",
				fmt.Sprintf("RustFS did not return a deployment ID for peer %q at %q.", peer.Name, peer.Endpoint),
			)
			return nil, false
		}

		if peerDeploymentID == localInfo.DeploymentID {
			continue
		}

		filtered = append(filtered, peer)
	}

	return filtered, true
}

func (r *SiteReplicationResource) refresh(ctx context.Context, data *siteReplicationResourceModel, removeWhenDisabled bool, addError func(string, string)) bool {
	info, err := r.client.SiteReplicationInfo(ctx)
	if err != nil {
		addError("Unable to Read Site Replication", fmt.Sprintf("RustFS returned an error while reading site replication: %s", err))
		return false
	}

	if removeWhenDisabled && !info.Enabled {
		return false
	}

	sites, diags := peerInfoListValue(ctx, info.Sites)
	if diags.HasError() {
		for _, diagnostic := range diags {
			addError(diagnostic.Summary(), diagnostic.Detail())
		}
		return false
	}

	data.ID = types.StringValue(siteReplicationResourceID)
	data.Enabled = types.BoolValue(info.Enabled)
	data.ServiceAccountAccessKey = nullableString(info.ServiceAccountAccessKey)
	data.APIVersion = nullableString(info.APIVersion)
	data.Site = sites

	return true
}
