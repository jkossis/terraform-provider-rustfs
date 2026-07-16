// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	providerschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestProviderMetadata(t *testing.T) {
	t.Parallel()

	providerUnderTest := &RustFSProvider{version: "test"}
	resp := &provider.MetadataResponse{}

	providerUnderTest.Metadata(context.Background(), provider.MetadataRequest{}, resp)

	if resp.TypeName != providerTypeName {
		t.Fatalf("expected provider type name %s, got %q", providerTypeName, resp.TypeName)
	}

	if resp.Version != "test" {
		t.Fatalf("expected provider version test, got %q", resp.Version)
	}
}

func TestProviderSchema_configAttributesOptionalAndSensitive(t *testing.T) {
	t.Parallel()

	providerUnderTest := &RustFSProvider{version: "test"}
	resp := &provider.SchemaResponse{}

	providerUnderTest.Schema(context.Background(), provider.SchemaRequest{}, resp)

	assertStringAttribute(t, resp.Schema.Attributes, stringAttributeExpectation{name: "endpoint", optional: true, sensitive: false})
	assertStringAttribute(t, resp.Schema.Attributes, stringAttributeExpectation{name: "access_key", optional: true, sensitive: true})
	assertStringAttribute(t, resp.Schema.Attributes, stringAttributeExpectation{name: "secret_key", optional: true, sensitive: true})
	boolAttr, ok := resp.Schema.Attributes["insecure_skip_tls_verify"].(providerschema.BoolAttribute)
	if !ok {
		t.Fatalf("expected insecure_skip_tls_verify to be a bool attribute, got %T", resp.Schema.Attributes["insecure_skip_tls_verify"])
	}
	if !boolAttr.IsOptional() {
		t.Fatalf("expected insecure_skip_tls_verify to be optional")
	}
	for name, envVar := range map[string]string{
		"endpoint":                 rustFSEndpointEnv,
		"access_key":               rustFSAccessKeyEnv,
		"secret_key":               rustFSSecretKeyEnv,
		"insecure_skip_tls_verify": rustFSInsecureSkipTLSVerifyEnv,
	} {
		if !strings.Contains(resp.Schema.Attributes[name].GetMarkdownDescription(), envVar) {
			t.Fatalf("expected %s schema description to contain %s", name, envVar)
		}
	}
}

func TestProviderRegistrations_exposeSiteReplicationSurfaces(t *testing.T) {
	t.Parallel()

	providerUnderTest := &RustFSProvider{version: "test"}

	resourceNames := resourceTypeNames(t, providerUnderTest.Resources(context.Background()))
	if strings.Join(resourceNames, ",") != "rustfs_site_replication" {
		t.Fatalf("expected site replication resource registration, got %v", resourceNames)
	}

	dataSourceNames := dataSourceTypeNames(t, providerUnderTest.DataSources(context.Background()))
	if strings.Join(dataSourceNames, ",") != "rustfs_site_replication_info,rustfs_site_replication_metainfo,rustfs_site_replication_status" {
		t.Fatalf("expected site replication data source registrations, got %v", dataSourceNames)
	}
}

func TestProviderConfigure_reportsAttributeDiagnosticsWhenCredentialsMissing(t *testing.T) {
	t.Setenv(rustFSEndpointEnv, "")
	t.Setenv(rustFSAccessKeyEnv, "")
	t.Setenv(rustFSSecretKeyEnv, "")

	providerUnderTest := &RustFSProvider{version: "test"}
	resp := &provider.ConfigureResponse{}

	providerUnderTest.Configure(context.Background(), provider.ConfigureRequest{Config: testProviderConfig(nil)}, resp)

	assertDiagnosticPath(t, resp.Diagnostics, path.Root("endpoint"))
	assertDiagnosticPath(t, resp.Diagnostics, path.Root("access_key"))
	assertDiagnosticPath(t, resp.Diagnostics, path.Root("secret_key"))
	if resp.ResourceData != nil || resp.DataSourceData != nil {
		t.Fatalf("expected missing credentials to prevent client configuration")
	}
}

func TestProviderConfigure_rejectsInvalidBoolEnvValue(t *testing.T) {
	t.Setenv(rustFSEndpointEnv, "https://rustfs.example.com:9000")
	t.Setenv(rustFSAccessKeyEnv, "access")
	t.Setenv(rustFSSecretKeyEnv, "secret")
	t.Setenv(rustFSInsecureSkipTLSVerifyEnv, "definitely")

	providerUnderTest := &RustFSProvider{version: "test"}
	resp := &provider.ConfigureResponse{}

	providerUnderTest.Configure(context.Background(), provider.ConfigureRequest{Config: testProviderConfig(nil)}, resp)

	assertDiagnosticPath(t, resp.Diagnostics, path.Root("insecure_skip_tls_verify"))
	if resp.ResourceData != nil || resp.DataSourceData != nil {
		t.Fatalf("expected invalid bool env value to prevent client configuration")
	}
}

func TestBoolConfigValue_parsesStandardEnvironmentBooleans(t *testing.T) {
	var diagnostics diag.Diagnostics

	got := boolConfigValue(providerBoolConfigSource{
		value:       types.BoolNull(),
		attribute:   "insecure_skip_tls_verify",
		envVar:      rustFSInsecureSkipTLSVerifyEnv,
		displayName: "RustFS Insecure Skip TLS Verify",
	}, func(name string) string {
		if name == rustFSInsecureSkipTLSVerifyEnv {
			return "1"
		}
		return ""
	}, &diagnostics)
	if diagnostics.HasError() {
		t.Fatalf("expected bool env parsing to succeed: %v", diagnostics)
	}
	if !got {
		t.Fatalf("expected standard true bool string to enable insecure_skip_tls_verify")
	}
}

func TestProviderConfigFrom_explicitValuesOverrideEnvironment(t *testing.T) {
	config, diagnostics := providerConfigFrom(RustFSProviderModel{
		Endpoint:              types.StringValue("https://configured.example.com:9000"),
		AccessKey:             types.StringValue("configured-access"),
		SecretKey:             types.StringValue("configured-secret"),
		InsecureSkipTLSVerify: types.BoolValue(false),
	}, func(name string) string {
		if name == rustFSInsecureSkipTLSVerifyEnv {
			return "true"
		}
		return "environment-value"
	})
	if diagnostics.HasError() {
		t.Fatalf("expected explicit configuration to be valid: %v", diagnostics)
	}
	if config.endpoint != "https://configured.example.com:9000" || config.accessKey != "configured-access" || config.secretKey != "configured-secret" {
		t.Fatalf("expected explicit string configuration, got %#v", config)
	}
	if config.insecureSkipTLSVerify {
		t.Fatalf("expected explicit false to override environment true")
	}
}

func TestProviderConfigFrom_nullValuesFallBackToEnvironment(t *testing.T) {
	env := map[string]string{
		rustFSEndpointEnv:              "https://environment.example.com:9000",
		rustFSAccessKeyEnv:             "environment-access",
		rustFSSecretKeyEnv:             "environment-secret",
		rustFSInsecureSkipTLSVerifyEnv: "true",
	}
	config, diagnostics := providerConfigFrom(RustFSProviderModel{
		Endpoint:              types.StringNull(),
		AccessKey:             types.StringNull(),
		SecretKey:             types.StringNull(),
		InsecureSkipTLSVerify: types.BoolNull(),
	}, func(name string) string { return env[name] })
	if diagnostics.HasError() {
		t.Fatalf("expected environment configuration to be valid: %v", diagnostics)
	}
	if config.endpoint != env[rustFSEndpointEnv] || config.accessKey != env[rustFSAccessKeyEnv] || config.secretKey != env[rustFSSecretKeyEnv] || !config.insecureSkipTLSVerify {
		t.Fatalf("expected environment fallback, got %#v", config)
	}
}

func TestProviderConfigFrom_unknownValuesDoNotFallBackToEnvironment(t *testing.T) {
	config, diagnostics := providerConfigFrom(RustFSProviderModel{
		Endpoint:              types.StringUnknown(),
		AccessKey:             types.StringValue("access"),
		SecretKey:             types.StringValue("secret"),
		InsecureSkipTLSVerify: types.BoolUnknown(),
	}, func(string) string { return "environment-value" })
	if !diagnostics.HasError() {
		t.Fatalf("expected unknown configuration diagnostics")
	}
	assertDiagnosticPath(t, diagnostics, path.Root("endpoint"))
	assertDiagnosticPath(t, diagnostics, path.Root("insecure_skip_tls_verify"))
	if config.endpoint != "" || config.insecureSkipTLSVerify {
		t.Fatalf("expected unknown values not to use environment values, got %#v", config)
	}
}

func TestProviderConfigFrom_rejectsExplicitEmptyString(t *testing.T) {
	_, diagnostics := providerConfigFrom(RustFSProviderModel{
		Endpoint:              types.StringValue(""),
		AccessKey:             types.StringValue("access"),
		SecretKey:             types.StringValue("secret"),
		InsecureSkipTLSVerify: types.BoolNull(),
	}, func(string) string { return "environment-value" })
	assertDiagnosticPath(t, diagnostics, path.Root("endpoint"))
}

func TestProviderConfigure_sharesConfiguredClient(t *testing.T) {
	providerUnderTest := &RustFSProvider{version: "test"}
	resp := &provider.ConfigureResponse{}

	providerUnderTest.Configure(context.Background(), provider.ConfigureRequest{Config: testProviderConfig(map[string]tftypes.Value{
		"endpoint":                 tftypes.NewValue(tftypes.String, "https://rustfs.example.com:9000"),
		"access_key":               tftypes.NewValue(tftypes.String, "access"),
		"secret_key":               tftypes.NewValue(tftypes.String, "secret"),
		"insecure_skip_tls_verify": tftypes.NewValue(tftypes.Bool, false),
	})}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("expected successful provider configuration: %v", resp.Diagnostics)
	}
	resourceClient, ok := resp.ResourceData.(*rustfsClient)
	if !ok {
		t.Fatalf("expected resource client, got %T", resp.ResourceData)
	}
	dataSourceClient, ok := resp.DataSourceData.(*rustfsClient)
	if !ok {
		t.Fatalf("expected data source client, got %T", resp.DataSourceData)
	}
	if resourceClient != dataSourceClient {
		t.Fatalf("expected resources and data sources to share the configured client")
	}
}

type stringAttributeExpectation struct {
	name      string
	optional  bool
	sensitive bool
}

func assertStringAttribute(t *testing.T, attrs map[string]providerschema.Attribute, expected stringAttributeExpectation) {
	t.Helper()

	attr, ok := attrs[expected.name].(providerschema.StringAttribute)
	if !ok {
		t.Fatalf("expected %s to be a string attribute, got %T", expected.name, attrs[expected.name])
	}
	if attr.IsOptional() != expected.optional {
		t.Fatalf("expected %s optional=%t, got %t", expected.name, expected.optional, attr.IsOptional())
	}
	if attr.IsSensitive() != expected.sensitive {
		t.Fatalf("expected %s sensitive=%t, got %t", expected.name, expected.sensitive, attr.IsSensitive())
	}
}

func resourceTypeNames(t *testing.T, constructors []func() resource.Resource) []string {
	t.Helper()

	names := make([]string, 0, len(constructors))
	for _, constructor := range constructors {
		resp := &resource.MetadataResponse{}
		constructor().Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: providerTypeName}, resp)
		names = append(names, resp.TypeName)
	}

	return names
}

func dataSourceTypeNames(t *testing.T, constructors []func() datasource.DataSource) []string {
	t.Helper()

	names := make([]string, 0, len(constructors))
	for _, constructor := range constructors {
		resp := &datasource.MetadataResponse{}
		constructor().Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: providerTypeName}, resp)
		names = append(names, resp.TypeName)
	}

	return names
}

func testProviderConfig(values map[string]tftypes.Value) tfsdk.Config {
	attributeTypes := map[string]tftypes.Type{
		"endpoint":                 tftypes.String,
		"access_key":               tftypes.String,
		"secret_key":               tftypes.String,
		"insecure_skip_tls_verify": tftypes.Bool,
	}
	if values == nil {
		values = map[string]tftypes.Value{
			"endpoint":                 tftypes.NewValue(tftypes.String, nil),
			"access_key":               tftypes.NewValue(tftypes.String, nil),
			"secret_key":               tftypes.NewValue(tftypes.String, nil),
			"insecure_skip_tls_verify": tftypes.NewValue(tftypes.Bool, nil),
		}
	}

	providerUnderTest := &RustFSProvider{version: "test"}
	schemaResp := &provider.SchemaResponse{}
	providerUnderTest.Schema(context.Background(), provider.SchemaRequest{}, schemaResp)

	return tfsdk.Config{
		Raw:    tftypes.NewValue(tftypes.Object{AttributeTypes: attributeTypes}, values),
		Schema: schemaResp.Schema,
	}
}

func assertDiagnosticPath(t *testing.T, diagnostics diag.Diagnostics, expected path.Path) {
	t.Helper()

	for _, diagnostic := range diagnostics {
		withPath, ok := diagnostic.(diag.DiagnosticWithPath)
		if ok && withPath.Path().Equal(expected) {
			return
		}
	}

	t.Fatalf("expected diagnostic path %s, got %v", expected, diagnostics)
}
