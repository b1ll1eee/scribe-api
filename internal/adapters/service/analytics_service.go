package service

import (
	"context"
	"fmt"
	"time"

	"scribe/backend/internal/domain/analytics"
	"scribe/backend/internal/ports"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type AnalyticsServiceImpl struct {
	analyticsRepo ports.AnalyticsRepository
	scribeRepo      ports.ScribeRepository
	logger        *zap.Logger
}

func NewAnalyticsService(
	analyticsRepo ports.AnalyticsRepository,
	scribeRepo ports.ScribeRepository,
	logger *zap.Logger,
) *AnalyticsServiceImpl {
	return &AnalyticsServiceImpl{
		analyticsRepo: analyticsRepo,
		scribeRepo:      scribeRepo,
		logger:        logger,
	}
}

var _ ports.AnalyticsService = (*AnalyticsServiceImpl)(nil)

func (s *AnalyticsServiceImpl) TrackEvent(ctx context.Context, event *analytics.AnalyticsEvent) {
	if event.EventID == "" {
		event.EventID = uuid.NewString()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}
	if err := s.analyticsRepo.Save(ctx, event); err != nil {
		s.logger.Warn("failed to save analytics event",
			zap.String("event_type", event.EventType),
			zap.Error(err))
	}
}

func (s *AnalyticsServiceImpl) GetDashboard(ctx context.Context, userID string) (*analytics.DashboardData, error) {
	dau, err := s.analyticsRepo.GetDailyActiveUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("get daily active users: %w", err)
	}

	mau, err := s.analyticsRepo.GetMonthlyActiveUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("get monthly active users: %w", err)
	}

	totalScribes, err := s.scribeRepo.CountByOwner(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("count user scribes: %w", err)
	}

	mostViewed, err := s.analyticsRepo.GetMostViewedScribes(ctx, 10)
	if err != nil {
		s.logger.Warn("failed to get most viewed scribes", zap.Error(err))
		mostViewed = []analytics.ScribeViewCount{}
	}

	timeline, err := s.analyticsRepo.GetActivityTimeline(ctx, 30)
	if err != nil {
		s.logger.Warn("failed to get activity timeline", zap.Error(err))
		timeline = []analytics.ActivityPoint{}
	}

	return &analytics.DashboardData{
		DailyActiveUsers:   dau,
		MonthlyActiveUsers: mau,
		TotalScribes:         totalScribes,
		MostViewedScribes:    mostViewed,
		ActivityTimeline:   timeline,
	}, nil
}
