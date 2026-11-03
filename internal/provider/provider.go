// Copyright (c) Bogware. SPDX-License-Identifier: MPL-2.0
package provider

import (
	"context"
	"os"
	"strings"

	"github.com/bogware/terraform-provider-mergeah/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type MergeAHProvider struct{ version string }
type providerModel struct {
	APIKey types.String `tfsdk:"api_key"`
	APIURL types.String `tfsdk:"api_url"`
}

var _ provider.Provider = &MergeAHProvider{}

func New(version string) func() provider.Provider {
	return func() provider.Provider { return &MergeAHProvider{version: version} }
}
func (p *MergeAHProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "mergeah"
	resp.Version = p.version
}
func (p *MergeAHProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Manage the Merge Agent Handler management plane.", Attributes: map[string]schema.Attribute{
		"api_key": schema.StringAttribute{Optional: true, Sensitive: true, MarkdownDescription: "Merge access key. Defaults to MERGEAH_API_KEY. Requires management permissions."},
		"api_url": schema.StringAttribute{Optional: true, MarkdownDescription: "API base URL including /api/v1. Defaults to MERGEAH_API_URL, then https://ah-api.merge.dev/api/v1. For EU use https://ah-api-eu.merge.dev/api/v1. HTTPS is required except for loopback IP addresses used by local fixtures."},
	}}
}
func (p *MergeAHProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	for name, v := range map[string]types.String{"api_key": data.APIKey, "api_url": data.APIURL} {
		if v.IsUnknown() {
			resp.Diagnostics.AddAttributeError(path.Root(name), "Unknown provider configuration", name+" must be known before configuring the provider.")
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}
	key := os.Getenv("MERGEAH_API_KEY")
	if !data.APIKey.IsNull() {
		key = data.APIKey.ValueString()
	}
	if strings.TrimSpace(key) == "" {
		resp.Diagnostics.AddAttributeError(path.Root("api_key"), "Missing access key", "Set api_key or MERGEAH_API_KEY.")
		return
	}
	base := os.Getenv("MERGEAH_API_URL")
	if base == "" {
		base = client.DefaultURL
	}
	if !data.APIURL.IsNull() {
		base = data.APIURL.ValueString()
	}
	c, err := client.New(base, key, p.version)
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("api_url"), "Invalid API URL", err.Error())
		return
	}
	resp.ResourceData = c
	resp.DataSourceData = c
}
func (p *MergeAHProvider) Resources(_ context.Context) []func() resource.Resource {
	out := []func() resource.Resource{}
	for _, s := range resourceSpecs() {
		out = append(out, func() resource.Resource { return &managementResource{spec: s} })
	}
	return out
}
func (p *MergeAHProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	out := []func() datasource.DataSource{}
	for _, s := range dataSpecs() {
		out = append(out, func() datasource.DataSource { return &managementDataSource{spec: s} })
	}
	return out
}
