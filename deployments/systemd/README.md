# systemd deployment

This directory contains systemd unit templates for host deployments.

## Files

- `orivis-server.service`
- `orivis-agent.service`
- `orivis-server.env.example`
- `orivis-agent.env.example`

## Usage

Build or copy binaries:

```powershell
go build -o bin/orivis-server ./cmd/orivis-server
go build -o bin/orivis-agent ./cmd/orivis-agent
```

Install on a Linux host:

```bash
sudo install -m 0755 bin/orivis-server /usr/local/bin/orivis-server
sudo install -m 0755 bin/orivis-agent /usr/local/bin/orivis-agent
sudo install -m 0644 deployments/systemd/orivis-server.service /etc/systemd/system/orivis-server.service
sudo install -m 0644 deployments/systemd/orivis-agent.service /etc/systemd/system/orivis-agent.service
sudo install -m 0640 deployments/systemd/orivis-server.env.example /etc/orivis/server.env
sudo install -m 0640 deployments/systemd/orivis-agent.env.example /etc/orivis/agent.env
sudo systemctl daemon-reload
sudo systemctl enable --now orivis-server
sudo systemctl enable --now orivis-agent
```

Before production use:

- Change all tokens and passwords.
- Create `/var/lib/orivis`.
- Choose a storage driver in `orivis-server.env.example`:
  - `ORIVIS_DB__DRIVER=pgx` with PostgreSQL DSN (recommended for production),
  - `ORIVIS_DB__DRIVER=mysql` with MySQL DSN, or
  - `ORIVIS_DB__DRIVER=sqlite` for local/single-node deployments.
- `postgres` / `pg` are not accepted as storage driver names for server configuration; use `pgx`.
- Run behind HTTPS.
