// Copyright (c) Bogware. SPDX-License-Identifier: MPL-2.0
package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/bogware/terraform-provider-mergeah/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type managementDataSource struct {
	spec   dataSpec
	client *client.Client
}

func (d *managementDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + d.spec.Name
}
func (d *managementDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := map[string]schema.Attribute{
		"response_json": schema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "JSON object for a detail endpoint, or JSON array containing every item for a collection. Use jsondecode to read fields. Sensitive because API records may contain personal data."},
		"query":         schema.MapAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Additional documented API query parameters. All pages are fetched automatically. Values must be strings."},
	}
	for _, f := range d.spec.Fields {
		switch f.Type {
		case types.BoolType:
			attrs[f.Name] = schema.BoolAttribute{Required: f.Required, Optional: !f.Required, MarkdownDescription: f.Description}
		default:
			attrs[f.Name] = schema.StringAttribute{Required: f.Required, Optional: !f.Required, MarkdownDescription: f.Description}
		}
	}
	resp.Schema = schema.Schema{MarkdownDescription: d.spec.Description, Attributes: attrs}
}
func (d *managementDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Invalid provider client", "Expected a Merge Agent Handler client.")
		return
	}
	d.client = c
}
func (d *managementDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	fields := append([]field{}, d.spec.Fields...)
	fields = append(fields, field{Name: "query", Type: types.MapType{ElemType: types.StringType}}, computed("response_json", types.StringType))
	values, err := readValues(ctx, req.Config.Raw, fields)
	if err != nil {
		resp.Diagnostics.AddError("Invalid data source configuration", err.Error())
		return
	}
	ep, err := endpoint(d.spec.Endpoint, values)
	if err != nil {
		resp.Diagnostics.AddError("Invalid identifier", err.Error())
		return
	}
	q := url.Values{}
	if !values["query"].IsNull() {
		raw, e := toJSON(ctx, values["query"])
		if e != nil {
			resp.Diagnostics.AddError("Unknown query parameters", e.Error())
			return
		}
		for k, v := range raw.(map[string]any) {
			q.Set(k, v.(string))
		}
	}
	for _, f := range d.spec.Fields {
		if strings.Contains(d.spec.Endpoint, "{"+f.Name+"}") {
			continue
		}
		if values[f.Name].IsNull() {
			continue
		}
		v, e := toJSON(ctx, values[f.Name])
		if e != nil {
			resp.Diagnostics.AddError("Unknown filter", e.Error())
			return
		}
		name := f.Name
		if f.Query != "" {
			name = f.Query
		}
		switch x := v.(type) {
		case string:
			q.Set(name, x)
		case bool:
			if x {
				q.Set(name, "true")
			} else {
				q.Set(name, "false")
			}
		}
	}
	if len(q) > 0 {
		ep += "?" + q.Encode()
	}
	var result any
	if d.client == nil {
		resp.Diagnostics.AddError("Missing provider client", "Configure the provider before reading data sources.")
		return
	}
	if d.spec.List {
		result, err = d.client.List(ctx, ep)
	} else {
		result, err = d.client.Do(ctx, http.MethodGet, ep, nil)
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read "+d.spec.Name, err.Error())
		return
	}
	data, err := json.Marshal(result)
	if err != nil {
		resp.Diagnostics.AddError("Invalid API response", err.Error())
		return
	}
	resp.State.Raw = req.Config.Raw
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("response_json"), string(data))...)
}
