CREATE DATABASE IF NOT EXISTS analytics;

CREATE TABLE IF NOT EXISTS analytics.analytics_events (
    timestamp DateTime,
    event_name LowCardinality(String),
    entity_type LowCardinality(String),
    entity_id String,

    actor_user_id String,
    actor_entity_id String,
    actor_entity_type LowCardinality(String),

    session_id String,

    source LowCardinality(String),
    platform LowCardinality(String),
    app_version String,
    device_type LowCardinality(String),

    user_agent String,
    ip_hash String,

    properties String
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(timestamp)
ORDER BY (event_name, entity_type, entity_id, timestamp)
SETTINGS index_granularity = 8192;
