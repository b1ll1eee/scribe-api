package analytics

import (
	"errors"
	"strings"
	"time"
)

type AnalyticsEvent struct {
	EventID   string    `json:"event_id"`
	UserID    string    `json:"user_id"`
	EventType string    `json:"event_type"`
	EntityID  string    `json:"entity_id"`
	Metadata  string    `json:"metadata"`
	CreatedAt time.Time `json:"created_at"`
}

const (
	EventUserLogin      = "user_login"
	EventScribeCreated    = "scribe_created"
	EventScribeUpdated    = "scribe_updated"
	EventScribeDeleted    = "scribe_deleted"
	EventScribeArchived   = "scribe_archived"
	EventScribeViewed     = "scribe_viewed"
	EventSearchPerformed = "search_performed"
)

type DashboardData struct {
	DailyActiveUsers  int64           `json:"daily_active_users"`
	MonthlyActiveUsers int64          `json:"monthly_active_users"`
	TotalScribes        int64           `json:"total_scribes"`
	MostViewedScribes   []ScribeViewCount `json:"most_viewed_scribes"`
	ActivityTimeline  []ActivityPoint `json:"activity_timeline"`
}

type ScribeViewCount struct {
	EntityID string `json:"entity_id"`
	Views    int64  `json:"views"`
}

type ActivityPoint struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

func (e *AnalyticsEvent) Validate() error {
	if strings.TrimSpace(e.EventID) == "" {
		return errors.New("event id cannot be empty")
	}
	if strings.TrimSpace(e.UserID) == "" {
		return errors.New("user id cannot be empty")
	}
	if strings.TrimSpace(e.EventType) == "" {
		return errors.New("event type cannot be empty")
	}
	return nil
}
