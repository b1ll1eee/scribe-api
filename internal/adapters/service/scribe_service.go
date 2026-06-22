package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"scribe/backend/internal/domain/analytics"
	"scribe/backend/internal/domain/scribe"
	"scribe/backend/internal/ports"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type ScribeServiceImpl struct {
	scribeRepo      ports.ScribeRepository
	tagRepo       ports.TagRepository
	analyticsRepo ports.AnalyticsRepository
	logger        *zap.Logger
}

func NewScribeService(
	scribeRepo ports.ScribeRepository,
	tagRepo ports.TagRepository,
	analyticsRepo ports.AnalyticsRepository,
	logger *zap.Logger,
) *ScribeServiceImpl {
	return &ScribeServiceImpl{
		scribeRepo:      scribeRepo,
		tagRepo:       tagRepo,
		analyticsRepo: analyticsRepo,
		logger:        logger,
	}
}

var _ ports.ScribeService = (*ScribeServiceImpl)(nil)

func (s *ScribeServiceImpl) CreateScribe(ctx context.Context, ownerID, title, content string, tags []string) (*scribe.Scribe, error) {
	n := &scribe.Scribe{
		OwnerID: ownerID,
		Title:   title,
		Content: content,
	}
	if err := n.Validate(); err != nil {
		return nil, fmt.Errorf("validate scribe: %w", err)
	}

	if err := s.scribeRepo.Create(ctx, n); err != nil {
		return nil, fmt.Errorf("create scribe: %w", err)
	}

	if err := s.associateTags(ctx, n, tags); err != nil {
		return nil, err
	}

	go s.trackEvent(ownerID, analytics.EventScribeCreated, n.ID, map[string]interface{}{
		"title_length": len(n.Title),
		"tags_count":   len(n.Tags),
	})

	return n, nil
}

func (s *ScribeServiceImpl) UpdateScribe(ctx context.Context, ownerID, scribeID, title, content string, tags []string) (*scribe.Scribe, error) {
	n, err := s.scribeRepo.GetByID(ctx, scribeID, ownerID)
	if err != nil {
		return nil, fmt.Errorf("get scribe for update: %w", err)
	}

	n.Title = title
	n.Content = content
	if err := n.Validate(); err != nil {
		return nil, fmt.Errorf("validate scribe: %w", err)
	}

	if err := s.scribeRepo.Update(ctx, n); err != nil {
		return nil, fmt.Errorf("update scribe: %w", err)
	}

	if err := s.tagRepo.ClearScribeTags(ctx, n.ID); err != nil {
		return nil, fmt.Errorf("clear scribe tags: %w", err)
	}

	if err := s.associateTags(ctx, n, tags); err != nil {
		return nil, err
	}

	go s.trackEvent(ownerID, analytics.EventScribeUpdated, n.ID, map[string]interface{}{
		"title_length": len(n.Title),
		"tags_count":   len(n.Tags),
	})

	return n, nil
}

func (s *ScribeServiceImpl) GetScribe(ctx context.Context, scribeID, ownerID string) (*scribe.Scribe, error) {
	n, err := s.scribeRepo.GetByID(ctx, scribeID, ownerID)
	if err != nil {
		return nil, fmt.Errorf("get scribe: %w", err)
	}

	go s.trackEvent(ownerID, analytics.EventScribeViewed, n.ID, nil)

	return n, nil
}

func (s *ScribeServiceImpl) DeleteScribe(ctx context.Context, scribeID, ownerID string) error {
	if err := s.scribeRepo.SoftDelete(ctx, scribeID, ownerID); err != nil {
		return fmt.Errorf("delete scribe: %w", err)
	}

	go s.trackEvent(ownerID, analytics.EventScribeDeleted, scribeID, nil)

	return nil
}

func (s *ScribeServiceImpl) RestoreScribe(ctx context.Context, scribeID, ownerID string) (*scribe.Scribe, error) {
	if err := s.scribeRepo.Restore(ctx, scribeID, ownerID); err != nil {
		return nil, fmt.Errorf("restore scribe: %w", err)
	}

	n, err := s.scribeRepo.GetByID(ctx, scribeID, ownerID)
	if err != nil {
		return nil, fmt.Errorf("get restored scribe: %w", err)
	}
	return n, nil
}

func (s *ScribeServiceImpl) ArchiveScribe(ctx context.Context, scribeID, ownerID string) (*scribe.Scribe, error) {
	if err := s.scribeRepo.Archive(ctx, scribeID, ownerID); err != nil {
		return nil, fmt.Errorf("archive scribe: %w", err)
	}

	n, err := s.scribeRepo.GetByID(ctx, scribeID, ownerID)
	if err != nil {
		return nil, fmt.Errorf("get archived scribe: %w", err)
	}

	go s.trackEvent(ownerID, analytics.EventScribeArchived, scribeID, nil)

	return n, nil
}

func (s *ScribeServiceImpl) TogglePinScribe(ctx context.Context, scribeID, ownerID string) (*scribe.Scribe, error) {
	existing, err := s.scribeRepo.GetByID(ctx, scribeID, ownerID)
	if err != nil {
		return nil, fmt.Errorf("get scribe for pin toggle: %w", err)
	}

	newPinState := !existing.IsPinned
	if err := s.scribeRepo.TogglePin(ctx, scribeID, ownerID, newPinState); err != nil {
		return nil, fmt.Errorf("toggle pin: %w", err)
	}

	existing.IsPinned = newPinState
	return existing, nil
}

func (s *ScribeServiceImpl) ListScribes(ctx context.Context, params ports.ScribeListParams) ([]*scribe.Scribe, int, error) {
	if params.Limit <= 0 {
		params.Limit = 10
	}
	if params.Offset < 0 {
		params.Offset = 0
	}

	scribes, total, err := s.scribeRepo.List(ctx, params)
	if err != nil {
		return nil, 0, fmt.Errorf("list scribes: %w", err)
	}

	if params.Search != "" {
		go s.trackEvent(params.OwnerID, analytics.EventSearchPerformed, "", map[string]interface{}{
			"query":       params.Search,
			"result_count": total,
		})
	}

	return scribes, total, nil
}

func (s *ScribeServiceImpl) associateTags(ctx context.Context, n *scribe.Scribe, tags []string) error {
	if len(tags) > 0 {
		created, err := s.tagRepo.GetOrCreateMany(ctx, tags)
		if err != nil {
			return fmt.Errorf("get or create tags: %w", err)
		}
		var tagIDs, tagNames []string
		for _, t := range created {
			tagIDs = append(tagIDs, t.ID)
			tagNames = append(tagNames, t.Name)
		}
		if err := s.tagRepo.AssociateWithScribe(ctx, n.ID, tagIDs); err != nil {
			return fmt.Errorf("associate tags: %w", err)
		}
		n.Tags = tagNames
	} else {
		n.Tags = []string{}
	}
	return nil
}

func (s *ScribeServiceImpl) trackEvent(userID, eventType, entityID string, meta map[string]interface{}) {
	metaJSON, _ := json.Marshal(meta)
	event := &analytics.AnalyticsEvent{
		EventID:   uuid.NewString(),
		UserID:    userID,
		EventType: eventType,
		EntityID:  entityID,
		Metadata:  string(metaJSON),
		CreatedAt: time.Now(),
	}
	if err := s.analyticsRepo.Save(context.Background(), event); err != nil {
		s.logger.Warn("failed to track analytics event",
			zap.String("event_type", eventType),
			zap.Error(err))
	}
}
