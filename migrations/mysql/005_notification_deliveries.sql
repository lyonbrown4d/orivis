CREATE TABLE IF NOT EXISTS notification_deliveries (
    id VARCHAR(255) PRIMARY KEY,
    channel TEXT NOT NULL,
    event TEXT NOT NULL,
    monitor_id VARCHAR(255) NOT NULL DEFAULT '',
    agent_id VARCHAR(255) NOT NULL DEFAULT '',
    region_id VARCHAR(255) NOT NULL DEFAULT '',
    environment_id VARCHAR(255) NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    attempt INTEGER NOT NULL,
    max_attempts INTEGER NOT NULL,
    http_status INTEGER NOT NULL DEFAULT 0,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    error_message TEXT NOT NULL DEFAULT '',
    checked_at TEXT NOT NULL,
    sent_at TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX idx_notification_deliveries_created_at ON notification_deliveries(created_at(64));
CREATE INDEX idx_notification_deliveries_monitor_created ON notification_deliveries(monitor_id, created_at(64));
CREATE INDEX idx_notification_deliveries_status_created ON notification_deliveries(status(64), created_at(64));
