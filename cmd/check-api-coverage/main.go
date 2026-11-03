// Compare the provider's API usage with the Merge Agent Handler OpenAPI spec.
//
// Usage (from the repository root):
//
//	go run ./cmd/check-api-coverage
//	go run ./cmd/check-api-coverage -spec path/to/agent-handler-api.json -v
//
// The tool reports spec operations the provider neither uses nor deliberately
// excludes, provider operations missing from the spec, stale exclusions, and
// documented query filters or request body fields the provider does not expose.
// It exits with status 1 when it finds anything, so it can gate a release.
// Run it before each release and whenever Merge announces API changes.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/bogware/terraform-provider-mergeah/internal/provider"
)

const defaultSpec = "https://docs.merge.dev/openapi/agent-handler-api.json"

// excludedOperations are runtime operations without a declarative lifecycle.
// Keys use "METHOD path" with every path parameter written as {}.
// Keep this list consistent with templates/guides/coverage.md.tmpl.
var excludedOperations = map[string]string{
	"POST registered-users/{}/link-token/":                  "short-lived Merge Link token",
	"POST access-keys/{}/regenerate/":                       "rotate keys by replacing mergeah_access_key",
	"DELETE credentials/registered-users/{}/connectors/{}/": "credential deletion is a runtime operation",
	"DELETE connections/{}/":                                "connection revocation is a runtime operation",
	"PATCH tool-access-requests/{}/":                        "approving, denying, or revoking tool access requests is a runtime decision",
	"POST tool-packs/{}/registered-users/{}/mcp/":           "MCP tool execution",
	"POST tool-packs/{}/registered-users/{}/search/":        "MCP tool search",
	"POST files/":            "file transfer",
	"PUT files/{}/upload/":   "file transfer",
	"GET files/{}/download/": "file transfer",
	"POST agent/signup/":     "agent signup",
	"POST agent/claim/":      "claim-link issuance",
}

// ignoredParams are documented parameters the provider deliberately omits.
// Keys use "METHOD path param".
var ignoredParams = map[string]string{
	"POST access-keys/ expires_in":                           "relative expiry changes on every apply; use expires_at",
	"POST tool-packs/ connectors":                            "membership is managed by mergeah_tool_pack_connectors or mergeah_tool_pack_connector",
	"PATCH tool-packs/{}/ connectors":                        "membership is managed by mergeah_tool_pack_connectors or mergeah_tool_pack_connector",
	"PUT tool-packs/{}/connectors/ connector_id":             "connectors are identified by slug",
	"PATCH tool-packs/{}/connectors/ connector_id":           "connectors are identified by slug",
	"PATCH tool-packs/{}/custom-regex-rules/{}/ id":          "server-assigned; the spec does not mark it read-only",
	"PATCH tool-packs/{}/custom-regex-rules/{}/ created_at":  "server-assigned; the spec does not mark it read-only",
	"PATCH tool-packs/{}/custom-regex-rules/{}/ modified_at": "server-assigned; the spec does not mark it read-only",
}

// pagingParams are handled by the client's pagination, not exposed as filters.
var pagingParams = map[string]bool{"page": true, "page_size": true, "cursor": true}

var placeholder = regexp.MustCompile(`\{[^}]+\}`)

func normalize(path string) string {
	path = strings.TrimPrefix(path, "/api/v1/")
	return placeholder.ReplaceAllString(path, "{}")
}

type specOperation struct {
	Method, Path, Summary string
	Query, Body           []string
}

func main() {
	specFlag := flag.String("spec", defaultSpec, "OpenAPI spec URL or local file path")
	verbose := flag.Bool("v", false, "Also list every covered and excluded operation")
	flag.Parse()

	raw, err := load(*specFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	specOps, err := parseSpec(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	findings := compare(specOps, provider.CoveredOperations(), excludedOperations, ignoredParams, *verbose, os.Stdout)
	if findings > 0 {
		fmt.Printf("\n%d finding(s). Update the provider, or record a deliberate exclusion in cmd/check-api-coverage.\n", findings)
		os.Exit(1)
	}
	fmt.Println("\nNo coverage gaps found.")
}

func load(src string) ([]byte, error) {
	if !strings.HasPrefix(src, "https://") && !strings.HasPrefix(src, "http://") {
		return os.ReadFile(src)
	}
	c := &http.Client{Timeout: 60 * time.Second}
	resp, err := c.Get(src)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s: %s", src, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 32<<20))
}

func parseSpec(raw []byte) (map[string]specOperation, error) {
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parsing OpenAPI spec: %w", err)
	}
	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("OpenAPI spec has no paths")
	}
	schemas := map[string]any{}
	if comps, ok := doc["components"].(map[string]any); ok {
		if s, ok := comps["schemas"].(map[string]any); ok {
			schemas = s
		}
	}
	out := map[string]specOperation{}
	for path, item := range paths {
		methods, _ := item.(map[string]any)
		for method, rawOp := range methods {
			m := strings.ToUpper(method)
			switch m {
			case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			default:
				continue
			}
			op, _ := rawOp.(map[string]any)
			so := specOperation{Method: m, Path: normalize(path)}
			so.Summary, _ = op["summary"].(string)
			params, _ := op["parameters"].([]any)
			for _, p := range params {
				pm, _ := p.(map[string]any)
				if pm["in"] == "query" {
					if name, _ := pm["name"].(string); !pagingParams[name] {
						so.Query = append(so.Query, name)
					}
				}
			}
			if body, ok := op["requestBody"].(map[string]any); ok {
				content, _ := body["content"].(map[string]any)
				if js, ok := content["application/json"].(map[string]any); ok {
					so.Body = properties(js["schema"], schemas, 0)
				}
			}
			sort.Strings(so.Query)
			sort.Strings(so.Body)
			out[so.Method+" "+so.Path] = so
		}
	}
	return out, nil
}

// properties lists the property names of a request schema, following $ref,
// composition keywords, and array items.
func properties(schema any, schemas map[string]any, depth int) []string {
	s, ok := schema.(map[string]any)
	if !ok || depth > 8 {
		return nil
	}
	if ref, ok := s["$ref"].(string); ok {
		return properties(schemas[ref[strings.LastIndex(ref, "/")+1:]], schemas, depth+1)
	}
	var out []string
	if props, ok := s["properties"].(map[string]any); ok {
		for name, p := range props {
			if pm, ok := p.(map[string]any); ok && pm["readOnly"] == true {
				continue
			}
			out = append(out, name)
		}
	}
	for _, key := range []string{"allOf", "oneOf", "anyOf"} {
		list, _ := s[key].([]any)
		for _, sub := range list {
			out = append(out, properties(sub, schemas, depth+1)...)
		}
	}
	if items, ok := s["items"]; ok {
		out = append(out, properties(items, schemas, depth+1)...)
	}
	return out
}

// compare writes a report and returns the number of findings.
func compare(specOps map[string]specOperation, covered []provider.Operation, excluded, ignored map[string]string, verbose bool, w io.Writer) int {
	coveredByKey := map[string]provider.Operation{}
	for _, op := range covered {
		coveredByKey[op.Method+" "+normalize(op.Path)] = op
	}
	keys := make([]string, 0, len(specOps))
	for k := range specOps {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var uncovered, missingParams, stale, staleExclusions, coveredLines, excludedLines []string
	for _, k := range keys {
		so := specOps[k]
		op, isCovered := coveredByKey[k]
		reason, isExcluded := excluded[k]
		switch {
		case isCovered:
			coveredLines = append(coveredLines, fmt.Sprintf("%-60s %s", k, strings.Join(op.Owners, ", ")))
			have := map[string]bool{}
			for _, p := range op.Params {
				have[p] = true
			}
			documented := so.Body
			kind := "body field"
			if so.Method == http.MethodGet {
				documented, kind = so.Query, "query filter"
			}
			for _, p := range documented {
				if !have[p] && ignored[k+" "+p] == "" {
					missingParams = append(missingParams, fmt.Sprintf("%-60s %s %q (used by %s)", k, kind, p, strings.Join(op.Owners, ", ")))
				}
			}
		case isExcluded:
			excludedLines = append(excludedLines, fmt.Sprintf("%-60s %s", k, reason))
		default:
			uncovered = append(uncovered, fmt.Sprintf("%-60s %s", k, so.Summary))
		}
	}
	for k, op := range coveredByKey {
		if _, ok := specOps[k]; !ok {
			stale = append(stale, fmt.Sprintf("%-60s %s", k, strings.Join(op.Owners, ", ")))
		}
	}
	for k := range excluded {
		if _, ok := specOps[k]; !ok {
			staleExclusions = append(staleExclusions, k)
		}
	}
	sort.Strings(stale)
	sort.Strings(staleExclusions)

	fmt.Fprintf(w, "Spec operations: %d  covered: %d  excluded: %d\n", len(specOps), len(coveredLines), len(excludedLines))
	section := func(title string, lines []string) {
		if len(lines) == 0 {
			return
		}
		fmt.Fprintf(w, "\n%s (%d):\n", title, len(lines))
		for _, l := range lines {
			fmt.Fprintln(w, "  "+l)
		}
	}
	section("Spec operations not used by the provider and not excluded", uncovered)
	section("Documented parameters the provider does not expose", missingParams)
	section("Provider operations missing from the spec", stale)
	section("Exclusions no longer in the spec", staleExclusions)
	if verbose {
		section("Covered", coveredLines)
		section("Excluded", excludedLines)
	}
	return len(uncovered) + len(missingParams) + len(stale) + len(staleExclusions)
}
