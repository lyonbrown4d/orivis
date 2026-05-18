# Docker Compose deployment

This directory contains a production-oriented Compose deployment template for Orivis.

Use the root `docker-compose.yml` for local source builds. Use this directory when you want a deployment-oriented template that can be copied to a host and pointed at published images.

## Files

- `compose.yml`: server and agent services.
- `server.env.example`: server environment variables.
- `agent.env.example`: agent environment variables.
- `compose.hcl.yml`: optional override for mounting an HCL agent config.

## Usage

```powershell
Copy-Item deployments/docker-compose/server.env.example deployments/docker-compose/server.env
Copy-Item deployments/docker-compose/agent.env.example deployments/docker-compose/agent.env
docker compose -f deployments/docker-compose/compose.yml up -d
```

The image tag defaults to `latest`. For production, pin it to a released tag:

```powershell
$env:ORIVIS_IMAGE_TAG = "v0.0.2"
docker compose -f deployments/docker-compose/compose.yml up -d
```

The HTTP port defaults to `8080`. Override it when the host already has a local server running:

```powershell
$env:ORIVIS_HTTP_PORT = "18080"
docker compose -f deployments/docker-compose/compose.yml up -d
```

The server image already enables the bundled SPA and serves `/app/web`, so `ORIVIS_WEB__ENABLED` and `ORIVIS_WEB__ROOT` are intentionally not part of `server.env`.

The agent reads Docker labels and container metadata through the Docker socket. Add labels to application containers that the agent can reach from its Docker network:

```yaml
labels:
  orivis.enable: "true"
  orivis.group: "datastores"
  orivis.monitor.type: "redis"
  orivis.monitor.interval: "10s"
  orivis.monitor.timeout: "3s"
```

When credentials are required, keep an explicit target:

```yaml
labels:
  orivis.enable: "true"
  orivis.group: "datastores"
  orivis.monitor.type: "postgres"
  orivis.monitor.target: "postgres://orivis:orivis@postgres:5432/orivis?sslmode=disable"
```

Before production use:

- Change `ORIVIS_AUTH__AGENT__TOKEN`.
- Change `ORIVIS_AUTH__DASHBOARD__PASSWORD`.
- Change `ORIVIS_AUTH__DASHBOARD__JWT_SECRET`.
- Put the dashboard behind HTTPS.
- Set `ORIVIS_AUTH__DASHBOARD__SECURE_COOKIE=true` when HTTPS is terminated directly at this service.
