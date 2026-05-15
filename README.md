# Orivis

Orivis is a Go multi-command application framework for a distributed availability observability platform.

## Commands

```powershell
go run ./cmd/orivis-server
go run ./cmd/orivis-agent
go run ./cmd/orivis-cli version
go run ./cmd/orivis-cli health --addr http://127.0.0.1:8080
```

## Build

```powershell
go build -o bin/orivis-server.exe ./cmd/orivis-server
go build -o bin/orivis-agent.exe ./cmd/orivis-agent
go build -o bin/orivis-cli.exe ./cmd/orivis-cli
```

## Layout

```text
cmd/
  orivis-server/  server entrypoint
  orivis-agent/   agent entrypoint
  orivis-cli/     operator CLI entrypoint
internal/
  server/         server API and bootstrap code
  agent/          agent runtime skeleton
  cli/            CLI commands
  shared/         common build, logging, and model packages
migrations/       database-specific migrations
deployments/      deployment examples and manifests
docs/             project documentation
```

## Configuration

The server reads:

| Variable | Default | Description |
| --- | --- | --- |
| `ORIVIS_APP_ENV` | `development` | Runtime environment. |
| `ORIVIS_HTTP_ADDR` | `:8080` | Server listen address. |
| `ORIVIS_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error`. |
| `ORIVIS_DB_DRIVER` | `sqlite` | Database driver name. |
| `ORIVIS_DB_DSN` | `file:orivis.db` | Database connection string. |
| `ORIVIS_AUTH_AGENT_TOKEN` | empty | Shared agent token placeholder. |
| `ORIVIS_OBSERVABILITY_PROMETHEUS_ENABLED` | `false` | Enable Prometheus observability adapter. |

The agent reads:

| Variable | Default | Description |
| --- | --- | --- |
| `ORIVIS_SERVER_URL` | `http://127.0.0.1:8080` | Server base URL. |
| `ORIVIS_AGENT_NAME` | `local-agent` | Agent name. |
| `ORIVIS_AGENT_TOKEN` | empty | Agent token. |
| `ORIVIS_REGION` | `local` | Agent region code. |
| `ORIVIS_RUNTIME` | `host` | Agent runtime type. |
| `ORIVIS_POLL_INTERVAL` | `30s` | Task polling interval. |
| `ORIVIS_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error`. |

## HTTP Endpoints

```text
GET /                  application metadata
GET /healthz          health probe
GET /readyz           readiness probe
GET /api/agent/tasks  agent task placeholder
POST /api/agent/results agent result placeholder
```

## Verify

```powershell
go fmt ./...
go test ./...
```
