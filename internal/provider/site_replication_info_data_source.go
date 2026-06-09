// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &SiteReplicationInfoDataSource{}

func NewSiteReplicationInfoDataSource() datasource.DataSource {
	return &SiteReplicationInfoDataSource{}
}

type SiteReplicationInfoDataSource struct {
	client siteReplicationAdminClient
}

type siteReplicationInfoDataSourceModel struct {
	ID                      types.String `tfsdk:"id"`
	Enabled                 types.Bool   `tfsdk:"enabled"`
	Name                    types.String `tfsdk:"name"`
	Sites                   types.List   `tfsdk:"sites"`
	ServiceAccountAccessKey types.String `tfsdk:"service_account_access_key"`
	APIVersion              types.String `tfsdk:"api_version"`
	RawJSON                 types.String `tfsdk:"raw_json"`
}

func (d *SiteReplicationInfoDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_site_replication_info"
}

func (d *SiteReplicationInfoDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads RustFS site replication summary information.",
		Attributes: map[string]schema.Attribute{
			"id":                         schema.StringAttribute{Computed: true, MarkdownDescription: "Data source identifier."},
			"enabled":                    schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether RustFS reports site replication as enabled."},
			"name":                       schema.StringAttribute{Computed: true, MarkdownDescription: "Local site name reported by RustFS."},
			"sites":                      siteReplicationSitesDataSourceAttribute(),
			"service_account_access_key": schema.StringAttribute{Computed: true, MarkdownDescription: "RustFS site replication service account access key."},
			"api_version":                schema.StringAttribute{Computed: true, MarkdownDescription: "Site replication API version reported by RustFS."},
			"raw_json":                   schema.StringAttribute{Computed: true, MarkdownDescription: "Raw JSON response body returned by the RustFS admin API."},
		},
	}
}

func (d *SiteReplicationInfoDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *SiteReplicationInfoDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	info, err := d.client.SiteReplicationInfo(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Site Replication Info", fmt.Sprintf("RustFS returned an error while reading site replication info: %s", err))
		return
	}

	sites, diags := peerInfoListValue(ctx, info.Sites)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rawJSON, err := rawJSONString(info)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Serialize Site Replication Info", fmt.Sprintf("Unable to serialize site replication info: %s", err))
		return
	}

	data := siteReplicationInfoDataSourceModel{
		ID:                      types.StringValue("site-replication-info"),
		Enabled:                 types.BoolValue(info.Enabled),
		Name:                    nullableString(info.Name),
		Sites:                   sites,
		ServiceAccountAccessKey: nullableString(info.ServiceAccountAccessKey),
		APIVersion:              nullableString(info.APIVersion),
		RawJSON:                 types.StringValue(rawJSON),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
