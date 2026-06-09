// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	datasourceSchema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	resourceSchema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func siteReplicationPeerResourceAttribute() resourceSchema.Attribute {
	return resourceSchema.ListNestedAttribute{
		MarkdownDescription: "Remote RustFS sites to configure for site replication. The local site is inferred by RustFS from the request endpoint.",
		Required:            true,
		NestedObject: resourceSchema.NestedAttributeObject{
			Attributes: map[string]resourceSchema.Attribute{
				"name": resourceSchema.StringAttribute{
					MarkdownDescription: "Remote site name.",
					Required:            true,
				},
				"endpoint": resourceSchema.StringAttribute{
					MarkdownDescription: "Remote RustFS site endpoint, including scheme and port.",
					Required:            true,
				},
				"access_key": resourceSchema.StringAttribute{
					MarkdownDescription: "Administrator access key for the remote site, used by RustFS while joining peers.",
					Required:            true,
					Sensitive:           true,
				},
				"secret_key": resourceSchema.StringAttribute{
					MarkdownDescription: "Administrator secret key for the remote site, used by RustFS while joining peers.",
					Required:            true,
					Sensitive:           true,
				},
			},
		},
	}
}

func siteReplicationSiteResourceAttribute() resourceSchema.Attribute {
	return resourceSchema.ListNestedAttribute{
		MarkdownDescription: "Sites currently reported by RustFS site replication.",
		Computed:            true,
		NestedObject: resourceSchema.NestedAttributeObject{
			Attributes: siteReplicationSiteResourceAttributes(),
		},
	}
}

func siteReplicationSiteDataSourceAttribute() datasourceSchema.Attribute {
	return datasourceSchema.ListNestedAttribute{
		MarkdownDescription: "Sites currently reported by RustFS site replication.",
		Computed:            true,
		NestedObject: datasourceSchema.NestedAttributeObject{
			Attributes: siteReplicationSiteDataSourceAttributes(),
		},
	}
}

func siteReplicationSiteResourceAttributes() map[string]resourceSchema.Attribute {
	return map[string]resourceSchema.Attribute{
		"name":                         resourceSchema.StringAttribute{Computed: true, MarkdownDescription: "Site name."},
		"endpoint":                     resourceSchema.StringAttribute{Computed: true, MarkdownDescription: "Site endpoint."},
		"deployment_id":                resourceSchema.StringAttribute{Computed: true, MarkdownDescription: "Immutable RustFS deployment ID for the site."},
		"sync_state":                   resourceSchema.StringAttribute{Computed: true, MarkdownDescription: "Synchronous replication state for the site."},
		"replicate_ilm_expiry":         resourceSchema.BoolAttribute{Computed: true, MarkdownDescription: "Whether ILM expiry replication is enabled for the site."},
		"object_naming_mode":           resourceSchema.StringAttribute{Computed: true, MarkdownDescription: "Object naming mode reported by RustFS."},
		"tables_replica_enabled":       resourceSchema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the site is acting as a tables replica site."},
		"api_version":                  resourceSchema.StringAttribute{Computed: true, MarkdownDescription: "Site replication API version reported by RustFS."},
		"default_bandwidth_limit":      resourceSchema.Int64Attribute{Computed: true, MarkdownDescription: "Default bandwidth limit per bucket in bytes per second."},
		"default_bandwidth_set":        resourceSchema.BoolAttribute{Computed: true, MarkdownDescription: "Whether a default bandwidth limit is set."},
		"default_bandwidth_updated_at": resourceSchema.StringAttribute{Computed: true, MarkdownDescription: "RFC3339 timestamp for the last bandwidth setting update."},
	}
}

func siteReplicationSiteDataSourceAttributes() map[string]datasourceSchema.Attribute {
	return map[string]datasourceSchema.Attribute{
		"name":                         datasourceSchema.StringAttribute{Computed: true, MarkdownDescription: "Site name."},
		"endpoint":                     datasourceSchema.StringAttribute{Computed: true, MarkdownDescription: "Site endpoint."},
		"deployment_id":                datasourceSchema.StringAttribute{Computed: true, MarkdownDescription: "Immutable RustFS deployment ID for the site."},
		"sync_state":                   datasourceSchema.StringAttribute{Computed: true, MarkdownDescription: "Synchronous replication state for the site."},
		"replicate_ilm_expiry":         datasourceSchema.BoolAttribute{Computed: true, MarkdownDescription: "Whether ILM expiry replication is enabled for the site."},
		"object_naming_mode":           datasourceSchema.StringAttribute{Computed: true, MarkdownDescription: "Object naming mode reported by RustFS."},
		"tables_replica_enabled":       datasourceSchema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the site is acting as a tables replica site."},
		"api_version":                  datasourceSchema.StringAttribute{Computed: true, MarkdownDescription: "Site replication API version reported by RustFS."},
		"default_bandwidth_limit":      datasourceSchema.Int64Attribute{Computed: true, MarkdownDescription: "Default bandwidth limit per bucket in bytes per second."},
		"default_bandwidth_set":        datasourceSchema.BoolAttribute{Computed: true, MarkdownDescription: "Whether a default bandwidth limit is set."},
		"default_bandwidth_updated_at": datasourceSchema.StringAttribute{Computed: true, MarkdownDescription: "RFC3339 timestamp for the last bandwidth setting update."},
	}
}

func siteReplicationStatusFilterDataSourceAttributes() map[string]datasourceSchema.Attribute {
	return map[string]datasourceSchema.Attribute{
		"buckets": datasourceSchema.BoolAttribute{
			MarkdownDescription: "Request bucket replication status.",
			Optional:            true,
		},
		"policies": datasourceSchema.BoolAttribute{
			MarkdownDescription: "Request IAM policy replication status.",
			Optional:            true,
		},
		"users": datasourceSchema.BoolAttribute{
			MarkdownDescription: "Request IAM user replication status.",
			Optional:            true,
		},
		"groups": datasourceSchema.BoolAttribute{
			MarkdownDescription: "Request IAM group replication status.",
			Optional:            true,
		},
		"metrics": datasourceSchema.BoolAttribute{
			MarkdownDescription: "Request site replication metrics.",
			Optional:            true,
		},
		"peer_state": datasourceSchema.BoolAttribute{
			MarkdownDescription: "Request peer state details.",
			Optional:            true,
		},
		"ilm_expiry_rules": datasourceSchema.BoolAttribute{
			MarkdownDescription: "Request ILM expiry rule replication status.",
			Optional:            true,
		},
		"entity": datasourceSchema.StringAttribute{
			MarkdownDescription: "Optional entity filter. Valid values are `bucket`, `policy`, `user`, `group`, and `ilm-expiry-rule`.",
			Optional:            true,
		},
		"entity_value": datasourceSchema.StringAttribute{
			MarkdownDescription: "Optional entity value used with `entity`.",
			Optional:            true,
		},
		"show_deleted": datasourceSchema.BoolAttribute{
			MarkdownDescription: "Request deleted entity information when supported by the server.",
			Optional:            true,
		},
		"raw_json": datasourceSchema.StringAttribute{
			MarkdownDescription: "JSON response body re-serialized from the RustFS admin client model.",
			Computed:            true,
		},
		"id": datasourceSchema.StringAttribute{
			MarkdownDescription: "Data source identifier.",
			Computed:            true,
		},
	}
}

func boolValue(value types.Bool) bool {
	return !value.IsNull() && !value.IsUnknown() && value.ValueBool()
}
