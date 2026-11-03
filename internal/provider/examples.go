package provider

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// WriteExamples creates the sanitized examples consumed by tfplugindocs.
// It is used only by the documentation generation command.
func WriteExamples(root string) error {
	write := func(dir, name, content string) error {
		p := filepath.Join(root, "examples", dir)
		if err := os.MkdirAll(p, 0755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(p, name), []byte(content), 0644)
	}
	for _, s := range resourceSpecs() {
		body := ""
		for _, f := range s.Fields {
			if f.Required {
				body += fmt.Sprintf("  %s = %s\n", f.Name, exampleValue(f))
			}
		}
		if s.Kind == "access_key" {
			body += "  name = \"terraform-managed\"\n  scopes = [\"management:all\"]\n"
		}
		if s.Kind == "remote" {
			body += "  slug = \"support_agent\"\n  auth = { type = \"none\" }\n"
		}
		if s.Kind == "connector_member" {
			body += "  auth_scope = \"INDIVIDUAL\"\n  tool_names = [\"list_issues\"]\n"
		}
		if s.Name == "registered_user" {
			body += "  user_type = \"SYSTEM\"\n"
		}
		if s.Name == "custom_regex_rule" {
			body += "  inbound_action = \"REDACT\"\n  outbound_action = \"BLOCK\"\n"
		}
		if err := write("resources/mergeah_"+s.Name, "resource.tf", fmt.Sprintf("# Configure the provider as shown in examples/provider/provider.tf.\nresource \"mergeah_%s\" \"example\" {\n%s}\n", s.Name, body)); err != nil {
			return err
		}
		keys := s.Keys
		if len(keys) == 0 {
			keys = []string{"id"}
		}
		parts := []string{}
		for _, k := range keys {
			if s.Kind == "remote" && k == "slug" {
				parts = append(parts, "support_agent")
			} else {
				parts = append(parts, strings.Trim(exampleValue(str(k, true)), "\""))
			}
		}
		if err := write("resources/mergeah_"+s.Name, "import.sh", fmt.Sprintf("terraform import mergeah_%s.example '%s'\n", s.Name, strings.Join(parts, "/"))); err != nil {
			return err
		}
	}
	for _, s := range dataSpecs() {
		body := ""
		for _, f := range s.Fields {
			if f.Required {
				body += fmt.Sprintf("  %s = %s\n", f.Name, exampleValue(f))
			}
		}
		content := fmt.Sprintf("# Configure the provider as shown in examples/provider/provider.tf.\ndata \"mergeah_%s\" \"example\" {\n%s}\n\nlocals {\n  %s = jsondecode(data.mergeah_%s.example.response_json)\n}\n", s.Name, body, s.Name, s.Name)
		if err := write("data-sources/mergeah_"+s.Name, "data-source.tf", content); err != nil {
			return err
		}
	}
	return nil
}
func exampleValue(f field) string {
	switch f.Name {
	case "id", "resource_id":
		return `"134e0111-0f67-44f6-98f0-597000290bb3"`
	case "tool_pack_id":
		return `"a6d21b9d-6235-4f85-b175-0799fc4bfed1"`
	case "registered_user_id", "connection_id":
		return `"ed2988b5-e421-429c-87ab-769fa3c1bdd8"`
	case "slug", "connector_slug":
		return `"linear"`
	case "entity_type":
		return `"EMAIL_ADDRESS"`
	case "tool_name":
		return `"list_issues"`
	case "inbound_action":
		return `"REDACT"`
	case "outbound_action":
		return `"BLOCK"`
	case "name":
		return `"Support agent"`
	case "description":
		return `"Tools for the support agent"`
	case "origin_user_id":
		return `"support-agent"`
	case "origin_user_name":
		return `"Support agent"`
	case "url":
		return `"https://mcp.example.com/mcp"`
	case "regex":
		return `"CUST-[0-9]{5}"`
	case "client_id":
		return `"your-oauth-client-id"`
	case "start_time":
		return `"2026-10-01T00:00:00Z"`
	case "end_time":
		return `"2026-11-01T00:00:00Z"`
	case "auth_scope":
		return `"INDIVIDUAL"`
	case "connectors":
		return `[{ slug = "linear", auth_scope = "INDIVIDUAL", tool_names = ["list_issues"] }]`
	}
	switch f.Type {
	case types.BoolType:
		return "true"
	case types.Float64Type:
		return "0.5"
	}
	if _, ok := f.Type.(types.SetType); ok {
		return "[]"
	}
	return `"example"`
}
