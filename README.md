# Terraform Provider Mergeah

Terraform Plugin Framework provider for the [Merge Agent Handler management API](https://docs.merge.dev/merge-agent-handler/agent-handler).

Provider source: `bogware/mergeah`. Go module: `github.com/bogware/terraform-provider-mergeah`.

## Configuration

```hcl
terraform {
  required_providers {
    mergeah = {
      source = "bogware/mergeah"
    }
  }
}

# Set MERGEAH_API_KEY in the environment.
provider "mergeah" {
  api_url = "https://ah-api.merge.dev/api/v1"
}

resource "mergeah_tool_pack" "support" {
  name        = "Support"
  description = "Tools available to the support agent"
}

resource "mergeah_tool_pack_connectors" "support" {
  tool_pack_id = mergeah_tool_pack.support.id
  connectors = [{
    slug       = "linear"
    auth_scope = "INDIVIDUAL"
    tool_names = ["list_issues"]
  }]
}

data "mergeah_connector" "linear" {
  slug                 = "linear"
  include_tool_details = true
}

locals {
  linear_connector = jsondecode(data.mergeah_connector.linear.response_json)
}
```

The access key needs management permissions. `api_key` overrides `MERGEAH_API_KEY`; `api_url` overrides `MERGEAH_API_URL`. EU deployments use `https://ah-api-eu.merge.dev/api/v1`. Test and production keys address separate environments.

API URLs must use HTTPS. HTTP is accepted only for literal loopback IP addresses, such as `127.0.0.1` or `::1`, for local fixtures.

The provider has not been published. Use a local development override until a signed release is available.

## Coverage

Fourteen resources cover Tool Packs, authoritative and individual connector membership, Registered Users, OAuth application credentials, access keys, remote MCP connectors, connector and Tool Pack tool description overrides, organization and Tool Pack custom regex rules, organization and Tool Pack standard entity settings, and connection exclusions. Thirty-two data sources cover detail/list endpoints, connector discovery, tool access requests, security rules, usage, audit events, and API/tool logs. See [API coverage and lifecycle behavior](docs/guides/coverage.md).

To check coverage against Merge's published OpenAPI spec, run `go run ./cmd/check-api-coverage` (add `-v` to list every operation, or `-spec <file>` to use a saved copy). It exits with status 1 if the spec has operations, filters, or request fields the provider neither supports nor deliberately excludes.

Use `mergeah_tool_pack_connector` when separate resources or modules should own individual connectors. Each resource manages one `tool_pack_id`/`slug` pair and detaches only that connector on destroy. Do not use it alongside `mergeah_tool_pack_connectors` for the same Tool Pack, because the latter owns the entire list. For example, as an alternative to the membership resource above:

```hcl
resource "mergeah_tool_pack_connector" "linear" {
  tool_pack_id = mergeah_tool_pack.support.id
  slug         = "linear"
  auth_scope   = "INDIVIDUAL"
  tool_names   = ["list_issues"]
}
```

Import existing membership with `terraform import mergeah_tool_pack_connector.linear 'TOOL_PACK_ID/linear'`. An omitted `tool_names` enables all tools for new membership and preserves the selection for existing membership. Removing `tool_names` from configuration does not reset the selection; provide the desired tool names explicitly. An empty selection is rejected because Merge interprets it as removal.

Data sources expose API records through sensitive `response_json`. Use `jsondecode` to access their fields. Collections return a JSON array with all pages aggregated; `query` accepts additional documented API filters. Enterprise log endpoints require the corresponding Merge plan.

## Development

Requires Go 1.25.8 or newer and Terraform CLI 1.13 or newer. The module follows the supplied HashiCorp framework scaffolding and uses Terraform Plugin Framework v1.19.0.

```powershell
go build ./...
go test ./...
go vet ./...
gofmt -w main.go internal
Push-Location tools
go generate -tags generate .
Pop-Location
```

Default tests use local HTTP fixtures, including Terraform CLI lifecycle tests. They require no Merge credentials, make no live Merge changes, and incur no Merge costs. The test harness uses the installed Terraform CLI; set `TF_ACC_TERRAFORM_PATH` if it is not on PATH. A separate, opt-in [live acceptance suite](docs/guides/live-testing.md) is prepared for a dedicated test key; it is excluded from default builds and CI. It has not been run against Merge.

If Windows denies access to an older Go build cache, set a project-local cache before running Go commands:

```powershell
$env:GOCACHE = "$PWD\.cache\go-build"
```

Build the local plugin:

```powershell
go build -o bin/terraform-provider-mergeah.exe .
```

Set `TF_CLI_CONFIG_FILE` to an absolute path to a local CLI configuration file containing:

```hcl
provider_installation {
  dev_overrides {
    "bogware/mergeah" = "C:/BogWare/terraform-provider-mergeah/bin"
  }
  direct {}
}
```

Run `terraform plan` and `terraform apply` directly with development overrides. `terraform init` attempts registry installation and is unnecessary for this provider during local development. The directory in `dev_overrides` must contain the executable built above.

## State and lifecycle

Secrets are marked sensitive but are still stored in Terraform state. Protect the state backend. Never commit API keys, `.tfvars`, or state files. Secret fields that Merge does not return cannot be checked for remote drift. Imported access keys cannot recover their original secrets.

Destroying organization standard entity settings or connection exclusions leaves the remote settings unchanged, as requested. Destroying a Tool Pack standard entity override restores inheritance; destroying authoritative connector membership detaches all connectors. Access key and remote connector changes require replacement. Regex rule names are immutable.

`inbound_action` is retained from configuration when Merge omits it in read responses. Import cannot recover that field, and drift cannot be detected until the API returns it. Import existing Registered Users rather than relying on the create endpoint's idempotent adoption behavior.

For application credentials, `oauth2_scopes_override` retains its configured string when Merge returns null or omits the field. Merge normalizes overrides equal to connector defaults to null, so Terraform cannot distinguish this from someone clearing the override remotely. A different non-null scope string is still detected as drift. Removing the override in configuration sends null on update; an empty string also requests clearing according to the API.

Writes are never automatically retried because Merge provides no general idempotency key. Read requests retry rate limits and server errors with bounded backoff, honoring `Retry-After`. Pagination links must remain within the configured API base, and redirects are rejected to avoid forwarding authentication unexpectedly.
