package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure ICSProvider satisfies various provider interfaces.
var _ provider.Provider = &ICSProvider{}

// ICSProvider defines the provider implementation.
type ICSProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// tests.
	version string
}

// ICSProviderModel describes the provider data model.
type ICSProviderModel struct {
	APIToken types.String `tfsdk:"api_token"`
	BaseURL  types.String `tfsdk:"base_url"`
}

func (p *ICSProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "ics"
	resp.Version = p.version
}

func (p *ICSProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"api_token": schema.StringAttribute{
				MarkdownDescription: "The API token for Ingenuity Cloud Services. Can also be set via the ICS_API_TOKEN environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"base_url": schema.StringAttribute{
				MarkdownDescription: "The base URL for the ICS API. Defaults to https://api.ingenuitycloudservices.com",
				Optional:            true,
			},
		},
	}
}

func (p *ICSProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data ICSProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Configuration values are now available.
	// Example code for configuration validation and default value setting.

	// If api_token is not provided in configuration, use environment variable
	apiToken := data.APIToken.ValueString()
	if apiToken == "" {
		apiToken = os.Getenv("ICS_API_TOKEN")
	}

	if apiToken == "" {
		resp.Diagnostics.AddError(
			"Missing API Token Configuration",
			"While configuring the provider, the API token was not found in the configuration or ICS_API_TOKEN environment variable.",
		)
		return
	}

	baseURL := data.BaseURL.ValueString()
	if baseURL == "" {
		baseURL = "https://api.ingenuitycloudservices.com"
	}

	// Create properly initialized client for data sources and resources
	client, err := NewICSClient(apiToken, baseURL)
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("base_url"), "Invalid Base URL Configuration", err.Error())
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *ICSProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewBareMetalServerResource,
		NewSSHKeyResource,
	}
}

func (p *ICSProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewInventoryDataSource,
		NewOperatingSystemsDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &ICSProvider{
			version: version,
		}
	}
}