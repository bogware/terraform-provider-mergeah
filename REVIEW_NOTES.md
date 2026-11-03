# Code review follow-up

Reviewed the ten supplied findings against implementation, existing tests, and the relevant API/framework documentation. All verification uses local HTTP fixtures; no live Merge operations were performed.

| Finding | Resolution |
| --- | --- |
| Created objects lost when response fields fail to decode | Every error path after assigning the ID returns the accumulated values. Decoding continues across independent fields so a malformed field before an access-key secret does not discard that secret. `Create` saves the ID and successfully decoded values while reporting the error. Regression tests cover keys, users, Tool Packs, and follow-up response errors. |
| `is_active = false` overwritten before follow-up PATCH | Build the follow-up body from plan values before merging any create response. The lifecycle fixture now creates a disabled regex rule despite the API's enabled default; the failure-path fixture also checks that PATCH sends false. |
| Override updates omit identifiers | Both override resources include `connector_slug` and `tool_name` on every PATCH. Their lifecycle fixture rejects unidentified items before modifying stored data. |
| Unencrypted remote bearer-token requests | Require HTTPS. Allow HTTP only for literal loopback IP addresses used by fixtures, never DNS hostnames or non-loopback addresses. Test both accepted and rejected URLs. |
| Retry-After integer overflow | Bound seconds before duration multiplication. Values beyond uint64 also trigger the existing over-60-second rejection. Tests cover boundaries, overflow, HTTP dates, and request counts. |
| Hard-coded `description` validation exception | Replace the attribute-name check with explicit `AllowEmpty` metadata on the intended fields. Tests verify the rule follows metadata rather than the name. |
| List/Float64 schema flags ignored | Propagate `Sensitive` and `Replace` for scalar lists and Float64 attributes. Tests verify sensitive metadata and invoke the replacement modifiers. Nested lists already supported both. |
| Computed values produce unknown plans | Apply `UseStateForUnknown` selectively through explicit `Stable` metadata: ID, creation timestamps in timestamp-bearing updateable resources, Registered User test-environment flag, and the retained one-time key. Keep mutable/unverified values unknown, including `modified_at`, `description`, `detection_method`, and `authenticated_connectors`. Tests cover stable planning and an application credential whose modification timestamp changes on update. |
| Null slugs counted as duplicates | Skip null/unknown slugs and null objects in resource-level duplicate detection; the framework handles required fields. Regression test includes null objects and repeated null slugs. |
| Redundant validation and unused bounds | Remove the redundant OptionalComputed/unknown branch and unused Min/Max fields. No existing spec configured these bounds. |

## Recovery limit

An ID cannot be saved if a lost or undecodable response never provides a recoverable identifier. The provider does not guess ownership or automatically retry writes. Diagnostics and the coverage guide instruct the operator to reconcile existing remote objects and import them before retrying. If a one-time key was not received or saved, revoke the remote key before creating a replacement. Live behavior remains to be verified with a dedicated test key.

## Validation

Validation passed: full `go test ./... -timeout 15m` (provider package 323.077 seconds), `go build ./...`, local Windows binary build, `go vet ./...`, compile-only live-suite check, tagged vet, documentation/example generation, Terraform example formatting, and Go formatting. The focused regression and affected lifecycle tests also passed before the full run.

## Sources

Override identifiers are required by both [organization override PATCH](https://docs.merge.dev/merge-agent-handler/agent-handler/tool-description-overrides/bulk-upsert-delete) and [Tool Pack override PATCH](https://docs.merge.dev/merge-agent-handler/agent-handler/tool-description-overrides/bulk-upsert-delete-tool-pack).

[HashiCorp's plan modification guidance](https://developer.hashicorp.com/terraform/plugin/framework/resources/plan-modification) limits `UseStateForUnknown` to values known to remain unchanged after an update; applying it indiscriminately can violate plan/state consistency.
