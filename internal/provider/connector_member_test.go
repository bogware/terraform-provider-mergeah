package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/bogware/terraform-provider-mergeah/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func connectorRecord(slug, scope string, names ...string) map[string]any {
	tools := make([]any, 0, len(names))
	for _, name := range names {
		tools = append(tools, map[string]any{"name": name, "description": "fixture"})
	}
	return map[string]any{"slug": slug, "auth_scope": scope, "tools": tools}
}

func TestSingleConnectorTerraformLifecycle(t *testing.T) {
	var mu sync.Mutex
	neighbor := connectorRecord("unmanaged", "ORGANIZATION", "keep_me")
	members := map[string]map[string]any{"unmanaged": neighbor}
	creates, deletes := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		const collection = "/api/v1/tool-packs/pack-1/connectors/"
		if r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, collection) {
			slug := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, collection), "/")
			if slug != "linear" && slug != "github" && slug != "jira" {
				t.Errorf("unexpected deletion: %s", slug)
				w.WriteHeader(400)
				return
			}
			delete(members, slug)
			deletes++
			w.WriteHeader(204)
			return
		}
		if r.URL.Path != collection || (r.Method != http.MethodGet && r.Method != http.MethodPatch) {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(400)
			return
		}
		if r.Method == http.MethodPatch {
			var body []map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body) != 1 {
				t.Errorf("expected one membership per PATCH: %v %v", body, err)
				w.WriteHeader(400)
				return
			}
			item := body[0]
			slug, _ := item["slug"].(string)
			if slug != "linear" && slug != "github" && slug != "jira" {
				t.Errorf("invalid PATCH slug: %q", slug)
				w.WriteHeader(400)
				return
			}
			for key := range item {
				if key != "slug" && key != "auth_scope" && key != "tool_names" {
					t.Errorf("unexpected PATCH field: %s", key)
				}
			}
			current, exists := members[slug]
			if !exists {
				current = connectorRecord(slug, "INDIVIDUAL", "list_issues", "create_issue")
				creates++
			}
			if scope, ok := item["auth_scope"]; ok {
				current["auth_scope"] = scope
			}
			if names, ok := item["tool_names"].([]any); ok {
				if len(names) == 0 {
					t.Error("PATCH must never remove membership using an empty selection")
				}
				tools := make([]any, 0, len(names))
				for _, name := range names {
					tools = append(tools, map[string]any{"name": name})
				}
				current["tools"] = tools
			}
			members[slug] = current
		}
		// The requested connector is deliberately not the first response item.
		out := []any{members["unmanaged"]}
		for _, slug := range []string{"github", "linear", "jira"} {
			if record, ok := members[slug]; ok {
				out = append(out, record)
			}
		}
		json.NewEncoder(w).Encode(out)
	}))
	defer server.Close()
	config := func(slug, attrs string) string {
		return fmt.Sprintf(`provider "mergeah" {
 api_key = "fixture"
 api_url = %q
}
resource "mergeah_tool_pack_connector" "test" {
 tool_pack_id = "pack-1"
 slug = %q
 %s
}
resource "mergeah_tool_pack_connector" "independent" {
 tool_pack_id = "pack-1"
 slug = "github"
 auth_scope = "INDIVIDUAL"
 tool_names = ["list_issues"]
}`, server.URL+"/api/v1", slug, attrs)
	}
	const address = "mergeah_tool_pack_connector.test"
	selection := `auth_scope = "SHARED"
 tool_names = ["list_issues"]`
	resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: factories(), Steps: []resource.TestStep{
		{Config: config("linear", ""), Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr(address, "id", "pack-1/linear"),
			resource.TestCheckResourceAttr(address, "auth_scope", "INDIVIDUAL"),
			resource.TestCheckResourceAttr(address, "tool_names.#", "2"))},
		{ResourceName: address, ImportState: true, ImportStateId: "pack-1/linear", ImportStateVerify: true},
		{Config: config("linear", selection), Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr(address, "auth_scope", "SHARED"),
			resource.TestCheckResourceAttr(address, "tool_names.#", "1"))},
		{PreConfig: func() {
			mu.Lock()
			defer mu.Unlock()
			members["linear"] = connectorRecord("linear", "ORGANIZATION", "create_issue")
		}, Config: config("linear", selection), Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr(address, "auth_scope", "SHARED"),
			resource.TestCheckTypeSetElemAttr(address, "tool_names.*", "list_issues"))},
		// Removing configuration preserves selection rather than resetting all tools.
		{Config: config("linear", `auth_scope = "INDIVIDUAL"`), Check: resource.TestCheckResourceAttr(address, "tool_names.#", "1")},
		{PreConfig: func() {
			mu.Lock()
			defer mu.Unlock()
			members["linear"] = connectorRecord("linear", "INDIVIDUAL", "create_issue")
		}, Config: config("linear", `auth_scope = "INDIVIDUAL"`), PlanOnly: true},
		{PreConfig: func() {
			mu.Lock()
			defer mu.Unlock()
			delete(members, "linear")
		}, Config: config("linear", selection), Check: resource.TestCheckResourceAttr(address, "id", "pack-1/linear")},
		{Config: config("jira", selection), Check: resource.TestCheckResourceAttr(address, "id", "pack-1/jira")},
	}})
	mu.Lock()
	defer mu.Unlock()
	if len(members) != 1 || !reflect.DeepEqual(members["unmanaged"], connectorRecord("unmanaged", "ORGANIZATION", "keep_me")) {
		t.Fatalf("neighbor changed or owned memberships not deleted: %#v", members)
	}
	if creates != 4 || deletes != 3 {
		t.Fatalf("unexpected lifecycle counts: creates=%d deletes=%d", creates, deletes)
	}
}

func TestSingleConnectorRejectsEmptySelection(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: factories(), Steps: []resource.TestStep{{
		Config: `provider "mergeah" {
 api_key = "fixture"
 api_url = "http://127.0.0.1:1/api/v1"
}
resource "mergeah_tool_pack_connector" "test" {
 tool_pack_id = "pack-1"
 slug = "linear"
 tool_names = []
}`, PlanOnly: true, ExpectError: regexp.MustCompile("Empty tool selection"),
	}}})
}

func TestSingleConnectorComputedEmptyToolsDoesNotDetach(t *testing.T) {
	ctx := context.Background()
	spec := specNamed("tool_pack_connector")
	values := fixtureValues(ctx, spec)
	values["tool_pack_id"] = types.StringValue("pack-1")
	values["slug"] = types.StringValue("empty")
	values["id"] = types.StringValue("pack-1/empty")
	values["auth_scope"] = types.StringValue("SHARED")
	values["tool_names"] = types.SetValueMust(types.StringType, []attr.Value{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/api/v1/tool-packs/pack-1/connectors/" {
			t.Errorf("unexpected write: %s %s", r.Method, r.URL.Path)
		}
		var body []map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body) != 1 {
			t.Errorf("invalid body: %v %v", body, err)
			w.WriteHeader(400)
			return
		}
		if _, ok := body[0]["tool_names"]; ok {
			t.Error("computed empty tool set sent as a deletion")
		}
		if body[0]["slug"] != "empty" || body[0]["auth_scope"] != "SHARED" {
			t.Errorf("unexpected update: %v", body)
		}
		json.NewEncoder(w).Encode([]any{connectorRecord("empty", "SHARED")})
	}))
	defer server.Close()
	c, _ := client.New(server.URL+"/api/v1", "fixture", "test")
	r := managementResource{spec: spec, client: c}
	got, err := r.write(ctx, values, false)
	if err != nil || textValue(got, "id") != "pack-1/empty" || len(got["tool_names"].(types.Set).Elements()) != 0 {
		t.Fatalf("empty connector update failed: %v %v", got, err)
	}
}

func TestSingleConnectorReadPaginationAndMissing(t *testing.T) {
	for _, mode := range []string{"found", "missing", "parent_missing"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			spec := specNamed("tool_pack_connector")
			values := fixtureValues(ctx, spec)
			values["tool_pack_id"] = types.StringValue("pack-1")
			values["slug"] = types.StringValue("linear")
			values["id"] = types.StringValue("pack-1/linear")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/tool-packs/pack-1/connectors/" {
					t.Errorf("unexpected read path: %s", r.URL.Path)
				}
				if mode == "parent_missing" {
					w.WriteHeader(404)
					return
				}
				if r.URL.Query().Get("cursor") == "" {
					json.NewEncoder(w).Encode(map[string]any{"data": []any{connectorRecord("other", "INDIVIDUAL")}, "has_more": true, "next_cursor": "second"})
					return
				}
				items := []any{}
				if mode == "found" {
					items = append(items, connectorRecord("linear", "SHARED", "list_issues"))
				}
				json.NewEncoder(w).Encode(map[string]any{"data": items, "has_more": false})
			}))
			defer server.Close()
			c, _ := client.New(server.URL+"/api/v1", "fixture", "test")
			r := managementResource{spec: spec, client: c}
			raw, err := objectValue(resourceFields(spec), values).ToTerraformValue(ctx)
			if err != nil {
				t.Fatal(err)
			}
			state := tfsdk.State{Schema: resourceSchema(spec), Raw: raw}
			resp := frameworkresource.ReadResponse{State: state}
			r.Read(ctx, frameworkresource.ReadRequest{State: state}, &resp)
			if resp.Diagnostics.HasError() {
				t.Fatal(resp.Diagnostics)
			}
			if mode != "found" {
				if !resp.State.Raw.IsNull() {
					t.Fatal("missing membership not removed from state")
				}
				return
			}
			got, err := readValues(ctx, resp.State.Raw, resourceFields(spec))
			want := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("list_issues")})
			if err != nil || textValue(got, "auth_scope") != "SHARED" || !got["tool_names"].Equal(want) {
				t.Fatalf("paginated read failed: %v %v", got, err)
			}
		})
	}
}
