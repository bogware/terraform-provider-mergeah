package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestManagementTerraformLifecycles(t *testing.T) {
	cases := []struct {
		name, initial, updated, importID string
		retain                           bool
	}{
		{"registered_user", "origin_user_id = \"user-1\"\norigin_user_name = \"Initial\"", "origin_user_id = \"user-1\"\norigin_user_name = \"Updated\"", "fixture-id", false},
		{"application_credential", "connector_slug = \"linear\"\nclient_id = \"client-1\"\nclient_secret = \"fixture-secret\"\noauth2_scopes_override = \"read write\"", "connector_slug = \"linear\"\nclient_id = \"client-2\"\nclient_secret = \"fixture-secret\"\noauth2_scopes_override = \"read write\"", "fixture-id", false},
		{"access_key", "name = \"fixture\"\nno_expiration = true", "", "fixture-id", false},
		{"remote_mcp_connector", "name = \"Fixture MCP\"\nurl = \"https://mcp.example.com/mcp\"\nauth = {type = \"none\"}", "", "fixture_mcp", false},
		{"custom_regex_rule", "name = \"Fixture rule\"\nregex = \"initial\"\nis_active = false", "name = \"Fixture rule\"\nregex = \"updated\"\nis_active = true", "fixture-id", false},
		{"tool_pack_custom_regex_rule", "tool_pack_id = \"pack-1\"\nname = \"Fixture rule\"\nregex = \"initial\"\nscore = 0.5\ncontext_keywords = []\nis_active = true\ninbound_action = \"REDACT\"\noutbound_action = \"BLOCK\"", "tool_pack_id = \"pack-1\"\nname = \"Fixture rule\"\nregex = \"updated\"\nscore = 0.5\ncontext_keywords = []\nis_active = true\ninbound_action = \"REDACT\"\noutbound_action = \"BLOCK\"", "pack-1/fixture-id", false},
		{"standard_entity_rule", "entity_type = \"EMAIL_ADDRESS\"\nis_active = true\ninbound_action = \"REDACT\"\noutbound_action = \"BLOCK\"", "entity_type = \"EMAIL_ADDRESS\"\nis_active = false\ninbound_action = \"REDACT\"\noutbound_action = \"BLOCK\"", "EMAIL_ADDRESS", true},
		{"tool_pack_standard_entity_rule", "tool_pack_id = \"pack-1\"\nentity_type = \"EMAIL_ADDRESS\"\nis_active = true\ninbound_action = \"REDACT\"\noutbound_action = \"BLOCK\"", "tool_pack_id = \"pack-1\"\nentity_type = \"EMAIL_ADDRESS\"\nis_active = false\ninbound_action = \"REDACT\"\noutbound_action = \"BLOCK\"", "pack-1/EMAIL_ADDRESS", false},
		{"connector_tool_description_override", "connector_slug = \"linear\"\ntool_name = \"list_issues\"\ndescription = \"initial\"", "connector_slug = \"linear\"\ntool_name = \"list_issues\"\ndescription = \"updated\"", "linear/list_issues", false},
		{"tool_pack_tool_description_override", "tool_pack_id = \"pack-1\"\nconnector_slug = \"linear\"\ntool_name = \"list_issues\"\ndescription = \"initial\"", "tool_pack_id = \"pack-1\"\nconnector_slug = \"linear\"\ntool_name = \"list_issues\"\ndescription = \"updated\"", "pack-1/linear/list_issues", false},
		{"connection_access", "connection_id = \"connection-1\"\nexcluded_registered_users = [\"user-1\"]", "connection_id = \"connection-1\"\nexcluded_registered_users = []", "connection-1", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			stored := map[string]any{}
			exists := false
			deletes := 0
			writes := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				isOverride := strings.Contains(r.URL.Path, "tool-description-overrides")
				if r.Method == "DELETE" || strings.HasSuffix(r.URL.Path, "/revoke/") {
					deletes++
					exists = false
					w.WriteHeader(204)
					return
				}
				if r.Method != "GET" {
					writes++
					var body any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
						w.WriteHeader(400)
						return
					}
					if isOverride {
						body = body.([]any)[0]
						item := body.(map[string]any)
						if item["connector_slug"] != "linear" || item["tool_name"] != "list_issues" {
							t.Error("override write omitted its identifiers")
							w.WriteHeader(400)
							return
						}
					}
					for k, v := range body.(map[string]any) {
						stored[k] = v
					}
					if isOverride && stored["description"] == nil {
						deletes++
						exists = false
						json.NewEncoder(w).Encode([]any{})
						return
					}
					exists = true
					stored["id"] = "fixture-id"
					if tc.name == "registered_user" {
						stored["is_test"] = false
						stored["authenticated_connectors"] = []any{}
					}
					if tc.name == "application_credential" {
						stored["oauth2_scopes_override"] = nil
						stored["created_at"] = "2026-01-01T00:00:00Z"
						stored["modified_at"] = fmt.Sprintf("2026-01-01T00:00:%02dZ", writes)
					}
					if tc.name == "access_key" {
						stored["key"] = "created-secret"
						stored["key_masked"] = "masked"
						if _, ok := stored["scopes"]; !ok {
							stored["scopes"] = []any{"management:all", "runtime:all"}
						}
					}
					if tc.name == "remote_mcp_connector" {
						stored["slug"] = "fixture_mcp"
						stored["source_url"] = stored["url"]
						delete(stored, "url")
						delete(stored, "auth")
					}
					if strings.Contains(tc.name, "standard_entity_rule") {
						stored["entity_type"] = "EMAIL_ADDRESS"
					}
					if tc.name == "custom_regex_rule" {
						if _, ok := stored["is_active"]; !ok {
							stored["is_active"] = true
						}
					}
					delete(stored, "client_secret")
					delete(stored, "inbound_action")
				} else if !exists {
					if isOverride {
						json.NewEncoder(w).Encode([]any{})
						return
					}
					w.WriteHeader(404)
					return
				}
				result := map[string]any{}
				for k, v := range stored {
					result[k] = v
				}
				if r.Method == "GET" {
					delete(result, "key")
				}
				if isOverride {
					json.NewEncoder(w).Encode([]any{result})
				} else {
					json.NewEncoder(w).Encode(result)
				}
			}))
			defer s.Close()
			config := func(body string) string {
				return fmt.Sprintf("provider \"mergeah\" {\napi_key = \"fixture\"\napi_url = %q\n}\nresource \"mergeah_%s\" \"test\" {\n%s\n}\n", s.URL+"/api/v1", tc.name, body)
			}
			name := "mergeah_" + tc.name + ".test"
			steps := []resource.TestStep{{Config: config(tc.initial), Check: resource.TestCheckResourceAttr(name, "id", tc.importID)}, {ResourceName: name, ImportState: true, ImportStateId: tc.importID}}
			if tc.updated != "" {
				steps = append(steps, resource.TestStep{Config: config(tc.updated)})
			}
			resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: factories(), Steps: steps})
			mu.Lock()
			defer mu.Unlock()
			if writes == 0 {
				t.Error("no writes")
			}
			if tc.retain && deletes != 0 {
				t.Error("destroy changed retained remote settings")
			}
			if !tc.retain && deletes != 1 {
				t.Errorf("expected one delete, got %d", deletes)
			}
		})
	}
}

func TestAllDataSourceEndpoints(t *testing.T) {
	for _, spec := range dataSpecs() {
		t.Run(spec.Name, func(t *testing.T) {
			requested := false
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requested = true
				if r.Method != "GET" {
					t.Errorf("unexpected write %s", r.Method)
				}
				if spec.List {
					json.NewEncoder(w).Encode(map[string]any{"results": []any{map[string]any{"id": "read-only"}}, "next": nil})
				} else {
					json.NewEncoder(w).Encode(map[string]any{"id": "read-only"})
				}
			}))
			defer s.Close()
			body := ""
			for _, f := range spec.Fields {
				if f.Required {
					body += fmt.Sprintf("%s = \"fixture\"\n", f.Name)
				}
			}
			config := fmt.Sprintf("provider \"mergeah\" {\napi_key = \"fixture\"\napi_url = %q\n}\ndata \"mergeah_%s\" \"test\" {\n%s\n}\n", s.URL+"/api/v1", spec.Name, body)
			resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: factories(), Steps: []resource.TestStep{{Config: config, Check: resource.TestCheckResourceAttrSet("data.mergeah_"+spec.Name+".test", "response_json")}}})
			if !requested {
				t.Error("data source did not request API")
			}
		})
	}
}
