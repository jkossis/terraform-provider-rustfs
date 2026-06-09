// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type siteReplicationPeerConfigModel struct {
	Name      types.String `tfsdk:"name"`
	Endpoint  types.String `tfsdk:"endpoint"`
	AccessKey types.String `tfsdk:"access_key"`
	SecretKey types.String `tfsdk:"secret_key"`
}

type siteReplicationSiteModel struct {
	Name                      types.String `tfsdk:"name"`
	Endpoint                  types.String `tfsdk:"endpoint"`
	DeploymentID              types.String `tfsdk:"deployment_id"`
	SyncState                 types.String `tfsdk:"sync_state"`
	ReplicateILMExpiry        types.Bool   `tfsdk:"replicate_ilm_expiry"`
	ObjectNamingMode          types.String `tfsdk:"object_naming_mode"`
	TablesReplicaEnabled      types.Bool   `tfsdk:"tables_replica_enabled"`
	APIVersion                types.String `tfsdk:"api_version"`
	DefaultBandwidthLimit     types.Int64  `tfsdk:"default_bandwidth_limit"`
	DefaultBandwidthSet       types.Bool   `tfsdk:"default_bandwidth_set"`
	DefaultBandwidthUpdatedAt types.String `tfsdk:"default_bandwidth_updated_at"`
}

var siteReplicationSiteObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"name":                         types.StringType,
	"endpoint":                     types.StringType,
	"deployment_id":                types.StringType,
	"sync_state":                   types.StringType,
	"replicate_ilm_expiry":         types.BoolType,
	"object_naming_mode":           types.StringType,
	"tables_replica_enabled":       types.BoolType,
	"api_version":                  types.StringType,
	"default_bandwidth_limit":      types.Int64Type,
	"default_bandwidth_set":        types.BoolType,
	"default_bandwidth_updated_at": types.StringType,
}}

func peerSitesFromList(ctx context.Context, peers types.List) ([]peerSite, diag.Diagnostics) {
	var peerModels []siteReplicationPeerConfigModel
	diags := peers.ElementsAs(ctx, &peerModels, false)
	if diags.HasError() {
		return nil, diags
	}

	sites := make([]peerSite, 0, len(peerModels))
	for _, peer := range peerModels {
		sites = append(sites, peerSite{
			Name:      peer.Name.ValueString(),
			Endpoint:  peer.Endpoint.ValueString(),
			AccessKey: peer.AccessKey.ValueString(),
			SecretKey: peer.SecretKey.ValueString(),
		})
	}

	return sites, diags
}

func peerInfoListValue(ctx context.Context, peers []peerInfo) (types.List, diag.Diagnostics) {
	sortedPeers := append([]peerInfo(nil), peers...)
	sort.Slice(sortedPeers, func(i, j int) bool {
		if sortedPeers[i].DeploymentID != sortedPeers[j].DeploymentID {
			return sortedPeers[i].DeploymentID < sortedPeers[j].DeploymentID
		}
		if sortedPeers[i].Name != sortedPeers[j].Name {
			return sortedPeers[i].Name < sortedPeers[j].Name
		}
		return sortedPeers[i].Endpoint < sortedPeers[j].Endpoint
	})

	models := make([]siteReplicationSiteModel, 0, len(sortedPeers))
	for _, peer := range sortedPeers {
		models = append(models, peerInfoModel(peer))
	}

	return types.ListValueFrom(ctx, siteReplicationSiteObjectType, models)
}

func peerInfoMapListValue(ctx context.Context, peers map[string]peerInfo) (types.List, diag.Diagnostics) {
	peerValues := make([]peerInfo, 0, len(peers))
	for _, peer := range peers {
		peerValues = append(peerValues, peer)
	}

	return peerInfoListValue(ctx, peerValues)
}

func peerInfoModel(peer peerInfo) siteReplicationSiteModel {
	return siteReplicationSiteModel{
		Name:                      nullableString(peer.Name),
		Endpoint:                  nullableString(peer.Endpoint),
		DeploymentID:              nullableString(peer.DeploymentID),
		SyncState:                 nullableString(string(peer.SyncState)),
		ReplicateILMExpiry:        types.BoolValue(peer.ReplicateILMExpiry),
		ObjectNamingMode:          nullableString(peer.ObjectNamingMode),
		TablesReplicaEnabled:      types.BoolValue(peer.TablesReplicaEnabled),
		APIVersion:                nullableString(peer.APIVersion),
		DefaultBandwidthLimit:     uint64Int64(peer.DefaultBandwidth.Limit),
		DefaultBandwidthSet:       types.BoolValue(peer.DefaultBandwidth.IsSet),
		DefaultBandwidthUpdatedAt: nullableTime(peer.DefaultBandwidth.UpdatedAt),
	}
}

func nullableString(value string) types.String {
	if value == "" {
		return types.StringNull()
	}

	return types.StringValue(value)
}

func nullableTime(value time.Time) types.String {
	if value.IsZero() {
		return types.StringNull()
	}

	return types.StringValue(value.Format(time.RFC3339))
}

func uint64Int64(value uint64) types.Int64 {
	if value > math.MaxInt64 {
		return types.Int64Value(math.MaxInt64)
	}

	return types.Int64Value(int64(value))
}
