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

// TestDataSourceFiltersUseDocumentedQueryParameters checks that named filter
// attributes reach the API under their documented query parameter names.
func TestDataSourceFiltersUseDocumentedQueryParameters(t *testing.T) {
	cases := []struct {
		name, path, body, want string
	}{
		{"connector_tool_description_overrides", "/api/v1/connectors/tool-description-overrides/", `connector_slug = "linear"`, "slug=linear"},
		{"tool_pack_tool_description_overrides", "/api/v1/tool-packs/pack-1/tool-description-overrides/", "tool_pack_id = \"pack-1\"\nconnector_slug = \"linear\"", "slug=linear"},
		{"registered_users", "/api/v1/registered-users/", "origin_user_id = \"agent-1\"\nis_test = false\ninclude_test_users = true", "include_test_users=true&is_test=false&origin_user_id=agent-1"},
		{"application_credentials", "/api/v1/application-credentials/", "slug = \"linear\"\nexternal_id = \"ext-1\"", "external_id=ext-1&slug=linear"},
		{"connections", "/api/v1/connections/", "connector = \"linear\"\nstatus = \"ACTIVE,ERROR\"\nauth_scope = \"SHARED\"\ncompany = \"acme\"\ncreated_at_after = \"2026-10-01T00:00:00Z\"\ncreated_at_before = \"2026-11-01T00:00:00Z\"", "auth_scope=SHARED&company=acme&connector=linear&created_at_after=2026-10-01T00%3A00%3A00Z&created_at_before=2026-11-01T00%3A00%3A00Z&status=ACTIVE%2CERROR"},
		{"tool_pack_custom_regex_rules", "/api/v1/tool-packs/pack-1/custom-regex-rules/", "tool_pack_id = \"pack-1\"\ndefined_at = \"TOOL_PACK\"", "defined_at=TOOL_PACK"},
		{"tool_pack_standard_entity_rules", "/api/v1/tool-packs/pack-1/standard-entity-rules/", "tool_pack_id = \"pack-1\"\ndefined_at = \"ORGANIZATION\"", "defined_at=ORGANIZATION"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var got []string
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != tc.path {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				mu.Lock()
				got = append(got, r.URL.RawQuery)
				mu.Unlock()
				json.NewEncoder(w).Encode(map[string]any{"results": []any{}, "next": nil})
			}))
			defer s.Close()
			config := fmt.Sprintf("provider \"mergeah\" {\napi_key = \"fixture\"\napi_url = %q\n}\ndata \"mergeah_%s\" \"test\" {\n%s\n}\n", s.URL+"/api/v1", tc.name, tc.body)
			resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: factories(), Steps: []resource.TestStep{{Config: config, Check: resource.TestCheckResourceAttr("data.mergeah_"+tc.name+".test", "response_json", "[]")}}})
			mu.Lock()
			defer mu.Unlock()
			if len(got) == 0 {
				t.Fatal("data source did not request API")
			}
			for _, q := range got {
				if q != tc.want {
					t.Errorf("query = %q, want %q", q, tc.want)
				}
			}
		})
	}
}
