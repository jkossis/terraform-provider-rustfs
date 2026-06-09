// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure RustFSProvider satisfies provider interfaces.
var _ provider.Provider = &RustFSProvider{}

// RustFSProvider defines the provider implementation.
type RustFSProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// RustFSProviderModel describes the provider data model.
type RustFSProviderModel struct {
	Endpoint              types.String `tfsdk:"endpoint"`
	AccessKey             types.String `tfsdk:"access_key"`
	SecretKey             types.String `tfsdk:"secret_key"`
	InsecureSkipTLSVerify types.Bool   `tfsdk:"insecure_skip_tls_verify"`
}

func (p *RustFSProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "rustfs"
	resp.Version = p.version
}

func (p *RustFSProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Terraform provider for RustFS administration APIs.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "RustFS endpoint. Include `http://` or `https://` to control transport security. Can also be set with `RUSTFS_ENDPOINT`.",
				Optional:            true,
			},
			"access_key": schema.StringAttribute{
				MarkdownDescription: "RustFS administrator access key. Can also be set with `RUSTFS_ACCESS_KEY`.",
				Optional:            true,
				Sensitive:           true,
			},
			"secret_key": schema.StringAttribute{
				MarkdownDescription: "RustFS administrator secret key. Can also be set with `RUSTFS_SECRET_KEY`.",
				Optional:            true,
				Sensitive:           true,
			},
			"insecure_skip_tls_verify": schema.BoolAttribute{
				MarkdownDescription: "Skip TLS certificate verification when connecting to RustFS over HTTPS. Can also be set with `RUSTFS_INSECURE_SKIP_TLS_VERIFY=true`.",
				Optional:            true,
			},
		},
	}
}

func (p *RustFSProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data RustFSProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := stringConfigValue(data.Endpoint, "RUSTFS_ENDPOINT")
	accessKey := stringConfigValue(data.AccessKey, "RUSTFS_ACCESS_KEY")
	secretKey := stringConfigValue(data.SecretKey, "RUSTFS_SECRET_KEY")
	insecureSkipTLSVerify := boolConfigValue(data.InsecureSkipTLSVerify, "RUSTFS_INSECURE_SKIP_TLS_VERIFY")

	if endpoint == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("endpoint"),
			"Missing RustFS Endpoint",
			"Configure endpoint in the provider block or set the RUSTFS_ENDPOINT environment variable.",
		)
	}

	if accessKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("access_key"),
			"Missing RustFS Access Key",
			"Configure access_key in the provider block or set the RUSTFS_ACCESS_KEY environment variable.",
		)
	}

	if secretKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("secret_key"),
			"Missing RustFS Secret Key",
			"Configure secret_key in the provider block or set the RUSTFS_SECRET_KEY environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	client, err := newRustFSClient(endpoint, accessKey, secretKey, insecureSkipTLSVerify)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Configure RustFS Client", fmt.Sprintf("Unable to create RustFS admin client: %s", err))
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *RustFSProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewSiteReplicationResource,
	}
}

func (p *RustFSProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewSiteReplicationInfoDataSource,
		NewSiteReplicationMetaInfoDataSource,
		NewSiteReplicationStatusDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &RustFSProvider{
			version: version,
		}
	}
}

func stringConfigValue(value types.String, envName string) string {
	if !value.IsNull() && !value.IsUnknown() {
		return value.ValueString()
	}

	return os.Getenv(envName)
}

func boolConfigValue(value types.Bool, envName string) bool {
	if !value.IsNull() && !value.IsUnknown() {
		return value.ValueBool()
	}

	return os.Getenv(envName) == "true"
}
