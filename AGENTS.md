# Orivis agent instructions

This file captures the project-specific rules that should be followed by coding agents working on Orivis.

## Working style

- Keep changes incremental and production-oriented.
- Preserve existing behavior unless the user explicitly asks for behavior changes.
- Do not revert user changes or unrelated work.
- Do not include unrelated untracked files in commits.
- Prefer clear, boring implementations over clever abstractions.
- If a dependency or package is not resolvable locally or through Go modules, do not force it into the build.

## Validation rules

- After each implementation iteration, run tests before lint.
- Use `go test ./...` for Go validation.
- Use `golangci-lint run ./...` for Go linting.
- Frontend UI changes are server-rendered templates and should be covered by Go tests.
- Do not change lint configuration to make code pass.
- Do not add `nolint` suppressions.
- Run `gofmt -w` on touched Go files before tests.
- Use the default Go build cache. Do not configure a custom `GOCACHE`.

## Commit rules

- Commit only after the user asks for a commit.
- Push only after the user asks for a push.
- Commit related files only.
- Leave unrelated untracked files untouched unless the user explicitly asks otherwise.
- Use concise conventional commit messages, for example `feat: ...`, `fix: ...`, or `docs: ...`.

## Architecture rules

- Keep `cmd/orivis-server` and `cmd/orivis-agent` responsible for binary assembly.
- Keep reusable application logic under `internal` as capability modules, not `internal/server/*` or `internal/agent/*` silos.
- Do not introduce generic runtime packages that hide binary-specific assembly. The binaries should compose `dix` modules directly.
- Use `dix` for shared construction and lifecycle management.
- Inject config, logger, stores, repositories, cache, event bus, worker pools, endpoint lists, and runtime factories through `dix` where practical.
- Keep `logx` slog logger and `configx` config values as DI-provided dependencies.
- Use endpoint-first HTTP organization. Each endpoint should register independently, and the HTTP server should consume an injected list of endpoints.

## Configuration rules

- Let `configx` own the configuration flow.
- Support dotenv through `configx`; do not add custom environment override logic.
- Use configx-compatible double underscore environment variable nesting.
- Agent config may support HCL through configx HCL support.
- Prefer typed defaults when available from configx.
- Runtime hot reload should restart internal runtime components cleanly instead of mutating running components in place.

## Data and storage rules

- Use `dbx` repository-style APIs for database access.
- Do not reintroduce direct `QueryContext`-style data access in application code.
- SQLite is the default database and may use in-memory mode.
- PostgreSQL and MySQL should be supported through dbx where practical.
- Server cache should remain pluggable with memory as default and Redis as optional.
- Agent result buffering should remain bounded and FIFO-oriented.
- Use the agent result buffer as the default reporting path when enabled, then flush to server in batches.

## Collection and error handling rules

- Prefer `collectionx` collection types and helpers for internal collection transformations when it improves readability.
- Prefer `collectionx/list`, `collectionx/mapping`, and `collectionx/set` over ad hoc slice/map/set handling in core code paths.
- Use `lo` only when it clearly removes boilerplate without reducing readability.
- Use `mo` only when option/result semantics materially improve the code. Do not force functional style into simple branches.
- Use `oops` for newly wrapped domain errors when context matters.
- For byte-domain helpers, prefer `github.com/arcgolabs/collectionx/bytex` when it improves consistency with collectionx usage.

## Agent rules

- Agent should support static config, Docker, and Docker Swarm discovery.
- Discovery provider should be explicit when required; unsupported or unavailable configured providers should fail clearly.
- Agent multi-replica behavior should rely on server-side assignment decisions while agents remain simple executors.
- Agent should use bounded concurrency through the shared concurrency module.
- Probe execution should support retries and eventual consistency without duplicate result ingestion.
- Log discovered monitor counts, provider mode, and scheduled tasks clearly enough for production troubleshooting.

## Server rules

- Server remains the system of record for metadata, assignments, ingested results, UI data, and notification state.
- Server ingest path should be asynchronous and batch-oriented where possible.
- Server should use eventx for decoupled events such as result recording and notifications.
- Server should keep UI snapshot caching safe to invalidate after result ingestion.
- Server should expose useful observability metrics without making metrics mandatory for startup.

## Frontend rules

- The active frontend is the server-rendered template UI embedded under `internal/ui`.
- Do not reintroduce a full frontend build toolchain unless the user explicitly reverses this decision.
- CDN-only UI helpers are acceptable, for example Bootstrap, Tailwind CDN, or htmx.
- Keep CSS and small browser scripts as embedded static assets.
- Do not require users to configure frontend root paths in production containers.
- Keep dashboard empty states robust. API arrays and template slices may be empty and should not crash the UI.

## Docker, release, and deployment rules

- Server and agent are separate images.
- The server image should include the embedded template UI so users do not configure frontend paths.
- Use the standard Dockerfiles and existing release pipeline.
- Use GoReleaser for releases and packages.
- Keep production docker compose examples current with supported environment variables and label syntax.
- Docker provider should cover plain Docker, Compose, and Swarm where possible, with explicit provider behavior when auto-detection is unreliable.

## Security rules

- Dashboard authentication may be disabled for public status pages.
- When dashboard auth is enabled, use the configured auth flow instead of raw 401 JSON for browser UI.
- Agent token authentication must remain supported.
- Do not remove token checks when adding discovery or multi-agent behavior.
