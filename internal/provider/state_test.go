package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bogware/terraform-provider-mergeah/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func specNamed(name string) resourceSpec {
	for _, s := range resourceSpecs() {
		if s.Name == name {
			return s
		}
	}
	panic("missing test spec")
}
func fixtureValues(ctx context.Context, s resourceSpec) map[string]attr.Value {
	out := map[string]attr.Value{}
	for _, f := range resourceFields(s) {
		out[f.Name] = nullValue(ctx, f.Type)
		if f.Default != nil {
			v, err := fromJSON(ctx, f.Type, f.Default)
			if err != nil {
				panic(err)
			}
			out[f.Name] = v
		}
	}
	return out
}

func TestNotFoundRemovesState(t *testing.T) {
	ctx := context.Background()
	s := specNamed("tool_pack")
	values := fixtureValues(ctx, s)
	values["id"] = types.StringValue("missing")
	values["name"] = types.StringValue("fixture")
	values["description"] = types.StringValue("")
	raw, err := objectValue(resourceFields(s), values).ToTerraformValue(ctx)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) }))
	defer server.Close()
	c, _ := client.New(server.URL+"/api/v1", "fixture", "test")
	r := managementResource{spec: s, client: c}
	state := tfsdk.State{Schema: resourceSchema(s), Raw: raw}
	resp := resource.ReadResponse{State: state}
	r.Read(ctx, resource.ReadRequest{State: state}, &resp)
	if resp.Diagnostics.HasError() || !resp.State.Raw.IsNull() {
		t.Fatalf("404 was not removed: %v", resp.Diagnostics)
	}
}
func TestPartialCreateFailureKeepsID(t *testing.T) {
	ctx := context.Background()
	s := specNamed("custom_regex_rule")
	values := fixtureValues(ctx, s)
	values["name"] = types.StringValue("fixture")
	values["regex"] = types.StringValue("CUST-[0-9]+")
	values["is_active"] = types.BoolValue(false)
	raw, err := objectValue(resourceFields(s), values).ToTerraformValue(ctx)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			var patch map[string]any
			if err := json.NewDecoder(r.Body).Decode(&patch); err != nil || patch["is_active"] != false {
				t.Errorf("follow-up PATCH lost configured false: %v %v", patch, err)
			}
			w.WriteHeader(500)
			return
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		body["id"] = "created-id"
		body["is_active"] = true
		json.NewEncoder(w).Encode(body)
	}))
	defer server.Close()
	c, _ := client.New(server.URL+"/api/v1", "fixture", "test")
	r := managementResource{spec: s, client: c}
	resp := resource.CreateResponse{State: tfsdk.State{Schema: resourceSchema(s)}}
	r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: resourceSchema(s), Raw: raw}}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected follow-up update failure")
	}
	var id types.String
	diags := resp.State.GetAttribute(ctx, path.Root("id"), &id)
	if diags.HasError() || id.ValueString() != "created-id" {
		t.Fatalf("created resource ID lost: %v %v", id, diags)
	}
}
func TestInboundActionRetentionAndDrift(t *testing.T) {
	ctx := context.Background()
	s := specNamed("standard_entity_rule")
	r := managementResource{spec: s}
	values := fixtureValues(ctx, s)
	values["inbound_action"] = types.StringValue("REDACT")
	values["outbound_action"] = types.StringValue("BLOCK")
	values["is_active"] = types.BoolValue(true)
	got, err := r.merge(ctx, values, map[string]any{"outbound_action": "LOG", "is_active": false})
	if err != nil {
		t.Fatal(err)
	}
	if textValue(got, "inbound_action") != "REDACT" || textValue(got, "outbound_action") != "LOG" || got["is_active"].(types.Bool).ValueBool() {
		t.Fatal("retained unreadable field or readable drift handled incorrectly")
	}
}
func TestInheritedRuleIsMissingOverride(t *testing.T) {
	ctx := context.Background()
	s := specNamed("tool_pack_standard_entity_rule")
	values := fixtureValues(ctx, s)
	values["tool_pack_id"] = types.StringValue("pack-1")
	values["entity_type"] = types.StringValue("EMAIL_ADDRESS")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"entity_type":"EMAIL_ADDRESS","defined_at":"ORGANIZATION"}`))
	}))
	defer server.Close()
	c, _ := client.New(server.URL+"/api/v1", "fixture", "test")
	r := managementResource{spec: s, client: c}
	if _, err := r.fetch(ctx, values); !client.IsNotFound(err) {
		t.Fatalf("inherited rule treated as owned: %v", err)
	}
}

func TestScopeOverrideRetentionAndDrift(t *testing.T) {
	for _, tc := range []struct {
		name     string
		prior    types.String
		response map[string]any
		want     types.String
	}{
		{"normalized", types.StringValue("read write"), map[string]any{"oauth2_scopes_override": nil}, types.StringValue("read write")},
		{"omitted", types.StringValue("read write"), map[string]any{}, types.StringValue("read write")},
		{"readable drift", types.StringValue("read write"), map[string]any{"oauth2_scopes_override": "read"}, types.StringValue("read")},
		{"cleared configuration", types.StringNull(), map[string]any{"oauth2_scopes_override": nil}, types.StringNull()},
		{"empty override", types.StringValue(""), map[string]any{"oauth2_scopes_override": nil}, types.StringValue("")},
		{"unknown", types.StringUnknown(), map[string]any{"oauth2_scopes_override": nil}, types.StringNull()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			s := specNamed("application_credential")
			r := managementResource{spec: s}
			values := fixtureValues(ctx, s)
			values["oauth2_scopes_override"] = tc.prior
			got, err := r.merge(ctx, values, tc.response)
			if err != nil {
				t.Fatal(err)
			}
			if !got["oauth2_scopes_override"].Equal(tc.want) {
				t.Fatalf("scope override = %v; want %v", got["oauth2_scopes_override"], tc.want)
			}
		})
	}
}
