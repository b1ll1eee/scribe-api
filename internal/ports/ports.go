package ports

import (
	"context"
	"scribe/backend/internal/domain/analytics"
	"scribe/backend/internal/domain/scribe"
	"scribe/backend/internal/domain/user"
)

type UserRepository interface {
	Create(ctx context.Context, u *user.User) error
	GetByID(ctx context.Context, id string) (*user.User, error)
	GetByFirebaseUID(ctx context.Context, uid string) (*user.User, error)
	Update(ctx context.Context, u *user.User) error
}

type RefreshTokenRepository interface {
	Save(ctx context.Context, token *user.RefreshToken) error
	GetByToken(ctx context.Context, token string) (*user.RefreshToken, error)
	Delete(ctx context.Context, id string) error
	DeleteByUserID(ctx context.Context, userID string) error
}

type ScribeRepository interface {
	Create(ctx context.Context, n *scribe.Scribe) error
	Update(ctx context.Context, n *scribe.Scribe) error
	GetByID(ctx context.Context, id string, ownerID string) (*scribe.Scribe, error)
	Delete(ctx context.Context, id string, ownerID string) error
	SoftDelete(ctx context.Context, id string, ownerID string) error
	Restore(ctx context.Context, id string, ownerID string) error
	Archive(ctx context.Context, id string, ownerID string) error
	TogglePin(ctx context.Context, id string, ownerID string, pinned bool) error
	List(ctx context.Context, params ScribeListParams) ([]*scribe.Scribe, int, error)
	CountByOwner(ctx context.Context, ownerID string) (int64, error)
}

type ScribeListParams struct {
	OwnerID        string
	Search         string
	Tags           []string
	IsArchived     *bool
	IsPinned       *bool
	IncludeDeleted bool
	Limit          int
	Offset         int
	SortBy         string
	SortOrder      string
}

type TagRepository interface {
	GetOrCreateMany(ctx context.Context, names []string) ([]*scribe.Tag, error)
	AssociateWithScribe(ctx context.Context, scribeID string, tagIDs []string) error
	ClearScribeTags(ctx context.Context, scribeID string) error
}

type AnalyticsRepository interface {
	Save(ctx context.Context, event *analytics.AnalyticsEvent) error
	SaveBatch(ctx context.Context, events []*analytics.AnalyticsEvent) error
	GetDailyActiveUsers(ctx context.Context) (int64, error)
	GetMonthlyActiveUsers(ctx context.Context) (int64, error)
	GetMostViewedScribes(ctx context.Context, limit int) ([]analytics.ScribeViewCount, error)
	GetActivityTimeline(ctx context.Context, days int) ([]analytics.ActivityPoint, error)
}

type FirebaseAuthVerifier interface {
	VerifyIDToken(ctx context.Context, idToken string) (*user.User, error)
}

type TokenService interface {
	GenerateAccessToken(u *user.User) (string, error)
	GenerateRefreshToken(u *user.User) (string, error)
	ValidateAccessToken(tokenStr string) (*user.User, error)
}

type AuthService interface {
	LoginWithFirebase(ctx context.Context, idToken string) (*user.User, string, string, error)
	RefreshToken(ctx context.Context, refreshTokenStr string) (string, string, error)
	Logout(ctx context.Context, refreshTokenStr string) error
	GetUserProfile(ctx context.Context, userID string) (*user.User, error)
}

type ScribeService interface {
	CreateScribe(ctx context.Context, ownerID string, title string, content string, tags []string) (*scribe.Scribe, error)
	UpdateScribe(ctx context.Context, ownerID string, scribeID string, title string, content string, tags []string) (*scribe.Scribe, error)
	GetScribe(ctx context.Context, scribeID string, ownerID string) (*scribe.Scribe, error)
	DeleteScribe(ctx context.Context, scribeID string, ownerID string) error
	RestoreScribe(ctx context.Context, scribeID string, ownerID string) (*scribe.Scribe, error)
	ArchiveScribe(ctx context.Context, scribeID string, ownerID string) (*scribe.Scribe, error)
	TogglePinScribe(ctx context.Context, scribeID string, ownerID string) (*scribe.Scribe, error)
	ListScribes(ctx context.Context, params ScribeListParams) ([]*scribe.Scribe, int, error)
}

type AnalyticsService interface {
	TrackEvent(ctx context.Context, event *analytics.AnalyticsEvent)
	GetDashboard(ctx context.Context, userID string) (*analytics.DashboardData, error)
}
