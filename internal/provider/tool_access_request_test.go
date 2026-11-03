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

func TestToolAccessRequestDataSourcesSendPathAndFilters(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]string{}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected write %s %s", r.Method, r.URL.Path)
		}
		mu.Lock()
		seen[r.URL.Path] = r.URL.RawQuery
		mu.Unlock()
		switch r.URL.Path {
		case "/api/v1/tool-access-requests/":
			json.NewEncoder(w).Encode(map[string]any{"results": []any{map[string]any{"id": "req-1", "status": "pending"}}, "next": nil})
		case "/api/v1/tool-access-requests/req-1/":
			json.NewEncoder(w).Encode(map[string]any{"id": "req-1", "status": "pending"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer s.Close()

	config := fmt.Sprintf(`provider "mergeah" {
  api_key = "fixture"
  api_url = %q
}

data "mergeah_tool_access_requests" "pending" {
  status             = "pending"
  registered_user_id = "ed2988b5-e421-429c-87ab-769fa3c1bdd8"
  search             = "linear"
  query              = { page_size = "50" }
}

data "mergeah_tool_access_request" "one" {
  resource_id = "req-1"
}
`, s.URL+"/api/v1")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories(),
		Steps: []resource.TestStep{{
			Config: config,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.mergeah_tool_access_requests.pending", "response_json", `[{"id":"req-1","status":"pending"}]`),
				resource.TestCheckResourceAttr("data.mergeah_tool_access_request.one", "response_json", `{"id":"req-1","status":"pending"}`),
			),
		}},
	})

	mu.Lock()
	defer mu.Unlock()
	want := "page_size=50&registered_user_id=ed2988b5-e421-429c-87ab-769fa3c1bdd8&search=linear&status=pending"
	if got, ok := seen["/api/v1/tool-access-requests/"]; !ok || got != want {
		t.Errorf("list query = %q, want %q", got, want)
	}
	if got, ok := seen["/api/v1/tool-access-requests/req-1/"]; !ok || got != "" {
		t.Errorf("detail request missing or had query %q", got)
	}
}
