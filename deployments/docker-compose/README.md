# Docker Compose deployment

This directory contains a Compose deployment template for Orivis.

Use the root `docker-compose.yml` for local source builds. Use this directory when you want a deployment-oriented template that can be copied to a host and pointed at published images.

## Files

- `compose.yml`: server and agent services.
- `server.env.example`: server environment variables.
- `agent.env.example`: agent environment variables.

## Usage

```powershell
Copy-Item deployments/docker-compose/server.env.example deployments/docker-compose/server.env
Copy-Item deployments/docker-compose/agent.env.example deployments/docker-compose/agent.env
docker compose -f deployments/docker-compose/compose.yml up -d
```

The image tag defaults to `latest`. Override it for local smoke deployments:

```powershell
$env:ORIVIS_IMAGE_TAG = "local-smoke"
docker compose -f deployments/docker-compose/compose.yml up -d
```

The HTTP port defaults to `8080`. Override it when the host already has a local server running:

```powershell
$env:ORIVIS_HTTP_PORT = "18080"
docker compose -f deployments/docker-compose/compose.yml up -d
```

The template also starts Redis and PostgreSQL. The agent reads their Docker labels through the Docker socket and reports Redis/PostgreSQL uptime probes to the server.

Before production use:

- Change `ORIVIS_AUTH__AGENT__TOKEN`.
- Enable dashboard auth.
- Use a persistent SQLite DSN.
- Put the dashboard behind HTTPS.
