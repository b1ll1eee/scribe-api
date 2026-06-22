package postgres

import (
	"context"

	"scribe/backend/internal/domain/scribe"
	"scribe/backend/internal/ports"
)

type ScribeRepositoryAdapter struct {
	repo *PostgresRepository
}

func NewScribeRepositoryAdapter(repo *PostgresRepository) *ScribeRepositoryAdapter {
	return &ScribeRepositoryAdapter{repo: repo}
}

var _ ports.ScribeRepository = (*ScribeRepositoryAdapter)(nil)

func (a *ScribeRepositoryAdapter) Create(ctx context.Context, n *scribe.Scribe) error {
	return a.repo.CreateScribe(ctx, n)
}

func (a *ScribeRepositoryAdapter) Update(ctx context.Context, n *scribe.Scribe) error {
	return a.repo.UpdateScribe(ctx, n)
}

func (a *ScribeRepositoryAdapter) GetByID(ctx context.Context, id string, ownerID string) (*scribe.Scribe, error) {
	return a.repo.GetScribeByID(ctx, id, ownerID)
}

func (a *ScribeRepositoryAdapter) Delete(ctx context.Context, id string, ownerID string) error {
	return a.repo.DeleteScribe(ctx, id, ownerID)
}

func (a *ScribeRepositoryAdapter) SoftDelete(ctx context.Context, id string, ownerID string) error {
	return a.repo.SoftDeleteScribe(ctx, id, ownerID)
}

func (a *ScribeRepositoryAdapter) Restore(ctx context.Context, id string, ownerID string) error {
	return a.repo.RestoreScribe(ctx, id, ownerID)
}

func (a *ScribeRepositoryAdapter) Archive(ctx context.Context, id string, ownerID string) error {
	return a.repo.ArchiveScribe(ctx, id, ownerID)
}

func (a *ScribeRepositoryAdapter) TogglePin(ctx context.Context, id string, ownerID string, pinned bool) error {
	return a.repo.TogglePinScribe(ctx, id, ownerID, pinned)
}

func (a *ScribeRepositoryAdapter) List(ctx context.Context, params ports.ScribeListParams) ([]*scribe.Scribe, int, error) {
	return a.repo.ListScribes(ctx, params)
}

func (a *ScribeRepositoryAdapter) CountByOwner(ctx context.Context, ownerID string) (int64, error) {
	return a.repo.CountByOwner(ctx, ownerID)
}
