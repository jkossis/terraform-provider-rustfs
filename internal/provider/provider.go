// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	providerTypeName = "rustfs"
	typeNamePrefix   = providerTypeName

	rustFSEndpointEnv              = "RUSTFS_ENDPOINT"
	rustFSAccessKeyEnv             = "RUSTFS_ACCESS_KEY"
	rustFSSecretKeyEnv             = "RUSTFS_SECRET_KEY"
	rustFSInsecureSkipTLSVerifyEnv = "RUSTFS_INSECURE_SKIP_TLS_VERIFY"
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

type providerConfig struct {
	endpoint              string
	accessKey             string
	secretKey             string
	insecureSkipTLSVerify bool
}

type providerStringConfigSource struct {
	value       types.String
	attribute   string
	envVar      string
	displayName string
}

type providerBoolConfigSource struct {
	value       types.Bool
	attribute   string
	envVar      string
	displayName string
}

func (p *RustFSProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = providerTypeName
	resp.Version = p.version
}

func (p *RustFSProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Terraform provider for RustFS administration APIs.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "RustFS endpoint. Include `http://` or `https://` to control transport security. Can also be set with " + rustFSEndpointEnv + ".",
				Optional:            true,
			},
			"access_key": schema.StringAttribute{
				MarkdownDescription: "RustFS administrator access key. Can also be set with " + rustFSAccessKeyEnv + ".",
				Optional:            true,
				Sensitive:           true,
			},
			"secret_key": schema.StringAttribute{
				MarkdownDescription: "RustFS administrator secret key. Can also be set with " + rustFSSecretKeyEnv + ".",
				Optional:            true,
				Sensitive:           true,
			},
			"insecure_skip_tls_verify": schema.BoolAttribute{
				MarkdownDescription: "Skip TLS certificate verification when connecting to RustFS over HTTPS. Can also be set with " + rustFSInsecureSkipTLSVerifyEnv + ".",
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

	config, diags := providerConfigFrom(data, os.Getenv)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	client, err := newRustFSClient(config.endpoint, config.accessKey, config.secretKey, config.insecureSkipTLSVerify)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Configure RustFS Client", fmt.Sprintf("Unable to create RustFS admin client: %s", err))
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func providerConfigFrom(data RustFSProviderModel, lookupEnv func(string) string) (providerConfig, diag.Diagnostics) {
	var diags diag.Diagnostics

	config := providerConfig{
		endpoint: stringConfigValue(providerStringConfigSource{
			value:       data.Endpoint,
			attribute:   "endpoint",
			envVar:      rustFSEndpointEnv,
			displayName: "RustFS Endpoint",
		}, lookupEnv, &diags),
		accessKey: stringConfigValue(providerStringConfigSource{
			value:       data.AccessKey,
			attribute:   "access_key",
			envVar:      rustFSAccessKeyEnv,
			displayName: "RustFS Access Key",
		}, lookupEnv, &diags),
		secretKey: stringConfigValue(providerStringConfigSource{
			value:       data.SecretKey,
			attribute:   "secret_key",
			envVar:      rustFSSecretKeyEnv,
			displayName: "RustFS Secret Key",
		}, lookupEnv, &diags),
	}
	config.insecureSkipTLSVerify = boolConfigValue(providerBoolConfigSource{
		value:       data.InsecureSkipTLSVerify,
		attribute:   "insecure_skip_tls_verify",
		envVar:      rustFSInsecureSkipTLSVerifyEnv,
		displayName: "RustFS Insecure Skip TLS Verify",
	}, lookupEnv, &diags)

	return config, diags
}

func stringConfigValue(source providerStringConfigSource, lookupEnv func(string) string, diags *diag.Diagnostics) string {
	if source.value.IsUnknown() {
		addUnknownProviderConfigDiagnostic(source.attribute, source.displayName, diags)
		return ""
	}
	if !source.value.IsNull() {
		value := source.value.ValueString()
		if value == "" {
			addMissingProviderConfigDiagnostic(source, diags)
		}
		return value
	}

	value := lookupEnv(source.envVar)
	if value == "" {
		addMissingProviderConfigDiagnostic(source, diags)
	}
	return value
}

func boolConfigValue(source providerBoolConfigSource, lookupEnv func(string) string, diags *diag.Diagnostics) bool {
	if source.value.IsUnknown() {
		addUnknownProviderConfigDiagnostic(source.attribute, source.displayName, diags)
		return false
	}
	if !source.value.IsNull() {
		return source.value.ValueBool()
	}

	value := lookupEnv(source.envVar)
	if value == "" {
		return false
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		diags.AddAttributeError(
			path.Root(source.attribute),
			"Invalid "+source.displayName,
			fmt.Sprintf("Set the %s provider attribute to a boolean value or the %s environment variable to a valid boolean string: %s.", source.attribute, source.envVar, err),
		)
		return false
	}

	return parsed
}

func addMissingProviderConfigDiagnostic(source providerStringConfigSource, diags *diag.Diagnostics) {
	diags.AddAttributeError(
		path.Root(source.attribute),
		"Missing "+source.displayName,
		"Set the "+source.attribute+" provider attribute or the "+source.envVar+" environment variable.",
	)
}

func addUnknownProviderConfigDiagnostic(attribute, displayName string, diags *diag.Diagnostics) {
	diags.AddAttributeError(
		path.Root(attribute),
		"Unknown "+displayName,
		"The "+attribute+" provider attribute cannot be unknown during provider configuration.",
	)
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
