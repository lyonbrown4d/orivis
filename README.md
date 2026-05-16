# Orivis

Orivis is a lightweight distributed uptime service.

It runs as two binaries:

- `orivis-server`: receives agent data, stores probe results, and serves the dashboard UI.
- `orivis-agent`: discovers monitors, runs scheduled checks, and reports results to the server.

The default storage backend is SQLite. The default configuration is zero-config and uses an in-memory SQLite database.

## Current status

Orivis is suitable for `v0.1-alpha` testing. The core loop is implemented:

```text
agent discovery -> server sync -> task pull -> probe check -> result report -> eventx ingest -> sqlite -> dashboard
```

## Quick start with Docker Compose

```powershell
docker compose up --build
```

Then open:

```text
http://127.0.0.1:8080
```

The Compose stack starts one server and one agent. The agent registers a static HTTP monitor for the server health endpoint.

## Local run

```powershell
go run ./cmd/orivis-server --config config.example.yaml
go run ./cmd/orivis-agent --config config.example.yaml
```

Or use dotenv:

```powershell
Copy-Item .env.example .env
go run ./cmd/orivis-server
go run ./cmd/orivis-agent
```

## Smoke test

```powershell
./scripts/smoke-local.ps1
./scripts/smoke-compose.ps1 -Tag local-smoke -HostPort 18080 -KeepRunning
./scripts/smoke-compose.ps1 -Tag local-smoke -HostPort 18080 -SkipBuild -KeepRunning
```

The script starts a temporary server and agent, waits for a full probe loop, and checks that the dashboard responds.
The Compose smoke script builds tagged local images, starts Orivis with Redis and PostgreSQL, and checks the discovered monitors on the dashboard.

## Build

```powershell
go build -o bin/orivis-server.exe ./cmd/orivis-server
go build -o bin/orivis-agent.exe ./cmd/orivis-agent
```

Docker images can be built from the same Dockerfile:

```powershell
docker build --build-arg APP=orivis-server -t orivis-server:local .
docker build --build-arg APP=orivis-agent -t orivis-agent:local .
```

The Docker build compresses the runtime binary with UPX by default.

Release artifacts are described by GoReleaser:

```powershell
goreleaser check
goreleaser release --snapshot --clean
```

The GoReleaser pipeline builds:

- `orivis-server` and `orivis-agent` binaries for Linux, macOS, and Windows.
- Per-binary archives and `checksums.txt`.
- Linux `.deb` and `.rpm` packages for `orivis-server` and `orivis-agent`.
- Optional multi-arch container images through `dockers_v2`. Release images also compress the runtime binary with UPX.

Tagged releases are published by GitHub Actions:

```powershell
git tag v0.1.0-alpha.1
git push origin v0.1.0-alpha.1
```

Deployment templates are available in:

- `deployments/docker-compose`
- `deployments/systemd`

## Configuration

Orivis uses `configx`, so config can come from YAML, dotenv, or environment variables.

Environment variables use `__` for nested keys:

```env
ORIVIS_HTTP__ADDR=:8080
ORIVIS_DB__DSN=file:orivis.db
```

### Server

| Variable | Default | Description |
| --- | --- | --- |
| `ORIVIS_APP__ENV` | `development` | Runtime environment. |
| `ORIVIS_HTTP__ADDR` | `:8080` | Server listen address. |
| `ORIVIS_LOG__LEVEL` | `info` | Log level. |
| `ORIVIS_DB__DRIVER` | `sqlite` | Storage driver. |
| `ORIVIS_DB__DSN` | `file:orivis?mode=memory&cache=shared` | SQLite DSN. Use a file DSN for persistence. |
| `ORIVIS_INGEST__QUEUESIZE` | `4096` | Async ingest queue size. |
| `ORIVIS_INGEST__BATCHSIZE` | `100` | Async ingest flush batch size. |
| `ORIVIS_INGEST__FLUSHINTERVAL` | `1s` | Async ingest periodic flush interval. |
| `ORIVIS_RETENTION__ENABLED` | `true` | Enable probe result cleanup. |
| `ORIVIS_RETENTION__RESULTTTL` | `168h` | Probe result retention TTL. |
| `ORIVIS_RETENTION__CLEANUPINTERVAL` | `1h` | Probe result cleanup interval. |
| `ORIVIS_AUTH__AGENT__TOKEN` | empty | Optional shared bootstrap token for agent registration. |
| `ORIVIS_AUTH__DASHBOARD__ENABLED` | `false` | Enable dashboard Basic Auth. |
| `ORIVIS_AUTH__DASHBOARD__USERNAME` | `admin` | Dashboard Basic Auth username. |
| `ORIVIS_AUTH__DASHBOARD__PASSWORD` | empty | Dashboard Basic Auth password. |
| `ORIVIS_OBSERVABILITY__PROMETHEUS__ENABLED` | `false` | Enable Prometheus metrics. |

### Agent

| Variable | Default | Description |
| --- | --- | --- |
| `ORIVIS_SERVER__URL` | `http://127.0.0.1:8080` | Server base URL. |
| `ORIVIS_AGENT__NAME` | `local-agent` | Agent name. |
| `ORIVIS_AGENT__TOKEN` | empty | Agent token. Must match server token when configured. |
| `ORIVIS_AGENT__REGION` | `local` | Agent region code. |
| `ORIVIS_AGENT__ENVIRONMENTS` | empty | Comma-separated environment codes. |
| `ORIVIS_RUNTIME` | `host` | Agent runtime type. |
| `ORIVIS_POLL__INTERVAL` | `30s` | Task sync fallback interval. |
| `ORIVIS_DISCOVERY__DOCKER__ENABLED` | `false` | Enable Docker label discovery. |
| `ORIVIS_DISCOVERY__DOCKER__MODE` | `container` | Docker discovery mode: `container` or `swarm`. |

## Security

Agent registration can be protected with a shared token:

```env
ORIVIS_AUTH__AGENT__TOKEN=change-me
ORIVIS_AGENT__TOKEN=change-me
```

Dashboard Basic Auth is disabled by default for local zero-config usage. Enable it when exposing the dashboard outside localhost:

```env
ORIVIS_AUTH__DASHBOARD__ENABLED=true
ORIVIS_AUTH__DASHBOARD__USERNAME=admin
ORIVIS_AUTH__DASHBOARD__PASSWORD=change-me
```

For production, run Orivis behind a reverse proxy that terminates HTTPS.

## Supported probes

| Type | Target example | Behavior |
| --- | --- | --- |
| `http` | `https://example.com/health` | Sends `GET`; `2xx` and `3xx` are up. |
| `tcp` | `example.com:443` | Opens a TCP connection. |
| `ping` | `1.1.1.1` | Sends one ICMP/UDP ping via `pro-bing`. Containers may need network capabilities depending on platform. |
| `redis` | `redis://127.0.0.1:6379` | Uses `go-redis` `PING`. |
| `database` | `sqlite://file:orivis.db` | Generic database probe. |
| `sqlite` | `file:orivis.db` | SQLite `database/sql` ping. |
| `mysql` | `mysql://root:password@127.0.0.1:3306/app?parseTime=true` | MySQL `database/sql` ping. |
| `postgres` / `pg` | `postgres://postgres:password@127.0.0.1:5432/app?sslmode=disable` | PostgreSQL `database/sql` ping through pgx. |
| `dns` | `example.com` | Resolves host addresses. |
| `tls` | `example.com:443` | Opens a verified TLS connection. |

Kafka is not implemented yet and is not advertised as a supported probe.

## Docker labels

Orivis monitor discovery uses labels with an `orivis.` prefix.

```yaml
labels:
  orivis.enable: "true"
  orivis.monitor.type: "redis"
  orivis.monitor.interval: "30s"
```

When Docker discovery is enabled, Orivis uses container metadata to infer the monitor name, environment, target host, and ports.
For Redis, `orivis.monitor.type=redis` is enough to infer `redis://<service>:6379`.
For database probes that need credentials, keep `orivis.monitor.target` explicit.

Multiple monitors can still be grouped by `orivis.monitor.<name>.*`.

Supported fields are `type`, `target`, `name`, `enabled`, `interval`, `timeout`, `retry`, and `aggregation`.

Enable Docker container labels:

```env
ORIVIS_DISCOVERY__DOCKER__ENABLED=true
ORIVIS_DISCOVERY__DOCKER__MODE=container
```

Enable Docker Swarm service labels:

```env
ORIVIS_DISCOVERY__DOCKER__ENABLED=true
ORIVIS_DISCOVERY__DOCKER__MODE=swarm
```

## HTTP endpoints

```text
GET  /                         dashboard UI
GET  /api/server/metadata      application metadata
GET  /healthz                  health probe
GET  /readyz                   readiness probe
POST /api/agent/register       agent registration
POST /api/agent/heartbeat      agent heartbeat
GET  /api/agent/tasks          pull assigned monitor tasks
POST /api/agent/results        report probe result
```

## Verify

```powershell
./scripts/verify.ps1
./scripts/verify.ps1 -Docker
./scripts/verify.ps1 -Release
```

See [release-checklist.md](docs/release-checklist.md) for the alpha release checklist.
