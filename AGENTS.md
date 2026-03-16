# AGENTS.md

This file gives repository-specific guidance to coding agents working in `strimzi-shutdown`.
It is intentionally practical and biased toward the conventions already present in this codebase.

## Repository Snapshot

- Language: Go
- Module: `github.com/scholzj/strimzi-shutdown`
- Go version: `1.25.0`
- Dependency baseline: `github.com/scholzj/strimzi-go v0.8.0`, Kubernetes modules `v0.35.2`
- App type: small Cobra-based CLI for stopping and restarting Strimzi Kafka clusters
- Main packages:
  - `main.go` wires the CLI entrypoint
  - `cmd/` contains Cobra commands and shared Kubernetes helpers
- CI currently runs tests and a basic build only; there is no dedicated lint workflow

## Existing Agent Rules

- No repository-local Cursor rules were found in `.cursor/rules/`
- No `.cursorrules` file was found
- No Copilot instructions were found in `.github/copilot-instructions.md`
- Do not invent hidden policy files; rely on this document and the existing codebase

## Important Working Assumptions

- This is a CLI utility, not a long-running controller
- Behavior is user-facing through command output, so preserve clear log messages
- The project interacts with Kubernetes and Strimzi APIs directly through typed clients
- Backward compatibility matters for command names and existing flags
- The repository is small, so consistency with current files matters more than introducing frameworks

## Build Commands

- Build the default binary in the repo root:

```bash
go build
```

- Preferred local workflow via `Makefile`:

```bash
make build
```

- `make all` runs `build` first and `test` second

- `make build` runs formatting, vet, module tidy, and a verbose build:

```bash
gofmt -w main.go cmd/*.go
go vet -v ./...
go mod tidy -v
go build -v -o strimzi-shutdown
```

- Build all packages:

```bash
go build ./...
```

- Build a static-style binary like CI does for PR validation:

```bash
CGO_ENABLED=0 go build
```

- Build a named local binary:

```bash
go build -o strimzi-shutdown
```

- Cross-compile one target the same way the release workflow does:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o strimzi-shutdown-linux-amd64
```

## Test Commands

- Run the full test suite:

```bash
go test ./...
```

- Preferred local test command via `Makefile`:

```bash
make test
```

- Run verbose tests, matching CI more closely:

```bash
go test -v ./...
```

- Run verbose tests with CGO disabled, matching PR CI exactly:

```bash
CGO_ENABLED=0 go test -v ./...
```

- Current state: there are no `*_test.go` files in the repository yet, so `go test ./...` reports `[no test files]`

## Running A Single Test

- Run one test by exact name in one package:

```bash
go test ./cmd -run '^TestName$'
```

- Run one subtest:

```bash
go test ./cmd -run '^TestParent$/^subcase$'
```

- Run one test across all packages when you are not sure where it lives:

```bash
go test ./... -run '^TestName$'
```

- Run one test with verbose output:

```bash
go test -v ./cmd -run '^TestName$'
```

- If you add tests for Kubernetes-facing helpers, prefer package-scoped runs first, then `go test ./...`

## Lint And Formatting Commands

- There is no checked-in lint config such as `.golangci.yml`
- A basic `Makefile` exists for common local workflows
- Minimum local hygiene before finishing changes:

```bash
gofmt -w main.go cmd/*.go
```

```bash
go vet ./...
```

- Tidy module metadata when dependencies or imports change:

```bash
go mod tidy -v
```

- `make build` already runs `gofmt`, `go vet -v`, `go mod tidy -v`, and `go build -v`
- `make clean` runs `go clean -x`, `go clean -testcache -x`, and removes local binaries

- If you touched imports heavily, use `gofmt` and rely on Go's default import grouping
- Do not introduce a new lint tool unless the user explicitly asks for it

## CI Behavior To Match

- `.github/workflows/pr.yaml` runs on Go `1.25.x` and executes:
  - `make build`
  - `make test`
- `.github/workflows/build.yaml` runs on Go `1.25.x` and executes:
  - `make build`
  - `make test`
  - multi-platform `go build` loops
  - Docker image build with `docker buildx`
- If you can only run a minimal verification locally, prioritize `go test ./...` and `go build ./...`

## Repository Structure Conventions

- Keep Cobra command definitions in `cmd/`
- Keep shared client and wait helper functions in `cmd/utils.go` unless a new helper file clearly improves clarity
- Keep `main.go` minimal; it should only bootstrap the root command
- Prefer extending existing command files over creating many tiny files unless the command becomes meaningfully complex

## Code Style Guidelines

## Formatting

- Follow standard Go formatting; use `gofmt`, not hand-aligned formatting
- Keep lines and blocks simple; this repository does not use dense helper abstractions
- Preserve the existing Apache license header style in Go files when creating new source files

## Imports

- Use Go's default import grouping enforced by `gofmt`
- Standard library imports come first, then a blank line, then external imports
- Use import aliases only when they add clarity or avoid collisions
- Existing examples to follow:
  - `kafkaapi` for Strimzi API package aliases
  - `metav1` for Kubernetes meta API imports
- Do not alphabetize manually after `gofmt`; let tooling decide

## Types And APIs

- Prefer concrete typed Kubernetes and Strimzi clients over generic maps or unstructured objects
- Use the existing helper functions `kubeClient`, `strimziClient`, `waitUntilReady`, and related utilities before introducing new abstractions
- Pass `context.Context` into API calls; current code uses `context.TODO()` or scoped timeout contexts
- Prefer explicit types when they improve readability at API boundaries; inside local code, normal Go type inference is fine
- Keep function signatures small and purpose-driven

## Naming

- Use short, direct CamelCase names that match Go conventions
- Follow the repository's existing vocabulary:
  - `kubeConfig`, not `kubernetesConfig`
  - `namespace`, `timeout`, `poolName`, `clusterName`
  - `isReady`, `deleteDeployment`, `waitUntilReady`
- Boolean helper names should read as predicates like `isReady` or `isReconciliationPaused`
- Wait functions should start with `wait...`; delete functions should start with `delete...`
- Avoid vague names like `data`, `obj`, or `thing` unless forced by callback interfaces

## Error Handling

- In reusable helpers, return `error` values instead of terminating the process
- In Cobra command `Run` functions, it is acceptable to terminate with `log.Fatal` or `log.Fatalf`; that is the dominant current pattern
- Include resource name and namespace in Kubernetes error messages whenever possible
- Prefer `fmt.Errorf` with clear context for wrapped or newly created errors
- Do not ignore returned errors unless the codebase already does so for a narrow reason
- Avoid `panic` for normal operational failures; if you touch code that currently panics on watch setup, consider whether returning an error is cleaner and still consistent with the change scope

## Logging And User Output

- Use `log.Printf` for progress messages in command execution paths
- Use `log.Fatal` / `log.Fatalf` for fatal CLI failures in command handlers
- Use `fmt.Println` only for explicit user-facing value output such as the `version` command
- Keep messages operational and specific: include the resource kind, name, and namespace
- Do not add noisy debug logging by default

## Control Flow And Concurrency

- The code already uses goroutines plus `sync.WaitGroup` for parallel deletion of independent resources; keep that pattern when extending similar behavior
- Be careful with captured loop variables in goroutines; pass values explicitly if you refactor loops
- Prefer simple sequential logic unless there is clear parallelism across independent Kubernetes resources
- Use timeout-based waits for remote operations rather than indefinite polling

## CLI And Cobra Conventions

- Add new commands through `rootCmd.AddCommand(...)` in the command file's `init()`
- Reuse persistent flags on `rootCmd` when the option applies to multiple commands
- Validate required flags early in `Run`
- Keep `Use`, `Short`, and `Long` strings concise and consistent with existing commands
- Preserve existing command names such as `stop`, `continue`, and `version`

## File Creation And Editing Guidance

- Prefer small, focused edits over broad refactors
- Do not rename commands, flags, or public package paths without a strong reason
- Do not add new dependencies lightly; the current dependency set is small and purposeful
- If you add tests, place them beside the package under test using standard Go `_test.go` files
- If you add helper files under `cmd/`, name them by responsibility, for example `wait.go` or `kube.go`

## Verification Checklist Before Finishing

- `gofmt -w` on touched Go files
- `go mod tidy -v` if dependencies or imports changed
- `go test ./...`
- `go build ./...`
- If behavior changed in command execution, review log/error messages for clarity
- If flags or command help changed, update `README.md` when appropriate

## What Not To Do

- Do not introduce non-standard formatting rules
- Do not add a new framework for a small CLI command set
- Do not replace typed Kubernetes client usage with shelling out to `kubectl`
- Do not silently change operational semantics around pause/unpause or deletion ordering
- Do not assume hidden repository rules exist beyond what is present on disk
