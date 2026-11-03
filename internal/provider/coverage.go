// Copyright (c) Bogware. SPDX-License-Identifier: MPL-2.0
package provider

import (
	"net/http"
	"slices"
	"sort"
	"strings"
)

// Operation is one Merge API operation the provider calls. Paths are relative
// to the API base (for example "tool-packs/{tool_pack_id}/connectors/").
type Operation struct {
	Method, Path string
	// Owners lists the Terraform resources and data sources that call it.
	Owners []string
	// Params lists named query parameters (GET) or writable body attributes.
	Params []string
}

// CoveredOperations reports the API operations used by every resource and data
// source. It mirrors the request logic in resource.go and data_source.go; keep
// them in sync. cmd/check-api-coverage compares the result with the OpenAPI spec.
func CoveredOperations() []Operation {
	ops := map[string]*Operation{}
	add := func(owner, method, path string, params []string) {
		key := method + " " + path
		op, ok := ops[key]
		if !ok {
			op = &Operation{Method: method, Path: path}
			ops[key] = op
		}
		op.Owners = appendUnique(op.Owners, owner)
		for _, p := range params {
			op.Params = appendUnique(op.Params, p)
		}
	}
	for _, s := range dataSpecs() {
		owner := "data.mergeah_" + s.Name
		var params []string
		for _, f := range s.Fields {
			if strings.Contains(s.Endpoint, "{"+f.Name+"}") {
				continue
			}
			name := f.Name
			if f.Query != "" {
				name = f.Query
			}
			params = append(params, name)
		}
		add(owner, http.MethodGet, s.Endpoint, params)
	}
	for _, s := range resourceSpecs() {
		owner := "mergeah_" + s.Name
		record := resourceRecordPath(s)
		writable := writableFieldNames(s.Fields)

		create, createMethod := s.Collection, s.CreateMethod
		if createMethod == "" {
			createMethod = http.MethodPost
		}
		if s.Kind == "standard" || s.Kind == "standard_override" || s.Kind == "connection_access" {
			create = record
		}
		add(owner, createMethod, create, writable)

		switch s.Kind {
		case "connector_member", "override", "connectors":
			add(owner, http.MethodGet, s.Collection, nil)
		default:
			add(owner, http.MethodGet, record, nil)
		}

		if hasUpdatableField(s.Fields) {
			update, updateMethod := record, s.UpdateMethod
			if updateMethod == "" {
				updateMethod = http.MethodPatch
			}
			if s.Kind == "connector_member" {
				update = s.Collection
			}
			add(owner, updateMethod, update, writable)
		}

		switch s.Kind {
		case "standard", "connection_access":
			// Destroy only removes the resource from state.
		case "access_key":
			add(owner, s.DeleteMethod, record+"revoke/", nil)
		case "connectors":
			add(owner, http.MethodPut, s.Collection, nil)
		case "override":
			add(owner, http.MethodPatch, s.Collection, nil)
		case "connector_member":
			add(owner, http.MethodDelete, s.Collection+"{slug}/", nil)
		default:
			method := s.DeleteMethod
			if method == "" {
				method = http.MethodDelete
			}
			add(owner, method, record, nil)
		}
	}
	out := make([]Operation, 0, len(ops))
	for _, op := range ops {
		sort.Strings(op.Owners)
		sort.Strings(op.Params)
		out = append(out, *op)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Method < out[j].Method
	})
	return out
}

// resourceRecordPath mirrors managementResource.recordEndpoint.
func resourceRecordPath(s resourceSpec) string {
	switch s.Kind {
	case "override", "connectors":
		return s.Collection
	case "remote", "connector_member":
		return s.Collection + "{slug}/"
	case "standard", "standard_override":
		return s.Collection + "{entity_type}/"
	case "connection_access":
		return s.Collection + "{connection_id}/"
	default:
		return s.Collection + "{id}/"
	}
}

// writableFieldNames returns configurable attribute names, including nested ones.
func writableFieldNames(fields []field) []string {
	var out []string
	for _, f := range fields {
		if f.Computed || f.Local {
			continue
		}
		out = append(out, f.Name)
		out = append(out, writableFieldNames(f.Children)...)
	}
	return out
}

// hasUpdatableField reports whether any configurable attribute changes in place.
func hasUpdatableField(fields []field) bool {
	for _, f := range fields {
		if !f.Computed && !f.Replace {
			return true
		}
	}
	return false
}

func appendUnique(list []string, v string) []string {
	if slices.Contains(list, v) {
		return list
	}
	return append(list, v)
}
