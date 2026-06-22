package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"scribe/backend/internal/domain/analytics"
	"scribe/backend/internal/domain/user"
	"scribe/backend/internal/ports"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type AuthServiceImpl struct {
	userRepo     ports.UserRepository
	tokenRepo    ports.RefreshTokenRepository
	firebaseAuth ports.FirebaseAuthVerifier
	jwtService   ports.TokenService
	analyticsRepo ports.AnalyticsRepository
	logger       *zap.Logger
}

func NewAuthService(
	userRepo ports.UserRepository,
	tokenRepo ports.RefreshTokenRepository,
	firebaseAuth ports.FirebaseAuthVerifier,
	jwtService ports.TokenService,
	analyticsRepo ports.AnalyticsRepository,
	logger *zap.Logger,
) *AuthServiceImpl {
	return &AuthServiceImpl{
		userRepo:      userRepo,
		tokenRepo:     tokenRepo,
		firebaseAuth:  firebaseAuth,
		jwtService:    jwtService,
		analyticsRepo: analyticsRepo,
		logger:        logger,
	}
}

var _ ports.AuthService = (*AuthServiceImpl)(nil)

func (s *AuthServiceImpl) LoginWithFirebase(ctx context.Context, idToken string) (*user.User, string, string, error) {
	fbUser, err := s.firebaseAuth.VerifyIDToken(ctx, idToken)
	if err != nil {
		return nil, "", "", fmt.Errorf("verify firebase token: %w", err)
	}

	existingUser, err := s.userRepo.GetByFirebaseUID(ctx, fbUser.FirebaseUID)
	if err != nil {
		return nil, "", "", fmt.Errorf("lookup user by firebase uid: %w", err)
	}

	var localUser *user.User
	if existingUser == nil {
		newUser := &user.User{
			FirebaseUID: fbUser.FirebaseUID,
			Email:       fbUser.Email,
			Name:        fbUser.Name,
			AvatarURL:   fbUser.AvatarURL,
		}
		if err := s.userRepo.Create(ctx, newUser); err != nil {
			return nil, "", "", fmt.Errorf("create user: %w", err)
		}
		localUser = newUser
	} else {
		needsUpdate := false
		if fbUser.Name != "" && fbUser.Name != existingUser.Name {
			existingUser.Name = fbUser.Name
			needsUpdate = true
		}
		if fbUser.AvatarURL != "" && fbUser.AvatarURL != existingUser.AvatarURL {
			existingUser.AvatarURL = fbUser.AvatarURL
			needsUpdate = true
		}
		if needsUpdate {
			if err := s.userRepo.Update(ctx, existingUser); err != nil {
				s.logger.Warn("failed to update user profile on login", zap.Error(err))
			}
		}
		localUser = existingUser
	}

	accessToken, err := s.jwtService.GenerateAccessToken(localUser)
	if err != nil {
		return nil, "", "", fmt.Errorf("generate access token: %w", err)
	}

	refreshTokenStr, err := s.jwtService.GenerateRefreshToken(localUser)
	if err != nil {
		return nil, "", "", fmt.Errorf("generate refresh token: %w", err)
	}

	rfToken := &user.RefreshToken{
		UserID:    localUser.ID,
		Token:     refreshTokenStr,
		ExpiredAt: time.Now().Add(7 * 24 * time.Hour),
	}
	if err := s.tokenRepo.Save(ctx, rfToken); err != nil {
		return nil, "", "", fmt.Errorf("save refresh token: %w", err)
	}

	go s.trackEvent(localUser.ID, analytics.EventUserLogin, "", map[string]interface{}{
		"email": localUser.Email,
		"name":  localUser.Name,
	})

	return localUser, accessToken, refreshTokenStr, nil
}

func (s *AuthServiceImpl) RefreshToken(ctx context.Context, refreshTokenStr string) (string, string, error) {
	dbToken, err := s.tokenRepo.GetByToken(ctx, refreshTokenStr)
	if err != nil {
		return "", "", fmt.Errorf("invalid or revoked refresh token")
	}

	if time.Now().After(dbToken.ExpiredAt) {
		_ = s.tokenRepo.Delete(ctx, dbToken.ID)
		return "", "", fmt.Errorf("refresh token expired, please log in again")
	}

	localUser, err := s.userRepo.GetByID(ctx, dbToken.UserID)
	if err != nil {
		return "", "", fmt.Errorf("get user for token refresh: %w", err)
	}

	newAccessToken, err := s.jwtService.GenerateAccessToken(localUser)
	if err != nil {
		return "", "", fmt.Errorf("generate new access token: %w", err)
	}

	newRefreshTokenStr, err := s.jwtService.GenerateRefreshToken(localUser)
	if err != nil {
		return "", "", fmt.Errorf("generate new refresh token: %w", err)
	}

	if err := s.tokenRepo.Delete(ctx, dbToken.ID); err != nil {
		return "", "", fmt.Errorf("delete old refresh token: %w", err)
	}

	newRfToken := &user.RefreshToken{
		UserID:    localUser.ID,
		Token:     newRefreshTokenStr,
		ExpiredAt: time.Now().Add(7 * 24 * time.Hour),
	}
	if err := s.tokenRepo.Save(ctx, newRfToken); err != nil {
		return "", "", fmt.Errorf("save new refresh token: %w", err)
	}

	return newAccessToken, newRefreshTokenStr, nil
}

func (s *AuthServiceImpl) Logout(ctx context.Context, refreshTokenStr string) error {
	dbToken, err := s.tokenRepo.GetByToken(ctx, refreshTokenStr)
	if err != nil {
		return fmt.Errorf("invalid refresh token")
	}

	if err := s.tokenRepo.Delete(ctx, dbToken.ID); err != nil {
		return fmt.Errorf("delete refresh token: %w", err)
	}
	return nil
}

func (s *AuthServiceImpl) GetUserProfile(ctx context.Context, userID string) (*user.User, error) {
	u, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user profile: %w", err)
	}
	return u, nil
}

func (s *AuthServiceImpl) trackEvent(userID, eventType, entityID string, meta map[string]interface{}) {
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
