CREATE TABLE IF NOT EXISTS analytics_events (
    event_id UUID,
    user_id UUID,
    event_type LowCardinality(String),
    entity_id String DEFAULT '',
    metadata String DEFAULT '{}',
    created_at DateTime64(3, 'UTC')
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(created_at)
ORDER BY (event_type, user_id, created_at)
TTL toDateTime(created_at) + INTERVAL 1 YEAR;
