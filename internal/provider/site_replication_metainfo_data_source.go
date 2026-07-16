// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &SiteReplicationMetaInfoDataSource{}

func NewSiteReplicationMetaInfoDataSource() datasource.DataSource {
	return &SiteReplicationMetaInfoDataSource{}
}

type SiteReplicationMetaInfoDataSource struct {
	client siteReplicationAdminClient
}

type siteReplicationMetaInfoDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	Buckets        types.Bool   `tfsdk:"buckets"`
	Policies       types.Bool   `tfsdk:"policies"`
	Users          types.Bool   `tfsdk:"users"`
	Groups         types.Bool   `tfsdk:"groups"`
	Metrics        types.Bool   `tfsdk:"metrics"`
	PeerState      types.Bool   `tfsdk:"peer_state"`
	ILMExpiryRules types.Bool   `tfsdk:"ilm_expiry_rules"`
	Entity         types.String `tfsdk:"entity"`
	EntityValue    types.String `tfsdk:"entity_value"`
	ShowDeleted    types.Bool   `tfsdk:"show_deleted"`
	Enabled        types.Bool   `tfsdk:"enabled"`
	Name           types.String `tfsdk:"name"`
	DeploymentID   types.String `tfsdk:"deployment_id"`
	Sites          types.List   `tfsdk:"sites"`
	APIVersion     types.String `tfsdk:"api_version"`
	RawJSON        types.String `tfsdk:"raw_json"`
}

func (d *SiteReplicationMetaInfoDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = typeNamePrefix + "_site_replication_metainfo"
}

func (d *SiteReplicationMetaInfoDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := siteReplicationStatusFilterDataSourceAttributes()
	attrs["enabled"] = schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether RustFS reports site replication as enabled."}
	attrs["name"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Local site name reported by RustFS."}
	attrs["deployment_id"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Local deployment ID reported by RustFS."}
	attrs["sites"] = siteReplicationSitesDataSourceAttribute()
	attrs["api_version"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Site replication API version reported by RustFS."}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads RustFS local site replication metadata.",
		Attributes:          attrs,
	}
}

func (d *SiteReplicationMetaInfoDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(siteReplicationAdminClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected siteReplicationAdminClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *SiteReplicationMetaInfoDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data siteReplicationMetaInfoDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts, ok := buildSRStatusOptions(data.Entity, data.EntityValue, data.Buckets, data.Policies, data.Users, data.Groups, data.Metrics, data.PeerState, data.ILMExpiryRules, data.ShowDeleted)
	if !ok {
		resp.Diagnostics.AddAttributeError(path.Root("entity"), "Invalid Site Replication Entity", "Valid values are bucket, policy, user, group, and ilm-expiry-rule.")
		return
	}

	info, err := d.client.SRMetaInfo(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Site Replication Metainfo", fmt.Sprintf("RustFS returned an error while reading site replication metainfo: %s", err))
		return
	}

	sites, diags := peerInfoMapListValue(ctx, info.State.Peers)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rawJSON, err := rawJSONString(info)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Serialize Site Replication Metainfo", fmt.Sprintf("Unable to serialize site replication metainfo: %s", err))
		return
	}

	data.ID = types.StringValue("site-replication-metainfo")
	data.Enabled = types.BoolValue(info.Enabled)
	data.Name = nullableString(info.Name)
	data.DeploymentID = nullableString(info.DeploymentID)
	data.Sites = sites
	data.APIVersion = nullableString(info.APIVersion)
	data.RawJSON = types.StringValue(rawJSON)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
