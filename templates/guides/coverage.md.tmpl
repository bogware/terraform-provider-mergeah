---
page_title: "Management API coverage and lifecycle"
description: "Management API coverage, ownership, and drift limitations."
---

# Management API coverage and lifecycle

The provider manages the Merge Agent Handler management REST API. It includes 14 resources and 30 data sources; resource and data source pages contain schemas, examples, and import instructions.

Managed families include Tool Packs and connector membership, Registered Users, OAuth application credentials, access keys, remote MCP registrations, connection exclusions, tool description overrides, and standard entity/custom regex rules at organization and Tool Pack scope. Read-only data sources also expose connector discovery, usage, audit events, and API/tool logs.

Data sources expose sensitive `response_json`. Detail endpoints return objects; collection endpoints return arrays aggregated across all pages. Collection envelope metadata is not retained. Use `jsondecode` to access fields and the `query` map for additional documented filters. Log endpoints require the corresponding Merge plan.

## Ownership and destroy

- Tool Pack resources own name and description. A `mergeah_tool_pack_connectors` resource owns the entire membership list; destroy detaches all connectors. Omit a connector's `tool_names` to enable all tools. Remove the connector from the list to detach it; empty tool selections are rejected.
- A `mergeah_tool_pack_connector` resource owns one membership, identified and imported as `tool_pack_id/slug`. It uses partial updates and destroys only that membership, leaving other connectors untouched. Different modules may manage different memberships in the same Tool Pack. Never combine individual membership resources with `mergeah_tool_pack_connectors` for the same Tool Pack, and never manage the same pair in multiple resources or states. The provider cannot detect conflicting ownership across separate Terraform states.
- For individual membership, omitted `auth_scope` and `tool_names` use Merge defaults when attaching a new connector and preserve existing settings when the membership already exists. Returned values are recorded in state. Explicit tool selections and scopes are checked for drift; omitted selections accept remote changes. Removing `tool_names` from configuration does not reset all tools. Specify the desired names explicitly; an empty set is rejected because it would remove the membership. Import existing membership before managing it. Changes to `tool_pack_id` or `slug` require replacement.
- Access key changes require replacement. Destroy revokes the key. Use `create_before_destroy` if continuity is required; Merge prevents revocation of the last unrestricted production key. Omitted expiration uses Merge's default; `no_expiration = true` explicitly requests no expiration.
- Remote MCP connector changes require replacement because the API has no update endpoint. Destroy removes the registration and stored remote credentials, which can affect dependent Tool Packs.
- Registered User creation is idempotent by origin ID and can return an existing user unchanged. Import existing users explicitly rather than using creation as a lookup.
- Regex rule names require replacement. An organization rule's `is_active` setting is applied through a follow-up PATCH when configured during creation.
- Destroying organization standard entity settings leaves their remote settings unchanged. Destroying Tool Pack standard entity overrides restores organization inheritance.
- Connection access resources manage exclusions on existing connections. Destroy leaves exclusions unchanged; it does not revoke the connection or initiate OAuth.
- A tool description resource owns one connector/tool pair. Destroy removes only that override.

## Retained values and drift

Merge omits OAuth client secrets and remote authentication/header values on reads. The provider retains their prior state; imports cannot recover those values.

Security rule `inbound_action` is retained when absent from read responses. Drift for that field cannot be detected until Merge returns it. Imported security rules need their intended inbound action supplied in configuration.

For application credentials, `oauth2_scopes_override` retains the configured string when Merge returns null or omits the field. Merge normalizes overrides equal to connector defaults to null. Terraform therefore cannot distinguish normalization from someone clearing the override remotely. A different non-null scope string is still detected as drift. Removing the override in configuration sends null on update; an empty string also requests clearing according to the API.

Secrets marked sensitive still reside in Terraform state. Protect the backend and never commit keys, state, or secret variable files.

## Recovering from a failed create

When Merge returns an identifier but another response field cannot be decoded, the provider saves that identifier and all successfully decoded fields, including any one-time access-key secret. This lets Terraform track the created object even though the operation reports an error. Follow-up update failures also retain the created object in state.

A lost response, invalid JSON, or missing identifier can prevent recovery. Before applying again, check Merge for an object created by the failed request and import it if present. If an access-key secret was not received or could not be saved, revoke that key before creating a replacement; import cannot recover the secret. The provider never automatically retries writes.

## Excluded operations and validation

Short-lived Link tokens, signup/claim-link issuance, file transfer, MCP tool execution/search, credential deletion, connection revocation, and access-key regeneration are runtime operations without independent declarative lifecycles. They are not stateful resources or read-only management data sources. Rotate managed keys through resource replacement. Establish user connections through Merge Link or external OAuth workflows.

Tests use sanitized local HTTP fixtures and the real Terraform CLI. They require no Merge credentials or live operations. No live Merge account has been validated. Before production use, validate with a dedicated test key and disposable resources; remote MCP registration contacts the remote server. Review cleanup requirements, including settings deliberately retained on destroy.

The client supports both page and cursor pagination. It rejects looping pagination and links outside the configured API base. Writes are not automatically retried because the API has no general idempotency-key guarantee.

API URLs require HTTPS, except literal loopback IP addresses for local fixtures. Read retries reject Retry-After delays above 60 seconds, including oversized numeric values, rather than overflowing or retrying immediately.
