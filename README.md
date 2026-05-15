# Orivis

Orivis is a Go multi-command application framework for a distributed availability observability platform.

## Commands

```powershell
go run ./cmd/orivis-server
go run ./cmd/orivis-agent
```

## Build

```powershell
go build -o bin/orivis-server.exe ./cmd/orivis-server
go build -o bin/orivis-agent.exe ./cmd/orivis-agent
```

## Layout

```text
cmd/
  orivis-server/  server entrypoint
  orivis-agent/   agent entrypoint
internal/
  server/         server API and bootstrap code
  agent/          agent runtime skeleton
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
| `ORIVIS_DB_DRIVER` | `memory` | Storage mode. Supported values: `memory`, `sqlite`. |
| `ORIVIS_DB_DSN` | (empty) | Only required when driver is `sqlite` (example: `file:orivis.db`). |
| `ORIVIS_DB_MEMORY_RESULT_RETENTION` | `24h` | How long in-memory probe results are retained. |
| `ORIVIS_DB_MEMORY_CLEANUP_INTERVAL` | `1m` | How often expired in-memory probe results are removed. |
| `ORIVIS_AUTH_AGENT_TOKEN` | empty | Optional bootstrap token required for agent registration. |
| `ORIVIS_OBSERVABILITY_PROMETHEUS_ENABLED` | `false` | Enable Prometheus observability adapter. |

### Storage mode

`memory` is the default and gives a zero-configuration startup path.
Data is kept in process memory and is not persisted across restarts. Probe results are retained for `24h` by default and cleaned every `1m`.

Use `sqlite` when you need persistence:

```env
ORIVIS_DB_DRIVER=sqlite
ORIVIS_DB_DSN=file:orivis.db
```

When `ORIVIS_AUTH_AGENT_TOKEN` is set on the server, an agent must present the same token during registration. The server stores only a hashed agent token after registration.

The agent reads:

| Variable | Default | Description |
| --- | --- | --- |
| `ORIVIS_SERVER_URL` | `http://127.0.0.1:8080` | Server base URL. |
| `ORIVIS_AGENT_NAME` | `local-agent` | Agent name. |
| `ORIVIS_AGENT_TOKEN` | empty | Agent token. |
| `ORIVIS_AGENT_REGION` | `local` | Agent region code. |
| `ORIVIS_AGENT_ENVIRONMENTS` | empty | Comma-separated environment codes the agent can probe. |
| `ORIVIS_RUNTIME` | `host` | Agent runtime type. |
| `ORIVIS_POLL_INTERVAL` | `30s` | Task polling interval. |
| `ORIVIS_DISCOVERY_DOCKER_ENABLED` | `false` | Enable Docker label discovery. |
| `ORIVIS_DISCOVERY_DOCKER_MODE` | `container` | Docker discovery mode: `container` or `swarm`. |
| `ORIVIS_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error`. |

## HTTP Endpoints

```text
GET /                  application metadata
GET /healthz          health probe
GET /readyz           readiness probe
POST /api/agent/register  agent registration
POST /api/agent/heartbeat agent heartbeat
GET /api/agent/tasks  pull assigned monitor tasks
POST /api/agent/results record probe result
```

## Agent Probes

The agent uses `gocron` to periodically sync assigned tasks from the server.
Each monitor task is scheduled independently using its `interval_seconds` value, with `ORIVIS_POLL_INTERVAL` as the fallback interval.

Currently implemented probe types:

| Type | Behavior |
| --- | --- |
| `http` | Sends `GET` and treats `2xx`/`3xx` as up. |
| `tcp` | Opens a TCP connection to `host:port`. |
| `dns` | Resolves the target host. |
| `tls` | Opens a verified TLS connection to `host:port`. |

`ping` is defined in the shared model but is not implemented yet because ICMP needs a platform-specific adapter.

## Discovery Labels

Orivis monitor discovery uses labels with an `orivis.` prefix.
The parser is runtime-agnostic; Docker and Swarm adapters feed container or service labels into this format.

```yaml
labels:
  orivis.enable: "true"
  orivis.environment: "prod"
  orivis.monitor.http.type: "http"
  orivis.monitor.http.target: "http://web:8080/health"
  orivis.monitor.http.interval: "30s"
  orivis.monitor.http.timeout: "5s"
  orivis.monitor.http.retry: "2"
  orivis.monitor.http.aggregation: "majority_down"
```

Each monitor is grouped by `orivis.monitor.<name>.*`.
Supported fields are `type`, `target`, `name`, `enabled`, `interval`, `timeout`, `retry`, and `aggregation`.

Docker discovery is disabled by default. Enable container label discovery with:

```env
ORIVIS_DISCOVERY_DOCKER_ENABLED=true
ORIVIS_DISCOVERY_DOCKER_MODE=container
```

For Docker Swarm service labels, use:

```env
ORIVIS_DISCOVERY_DOCKER_ENABLED=true
ORIVIS_DISCOVERY_DOCKER_MODE=swarm
```

## Verify

```powershell
go fmt ./...
go test ./...
```
