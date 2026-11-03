---
page_title: "Live acceptance testing"
description: "Opt-in tests for disposable Merge test resources."
---

# Live acceptance testing

The default tests use local HTTP fixtures. Live acceptance tests are separate under `internal/acceptance/` and excluded unless built with the `liveacceptance` tag. No live test is run by the default CI workflow.

## Prerequisites and effects

Use a dedicated Merge test access key with management permissions and a disposable test account/environment. Supply it through `MERGEAH_API_KEY`; do not put it in a Terraform file, command history, or source control. Test and production keys address different users. The suite checks that the created Registered User reports `is_test = true`, but this check occurs after creation; it is not a substitute for choosing a test key.

The initial live suite creates one uniquely named Tool Pack and one Registered User, reads both through data sources, verifies imports, updates both, checks for a stable subsequent plan, and destroys them. It then independently checks their IDs return 404. Names/origin IDs start with `mergeah-acceptance-` and a random suffix, which is printed for manual cleanup.

These are real management API requests and consume API rate limits. They do not execute tools, establish connector credentials, create remote MCP servers, or deliberately incur tool-call credits. Account limits and any applicable Merge billing still apply. No existing organization security settings or connection exclusions are modified.

The suite does not yet provide live coverage for every resource. OAuth credentials, keys, remote servers, memberships, descriptions, and security rules still require later validation with suitable prerequisites. Local fixture tests cover their implementations in the meantime.

## Compile and check the disabled state without a key

```powershell
go test -tags liveacceptance ./internal/acceptance -run '^$'
go test -v -tags liveacceptance ./internal/acceptance
```

The first command compiles without executing tests. The second skips the live test unless both execution flags below are enabled. Do not enable those flags while only checking compilation.

## Run when the test key is available

Configure `MERGEAH_API_KEY` through your environment or secret manager. For EU, also set `MERGEAH_API_URL` to `https://ah-api-eu.merge.dev/api/v1`; otherwise the US base URL is used.

```powershell
$env:TF_ACC = '1'
$env:MERGEAH_LIVE_TESTS = '1'

try {
  go test -v -tags liveacceptance ./internal/acceptance -run '^TestLive' -count=1 -timeout 20m
} finally {
  Remove-Item Env:TF_ACC -ErrorAction SilentlyContinue
  Remove-Item Env:MERGEAH_LIVE_TESTS -ErrorAction SilentlyContinue
}
```

Remove `MERGEAH_API_KEY` from the session afterward if it was set temporarily. Avoid `TF_LOG` or other detailed HTTP/state logging when using live credentials.

## Cleanup and release readiness

Allow the harness to finish its destroy step. Interrupting the process, losing connectivity, or reaching the overall Go timeout can prevent cleanup. On failure, inspect the Merge dashboard for the printed resource name/origin ID and remove only the resources created by that run. Cleanup diagnostics include any captured IDs that could not be confirmed deleted. Treat test output and temporary Terraform state as sensitive.

A passing initial suite is a smoke test, not complete production validation. Record the tested environment, date, and resource coverage without saving keys. Validate the remaining resource families before describing a release as live-tested across the entire API surface. Publishing remains a separate explicit action.
