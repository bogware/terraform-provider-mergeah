package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestConnectorMembershipTerraformLifecycle(t *testing.T) {
	var mu sync.Mutex
	members := []any{}
	cleared := false
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/api/v1/tool-packs/pack-1/connectors/" {
			t.Errorf("unexpected endpoint %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		if r.Method == "PUT" {
			var input []map[string]any
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Error(err)
				return
			}
			members = []any{}
			for _, o := range input {
				names, ok := o["tool_names"].([]any)
				if !ok {
					names = []any{"list_issues", "create_issue"}
				}
				tools := []any{}
				for _, name := range names {
					tools = append(tools, map[string]any{"name": name, "description": "fixture tool"})
				}
				members = append(members, map[string]any{"slug": o["slug"], "auth_scope": o["auth_scope"], "tools": tools, "name": "Linear"})
			}
			if len(input) == 0 {
				cleared = true
			}
		}
		json.NewEncoder(w).Encode(members)
	}))
	defer s.Close()
	config := func(selection string) string {
		return fmt.Sprintf(`provider "mergeah" {
 api_key = "fixture"
 api_url = %q
}
resource "mergeah_tool_pack_connectors" "test" {
 tool_pack_id = "pack-1"
 connectors = [{
  slug = "linear"
  auth_scope = "INDIVIDUAL"
  %s
 }]
}`, s.URL+"/api/v1", selection)
	}
	resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: factories(), Steps: []resource.TestStep{
		{Config: config(""), Check: resource.TestCheckResourceAttr("mergeah_tool_pack_connectors.test", "id", "pack-1")},
		{ResourceName: "mergeah_tool_pack_connectors.test", ImportState: true, ImportStateId: "pack-1"},
		{Config: config(`tool_names = ["list_issues"]`), Check: resource.TestCheckResourceAttr("mergeah_tool_pack_connectors.test", "connectors.0.tool_names.#", "1")},
	}})
	mu.Lock()
	defer mu.Unlock()
	if !cleared {
		t.Error("destroy did not detach connector membership")
	}
}
