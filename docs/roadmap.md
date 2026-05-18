# Orivis roadmap

This roadmap captures the next production-hardening work items for Orivis. Priorities are ordered by expected impact on single-node reliability, agent/server throughput, and operational usefulness.

## P0

### Store batch write path

Status: initial implementation completed. `ResultStore.RecordBatch` now preloads monitor-agent assignments and monitor metadata for the incoming batch instead of querying once per result.

- Remove the current result-ingest N+1 pattern in `ResultStore.RecordBatch`.
- Batch preload monitor-agent assignments and monitor metadata for all `monitor_id + agent_id` pairs in the incoming batch.
- Reuse the preloaded records while preparing probe result rows.
- Keep authorization semantics unchanged: a result can only be recorded for a monitor assigned to the reporting agent.

Discussion points:

- Whether to keep the existing repository abstraction for preload queries or add one dedicated read model query for ingest.
- Whether invalid rows in a mixed batch should fail the whole batch or fall back to per-row recording.

### Notification delivery reliability

Status: initial implementation completed. Webhook notifications now use an internal bounded delivery queue with retry and bounded backoff.

- Add an internal delivery queue for webhook notifications.
- Add retry with bounded exponential backoff.
- Add delivery timeout and maximum attempts.
- Keep ingest and event handling non-blocking when notification delivery is slow.

Discussion points:

- Whether delivery attempts should be persisted in SQLite in the first implementation.
- Whether failed notifications should be visible in the dashboard immediately or only via logs/metrics.

### Webhook security and customization

Status: initial implementation completed. Webhook notifications now support configured headers and optional HMAC-SHA256 signatures through `X-Orivis-Signature`.

- Support configured request headers for webhook channels.
- Add optional payload signing, for example `X-Orivis-Signature`.
- Keep the default webhook shape simple and stable.

Discussion points:

- Signature algorithm: HMAC-SHA256 is the likely default.
- Header config shape: environment-friendly flat keys versus file-only structured config.

## P1

### Dashboard cache invalidation

Status: initial implementation completed. Ingest now invalidates the dashboard snapshot cache after successful result recording while keeping TTL cache as fallback.

- Keep TTL cache as a fallback.
- Invalidate dashboard snapshot cache after successful probe result flush.
- Invalidate only known dashboard keys first, then consider wildcard/prefix invalidation if needed.

Discussion points:

- Whether the cache interface should grow `DeletePrefix` or dashboard should maintain explicit known keys.
- Whether invalidation should happen in ingest or via an event subscriber.

### Ingest queue observability

Status: initial implementation completed. Ingest now records queue length, queue-full count, flush batch size, flush duration, and record errors through `observabilityx`.

- Add Prometheus metrics for queue length, queue full count, flush batch size, flush duration, and record errors.
- Expose enough labels to debug without creating high-cardinality metrics.

Discussion points:

- Metric names and label shape should stay stable before a beta release.
- Queue length can be sampled on flush instead of updated on every enqueue.

### Agent offline buffering

Status: initial implementation completed. Agents now keep a bounded FIFO buffer for failed result reports and drain it after server connectivity returns. The default driver is memory; `buffer.driver=file` enables a JSONL file-backed spool.

- Add bounded local buffering when the server is unavailable.
- First implementation can be memory-only.
- Later implementation can optionally use file-backed spool storage.

Discussion points:

- Whether offline buffering should preserve every probe result or only the latest status per monitor.
- Whether memory buffering should be enabled by default.

### Probe scheduling jitter

Status: initial implementation completed. Agent probe jobs now support `poll.jitter` and use stable per-monitor initial jitter to reduce synchronized startup checks.

- Add initial jitter to scheduled probe execution.
- Keep configured intervals stable after startup.
- Avoid thundering herd behavior when many agents start together.

Discussion points:

- Jitter should probably be a percentage of interval with a small maximum cap.
- Need deterministic mode for tests and local debugging.

## P2

### More production probes

Status: initial implementation completed for MongoDB, RabbitMQ/AMQP, NATS, and Kafka using existing Go client libraries instead of hand-rolled protocol commands. TLS/cert probes now report `degraded` before expiry and `down` after expiry.

- Add MongoDB probe.
- Add RabbitMQ/AMQP probe.
- Add NATS probe.
- Upgrade Kafka from basic connectivity to broker metadata checks.
- Enhance TLS/cert probe with certificate expiry thresholds and degraded status.

Discussion points:

- Each protocol probe should use the official or de-facto standard Go client where practical.
- Avoid heavy transitive dependencies unless the probe requires real protocol semantics.

### Dashboard API efficiency

Status: initial implementation completed. Snapshot endpoints now emit stable semantic ETags and the React client sends `If-None-Match` during polling, reusing cached data on `304 Not Modified`.

- Add `ETag` or `Last-Modified` support for dashboard snapshot endpoints.
- Consider splitting summary and history into separate endpoints.
- Keep React Query polling, but reduce payload churn.

Discussion points:

- Snapshot payload compatibility matters because the frontend is now separated.
- ETag should include group and result-limit inputs.

### Notification history UI

Status: initial implementation completed. Webhook delivery attempts are now persisted and surfaced in the dashboard snapshot and UI sidebar.

- Persist notification delivery attempts.
- Show recent notification status in the dashboard.
- Provide enough detail to debug webhook failures without reading server logs.

Discussion points:

- This likely depends on the P0 delivery queue decision.
- A simple append-only table is enough for the first version.

## Current recommended next batch

1. Notification routing: add multiple webhook channels and monitor/group-level routing.
2. Dashboard detail views: add per-monitor history and notification delivery drill-down pages.
3. Agent file spool hardening: add compaction metrics and corruption recovery.
