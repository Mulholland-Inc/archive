package main

import (
	"cmp"
	"context"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type archiveProvider struct{ version string }

type providerModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	Token    types.String `tfsdk:"token"`
}

func (p *archiveProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName, resp.Version = "archive", p.version
}

func (p *archiveProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage what lives inside an Archive instance: its schema (tables and views), tokens and space membership.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{Optional: true, Description: "Base URL of the Archive API, like https://api.example.com. Defaults to ARCHIVE_URL, the web app's URL, with app. made api., so https://app.example.com calls https://api.example.com."},
			"token":    schema.StringAttribute{Optional: true, Sensitive: true, Description: "Archive token. Defaults to ARCHIVE_TOKEN. Managing tokens and policies needs a full token from archive token create -full."},
		},
	}
}

func (p *archiveProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var m providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	endpoint := m.Endpoint.ValueString()
	if endpoint == "" {
		endpoint = os.Getenv("ARCHIVE_URL")
		if u, err := url.Parse(endpoint); err == nil && strings.HasPrefix(u.Host, "app.") {
			endpoint = u.Scheme + "://api." + strings.TrimPrefix(u.Host, "app.")
		}
	}
	token := cmp.Or(m.Token.ValueString(), os.Getenv("ARCHIVE_TOKEN"))
	if endpoint == "" || token == "" {
		resp.Diagnostics.AddError("Missing Archive endpoint or token", "Set endpoint and token in the provider block, or ARCHIVE_URL and ARCHIVE_TOKEN.")
		return
	}
	c := &client{base: strings.TrimRight(endpoint, "/"), token: token, http: &http.Client{Timeout: 5 * time.Minute}}
	resp.DataSourceData, resp.ResourceData = c, c
}

func (p *archiveProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		func() resource.Resource { return &schemaResource{} },
		func() resource.Resource { return &tokenResource{} },
		func() resource.Resource { return &policyResource{} },
	}
}

func (p *archiveProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		func() datasource.DataSource { return &spacesDataSource{} },
	}
}

type configured struct{ c *client }

func (r *configured) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if c, ok := req.ProviderData.(*client); ok {
		r.c = c
	}
}

type dsConfigured struct{ c *client }

func (d *dsConfigured) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if c, ok := req.ProviderData.(*client); ok {
		d.c = c
	}
}
