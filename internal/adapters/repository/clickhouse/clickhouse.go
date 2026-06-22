package clickhouse

import (
	"context"
	"fmt"
	"time"

	"scribe/backend/internal/domain/analytics"
	"scribe/backend/internal/ports"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
)

type ClickHouseRepository struct {
	conn clickhouse.Conn
}

func NewClickHouseRepository(addr, dbName, username, password string) (*ClickHouseRepository, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{addr},
		Auth: clickhouse.Auth{
			Database: dbName,
			Username: username,
			Password: password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("open clickhouse connection: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping clickhouse: %w", err)
	}

	return &ClickHouseRepository{conn: conn}, nil
}

var _ ports.AnalyticsRepository = (*ClickHouseRepository)(nil)

func (r *ClickHouseRepository) Save(ctx context.Context, e *analytics.AnalyticsEvent) error {
	return r.SaveBatch(ctx, []*analytics.AnalyticsEvent{e})
}

func (r *ClickHouseRepository) SaveBatch(ctx context.Context, events []*analytics.AnalyticsEvent) error {
	if len(events) == 0 {
		return nil
	}

	batch, err := r.conn.PrepareBatch(ctx, "INSERT INTO analytics_events (event_id, user_id, event_type, entity_id, metadata, created_at)")
	if err != nil {
		return fmt.Errorf("prepare clickhouse batch: %w", err)
	}

	for _, e := range events {
		eventID := parseOrNewUUID(e.EventID)
		userID := parseOrNewUUID(e.UserID)

		if err := batch.Append(eventID, userID, e.EventType, e.EntityID, e.Metadata, e.CreatedAt); err != nil {
			return fmt.Errorf("append to clickhouse batch: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("send clickhouse batch: %w", err)
	}
	return nil
}

func (r *ClickHouseRepository) GetDailyActiveUsers(ctx context.Context) (int64, error) {
	var count int64
	err := r.conn.QueryRow(ctx, `SELECT uniq(user_id) FROM analytics_events WHERE created_at >= now() - INTERVAL 1 DAY`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("query daily active users: %w", err)
	}
	return count, nil
}

func (r *ClickHouseRepository) GetMonthlyActiveUsers(ctx context.Context) (int64, error) {
	var count int64
	err := r.conn.QueryRow(ctx, `SELECT uniq(user_id) FROM analytics_events WHERE created_at >= now() - INTERVAL 30 DAY`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("query monthly active users: %w", err)
	}
	return count, nil
}

func (r *ClickHouseRepository) GetMostViewedScribes(ctx context.Context, limit int) ([]analytics.ScribeViewCount, error) {
	rows, err := r.conn.Query(ctx,
		`SELECT entity_id, count() AS views FROM analytics_events WHERE event_type = 'scribe_viewed' AND entity_id != '' GROUP BY entity_id ORDER BY views DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query most viewed scribes: %w", err)
	}
	defer rows.Close()

	var results []analytics.ScribeViewCount
	for rows.Next() {
		var item analytics.ScribeViewCount
		if err := rows.Scan(&item.EntityID, &item.Views); err != nil {
			return nil, fmt.Errorf("scan most viewed scribe: %w", err)
		}
		results = append(results, item)
	}
	return results, nil
}

func (r *ClickHouseRepository) GetActivityTimeline(ctx context.Context, days int) ([]analytics.ActivityPoint, error) {
	rows, err := r.conn.Query(ctx,
		`SELECT toDate(created_at) AS date, count() AS cnt FROM analytics_events WHERE created_at >= now() - INTERVAL ? DAY GROUP BY date ORDER BY date ASC`, days)
	if err != nil {
		return nil, fmt.Errorf("query activity timeline: %w", err)
	}
	defer rows.Close()

	var results []analytics.ActivityPoint
	for rows.Next() {
		var item analytics.ActivityPoint
		var date time.Time
		if err := rows.Scan(&date, &item.Count); err != nil {
			return nil, fmt.Errorf("scan activity point: %w", err)
		}
		item.Date = date.Format("2006-01-02")
		results = append(results, item)
	}
	return results, nil
}

func (r *ClickHouseRepository) Close() error {
	return r.conn.Close()
}

func parseOrNewUUID(s string) uuid.UUID {
	if id, err := uuid.Parse(s); err == nil {
		return id
	}
	return uuid.New()
}
