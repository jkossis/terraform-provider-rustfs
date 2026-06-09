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

var _ datasource.DataSource = &SiteReplicationStatusDataSource{}

func NewSiteReplicationStatusDataSource() datasource.DataSource {
	return &SiteReplicationStatusDataSource{}
}

type SiteReplicationStatusDataSource struct {
	client siteReplicationAdminClient
}

type siteReplicationStatusDataSourceModel struct {
	ID                types.String `tfsdk:"id"`
	Buckets           types.Bool   `tfsdk:"buckets"`
	Policies          types.Bool   `tfsdk:"policies"`
	Users             types.Bool   `tfsdk:"users"`
	Groups            types.Bool   `tfsdk:"groups"`
	Metrics           types.Bool   `tfsdk:"metrics"`
	PeerState         types.Bool   `tfsdk:"peer_state"`
	ILMExpiryRules    types.Bool   `tfsdk:"ilm_expiry_rules"`
	Entity            types.String `tfsdk:"entity"`
	EntityValue       types.String `tfsdk:"entity_value"`
	ShowDeleted       types.Bool   `tfsdk:"show_deleted"`
	Enabled           types.Bool   `tfsdk:"enabled"`
	MaxBuckets        types.Int64  `tfsdk:"max_buckets"`
	MaxUsers          types.Int64  `tfsdk:"max_users"`
	MaxGroups         types.Int64  `tfsdk:"max_groups"`
	MaxPolicies       types.Int64  `tfsdk:"max_policies"`
	MaxILMExpiryRules types.Int64  `tfsdk:"max_ilm_expiry_rules"`
	Sites             types.List   `tfsdk:"sites"`
	APIVersion        types.String `tfsdk:"api_version"`
	RawJSON           types.String `tfsdk:"raw_json"`
}

func (d *SiteReplicationStatusDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_replication_status"
}

func (d *SiteReplicationStatusDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := siteReplicationStatusFilterDataSourceAttributes()
	attrs["enabled"] = schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether RustFS reports site replication as enabled."}
	attrs["max_buckets"] = schema.Int64Attribute{Computed: true, MarkdownDescription: "Maximum bucket count reported across sites."}
	attrs["max_users"] = schema.Int64Attribute{Computed: true, MarkdownDescription: "Maximum user count reported across sites."}
	attrs["max_groups"] = schema.Int64Attribute{Computed: true, MarkdownDescription: "Maximum group count reported across sites."}
	attrs["max_policies"] = schema.Int64Attribute{Computed: true, MarkdownDescription: "Maximum policy count reported across sites."}
	attrs["max_ilm_expiry_rules"] = schema.Int64Attribute{Computed: true, MarkdownDescription: "Maximum ILM expiry rule count reported across sites."}
	attrs["sites"] = siteReplicationSitesDataSourceAttribute()
	attrs["api_version"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Site replication API version reported by RustFS."}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads RustFS aggregate site replication status.",
		Attributes:          attrs,
	}
}

func (d *SiteReplicationStatusDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *SiteReplicationStatusDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data siteReplicationStatusDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts, ok := buildSRStatusOptions(data.Entity, data.EntityValue, data.Buckets, data.Policies, data.Users, data.Groups, data.Metrics, data.PeerState, data.ILMExpiryRules, data.ShowDeleted)
	if !ok {
		resp.Diagnostics.AddAttributeError(path.Root("entity"), "Invalid Site Replication Entity", "Valid values are bucket, policy, user, group, and ilm-expiry-rule.")
		return
	}

	info, err := d.client.SRStatusInfo(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Site Replication Status", fmt.Sprintf("RustFS returned an error while reading site replication status: %s", err))
		return
	}

	sites, diags := peerInfoMapListValue(ctx, info.Sites)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rawJSON, err := rawJSONString(info)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Serialize Site Replication Status", fmt.Sprintf("Unable to serialize site replication status: %s", err))
		return
	}

	data.ID = types.StringValue("site-replication-status")
	data.Enabled = types.BoolValue(info.Enabled)
	data.MaxBuckets = types.Int64Value(int64(info.MaxBuckets))
	data.MaxUsers = types.Int64Value(int64(info.MaxUsers))
	data.MaxGroups = types.Int64Value(int64(info.MaxGroups))
	data.MaxPolicies = types.Int64Value(int64(info.MaxPolicies))
	data.MaxILMExpiryRules = types.Int64Value(int64(info.MaxILMExpiryRules))
	data.Sites = sites
	data.APIVersion = nullableString(info.APIVersion)
	data.RawJSON = types.StringValue(rawJSON)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func buildSRStatusOptions(entityValue, entityValueName types.String, buckets, policies, users, groups, metrics, peerState, ilmExpiryRules, showDeleted types.Bool) (srStatusOptions, bool) {
	entity := srEntityUnspecified
	if !entityValue.IsNull() && !entityValue.IsUnknown() && entityValue.ValueString() != "" {
		entity = srEntityTypeForName(entityValue.ValueString())
		if entity == srEntityUnspecified {
			return srStatusOptions{}, false
		}
	}

	return srStatusOptions{
		Buckets:        boolValue(buckets),
		Policies:       boolValue(policies),
		Users:          boolValue(users),
		Groups:         boolValue(groups),
		Metrics:        boolValue(metrics),
		PeerState:      boolValue(peerState),
		ILMExpiryRules: boolValue(ilmExpiryRules),
		Entity:         entity,
		EntityValue:    stringValue(entityValueName),
		ShowDeleted:    boolValue(showDeleted),
	}, true
}

func stringValue(value types.String) string {
	if value.IsNull() || value.IsUnknown() {
		return ""
	}

	return value.ValueString()
}
