# Orivis Notifications

This document defines how Orivis sends notifications and how to configure notification routes.

## Feature summary

Orivis notification delivery is handled by the server and is triggered by probe status transitions.

- Default channel can be configured with:
  - `ORIVIS_NOTIFICATION__WEBHOOK__ENABLED`
  - `ORIVIS_NOTIFICATION__WEBHOOK__URL`
- Route-specific channels can be configured by:
  - `ORIVIS_NOTIFICATION__WEBHOOK__ROUTES`
- Route payload format can be selected by route `type`:
  - `webhook` (default): current Orivis event payload
  - `alertmanager`: Alertmanager-compatible alert payload wrapped as a one-item array

## Environment variables

| Variable | Default | Description |
| --- | --- | --- |
| `ORIVIS_NOTIFICATION__WEBHOOK__ENABLED` | `false` | Enable notification subsystem. |
| `ORIVIS_NOTIFICATION__WEBHOOK__URL` | empty | Default endpoint for route-less dispatch and inherited default values. |
| `ORIVIS_NOTIFICATION__WEBHOOK__METHOD` | `POST` | HTTP method for notifications. |
| `ORIVIS_NOTIFICATION__WEBHOOK__TIMEOUT` | `5s` | HTTP timeout. |
| `ORIVIS_NOTIFICATION__WEBHOOK__COOLDOWN` | `5m` | Cooldown between repeat alerts/recoveries for the same monitor. |
| `ORIVIS_NOTIFICATION__WEBHOOK__QUEUESIZE` | `128` | In-memory delivery queue size. |
| `ORIVIS_NOTIFICATION__WEBHOOK__MAXATTEMPTS` | `3` | Retry count. |
| `ORIVIS_NOTIFICATION__WEBHOOK__RETRYINTERVAL` | `5s` | Retry interval base delay. |
| `ORIVIS_NOTIFICATION__WEBHOOK__SECRET` | empty | HMAC secret for `X-Orivis-Signature`. |
| `ORIVIS_NOTIFICATION__WEBHOOK__HEADERS` | empty | Default headers for the default channel. |
| `ORIVIS_NOTIFICATION__WEBHOOK__ROUTES` | empty | Route definitions, each separated by newline or list entry. |
| `ORIVIS_NOTIFICATION__WEBHOOK__RECOVERYENABLED` | `true` | Emit recovery notifications. |

## Route format

`ORIVIS_NOTIFICATION__WEBHOOK__ROUTES` uses semicolon key-value entries (`;`) and `|` for list fields.

Supported keys:

- `type` (`webhook` | `alertmanager`)
- `name` (display name in logs/metrics)
- `url` (required unless inherited)
- `method` (`GET`, `POST`, etc.)
- `secret` (override signing secret)
- `headers` (comma-separated key/value list)
- `groups` (match monitor group names, `|` separated)
- `monitors` (match monitor IDs, `|` separated)

Matching logic:

- If a route has no `groups` and no `monitors`, it receives all events.
- If either list is set, it matches when any list item matches (case-insensitive).

### Supported route types

#### webhook

- URL: any HTTP endpoint that accepts a single JSON object.
- Body format: standard Orivis payload, fields include monitor details and `event`.

#### alertmanager

- URL: typically `http://alertmanager:9093/api/v2/alerts`.
- Body format: JSON array with one alert entry.
- Alert mapping:
  - `monitor_alert` -> `status: firing`
  - `monitor_recovered` -> `status: resolved`

## Examples

Enable base webhook and add one custom route:

```env
ORIVIS_NOTIFICATION__WEBHOOK__ENABLED=true
ORIVIS_NOTIFICATION__WEBHOOK__URL=https://hooks.example.internal/uptime
ORIVIS_NOTIFICATION__WEBHOOK__ROUTES=name=api-notify;type=webhook;url=https://hooks.example.internal/ops;groups=api
```

Send only specific monitors to Alertmanager:

```env
ORIVIS_NOTIFICATION__WEBHOOK__ENABLED=true
ORIVIS_NOTIFICATION__WEBHOOK__ROUTES=name=critical-alerts;type=alertmanager;url=http://alertmanager:9093/api/v2/alerts;monitors=monitor-1|monitor-2
```

## Signature and headers

- `secret` is optional. When set, requests include `X-Orivis-Signature: sha256=<hmac>`.
- Header list supports simple list values from route or default channel.

## Alertmanager labels and annotations

Orivis alertmanager payload includes:

- labels:
  - `alertname=orivis_monitor`
  - `channel=<channel-name>`
  - `monitor_id=<id>`
  - `agent_id=<id>`
  - `region_id=<id>`
  - `environment_id=<id>`
  - `check_status=<status>`
  - `event=<monitor_alert|monitor_recovered>`
  - `status=<up|down|degraded|...>`
  - `resolved=true|false`
  - `severity=critical|info`
- annotations:
  - `summary`
  - `description`

## Troubleshooting

1. Run server with debug log: `ORIVIS_LOG__LEVEL=debug`.
2. Confirm each route has at least `name` and `url` for explicit routes.
3. Confirm payload format:
   - `webhook`: single object.
   - `alertmanager`: array payload.
4. Confirm matching values (`group`, monitor IDs).
5. Check destination endpoint status and retry/timeout logs on delivery failure.
6. Use one test route first (`type=webhook`), then add alertmanager routes.

## Production checklist

- Keep default route scope intentional:
  - Prefer `groups` / `monitors` filters to avoid fan-out.
  - Start with one route and extend gradually.
- Make routes explicit:
  - Use stable `name` values for stable metrics/cardinality.
  - Prefer dedicated routes per sink (`team-ops`, `oncall`, `alertmanager`).
- Reduce alert fatigue:
  - Use `ORIVIS_NOTIFICATION__WEBHOOK__COOLDOWN` to suppress duplicates.
  - Keep `ORIVIS_NOTIFICATION__WEBHOOK__RECOVERYENABLED` enabled to close the loop.
- Verify idempotency at receiver:
  - Alertmanager style receives event-driven transitions; still keep receiver-level duplicate handling.
- Monitor delivery health:
  - Watch server log warnings and queue backpressure.
  - Track retry failures and delivery failures via notification metrics.
- Ensure safe payload compatibility:
  - Validate endpoint accepts `X-Orivis-Signature` when secret is configured.
  - Run a local test endpoint before opening firewall/proxy paths.
- Capacity planning:
  - Increase `ORIVIS_NOTIFICATION__WEBHOOK__QUEUESIZE` if bursty transitions are expected.
  - Tune `RETRYINTERVAL` / `MAXATTEMPTS` with endpoint rate limits in mind.

## Route-level observability

Notification route decisions expose per-route counters in Prometheus style naming:

- `notification_webhook_route_<route>_matched_total`
- `notification_webhook_route_<route>_not_matched_total`
- `notification_webhook_route_<route>_enqueued_total`
- `notification_webhook_route_<route>_delivery_success_total`
- `notification_webhook_route_<route>_delivery_failure_total`

where `<route>` is derived from:

- route name (preferred)
- `name` field from config
- if `name` is missing, `webhook` is used
- spaces and special chars are normalized to `_`

Global metrics:

- `notification_webhook_routes_unrouted_total`: no route matched the current probe result.
- `notification_webhook_delivery_queue_full_total`: delivery buffer full and event was dropped.

Quick interpretation:

- `matched` grows and `not_matched` stays low: most probes are being consumed by that route.
- `not_matched` grows on non-default routes: route filters (`groups`/`monitors`) may be too strict or stale.
- `delivery_failure_total` rises while `enqueued_total` grows: endpoint/network or payload compatibility issue.
- `unrouted_total` rises globally: ensure at least one default route or route scope is correctly configured.

## Production deployment examples

### Docker Compose (single server)

```env
# orivis-server
ORIVIS_NOTIFICATION__WEBHOOK__ENABLED=true
ORIVIS_NOTIFICATION__WEBHOOK__METHOD=POST
ORIVIS_NOTIFICATION__WEBHOOK__ROUTES=name=api-ops;type=webhook;url=https://hooks.example.com/ops;secret=change-me;groups=api
ORIVIS_NOTIFICATION__WEBHOOK__RETRYINTERVAL=10s
ORIVIS_NOTIFICATION__WEBHOOK__MAXATTEMPTS=5
ORIVIS_NOTIFICATION__WEBHOOK__QUEUESIZE=1024

# Optional recovery alerts
ORIVIS_NOTIFICATION__WEBHOOK__RECOVERYENABLED=true
```

If you also run Alertmanager:

```env
ORIVIS_NOTIFICATION__WEBHOOK__ROUTES=name=critical;type=alertmanager;url=http://alertmanager:9093/api/v2/alerts;groups=critical|api
```

### Docker Swarm

Use the same variables in the server task spec. The example below shows an Alertmanager route and an ops route:

```env
ORIVIS_NOTIFICATION__WEBHOOK__ENABLED=true
ORIVIS_NOTIFICATION__WEBHOOK__ROUTES=name=alertmanager;type=alertmanager;url=http://alertmanager:9093/api/v2/alerts;groups=prod|critical
ORIVIS_NOTIFICATION__WEBHOOK__ROUTES=name=ops-webhook;type=webhook;url=https://hooks.example.com/ops;groups=prod|api;secret=${ALERT_WEBHOOK_SECRET}
ORIVIS_NOTIFICATION__WEBHOOK__COOLDOWN=10m
ORIVIS_NOTIFICATION__WEBHOOK__RETRYINTERVAL=8s
```

Tip: in Swarm stacks, keep one webhook route per critical channel and avoid wildcarding all monitors unless you explicitly want broad fan-out.

For a compose-based production template that only enables notifications without changing monitor-related settings, see:

- `deployments/docker-compose/compose.notifications.yml`
- `deployments/docker-compose/server.notifications.env.example`

## Operational troubleshooting (quick playbook)

When notifications are not delivered as expected:

1. Verify notification is enabled:
   - `ORIVIS_NOTIFICATION__WEBHOOK__ENABLED=true`.
2. Verify route parse state in logs:
   - Look for route load/parse logs at startup.
   - Confirm each route has `name` and `url`.
3. Verify monitor-to-route matching:
   - Add temporary explicit `monitors=<monitor-id>` and confirm only expected ones match.
4. Verify event generation:
   - Check dashboard/API transitions from `up -> down` and `down -> up`.
5. Check queue and backpressure:
   - If you see queue full or drops, raise queue size and flush concurrency.
6. Validate downstream reachability:
   - `curl <target>` from the server container/host to ensure endpoint is reachable.
7. Validate payload shape:
   - `webhook`: single JSON object.
   - `alertmanager`: JSON array payload.
8. Validate auth/signature compatibility:
   - Confirm `X-Orivis-Signature` and required headers/headers format at receiver.
9. Check network/policy:
   - In containerized deployments confirm egress, DNS, and proxy rules for server.
10. Run a controlled test:
   - Point one route to a request-capture endpoint first (e.g. webhook.site) to verify payload and retry behavior before production URLs.
