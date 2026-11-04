## 0.1.0 (November 3, 2026)

Initial release of the Merge Agent Handler provider. Fixture tests cover every resource and data source; live validation against the Merge API is still in progress, so behavior may change before 1.0.

FEATURES:

* **New Resource:** `mergeah_tool_pack`
* **New Resource:** `mergeah_tool_pack_connectors` (authoritative connector membership)
* **New Resource:** `mergeah_tool_pack_connector` (individual connector membership)
* **New Resource:** `mergeah_registered_user`
* **New Resource:** `mergeah_application_credential`
* **New Resource:** `mergeah_access_key`
* **New Resource:** `mergeah_remote_mcp_connector`
* **New Resource:** `mergeah_connector_tool_description_override`
* **New Resource:** `mergeah_tool_pack_tool_description_override`
* **New Resource:** `mergeah_custom_regex_rule`
* **New Resource:** `mergeah_tool_pack_custom_regex_rule`
* **New Resource:** `mergeah_standard_entity_rule`
* **New Resource:** `mergeah_tool_pack_standard_entity_rule`
* **New Resource:** `mergeah_connection_access`
* **New Data Sources:** 32 read-only data sources covering connectors, Tool Packs and their membership, Registered Users, application credentials, access keys, connections, tool description overrides, security rules, tool access requests, usage, audit events, and API/tool call logs. Collection data sources fetch every page, and documented filters are available as named attributes.
