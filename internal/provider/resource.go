// Copyright (c) Bogware. SPDX-License-Identifier: MPL-2.0
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/bogware/terraform-provider-mergeah/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type managementResource struct {
	spec   resourceSpec
	client *client.Client
}

var _ resource.ResourceWithConfigure = &managementResource{}
var _ resource.ResourceWithImportState = &managementResource{}
var _ resource.ResourceWithValidateConfig = &managementResource{}

func (r *managementResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + r.spec.Name
}
func (r *managementResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema(r.spec)
}
func (r *managementResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Invalid provider client", "Expected a Merge Agent Handler client.")
		return
	}
	r.client = c
}
func (r *managementResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	values, err := readValues(ctx, req.Config.Raw, resourceFields(r.spec))
	if err != nil {
		resp.Diagnostics.AddError("Invalid configuration", err.Error())
		return
	}
	for _, f := range r.spec.Fields {
		v := values[f.Name]
		if v.IsNull() || v.IsUnknown() {
			continue
		}
		if s, ok := v.(types.String); ok {
			if f.Required && !f.AllowEmpty && strings.TrimSpace(s.ValueString()) == "" {
				resp.Diagnostics.AddAttributeError(path.Root(f.Name), "Empty value", f.Name+" must not be empty.")
			}
			if len(f.Enum) > 0 {
				found := false
				for _, allowed := range f.Enum {
					if allowed == s.ValueString() {
						found = true
					}
				}
				if !found {
					resp.Diagnostics.AddAttributeError(path.Root(f.Name), "Invalid value", "Allowed values: "+strings.Join(f.Enum, ", "))
				}
			}
			if f.JSON {
				var obj map[string]any
				if json.Unmarshal([]byte(s.ValueString()), &obj) != nil || obj == nil {
					resp.Diagnostics.AddAttributeError(path.Root(f.Name), "Invalid JSON object", "Use jsonencode to provide an object.")
				}
			}
		}
	}
	if r.spec.Kind == "access_key" {
		if v, ok := values["no_expiration"].(types.Bool); ok && v.ValueBool() && !values["expires_at"].IsNull() && !values["expires_at"].IsUnknown() {
			resp.Diagnostics.AddAttributeError(path.Root("expires_at"), "Conflicting expiration settings", "Use either expires_at or no_expiration, not both.")
		}
	}
	if r.spec.Kind == "connector_member" {
		selection := values["tool_names"].(types.Set)
		if !selection.IsNull() && !selection.IsUnknown() && len(selection.Elements()) == 0 {
			resp.Diagnostics.AddAttributeError(path.Root("tool_names"), "Empty tool selection", "An empty tool_names selection removes a connector in Merge. Remove the resource from configuration to detach the connector instead.")
		}
	}
	if r.spec.Kind == "connectors" {
		if list, ok := values["connectors"].(types.List); ok && !list.IsUnknown() && !list.IsNull() {
			seen := map[string]bool{}
			for _, element := range list.Elements() {
				obj, ok := element.(types.Object)
				if !ok || obj.IsNull() || obj.IsUnknown() {
					continue
				}
				slug := obj.Attributes()["slug"].(types.String)
				if slug.IsNull() || slug.IsUnknown() {
					continue
				}
				if seen[slug.ValueString()] {
					resp.Diagnostics.AddAttributeError(path.Root("connectors"), "Duplicate connector", "Each connector slug may appear only once.")
				}
				seen[slug.ValueString()] = true
				selection := obj.Attributes()["tool_names"].(types.Set)
				if !selection.IsNull() && !selection.IsUnknown() && len(selection.Elements()) == 0 {
					resp.Diagnostics.AddAttributeError(path.Root("connectors"), "Empty tool selection", "An empty tool_names selection removes a connector in Merge. Remove the connector from the connectors list instead.")
				}
			}
		}
	}
	if r.spec.Kind == "remote" {
		if v, ok := values["auth"].(types.Object); ok && !v.IsNull() && !v.IsUnknown() {
			a := v.Attributes()
			kind := a["type"].(types.String)
			if !kind.IsUnknown() && kind.ValueString() != "none" && kind.ValueString() != "secret" {
				resp.Diagnostics.AddAttributeError(path.Root("auth"), "Unsupported authentication type", "auth.type must be none or secret.")
			}
			if kind.ValueString() == "secret" && a["header"].IsNull() {
				resp.Diagnostics.AddAttributeError(path.Root("auth"), "Missing secret header", "auth.header is required when auth.type is secret.")
			}
		}
	}
}

func (r *managementResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	values, err := readValues(ctx, req.Plan.Raw, resourceFields(r.spec))
	if err == nil {
		values, err = r.write(ctx, values, true)
	}
	if err != nil {
		// A create followed by a failed update must retain the created ID so a
		// later plan can recover it rather than creating an orphaned duplicate.
		if values != nil && textValue(values, "id") != "" {
			for _, f := range resourceFields(r.spec) {
				if values[f.Name].IsUnknown() {
					values[f.Name] = nullValue(ctx, f.Type)
				}
			}
			resp.Diagnostics.Append(resp.State.Set(ctx, objectValue(resourceFields(r.spec), values))...)
		}
		detail := err.Error()
		if values == nil || textValue(values, "id") == "" {
			detail += " If Merge accepted the create request but its identifier could not be recovered, check Merge for an existing object before applying again and import it if present. A one-time access-key secret cannot be recovered if it was not received; revoke that key before creating a replacement."
		}
		resp.Diagnostics.AddError("Unable to create "+r.spec.Name, detail)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, objectValue(resourceFields(r.spec), values))...)
}
func (r *managementResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	values, err := readValues(ctx, req.Plan.Raw, resourceFields(r.spec))
	if err == nil {
		// Keep create-only secrets when unrelated computed fields become unknown.
		prior, e := readValues(ctx, req.State.Raw, resourceFields(r.spec))
		if e != nil {
			err = e
		} else {
			for _, f := range r.spec.Fields {
				if f.Computed && f.Retain && values[f.Name].IsUnknown() {
					values[f.Name] = prior[f.Name]
				}
			}
		}
	}
	if err == nil {
		values, err = r.write(ctx, values, false)
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to update "+r.spec.Name, err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, objectValue(resourceFields(r.spec), values))...)
}
func (r *managementResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	values, err := readValues(ctx, req.State.Raw, resourceFields(r.spec))
	if err == nil {
		var obj map[string]any
		obj, err = r.fetch(ctx, values)
		if err == nil {
			values, err = r.merge(ctx, values, obj)
		}
	}
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read "+r.spec.Name, err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, objectValue(resourceFields(r.spec), values))...)
}
func (r *managementResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.spec.Kind == "standard" || r.spec.Kind == "connection_access" {
		return
	}
	if r.client == nil {
		resp.Diagnostics.AddError("Missing provider client", "Configure the provider before deleting resources.")
		return
	}
	values, err := readValues(ctx, req.State.Raw, resourceFields(r.spec))
	if err != nil {
		resp.Diagnostics.AddError("Unable to read state", err.Error())
		return
	}
	ep, err := r.recordEndpoint(values)
	if err != nil {
		resp.Diagnostics.AddError("Invalid resource identifier", err.Error())
		return
	}
	method := r.spec.DeleteMethod
	if method == "" {
		method = http.MethodDelete
	}
	var body any
	switch r.spec.Kind {
	case "access_key":
		ep += "revoke/"
	case "connectors":
		method = http.MethodPut
		body = []any{}
	case "override":
		method = http.MethodPatch
		body = []any{map[string]any{"connector_slug": textValue(values, "connector_slug"), "tool_name": textValue(values, "tool_name"), "description": nil}}
	}
	_, err = r.client.Do(ctx, method, ep, body)
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete "+r.spec.Name, err.Error())
	}
}
func (r *managementResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	keys := r.spec.Keys
	if len(keys) == 0 {
		keys = []string{"id"}
	}
	parts := strings.Split(req.ID, "/")
	if len(parts) != len(keys) {
		resp.Diagnostics.AddError("Invalid import ID", "Expected "+strings.Join(keys, "/")+".")
		return
	}
	for i, key := range keys {
		if strings.TrimSpace(parts[i]) == "" || parts[i] == "." || parts[i] == ".." || strings.Contains(parts[i], "\\") {
			resp.Diagnostics.AddError("Invalid import ID", "Import identifiers must be nonempty path segments.")
			return
		}
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(key), parts[i])...)
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
func (r *managementResource) recordEndpoint(values map[string]attr.Value) (string, error) {
	template := r.spec.Collection
	switch r.spec.Kind {
	case "override", "connectors":
	case "remote", "connector_member":
		template += "{slug}/"
	case "standard", "standard_override":
		template += "{entity_type}/"
	case "connection_access":
		template += "{connection_id}/"
	default:
		// Nested imported IDs include their parent. The final segment is the API ID.
		id := textValue(values, "id")
		parts := strings.Split(id, "/")
		copyValues := map[string]attr.Value{}
		for k, v := range values {
			copyValues[k] = v
		}
		copyValues["id"] = types.StringValue(parts[len(parts)-1])
		return endpoint(template+"{id}/", copyValues)
	}
	return endpoint(template, values)
}
func (r *managementResource) fetch(ctx context.Context, values map[string]attr.Value) (map[string]any, error) {
	if r.client == nil {
		return nil, fmt.Errorf("provider client is not configured")
	}
	ep, err := r.recordEndpoint(values)
	if err != nil {
		return nil, err
	}
	if r.spec.Kind == "connector_member" {
		ep, err = endpoint(r.spec.Collection, values)
		if err != nil {
			return nil, err
		}
		items, err := r.client.List(ctx, ep)
		if err != nil {
			return nil, err
		}
		return findConnector(items, textValue(values, "slug"))
	}
	if r.spec.Kind == "override" {
		items, err := r.client.List(ctx, ep)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			obj, ok := item.(map[string]any)
			if ok && obj["connector_slug"] == textValue(values, "connector_slug") && obj["tool_name"] == textValue(values, "tool_name") {
				return obj, nil
			}
		}
		return nil, &client.APIError{Status: 404}
	}
	if r.spec.Kind == "connectors" {
		items, err := r.client.List(ctx, ep)
		return map[string]any{"connectors": items}, err
	}
	result, err := r.client.Do(ctx, http.MethodGet, ep, nil)
	if err != nil {
		return nil, err
	}
	obj, ok := result.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected a resource object from Merge")
	}
	// Tool Pack detail endpoints can return inherited organization defaults after
	// an override was removed. That is a missing managed override, not its state.
	if r.spec.Kind == "standard_override" || r.spec.Name == "tool_pack_custom_regex_rule" {
		if obj["defined_at"] == "ORGANIZATION" {
			return nil, &client.APIError{Status: 404}
		}
	}
	return obj, nil
}
func (r *managementResource) write(ctx context.Context, values map[string]attr.Value, create bool) (map[string]attr.Value, error) {
	if r.client == nil {
		return nil, fmt.Errorf("provider client is not configured")
	}
	body := map[string]any{}
	// Capture the requested follow-up settings before the create response can
	// replace them with remote defaults (notably is_active=false).
	patch := map[string]any{}
	if create {
		for _, f := range r.spec.Fields {
			if f.CreateOmit && !values[f.Name].IsUnknown() && !values[f.Name].IsNull() {
				v, err := toJSON(ctx, values[f.Name])
				if err != nil {
					return nil, err
				}
				patch[f.Name] = v
			}
		}
	}
	for _, f := range r.spec.Fields {
		if f.Computed || f.Local || (create && f.CreateOmit) || (!create && f.Replace) {
			continue
		}
		v := values[f.Name]
		if (create && v.IsNull()) || (f.OptionalComputed && v.IsUnknown()) {
			continue
		}
		x, err := toJSON(ctx, v)
		if err != nil {
			return nil, fmt.Errorf("%s cannot be sent while unknown", f.Name)
		}
		name := f.Name
		if f.JSON {
			name = strings.TrimSuffix(name, "_json")
			if x != nil {
				var decoded any
				if json.Unmarshal([]byte(x.(string)), &decoded) != nil {
					return nil, fmt.Errorf("%s is not valid JSON", f.Name)
				}
				x = decoded
			}
		}
		body[name] = x
	}
	if r.spec.Kind == "access_key" {
		if v, ok := values["no_expiration"].(types.Bool); ok && v.ValueBool() {
			body["expires_at"] = nil
		}
	}
	ep, err := endpoint(r.spec.Collection, values)
	if err != nil {
		return nil, err
	}
	method := r.spec.CreateMethod
	if method == "" {
		method = http.MethodPost
	}
	if !create {
		ep, err = r.recordEndpoint(values)
		if r.spec.Kind == "connector_member" {
			ep, err = endpoint(r.spec.Collection, values)
		}
		method = r.spec.UpdateMethod
		if method == "" {
			method = http.MethodPatch
		}
	} else if r.spec.Kind == "standard" || r.spec.Kind == "standard_override" || r.spec.Kind == "connection_access" {
		ep, err = r.recordEndpoint(values)
	}
	if err != nil {
		return nil, err
	}
	var payload any = body
	if r.spec.Kind == "connector_member" {
		// PATCH targets the collection but must identify this one membership,
		// including on updates where immutable fields are otherwise omitted.
		body["slug"] = textValue(values, "slug")
		// A computed empty tool set can describe a connector with no tools.
		// Never echo it as [], which the API interprets as removal.
		if names, ok := body["tool_names"].([]any); ok && len(names) == 0 {
			delete(body, "tool_names")
		}
		payload = []any{body}
	}
	if r.spec.Kind == "override" {
		body["connector_slug"] = textValue(values, "connector_slug")
		body["tool_name"] = textValue(values, "tool_name")
		payload = []any{body}
	}
	if r.spec.Kind == "connectors" {
		payload = body["connectors"]
	}
	result, err := r.client.Do(ctx, method, ep, payload)
	if err != nil {
		return nil, err
	}
	var obj map[string]any
	if r.spec.Kind == "connector_member" {
		items, ok := result.([]any)
		if !ok {
			return nil, fmt.Errorf("expected connector array in response")
		}
		obj, err = findConnector(items, textValue(values, "slug"))
		if err != nil {
			return nil, fmt.Errorf("updated connector missing from response: %w", err)
		}
	} else if r.spec.Kind == "connectors" {
		obj = map[string]any{"connectors": result}
	} else if r.spec.Kind == "override" {
		items, ok := result.([]any)
		if !ok || len(items) != 1 {
			return nil, fmt.Errorf("expected a single tool description override in response")
		}
		obj, _ = items[0].(map[string]any)
	} else {
		obj, _ = result.(map[string]any)
	}
	if obj == nil {
		return nil, fmt.Errorf("expected a resource object in response")
	}
	if create {
		keys := r.spec.Keys
		if len(keys) == 0 {
			keys = []string{"id"}
		}
		parts := []string{}
		for _, key := range keys {
			v := textValue(values, key)
			if returned, ok := obj[key].(string); ok {
				v = returned
			}
			if v == "" {
				return nil, fmt.Errorf("create response did not provide resource identifier %s", key)
			}
			parts = append(parts, v)
		}
		values["id"] = types.StringValue(strings.Join(parts, "/"))
	}
	values, err = r.merge(ctx, values, obj)
	if err != nil {
		return values, err
	}
	// Some settings are available only on PATCH, not on the create endpoint.
	if create {
		if len(patch) > 0 {
			record, e := r.recordEndpoint(values)
			if e != nil {
				return values, e
			}
			result, e := r.client.Do(ctx, http.MethodPatch, record, patch)
			if e != nil {
				return values, e
			}
			updated, ok := result.(map[string]any)
			if !ok {
				return values, fmt.Errorf("expected a resource object after follow-up update")
			}
			for k, v := range updated {
				obj[k] = v
			}
		}
	}
	return r.merge(ctx, values, obj)
}
func (r *managementResource) merge(ctx context.Context, values map[string]attr.Value, obj map[string]any) (map[string]attr.Value, error) {
	var mergeErr error
	for _, f := range r.spec.Fields {
		if f.Local {
			continue
		}
		raw, exists := obj[f.Name]
		if f.Name == "tool_names" && r.spec.Kind == "connector_member" {
			tools, ok := obj["tools"].([]any)
			if !ok {
				return values, fmt.Errorf("expected tools array for connector")
			}
			names := make([]any, 0, len(tools))
			for _, tool := range tools {
				t, ok := tool.(map[string]any)
				if !ok {
					return values, fmt.Errorf("invalid connector tool object")
				}
				name, ok := t["name"].(string)
				if !ok || name == "" {
					return values, fmt.Errorf("missing connector tool name")
				}
				names = append(names, name)
			}
			raw, exists = names, true
		}
		if f.Name == "url" && r.spec.Kind == "remote" {
			raw, exists = obj["source_url"]
		}
		if f.Retain && (!exists || raw == nil) {
			if values[f.Name].IsUnknown() {
				values[f.Name] = nullValue(ctx, f.Type)
			}
			continue
		}
		if !exists && !f.Computed && !values[f.Name].IsUnknown() {
			continue
		}
		if f.Name == "connectors" && r.spec.Kind == "connectors" {
			var err error
			raw, err = projectConnectors(ctx, values[f.Name], raw)
			if err != nil {
				return values, err
			}
		}
		v, err := fromJSON(ctx, f.Type, raw)
		if err != nil {
			if mergeErr == nil {
				mergeErr = fmt.Errorf("invalid %s response: %w", f.Name, err)
			}
			// Continue decoding so a later one-time secret is retained even
			// when an earlier response field has an unexpected type.
			continue
		}
		values[f.Name] = v
	}
	return values, mergeErr
}

func findConnector(items []any, slug string) (map[string]any, error) {
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid connector object")
		}
		if obj["slug"] == slug {
			return obj, nil
		}
	}
	return nil, &client.APIError{Status: http.StatusNotFound}
}

// The API returns tools rather than tool_names, and expands an omitted selection
// into all tools. Retain that omission while still checking connector membership.
func projectConnectors(ctx context.Context, prior attr.Value, raw any) (any, error) {
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("expected connector array")
	}
	priorBySlug := map[string]map[string]any{}
	if prior != nil && !prior.IsNull() && !prior.IsUnknown() {
		v, err := toJSON(ctx, prior)
		if err != nil {
			return nil, err
		}
		for _, entry := range v.([]any) {
			o := entry.(map[string]any)
			slug, _ := o["slug"].(string)
			priorBySlug[slug] = o
		}
	}
	out := []any{}
	seen := map[string]bool{}
	project := func(o map[string]any) map[string]any {
		slug, _ := o["slug"].(string)
		v := map[string]any{"slug": slug, "auth_scope": o["auth_scope"]}
		names := []any{}
		if tools, ok := o["tools"].([]any); ok {
			for _, tool := range tools {
				if t, ok := tool.(map[string]any); ok {
					names = append(names, t["name"])
				}
			}
		} else if n, ok := o["tool_names"].([]any); ok {
			names = n
		}
		if old, ok := priorBySlug[slug]; ok {
			if _, selected := old["tool_names"]; !selected {
				v["tool_names"] = nil
			} else {
				v["tool_names"] = names
			}
		} else {
			v["tool_names"] = names
		}
		return v
	}
	bySlug := map[string]map[string]any{}
	for _, item := range items {
		o, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid connector object")
		}
		s, _ := o["slug"].(string)
		bySlug[s] = o
	}
	// Match configured order: the API is allowed to reorder connectors.
	if prior != nil && !prior.IsNull() && !prior.IsUnknown() {
		v, _ := toJSON(ctx, prior)
		for _, entry := range v.([]any) {
			slug, _ := entry.(map[string]any)["slug"].(string)
			if o, ok := bySlug[slug]; ok {
				out = append(out, project(o))
				seen[slug] = true
			}
		}
	}
	for _, item := range items {
		o := item.(map[string]any)
		slug, _ := o["slug"].(string)
		if !seen[slug] {
			out = append(out, project(o))
		}
	}
	return out, nil
}
