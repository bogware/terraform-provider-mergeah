# Repository Guidelines

## Project Structure & Module Organization

The Go module is `github.com/bogware/terraform-provider-mergeah`, with Terraform source address `bogware/mergeah`. The provider uses Terraform Plugin Framework v1.19.0 and requires Go 1.25.8 or newer. No Git repository has been initialized by the agent.

`main.go` serves the provider. `internal/client/` contains HTTP transport and pagination; `internal/provider/` contains configuration, declarative API/schema specifications, lifecycle handling, data sources, and colocated tests. `examples/` contains Terraform examples and import commands; `docs/` contains generated reference pages and lifecycle guidance. `cmd/` contains example/schema generators and the API coverage checker, and the separate `tools/` Go module pins tfplugindocs. `.github/workflows/` contains CI, the weekly API coverage check, and the release workflow; `.goreleaser.yml` configures signed release artifacts for the Terraform Registry.

Releases: pushing a `v*` tag runs `.github/workflows/release.yml`, which builds and GPG-signs binaries with GoReleaser v2 and publishes a GitHub release that the Registry picks up through its webhook. It needs the `GPG_PRIVATE_KEY` and `PASSPHRASE` repository secrets. Update `CHANGELOG.md` and run `make api-coverage` before tagging. Tag or publish only when the user asks.

## Build, Test, and Development Commands

Development commands:

- `go build ./...`: compile all packages.
- `go test ./...`: run package tests.
- `go vet ./...`: check for common correctness issues.
- `gofmt -w .`: format Go source files.
- `go run ./cmd/check-api-coverage` (or `make api-coverage`): compare provider API usage with Merge's published OpenAPI spec. It needs network access, so it is not part of `go test`; `.github/workflows/api-coverage.yml` runs it every Monday (and on manual dispatch) and opens, updates, or closes one tracking issue titled "API coverage gaps against the Merge OpenAPI spec". Run it before releases; exit status 1 lists uncovered operations, unexposed filters or body fields, and stale exclusions. Record deliberate omissions with a reason in `cmd/check-api-coverage/main.go`, and keep `internal/provider/coverage.go` in sync with request logic in `resource.go` and `data_source.go`.

Terraform CLI 1.13 or newer is required for fixture lifecycle tests. Generate examples and docs from the root with `make generate`, or run `go generate -tags generate .` inside `tools/`. Generation exports protocol schemas directly to avoid temporary provider-installation failures on Windows. Build a local plugin with `go build -o bin/terraform-provider-mergeah.exe .`; README.md describes dev_overrides. If Windows denies access to the shared Go build cache, use `$env:GOCACHE = "$PWD\.cache\go-build"`. GNUmakefile supplies build, test, vet, and generate targets.

## Coding Style & Naming Conventions

For new Go code, use standard `gofmt` formatting, including tabs for indentation. Use descriptive package names, exported `MixedCaps` identifiers, and unexported `mixedCaps` identifiers. Use lowercase underscore-separated names for Terraform resources and attributes. Keep resource behavior and schema descriptions consistent.

## Testing Guidelines

Use Go's `testing` package and name tests `TestBehavior` in `*_test.go` files. Tests cover protocol schemas, HTTP behavior, pagination, secret retention, imports, and lifecycle behavior through the real Terraform CLI with local HTTP fixtures. They require no Merge credentials and perform no live Merge operations. No coverage threshold is configured. Keep future live tests separate; document credentials, costs, and cleanup before enabling them. Never change the default fixture tests to use a live API.

`internal/acceptance/` contains separate live tests guarded by the `liveacceptance` build tag, `TF_ACC=1`, and `MERGEAH_LIVE_TESTS=1`. CI compiles them with `-run '^$'` and never executes them. Credentials, effects, limits, and cleanup are documented in `templates/guides/live-testing.md.tmpl` (rendered to `docs/guides/live-testing.md`). Do not enable live execution unless the user authorizes a run with a dedicated test key.

## Commit & Pull Request Guidelines

No Git history is available to establish a commit convention. Use concise, imperative commit subjects such as `Add provider configuration schema`. Pull requests should describe the change, link relevant issues, and list validation commands and results. Include configuration examples for user-facing changes.

## Security & Configuration

Never commit API tokens, credentials, Terraform state, or local `.tfvars` files containing secrets. Use environment variables for credentials and sanitized fixtures for tests.
