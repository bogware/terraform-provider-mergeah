// Copyright (c) Bogware. SPDX-License-Identifier: MPL-2.0
package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// field describes one Terraform attribute and its documented API representation.
type field struct {
	Name                                                  string
	Type                                                  attr.Type
	Required, Computed, Replace, Sensitive, Local, Retain bool
	Description                                           string
	Children                                              []field
	JSON                                                  bool
	OptionalComputed, CreateOmit                          bool
	Default                                               any
	Enum                                                  []string
	AllowEmpty, Stable                                    bool
}

func str(name string, required bool) field {
	return field{Name: name, Type: types.StringType, Required: required, Description: "Merge API field `" + name + "`."}
}
func computed(name string, t attr.Type) field {
	return field{Name: name, Type: t, Computed: true, Description: "Value reported by Merge: `" + name + "`."}
}
func stringsField(name string, required bool) field {
	f := str(name, required)
	f.Type = types.SetType{ElemType: types.StringType}
	return f
}
func boolean(name string, required bool) field {
	f := str(name, required)
	f.Type = types.BoolType
	return f
}
func number(name string, required bool) field {
	f := str(name, required)
	f.Type = types.Float64Type
	return f
}
func immutable(f field) field              { f.Replace = true; return f }
func allowEmpty(f field) field             { f.AllowEmpty = true; return f }
func stable(f field) field                 { f.Stable = true; return f }
func local(f field) field                  { f.Local = true; return f }
func secret(f field) field                 { f.Sensitive = true; f.Retain = true; return f }
func retained(f field) field               { f.Retain = true; return f }
func optionalComputed(f field) field       { f.OptionalComputed = true; return f }
func createOmit(f field) field             { f.CreateOmit = true; return f }
func jsonField(f field) field              { f.JSON = true; return f }
func enum(f field, values ...string) field { f.Enum = values; return f }
func defaulted(f field, v any) field       { f.Default = v; return f }
func object(name string, required bool, children ...field) field {
	f := str(name, required)
	f.Children = children
	f.Type = types.ObjectType{AttrTypes: fieldTypes(children)}
	return f
}
func fieldTypes(fields []field) map[string]attr.Type {
	m := map[string]attr.Type{}
	for _, f := range fields {
		m[f.Name] = f.Type
	}
	return m
}
func timestamps() []field {
	return []field{stable(computed("created_at", types.StringType)), computed("modified_at", types.StringType)}
}

type resourceSpec struct {
	Name, Description, Collection, CreateMethod, UpdateMethod, DeleteMethod string
	Fields                                                                  []field
	// Keys are used for deterministic IDs and import, separated by '/'.
	Keys []string
	Kind string
}

func resourceSpecs() []resourceSpec {
	group := object("shared_credential_group", false, str("origin_company_id", false), str("origin_company_name", false), field{Name: "custom_groupings", Type: types.MapType{ElemType: types.StringType}, Description: "Custom credential-sharing groups."})
	// The reference does not enumerate every accepted action. Let the API validate
	// values rather than rejecting future actions at the provider boundary.
	actions := []string{}
	ruleFields := []field{immutable(str("name", true)), str("regex", true), defaulted(number("score", false), 0.5), defaulted(stringsField("context_keywords", false), []any{}), retained(defaulted(enum(str("inbound_action", false), actions...), "REDACT")), defaulted(enum(str("outbound_action", false), actions...), "REDACT")}
	std := []field{immutable(local(str("entity_type", true))), boolean("is_active", true), retained(enum(str("inbound_action", true), actions...)), enum(str("outbound_action", true), actions...), computed("description", types.StringType), computed("detection_method", types.StringType), computed("region", types.StringType), computed("defined_at", types.StringType)}
	override := []field{immutable(str("connector_slug", true)), immutable(str("tool_name", true)), allowEmpty(str("description", true)), computed("defined_at", types.StringType)}
	connector := object("connector", true, str("slug", true), defaulted(enum(str("auth_scope", false), "INDIVIDUAL", "SHARED", "ORGANIZATION"), "INDIVIDUAL"), stringsField("tool_names", false))
	connectors := field{Name: "connectors", Type: types.ListType{ElemType: connector.Type}, Required: true, Children: connector.Children, Description: "Authoritative connector list. Omit tool_names to enable all tools; an empty tool_names list removes the connector. Do not manage this list with another resource."}
	return []resourceSpec{
		{Name: "tool_pack", Description: "Manage a Tool Pack. Manage connector membership separately with mergeah_tool_pack_connectors or individual mergeah_tool_pack_connector resources.", Collection: "tool-packs/", Fields: []field{str("name", true), allowEmpty(str("description", true))}},
		{Name: "registered_user", Description: "Manage a Registered User. Creation is idempotent by origin_user_id; import existing users to avoid taking ownership unintentionally.", Collection: "registered-users/", Fields: []field{immutable(str("origin_user_id", true)), str("origin_user_name", true), group, defaulted(enum(str("user_type", false), "HUMAN", "SYSTEM"), "HUMAN"), computed("authenticated_connectors", types.SetType{ElemType: types.StringType}), stable(computed("is_test", types.BoolType))}},
		{Name: "application_credential", Description: "Manage an OAuth application credential. client_secret cannot be read back and is retained in sensitive Terraform state.", Collection: "application-credentials/", Fields: append([]field{immutable(str("connector_slug", true)), str("client_id", true), secret(str("client_secret", false)), str("external_id", false), retained(field{Name: "oauth2_scopes_override", Type: types.StringType, Description: "Space-separated OAuth scope override. Retains the configured string when Merge returns null or omits this field, including normalization to connector defaults. Remote clearing cannot then be distinguished from normalization; different non-null scopes are detected as drift."}), computed("is_global_default", types.BoolType)}, timestamps()...)},
		{Name: "access_key", Description: "Manage a Merge access key. Changes replace the key. The secret is returned only on creation and is stored in sensitive Terraform state; import cannot recover it. Omitted expires_at uses the API default (90 days). Set no_expiration to true to explicitly request no expiration.", Collection: "access-keys/", DeleteMethod: "POST", Kind: "access_key", Fields: []field{immutable(optionalComputed(str("name", false))), immutable(stringsField("tool_pack_ids", false)), immutable(stringsField("registered_user_ids", false)), immutable(optionalComputed(stringsField("scopes", false))), immutable(defaulted(boolean("is_test", false), false)), immutable(optionalComputed(str("expires_at", false))), immutable(local(defaulted(boolean("no_expiration", false), false))), stable(secret(computed("key", types.StringType))), computed("key_masked", types.StringType), computed("created_at", types.StringType)}},
		{Name: "remote_mcp_connector", Description: "Register a remote MCP connector. Changes replace the connector. Headers and auth are not returned by Merge and cannot be checked for drift. Import ID is the connector slug.", Collection: "connectors/", Kind: "remote", Keys: []string{"slug"}, Fields: []field{immutable(str("name", true)), immutable(retained(str("url", true))), immutable(optionalComputed(field{Name: "slug", Type: types.StringType, Description: "Connector slug; derived from its name when omitted."})), immutable(secret(field{Name: "headers", Type: types.MapType{ElemType: types.StringType}, Description: "Additional headers sent to the remote server; not returned by Merge."})), immutable(secret(object("auth", false, enum(str("type", true), "none", "secret"), object("header", false, str("key", true), secret(str("value", true)))))), computed("description", types.StringType), computed("source_url", types.StringType)}},
		{Name: "tool_pack_connectors", Description: "Manage the entire connector membership of a Tool Pack. Destroy detaches all connectors from that Tool Pack. Do not combine with mergeah_tool_pack_connector resources for the same Tool Pack; their ownership conflicts.", Collection: "tool-packs/{tool_pack_id}/connectors/", CreateMethod: "PUT", UpdateMethod: "PUT", Kind: "connectors", Keys: []string{"tool_pack_id"}, Fields: []field{immutable(local(str("tool_pack_id", true))), connectors}},
		{Name: "tool_pack_connector", Description: "Manage one connector membership in a Tool Pack without changing other connectors. Destroy detaches only this connector. Do not combine with mergeah_tool_pack_connectors for the same Tool Pack or manage the same membership in multiple resources. Import existing membership using tool_pack_id/slug before managing it.", Collection: "tool-packs/{tool_pack_id}/connectors/", CreateMethod: "PATCH", UpdateMethod: "PATCH", Kind: "connector_member", Keys: []string{"tool_pack_id", "slug"}, Fields: []field{
			immutable(local(str("tool_pack_id", true))), immutable(str("slug", true)),
			optionalComputed(enum(field{Name: "auth_scope", Type: types.StringType, Description: "Credential-sharing scope. When omitted, uses Merge's default for new membership and preserves the scope of existing membership."}, "INDIVIDUAL", "SHARED", "ORGANIZATION")),
			optionalComputed(field{Name: "tool_names", Type: types.SetType{ElemType: types.StringType}, Description: "Exact tool selection when configured; must not be empty. When omitted, new membership enables all tools and existing membership keeps its selection. State records the returned tool names. Removing this attribute from configuration does not reset the selection to all tools; specify the desired names explicitly."}),
		}},
		{Name: "connector_tool_description_override", Description: "Manage one organization-level tool description override; destroy removes this override.", Collection: "connectors/tool-description-overrides/", CreateMethod: "PATCH", UpdateMethod: "PATCH", Kind: "override", Keys: []string{"connector_slug", "tool_name"}, Fields: override},
		{Name: "tool_pack_tool_description_override", Description: "Manage one Tool Pack tool description override; destroy restores inheritance.", Collection: "tool-packs/{tool_pack_id}/tool-description-overrides/", CreateMethod: "PATCH", UpdateMethod: "PATCH", Kind: "override", Keys: []string{"tool_pack_id", "connector_slug", "tool_name"}, Fields: append([]field{immutable(local(str("tool_pack_id", true)))}, override...)},
		{Name: "custom_regex_rule", Description: "Manage an organization custom regex rule. inbound_action is retained when absent from GET responses; remote drift for that field cannot then be detected.", Collection: "security/custom-regex-rules/", Fields: append(ruleFields, createOmit(optionalComputed(boolean("is_active", false))), computed("defined_at", types.StringType))},
		{Name: "tool_pack_custom_regex_rule", Description: "Manage a Tool Pack custom regex rule. inbound_action is retained when absent from GET responses. A name matching an organization rule creates an override.", Collection: "tool-packs/{tool_pack_id}/custom-regex-rules/", Keys: []string{"tool_pack_id", "id"}, Fields: append([]field{immutable(local(str("tool_pack_id", true))), immutable(str("name", true)), str("regex", true), number("score", true), stringsField("context_keywords", true), boolean("is_active", true), retained(enum(str("inbound_action", true), actions...)), enum(str("outbound_action", true), actions...), computed("defined_at", types.StringType)}, timestamps()...)},
		{Name: "standard_entity_rule", Description: "Manage settings on an existing organization standard entity rule. Destroy stops management and leaves settings unchanged. inbound_action is retained when absent from GET responses.", Collection: "security/standard-entity-rules/", CreateMethod: "PUT", UpdateMethod: "PUT", Kind: "standard", Keys: []string{"entity_type"}, Fields: std},
		{Name: "tool_pack_standard_entity_rule", Description: "Manage a Tool Pack standard entity rule override. Destroy deletes the override and restores organization inheritance. inbound_action is retained when absent from GET responses.", Collection: "tool-packs/{tool_pack_id}/standard-entity-rules/", CreateMethod: "PUT", UpdateMethod: "PUT", Kind: "standard_override", Keys: []string{"tool_pack_id", "entity_type"}, Fields: append([]field{immutable(local(str("tool_pack_id", true)))}, std...)},
		{Name: "connection_access", Description: "Manage exclusions on an existing SHARED or ORGANIZATION connection. Destroy leaves exclusions unchanged and stops management. It does not revoke the connection.", Collection: "connections/", CreateMethod: "PATCH", UpdateMethod: "PATCH", Kind: "connection_access", Keys: []string{"connection_id"}, Fields: []field{immutable(local(str("connection_id", true))), stringsField("excluded_registered_users", true)}},
	}
}

type dataSpec struct {
	Name, Description, Endpoint string
	Fields                      []field
	List                        bool
}

func dataSpecs() []dataSpec {
	out := []dataSpec{}
	add := func(name, endpoint string, list bool, fields ...field) {
		out = append(out, dataSpec{Name: name, Endpoint: endpoint, List: list, Fields: fields, Description: "Read Merge Agent Handler " + name + ". Results are available as JSON via response_json; paginated collections are fetched completely."})
	}
	add("connector", "connectors/{slug}/", false, str("slug", true), boolean("include_tool_details", false), str("auth_mode", false))
	add("connectors", "connectors/", true, str("search", false))
	for _, pair := range []struct{ single, plural, endpoint string }{{"tool_pack", "tool_packs", "tool-packs/"}, {"registered_user", "registered_users", "registered-users/"}, {"application_credential", "application_credentials", "application-credentials/"}, {"access_key", "access_keys", "access-keys/"}, {"connection", "connections", "connections/"}, {"custom_regex_rule", "custom_regex_rules", "security/custom-regex-rules/"}} {
		add(pair.single, pair.endpoint+"{resource_id}/", false, str("resource_id", true))
		add(pair.plural, pair.endpoint, true)
	}
	add("tool_pack_connectors", "tool-packs/{tool_pack_id}/connectors/", true, str("tool_pack_id", true))
	add("connector_tool_description_overrides", "connectors/tool-description-overrides/", true, str("connector_slug", false))
	add("tool_pack_tool_description_overrides", "tool-packs/{tool_pack_id}/tool-description-overrides/", true, str("tool_pack_id", true), str("connector_slug", false))
	add("standard_entity_rule", "security/standard-entity-rules/{entity_type}/", false, str("entity_type", true))
	add("standard_entity_rules", "security/standard-entity-rules/", true)
	add("tool_pack_standard_entity_rule", "tool-packs/{tool_pack_id}/standard-entity-rules/{entity_type}/", false, str("tool_pack_id", true), str("entity_type", true))
	add("tool_pack_standard_entity_rules", "tool-packs/{tool_pack_id}/standard-entity-rules/", true, str("tool_pack_id", true))
	add("tool_pack_custom_regex_rule", "tool-packs/{tool_pack_id}/custom-regex-rules/{resource_id}/", false, str("tool_pack_id", true), str("resource_id", true))
	add("tool_pack_custom_regex_rules", "tool-packs/{tool_pack_id}/custom-regex-rules/", true, str("tool_pack_id", true))
	add("audit_log", "audit-log/", true, str("created_after", false), str("created_before", false), str("event_type", false), str("user_id", false))
	add("api_request_logs", "logs/api-calls/", true, str("connector_id", false), str("created_after", false), str("created_before", false), str("method", false), str("response_status", false), str("toolcall_id", false))
	add("tool_call_logs", "logs/tool-calls/", true, str("connector_id", false), str("created_after", false), str("created_before", false), str("status", false), str("tool_name", false), str("client_trace_id", false), str("error_reason", false), str("turn_id", false))
	add("usage", "usage/", false, str("start_time", true), str("end_time", true))
	add("registered_user_usage", "usage/registered-users/{registered_user_id}/", false, str("registered_user_id", true), str("start_time", true), str("end_time", true))
	add("usage_by_registered_user", "usage/registered-users/", true, str("start_time", true), str("end_time", true))
	add("usage_by_tool", "usage/tools/", true, str("start_time", true), str("end_time", true))
	return out
}
