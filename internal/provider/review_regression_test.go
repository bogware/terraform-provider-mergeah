package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bogware/terraform-provider-mergeah/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestCreateResponseFailureRetainsIdentityAndSecrets(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response map[string]any
	}{
		{"access_key", map[string]any{"id": "created-id", "scopes": "wrong-type", "key": "one-time-fixture", "key_masked": "masked"}},
		{"registered_user", map[string]any{"id": "created-id", "authenticated_connectors": false, "is_test": true}},
		{"tool_pack", map[string]any{"id": "created-id", "name": []any{}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			spec := specNamed(tc.name)
			values := fixtureValues(ctx, spec)
			for _, f := range spec.Fields {
				if f.Required {
					values[f.Name] = types.StringValue("fixture")
				}
				if f.Computed {
					values[f.Name] = nullValue(ctx, f.Type)
				}
			}
			values["id"] = types.StringUnknown()
			if tc.name == "access_key" {
				values["key"] = types.StringUnknown()
			}
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodPost {
					t.Errorf("unexpected method %s", r.Method)
				}
				json.NewEncoder(w).Encode(tc.response)
			}))
			defer server.Close()
			c, _ := client.New(server.URL+"/api/v1", "fixture", "test")
			r := managementResource{spec: spec, client: c}
			raw, err := objectValue(resourceFields(spec), values).ToTerraformValue(ctx)
			if err != nil {
				t.Fatal(err)
			}
			resp := resource.CreateResponse{State: tfsdk.State{Schema: resourceSchema(spec)}}
			r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: resourceSchema(spec), Raw: raw}}, &resp)
			if !resp.Diagnostics.HasError() || calls != 1 {
				t.Fatalf("expected one create and a decoding error: %v", resp.Diagnostics)
			}
			var id types.String
			if d := resp.State.GetAttribute(ctx, path.Root("id"), &id); d.HasError() || id.ValueString() != "created-id" {
				t.Fatalf("created ID lost: %v %v", id, d)
			}
			if tc.name == "access_key" {
				var key types.String
				if d := resp.State.GetAttribute(ctx, path.Root("key"), &key); d.HasError() || key.ValueString() != "one-time-fixture" {
					t.Fatal("one-time key lost after an earlier field failed to decode")
				}
			}
			for _, d := range resp.Diagnostics {
				if strings.Contains(d.Detail(), "one-time-fixture") {
					t.Fatal("diagnostic leaked key")
				}
			}
		})
	}
}

func TestCreateWithoutRecoverableIDExplainsReconciliation(t *testing.T) {
	ctx := context.Background()
	spec := specNamed("tool_pack")
	values := fixtureValues(ctx, spec)
	values["name"] = types.StringValue("fixture")
	values["description"] = types.StringValue("")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"secret":"do-not-log",`))
	}))
	defer server.Close()
	c, _ := client.New(server.URL+"/api/v1", "fixture", "test")
	r := managementResource{spec: spec, client: c}
	raw, _ := objectValue(resourceFields(spec), values).ToTerraformValue(ctx)
	resp := resource.CreateResponse{State: tfsdk.State{Schema: resourceSchema(spec)}}
	r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: resourceSchema(spec), Raw: raw}}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected malformed-response error")
	}
	for _, d := range resp.Diagnostics {
		if !strings.Contains(d.Detail(), "before applying again") || strings.Contains(d.Detail(), "do-not-log") {
			t.Fatalf("missing or unsafe recovery guidance: %v", d)
		}
	}
}

func TestFollowUpResponseFailureKeepsCreatedID(t *testing.T) {
	for _, response := range []string{`[]`, `{"is_active":false,"defined_at":42}`} {
		t.Run(response, func(t *testing.T) {
			ctx := context.Background()
			spec := specNamed("custom_regex_rule")
			values := fixtureValues(ctx, spec)
			values["name"] = types.StringValue("fixture")
			values["regex"] = types.StringValue("fixture")
			values["is_active"] = types.BoolValue(false)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPatch {
					w.Write([]byte(response))
					return
				}
				w.Write([]byte(`{"id":"created-id","is_active":true}`))
			}))
			defer server.Close()
			c, _ := client.New(server.URL+"/api/v1", "fixture", "test")
			r := managementResource{spec: spec, client: c}
			raw, _ := objectValue(resourceFields(spec), values).ToTerraformValue(ctx)
			resp := resource.CreateResponse{State: tfsdk.State{Schema: resourceSchema(spec)}}
			r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: resourceSchema(spec), Raw: raw}}, &resp)
			var id types.String
			d := resp.State.GetAttribute(ctx, path.Root("id"), &id)
			if !resp.Diagnostics.HasError() || d.HasError() || id.ValueString() != "created-id" {
				t.Fatalf("follow-up response failure lost ID or was ignored: %v %v", resp.Diagnostics, d)
			}
		})
	}
}

func TestValidationEmptyPolicyAndNullConnectors(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name             string
		allow, wantError bool
	}{
		{"description", false, true}, {"display_text", true, false},
	} {
		spec := resourceSpec{Fields: []field{{Name: tc.name, Type: types.StringType, Required: true, AllowEmpty: tc.allow}}}
		values := fixtureValues(ctx, spec)
		values[tc.name] = types.StringValue("")
		raw, _ := objectValue(resourceFields(spec), values).ToTerraformValue(ctx)
		r := managementResource{spec: spec}
		resp := resource.ValidateConfigResponse{}
		r.ValidateConfig(ctx, resource.ValidateConfigRequest{Config: tfsdk.Config{Schema: resourceSchema(spec), Raw: raw}}, &resp)
		if resp.Diagnostics.HasError() != tc.wantError {
			t.Fatalf("empty policy failed: %s %v", tc.name, resp.Diagnostics)
		}
	}
	spec := specNamed("tool_pack_connectors")
	values := fixtureValues(ctx, spec)
	values["tool_pack_id"] = types.StringValue("pack-1")
	listType := spec.Fields[1].Type.(types.ListType)
	objType := listType.ElemType.(types.ObjectType)
	nullSlug := types.ObjectValueMust(objType.AttrTypes, map[string]attr.Value{
		"slug": types.StringNull(), "auth_scope": types.StringNull(), "tool_names": types.SetNull(types.StringType),
	})
	values["connectors"] = types.ListValueMust(objType, []attr.Value{types.ObjectNull(objType.AttrTypes), nullSlug, nullSlug})
	raw, _ := objectValue(resourceFields(spec), values).ToTerraformValue(ctx)
	r := managementResource{spec: spec}
	resp := resource.ValidateConfigResponse{}
	r.ValidateConfig(ctx, resource.ValidateConfigRequest{Config: tfsdk.Config{Schema: resourceSchema(spec), Raw: raw}}, &resp)
	// Framework required-field validation handles null slugs separately.
	if resp.Diagnostics.HasError() {
		t.Fatalf("null elements produced resource-level errors: %v", resp.Diagnostics)
	}
}

func TestSchemaPreservesSensitiveAndReplacementFlags(t *testing.T) {
	ctx := context.Background()
	attrs := resourceAttributes([]field{
		secret(immutable(number("amount", false))),
		secret(immutable(field{Name: "names", Type: types.ListType{ElemType: types.StringType}})),
	})
	num := attrs["amount"].(schema.Float64Attribute)
	list := attrs["names"].(schema.ListAttribute)
	if !num.Sensitive || !list.Sensitive || len(num.PlanModifiers) != 1 || len(list.PlanModifiers) != 1 {
		t.Fatal("schema flags lost")
	}
	spec := resourceSpec{Fields: []field{number("amount", false)}}
	values := fixtureValues(ctx, spec)
	values["amount"] = types.Float64Value(1)
	raw, _ := objectValue(resourceFields(spec), values).ToTerraformValue(ctx)
	state := tfsdk.State{Raw: raw}
	plan := tfsdk.Plan{Raw: raw}
	numResp := planmodifier.Float64Response{}
	num.PlanModifiers[0].PlanModifyFloat64(ctx, planmodifier.Float64Request{State: state, Plan: plan, StateValue: types.Float64Value(1), PlanValue: types.Float64Value(2)}, &numResp)
	listResp := planmodifier.ListResponse{}
	list.PlanModifiers[0].PlanModifyList(ctx, planmodifier.ListRequest{State: state, Plan: plan, StateValue: types.ListValueMust(types.StringType, []attr.Value{types.StringValue("old")}), PlanValue: types.ListValueMust(types.StringType, []attr.Value{types.StringValue("new")})}, &listResp)
	if !numResp.RequiresReplace || !listResp.RequiresReplace {
		t.Fatal("changed immutable attributes did not request replacement")
	}
}

func TestOnlyStableComputedValuesUsePriorState(t *testing.T) {
	attrs := resourceSchema(specNamed("application_credential")).Attributes
	created := attrs["created_at"].(schema.StringAttribute)
	modified := attrs["modified_at"].(schema.StringAttribute)
	if len(created.PlanModifiers) != 1 || len(modified.PlanModifiers) != 0 {
		t.Fatal("stable and changing timestamps have incorrect modifiers")
	}
	ctx := context.Background()
	spec := specNamed("application_credential")
	values := fixtureValues(ctx, spec)
	values["created_at"] = types.StringValue("2026-01-01T00:00:00Z")
	raw, _ := objectValue(resourceFields(spec), values).ToTerraformValue(ctx)
	resp := planmodifier.StringResponse{PlanValue: types.StringUnknown()}
	created.PlanModifiers[0].PlanModifyString(ctx, planmodifier.StringRequest{State: tfsdk.State{Raw: raw}, StateValue: values["created_at"].(types.String), PlanValue: types.StringUnknown(), ConfigValue: types.StringNull()}, &resp)
	if !resp.PlanValue.Equal(values["created_at"]) {
		t.Fatal("stable timestamp not preserved in plan")
	}
}
