# Orivis release checklist

This checklist defines the minimum bar for an alpha release.

## v0.1-alpha

- Server and agent binaries build successfully.
- `go test ./...` passes.
- `golangci-lint run ./...` passes.
- `.goreleaser.yaml` passes `goreleaser check`.
- `goreleaser release --snapshot --clean` produces local release artifacts.
- `docker build --build-arg APP=orivis-server` succeeds.
- `docker build --build-arg APP=orivis-agent` succeeds.
- Docker images compress the runtime binary with UPX by default.
- `docker compose up --build` starts server and agent.
- `deployments/docker-compose` examples are up to date.
- `deployments/systemd` examples are up to date.
- Dashboard responds on `http://127.0.0.1:8080`.
- Agent can register with an optional shared token.
- Agent can sync a static monitor.
- Agent can pull tasks and report probe results.
- SQLite file DSN persists data.
- Retention cleanup is enabled by default.
- README documents the supported probe types.
- README explicitly states Kafka is not implemented.

## Manual smoke commands

```powershell
./scripts/verify.ps1
./scripts/verify.ps1 -Docker
./scripts/verify.ps1 -Release
goreleaser release --snapshot --clean
./scripts/smoke-local.ps1
docker compose up --build
```

## Before tagging

- Choose a version tag, for example `v0.1.0-alpha.1`.
- Confirm `go.mod` and `go.sum` are clean.
- Confirm `.env.example` and `config.example.yaml` expose all user-facing config.
- Confirm dashboard auth behavior is documented.
- Confirm agent token behavior is documented.
- Confirm any unsupported planned probes are not advertised as supported.
- Confirm `GITHUB_TOKEN` is available for GitHub release publishing.
- Confirm `ghcr.io/lyonbrown4d/orivis-server` and `ghcr.io/lyonbrown4d/orivis-agent` are the intended image names.
