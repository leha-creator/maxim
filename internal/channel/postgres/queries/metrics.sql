-- Суточные снимки метрик каналов (04 §3, S-32, R-19).

-- name: UpsertMetrics :exec
-- Повтор задания в те же сутки перезаписывает снимок (R-19).
INSERT INTO channel_metrics (id, channel_id, date, subscribers_count, avg_views, engagement_bp,
    posts_per_week, anomaly_flags, created_at, updated_at)
VALUES (@id, @channel_id, @date, @subscribers_count, sqlc.narg(avg_views), sqlc.narg(engagement_bp),
    sqlc.narg(posts_per_week)::float8, @anomaly_flags, @now::timestamptz, @now::timestamptz)
ON CONFLICT (channel_id, date) DO UPDATE
SET subscribers_count = EXCLUDED.subscribers_count, avg_views = EXCLUDED.avg_views,
    engagement_bp = EXCLUDED.engagement_bp, posts_per_week = EXCLUDED.posts_per_week,
    anomaly_flags = EXCLUDED.anomaly_flags, updated_at = EXCLUDED.updated_at;

-- name: LastMetricsBefore :one
SELECT date, subscribers_count, avg_views, engagement_bp, posts_per_week, anomaly_flags
FROM channel_metrics
WHERE channel_id = @channel_id AND date < @date
ORDER BY date DESC
LIMIT 1;

-- name: RecentMetrics :many
SELECT date, subscribers_count, avg_views, engagement_bp, posts_per_week, anomaly_flags
FROM channel_metrics
WHERE channel_id = @channel_id
ORDER BY date DESC
LIMIT @lim;
