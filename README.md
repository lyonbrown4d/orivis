# Orivis

Orivis is a Go multi-command application framework for a distributed availability observability platform.

## Commands

```powershell
go run ./cmd/orivis-server
go run ./cmd/orivis-agent
```

Local end-to-end run with the example config:

```powershell
go run ./cmd/orivis-server --config config.example.yaml
go run ./cmd/orivis-agent --config config.example.yaml
```

The example agent config registers a static `http` monitor for `http://127.0.0.1:8080/healthz`, syncs it to the server, pulls it back as a task, probes it, and reports the result.

Orivis also reads `.env` and `.env.local` automatically through `configx`, so local development can run without `--config`:

```powershell
Copy-Item .env.example .env
go run ./cmd/orivis-server
go run ./cmd/orivis-agent
```

This repository also includes an ignored `.env.local` for a ready-to-run local dotenv test environment.

Environment variables use `__` to separate nested config keys. Single underscores remain part of a field name.

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
  api/            HTTP API and dashboard
  agentclient/    agent HTTP client
  agentconfig/    agent config model and loader
  collector/      agent collection runner
  store/          sqlite storage and dashboard snapshots
  discovery/      monitor discovery adapters
  probe/          monitor probes
  model/          domain models
  protocol/       agent/server protocol DTOs
migrations/       database-specific migrations
deployments/      deployment examples and manifests
docs/             project documentation
```

## Configuration

The server reads:

| Variable | Default | Description |
| --- | --- | --- |
| `ORIVIS_APP__ENV` | `development` | Runtime environment. |
| `ORIVIS_HTTP__ADDR` | `:8080` | Server listen address. |
| `ORIVIS_LOG__LEVEL` | `info` | `debug`, `info`, `warn`, or `error`. |
| `ORIVIS_DB__DRIVER` | `sqlite` | Storage driver. |
| `ORIVIS_DB__DSN` | `file:orivis?mode=memory&cache=shared` | SQLite DSN. Use a file DSN for persistence. |
| `ORIVIS_AUTH__AGENT__TOKEN` | empty | Optional bootstrap token required for agent registration. |
| `ORIVIS_AUTH__DASHBOARD__ENABLED` | `false` | Enable HTTP Basic Auth for the dashboard. |
| `ORIVIS_AUTH__DASHBOARD__USERNAME` | `admin` | Dashboard Basic Auth username. |
| `ORIVIS_AUTH__DASHBOARD__PASSWORD` | empty | Dashboard Basic Auth password. Required when dashboard auth is enabled. |
| `ORIVIS_OBSERVABILITY__PROMETHEUS__ENABLED` | `false` | Enable Prometheus observability adapter. |

### Storage

SQLite is the only storage backend. The default DSN uses SQLite's in-memory mode for zero-configuration startup.

Use a file DSN when you need persistence:

```env
ORIVIS_DB__DRIVER=sqlite
ORIVIS_DB__DSN=file:orivis.db
```

When `ORIVIS_AUTH__AGENT__TOKEN` is set on the server, an agent must present the same token during registration. The server stores only a hashed agent token after registration.

The dashboard at `/` is public by default for local zero-configuration usage. Enable Basic Auth when exposing it outside localhost:

```env
ORIVIS_AUTH__DASHBOARD__ENABLED=true
ORIVIS_AUTH__DASHBOARD__USERNAME=admin
ORIVIS_AUTH__DASHBOARD__PASSWORD=change-me
```

The agent reads:

| Variable | Default | Description |
| --- | --- | --- |
| `ORIVIS_SERVER__URL` | `http://127.0.0.1:8080` | Server base URL. |
| `ORIVIS_AGENT__NAME` | `local-agent` | Agent name. |
| `ORIVIS_AGENT__TOKEN` | empty | Agent token. |
| `ORIVIS_AGENT__REGION` | `local` | Agent region code. |
| `ORIVIS_AGENT__ENVIRONMENTS` | empty | Comma-separated environment codes the agent can probe. |
| `ORIVIS_RUNTIME` | `host` | Agent runtime type. |
| `ORIVIS_POLL__INTERVAL` | `30s` | Task polling interval. |
| `ORIVIS_DISCOVERY__STATIC__MONITOR__NAME` | empty | Optional single static monitor name for dotenv-based local runs. |
| `ORIVIS_DISCOVERY__STATIC__MONITOR__TYPE` | empty | Optional single static monitor type. |
| `ORIVIS_DISCOVERY__STATIC__MONITOR__TARGET` | empty | Optional single static monitor target. |
| `ORIVIS_DISCOVERY__DOCKER__ENABLED` | `false` | Enable Docker label discovery. |
| `ORIVIS_DISCOVERY__DOCKER__MODE` | `container` | Docker discovery mode: `container` or `swarm`. |
| `ORIVIS_LOG__LEVEL` | `info` | `debug`, `info`, `warn`, or `error`. |

## HTTP Endpoints

```text
GET /                  dashboard UI
GET /api/server/metadata application metadata
GET /healthz          health probe
GET /readyz           readiness probe
POST /api/agent/register  agent registration
POST /api/agent/heartbeat agent heartbeat
GET /api/agent/tasks  pull assigned monitor tasks
POST /api/agent/results record probe result
```

## Agent Probes

The agent uses `gocron` to periodically sync assigned tasks from the server.
Each monitor task is scheduled independently using its `interval_seconds` value, with `ORIVIS_POLL__INTERVAL` as the fallback interval.

Currently implemented probe types:

| Type | Behavior |
| --- | --- |
| `http` | Sends `GET` and treats `2xx`/`3xx` as up. |
| `tcp` | Opens a TCP connection to `host:port`. |
| `dns` | Resolves the target host. |
| `tls` | Opens a verified TLS connection to `host:port`. |

`ping` is defined in the shared model but is not implemented yet because ICMP needs a platform-specific adapter.

## Discovery Labels

For local development, monitors can be declared directly in the agent config file:

```yaml
discovery:
  static:
    enabled: true
    monitors:
      - name: server-health
        type: http
        target: http://127.0.0.1:8080/healthz
        environment: dev
        interval: 15s
        timeout: 3s
        aggregation: majority_down
```

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
ORIVIS_DISCOVERY__DOCKER__ENABLED=true
ORIVIS_DISCOVERY__DOCKER__MODE=container
```

For Docker Swarm service labels, use:

```env
ORIVIS_DISCOVERY__DOCKER__ENABLED=true
ORIVIS_DISCOVERY__DOCKER__MODE=swarm
```

## Configuration Reconciliation (`.env` ↔ `config.example.yaml` ↔ design doc)

### 1) `.env` examples to `config.example.yaml`

| 环境变量 | 对应 YAML key | 备注 |
| --- | --- | --- |
| `ORIVIS_LOG__LEVEL` | `log.level` | 已对齐 |
| `ORIVIS_APP__ENV` | `app.env` | 已对齐 |
| `ORIVIS_HTTP__ADDR` | `http.addr` | 已对齐 |
| `ORIVIS_DB__DRIVER` | `db.driver` | 已对齐 |
| `ORIVIS_DB__DSN` | `db.dsn` | 已对齐 |
| `ORIVIS_DB__RESULTRETENTION` | `db.resultretention` | 已对齐 |
| `ORIVIS_DB__CLEANUPINTERVAL` | `db.cleanupinterval` | 已对齐 |
| `ORIVIS_AUTH__AGENT__TOKEN` | `auth.agent.token` | 已对齐 |
| `ORIVIS_AUTH__DASHBOARD__ENABLED` | `auth.dashboard.enabled` | 已对齐 |
| `ORIVIS_AUTH__DASHBOARD__USERNAME` | `auth.dashboard.username` | 已对齐 |
| `ORIVIS_AUTH__DASHBOARD__PASSWORD` | `auth.dashboard.password` | 已对齐 |
| `ORIVIS_OBSERVABILITY__PROMETHEUS__ENABLED` | `observability.prometheus.enabled` | 已对齐 |
| `ORIVIS_OBSERVABILITY__PROMETHEUS__NAMESPACE` | `observability.prometheus.namespace` | 已对齐 |
| `ORIVIS_SERVER__URL` | `server.url` | 已对齐 |
| `ORIVIS_AGENT__NAME` | `agent.name` | 已对齐 |
| `ORIVIS_AGENT__TOKEN` | `agent.token` | 已对齐 |
| `ORIVIS_AGENT__REGION` | `agent.region` | 已对齐 |
| `ORIVIS_AGENT__ENVIRONMENTS` | `agent.environments` | 已对齐 |
| `ORIVIS_RUNTIME` | `runtime` | 已对齐 |
| `ORIVIS_POLL__INTERVAL` | `poll.interval` | 已对齐 |
| `ORIVIS_DISCOVERY__STATIC__ENABLED` | `discovery.static.enabled` | 已对齐 |
| `ORIVIS_DISCOVERY__STATIC__MONITOR__SOURCE_KEY` | `discovery.static.monitors[n].source_key` | 已对齐（单个 monitor 快捷写法） |
| `ORIVIS_DISCOVERY__STATIC__MONITOR__NAME` | `discovery.static.monitors[n].name` | 已对齐（单个 monitor 快捷写法） |
| `ORIVIS_DISCOVERY__STATIC__MONITOR__TYPE` | `discovery.static.monitors[n].type` | 已对齐（单个 monitor 快捷写法） |
| `ORIVIS_DISCOVERY__STATIC__MONITOR__TARGET` | `discovery.static.monitors[n].target` | 已对齐（单个 monitor 快捷写法） |
| `ORIVIS_DISCOVERY__STATIC__MONITOR__ENVIRONMENT` | `discovery.static.monitors[n].environment` | 已对齐（单个 monitor 快捷写法） |
| `ORIVIS_DISCOVERY__STATIC__MONITOR__ENABLED` | `discovery.static.monitors[n].enabled` | 已对齐（单个 monitor 快捷写法） |
| `ORIVIS_DISCOVERY__STATIC__MONITOR__INTERVAL` | `discovery.static.monitors[n].interval` | 已对齐（单个 monitor 快捷写法） |
| `ORIVIS_DISCOVERY__STATIC__MONITOR__TIMEOUT` | `discovery.static.monitors[n].timeout` | 已对齐（单个 monitor 快捷写法） |
| `ORIVIS_DISCOVERY__STATIC__MONITOR__RETRY_COUNT` | `discovery.static.monitors[n].retry_count` | 已对齐（单个 monitor 快捷写法） |
| `ORIVIS_DISCOVERY__STATIC__MONITOR__AGGREGATION` | `discovery.static.monitors[n].aggregation` | 已对齐（单个 monitor 快捷写法） |
| `ORIVIS_DISCOVERY__DOCKER__ENABLED` | `discovery.docker.enabled` | 已对齐 |
| `ORIVIS_DISCOVERY__DOCKER__MODE` | `discovery.docker.mode` | 已对齐 |

### 2) 设计文档部署片段对照

| 场景 | 设计文档片段配置 | 与配置模型一致性 |
| --- | --- | --- |
| Docker Agent (`6.3`) | `ORIVIS_SERVER__URL` `ORIVIS_AGENT__TOKEN` `ORIVIS_AGENT__NAME` `ORIVIS_AGENT__REGION` `ORIVIS_RUNTIME` | ✅ 与 server/agent/runtime 配置完全一致 |
| Host + systemd (`6.4`) | `ORIVIS_SERVER__URL` `ORIVIS_AGENT__TOKEN` `ORIVIS_AGENT__NAME` `ORIVIS_AGENT__REGION` `ORIVIS_RUNTIME` | ✅ 与 server/agent/runtime 配置完全一致 |
| Docker Compose (`13.2`) | `ORIVIS_DB__DRIVER` `ORIVIS_DB__DSN` | ✅ 与 `db.*` 一致 |
| Docker Swarm (`13.3`) | `ORIVIS_SERVER__URL` `ORIVIS_AGENT__TOKEN` `ORIVIS_RUNTIME` | ✅ 与 server/agent/runtime 一致 |

### 3) 对账结论

- `.env` 与 `config.example.yaml`：字段映射完整。
- 设计文档：当前 `systemd` 示例与 `configx` 的 dotenv 方式统一，全部片段与当前配置路径一致。

## Verify

```powershell
go fmt ./...
go test ./...
```
