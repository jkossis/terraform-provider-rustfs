// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func TestSiteReplicationSchemas_sensitiveServiceAccountAndRawJSON(t *testing.T) {
	resourceResponse := &resource.SchemaResponse{}
	(&SiteReplicationResource{}).Schema(context.Background(), resource.SchemaRequest{}, resourceResponse)
	resourceAccessKey, ok := resourceResponse.Schema.Attributes["service_account_access_key"].(resourceschema.StringAttribute)
	if !ok || !resourceAccessKey.IsSensitive() {
		t.Fatalf("expected resource service_account_access_key to be sensitive")
	}

	for name, dataSourceUnderTest := range map[string]datasource.DataSource{
		"info":     &SiteReplicationInfoDataSource{},
		"status":   &SiteReplicationStatusDataSource{},
		"metainfo": &SiteReplicationMetaInfoDataSource{},
	} {
		response := &datasource.SchemaResponse{}
		dataSourceUnderTest.Schema(context.Background(), datasource.SchemaRequest{}, response)
		rawJSON, ok := response.Schema.Attributes["raw_json"].(datasourceschema.StringAttribute)
		if !ok || !rawJSON.IsSensitive() {
			t.Fatalf("expected %s raw_json to be sensitive", name)
		}
		if name == "info" {
			accessKey, ok := response.Schema.Attributes["service_account_access_key"].(datasourceschema.StringAttribute)
			if !ok || !accessKey.IsSensitive() {
				t.Fatalf("expected info service_account_access_key to be sensitive")
			}
		}
	}
}
