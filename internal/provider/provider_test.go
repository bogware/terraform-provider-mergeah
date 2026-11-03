package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestProviderSchema(t *testing.T) {
	server := providerserver.NewProtocol6(New("test")())()
	resp, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			t.Fatalf("%s: %s", d.Summary, d.Detail)
		}
	}
	if len(resp.ResourceSchemas) != 14 || len(resp.DataSourceSchemas) != 30 {
		t.Fatalf("unexpected schema count: %d resources, %d data sources", len(resp.ResourceSchemas), len(resp.DataSourceSchemas))
	}
}

func factories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){"mergeah": providerserver.NewProtocol6WithError(New("test")())}
}

// Uses the installed Terraform CLI against an in-memory HTTP API, never Merge.
func TestToolPackTerraformLifecycle(t *testing.T) {
	var mu sync.Mutex
	var stored map[string]any
	creates, updates, deletes := 0, 0, 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer fixture" {
			t.Error("missing key")
		}
		switch r.Method {
		case "POST":
			creates++
			json.NewDecoder(r.Body).Decode(&stored)
			stored["id"] = "pack-id"
		case "PATCH":
			updates++
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			for k, v := range body {
				stored[k] = v
			}
		case "DELETE":
			deletes++
			stored = nil
			w.WriteHeader(204)
			return
		case "GET":
			if stored == nil {
				w.WriteHeader(404)
				return
			}
		}
		json.NewEncoder(w).Encode(stored)
	}))
	defer s.Close()
	config := func(name string) string {
		return fmt.Sprintf(`provider "mergeah" {
  api_key = "fixture"
  api_url = %q
}
resource "mergeah_tool_pack" "test" {
  name = %q
  description = "fixture"
}`, s.URL+"/api/v1", name)
	}
	resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: factories(), Steps: []resource.TestStep{
		{Config: config("initial"), Check: resource.TestCheckResourceAttr("mergeah_tool_pack.test", "id", "pack-id")},
		{ResourceName: "mergeah_tool_pack.test", ImportState: true, ImportStateVerify: true},
		{Config: config("updated"), Check: resource.TestCheckResourceAttr("mergeah_tool_pack.test", "name", "updated")},
	}})
	mu.Lock()
	defer mu.Unlock()
	if creates != 1 || updates != 1 || deletes != 1 {
		t.Fatalf("unexpected lifecycle writes: %d %d %d", creates, updates, deletes)
	}
}
