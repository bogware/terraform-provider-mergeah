package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/bogware/terraform-provider-mergeah/internal/provider"
)

const fixtureSpec = `{
  "openapi": "3.1.0",
  "paths": {
    "/api/v1/widgets/": {
      "get":  {"summary": "List widgets", "parameters": [
        {"name": "page", "in": "query"},
        {"name": "color", "in": "query"},
        {"name": "size", "in": "query"}
      ]},
      "post": {"summary": "Create widget", "requestBody": {"content": {"application/json": {"schema": {"$ref": "#/components/schemas/Widget"}}}}}
    },
    "/api/v1/widgets/{widget_id}/run/": {"post": {"summary": "Run widget"}},
    "/api/v1/gadgets/{id}/": {"delete": {"summary": "Delete gadget"}},
    "/api/v1/new-thing/": {"get": {"summary": "List new things"}}
  },
  "components": {"schemas": {"Widget": {"allOf": [
    {"type": "object", "properties": {"name": {"type": "string"}, "id": {"type": "string", "readOnly": true}}},
    {"type": "object", "properties": {"secret_mode": {"type": "boolean"}, "legacy": {"type": "string"}}}
  ]}}}
}`

func TestCompareReportsEveryKindOfFinding(t *testing.T) {
	specOps, err := parseSpec([]byte(fixtureSpec))
	if err != nil {
		t.Fatal(err)
	}
	covered := []provider.Operation{
		{Method: "GET", Path: "widgets/", Owners: []string{"data.mergeah_widgets"}, Params: []string{"color"}},
		{Method: "POST", Path: "widgets/", Owners: []string{"mergeah_widget"}, Params: []string{"name"}},
		{Method: "PATCH", Path: "widgets/{id}/", Owners: []string{"mergeah_widget"}},
	}
	excluded := map[string]string{
		"POST widgets/{}/run/": "runtime",
		"POST removed/":        "no longer in spec",
	}
	ignored := map[string]string{"POST widgets/ legacy": "deprecated"}

	var out bytes.Buffer
	got := compare(specOps, covered, excluded, ignored, true, &out)
	report := out.String()
	if got != 6 {
		t.Errorf("findings = %d, want 6\n%s", got, report)
	}
	for _, want := range []string{
		"DELETE gadgets/{}/",       // uncovered
		"GET new-thing/",           // uncovered
		`query filter "size"`,      // missing filter
		`body field "secret_mode"`, // missing body field
		"PATCH widgets/{}/",        // provider operation not in spec
		"POST removed/",            // stale exclusion
		"Excluded (1)",             // listed with -v
	} {
		if !strings.Contains(report, want) {
			t.Errorf("report missing %q\n%s", want, report)
		}
	}
	for _, unwanted := range []string{`"page"`, `"id"`, `"legacy"`, `"color"`, `"name"`} {
		if strings.Contains(report, unwanted) {
			t.Errorf("report should not mention %s\n%s", unwanted, report)
		}
	}
}

func TestNormalizeIgnoresParameterNames(t *testing.T) {
	if a, b := normalize("/api/v1/tool-packs/{tool_pack_id}/rules/{rule_id}/"), normalize("tool-packs/{id}/rules/{resource_id}/"); a != b {
		t.Errorf("%q != %q", a, b)
	}
}
