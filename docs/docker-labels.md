# Docker label examples

Orivis uses Docker labels with the `orivis.` prefix. Set `ORIVIS_DISCOVERY__PROVIDER=docker` on the agent, then put labels on Docker containers, Docker Compose services, or Docker Swarm services.

## Minimal inferred TCP check

Use this when the exposed container port is enough. The agent infers name, host, group, environment, and target port from Docker metadata.

```yaml
labels:
  orivis.enable: "true"
```

## HTTP health check

```yaml
labels:
  orivis.enable: "true"
  orivis.group: "api"
  orivis.environment: "prod"
  orivis.monitor.type: "http"
  orivis.monitor.target: "http://api:8080/healthz"
  orivis.monitor.interval: "15s"
  orivis.monitor.timeout: "3s"
  orivis.monitor.retry: "1"
```

## TCP check

```yaml
labels:
  orivis.enable: "true"
  orivis.group: "edge"
  orivis.monitor.type: "tcp"
  orivis.monitor.target: "gateway:443"
  orivis.monitor.interval: "10s"
  orivis.monitor.timeout: "2s"
```

## Redis check with inferred target

For Redis, the agent can infer `redis://<service>:6379` from Docker metadata and the exposed port.

```yaml
labels:
  orivis.enable: "true"
  orivis.group: "datastores"
  orivis.monitor.type: "redis"
  orivis.monitor.interval: "10s"
  orivis.monitor.timeout: "3s"
```

## PostgreSQL check with explicit DSN

Credentials are not inferred from container environment variables, so keep database targets explicit.

```yaml
labels:
  orivis.enable: "true"
  orivis.group: "datastores"
  orivis.monitor.type: "postgres"
  orivis.monitor.target: "postgres://orivis:orivis@postgres:5432/orivis?sslmode=disable"
  orivis.monitor.interval: "30s"
  orivis.monitor.timeout: "5s"
```

## MySQL check

```yaml
labels:
  orivis.enable: "true"
  orivis.group: "datastores"
  orivis.monitor.type: "mysql"
  orivis.monitor.target: "mysql://root:password@mysql:3306/app?parseTime=true"
  orivis.monitor.interval: "30s"
  orivis.monitor.timeout: "5s"
```

## Kafka check

```yaml
labels:
  orivis.enable: "true"
  orivis.group: "messaging"
  orivis.monitor.type: "kafka"
  orivis.monitor.target: "kafka://kafka:9092?topic=orivis"
  orivis.monitor.interval: "30s"
  orivis.monitor.timeout: "5s"
```

## TLS certificate check

`degraded_before` controls when a valid but soon-expiring certificate becomes degraded.

```yaml
labels:
  orivis.enable: "true"
  orivis.group: "edge"
  orivis.monitor.type: "tls"
  orivis.monitor.target: "tls://example.com:443?degraded_before=336h"
  orivis.monitor.interval: "5m"
  orivis.monitor.timeout: "5s"
```

## Multiple monitors on one service

Use `orivis.monitor.<name>.*` when one container or service should emit more than one monitor.

```yaml
labels:
  orivis.enable: "true"
  orivis.group: "api"
  orivis.environment: "prod"
  orivis.monitor.http.type: "http"
  orivis.monitor.http.name: "api health"
  orivis.monitor.http.target: "http://api:8080/healthz"
  orivis.monitor.http.interval: "15s"
  orivis.monitor.http.timeout: "3s"
  orivis.monitor.tcp.type: "tcp"
  orivis.monitor.tcp.name: "api port"
  orivis.monitor.tcp.target: "api:8080"
  orivis.monitor.tcp.interval: "10s"
  orivis.monitor.tcp.timeout: "2s"
```

## Per-monitor group override

`orivis.group` applies to all monitors from the same source. `orivis.monitor.group` or `orivis.monitor.<name>.group` overrides one monitor.

```yaml
labels:
  orivis.enable: "true"
  orivis.group: "api"
  orivis.monitor.http.type: "http"
  orivis.monitor.http.target: "http://api:8080/healthz"
  orivis.monitor.http.group: "public-api"
  orivis.monitor.admin.type: "http"
  orivis.monitor.admin.target: "http://api:9090/admin/healthz"
  orivis.monitor.admin.group: "internal-api"
```

## Disabled monitor

```yaml
labels:
  orivis.enable: "true"
  orivis.monitor.type: "http"
  orivis.monitor.target: "http://api:8080/healthz"
  orivis.monitor.enabled: "false"
```

## Docker Compose service

In Docker Compose, labels belong directly under the service. The agent uses Compose metadata like `com.docker.compose.project` and `com.docker.compose.service` as fallbacks.

```yaml
services:
  api:
    image: example/api:latest
    labels:
      orivis.enable: "true"
      orivis.group: "api"
      orivis.monitor.type: "http"
      orivis.monitor.target: "http://api:8080/healthz"
      orivis.monitor.interval: "15s"
      orivis.monitor.timeout: "3s"
```

## Docker Swarm service

In Swarm, put Orivis labels under `deploy.labels`. The agent uses service labels on manager nodes.

```yaml
services:
  api:
    image: example/api:latest
    networks:
      - app
    deploy:
      replicas: 3
      labels:
        orivis.enable: "true"
        orivis.group: "api"
        orivis.monitor.type: "http"
        orivis.monitor.target: "http://api:8080/healthz"
        orivis.monitor.interval: "15s"
        orivis.monitor.timeout: "3s"

networks:
  app:
    driver: overlay
```

## Label reference

| Label | Description |
| --- | --- |
| `orivis.enable` | Enables discovery for the Docker source. Defaults to `true` only when monitor labels are present. |
| `orivis.group` | Dashboard group fallback for all monitors from this source. |
| `orivis.environment` | Environment code fallback for all monitors from this source. |
| `orivis.monitor.type` | Probe type for the default monitor. |
| `orivis.monitor.target` | Probe target for the default monitor. |
| `orivis.monitor.name` | Display name for the default monitor. |
| `orivis.monitor.group` | Group override for the default monitor. |
| `orivis.monitor.enabled` | Enables or disables the default monitor. |
| `orivis.monitor.interval` | Probe interval, for example `15s` or `1m`. |
| `orivis.monitor.timeout` | Probe timeout, for example `3s`. |
| `orivis.monitor.retry` | Retry count before reporting the probe result. |
| `orivis.monitor.aggregation` | Aggregation policy, for example `majority_down`. |
| `orivis.monitor.<name>.*` | Named monitor fields for multiple monitors on one Docker source. |
