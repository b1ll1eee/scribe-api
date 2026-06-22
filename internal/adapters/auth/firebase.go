package auth

import (
	"context"
	"fmt"
	"strings"

	"scribe/backend/internal/domain/user"
	"scribe/backend/internal/ports"

	firebase "firebase.google.com/go/v4"
	firebaseAuth "firebase.google.com/go/v4/auth"
	"go.uber.org/zap"
	"google.golang.org/api/option"
)

type FirebaseAuthService struct {
	client   *firebaseAuth.Client
	mockMode bool
	logger   *zap.Logger
}

func NewFirebaseAuthService(credentialJSON string, useMock bool, logger *zap.Logger) (*FirebaseAuthService, error) {
	if useMock || credentialJSON == "" {
		logger.Info("firebase auth initialized in mock mode")
		return &FirebaseAuthService{client: nil, mockMode: true, logger: logger}, nil
	}

	opt := option.WithCredentialsJSON([]byte(credentialJSON))
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		logger.Warn("failed to parse firebase credentials, falling back to mock mode", zap.Error(err))
		return &FirebaseAuthService{client: nil, mockMode: true, logger: logger}, nil
	}

	client, err := app.Auth(context.Background())
	if err != nil {
		logger.Warn("failed to create firebase auth client, falling back to mock mode", zap.Error(err))
		return &FirebaseAuthService{client: nil, mockMode: true, logger: logger}, nil
	}

	logger.Info("firebase auth initialized in production mode")
	return &FirebaseAuthService{client: client, mockMode: false, logger: logger}, nil
}

var _ ports.FirebaseAuthVerifier = (*FirebaseAuthService)(nil)

func (s *FirebaseAuthService) VerifyIDToken(ctx context.Context, idToken string) (*user.User, error) {
	if s.mockMode {
		return s.verifyMockToken(idToken)
	}

	token, err := s.client.VerifyIDToken(ctx, idToken)
	if err != nil {
		return nil, fmt.Errorf("invalid firebase id token: %w", err)
	}

	email, _ := token.Claims["email"].(string)
	name, _ := token.Claims["name"].(string)
	avatar, _ := token.Claims["picture"].(string)

	if email == "" {
		return nil, fmt.Errorf("firebase token missing email claim")
	}

	return &user.User{
		FirebaseUID: token.UID,
		Email:       email,
		Name:        name,
		AvatarURL:   avatar,
	}, nil
}

func (s *FirebaseAuthService) verifyMockToken(token string) (*user.User, error) {
	if token == "" {
		return nil, fmt.Errorf("empty mock authentication token")
	}

	if token == "mock-google-token" {
		return &user.User{
			FirebaseUID: "mock-google-uid-888",
			Email:       "google-developer@scribesapp.local",
			Name:        "Gopher Developer (Google)",
			AvatarURL:   "https://lh3.googleusercontent.com/a/default-user=s96-c",
		}, nil
	}

	if token == "mock-github-token" {
		return &user.User{
			FirebaseUID: "mock-github-uid-999",
			Email:       "github-developer@scribesapp.local",
			Name:        "Octocat Developer (GitHub)",
			AvatarURL:   "https://avatars.githubusercontent.com/u/9919?v=4",
		}, nil
	}

	if strings.HasPrefix(token, "mock-token-") {
		email := strings.TrimPrefix(token, "mock-token-")
		name := strings.Split(email, "@")[0]

		return &user.User{
			FirebaseUID: "mock-uid-" + name,
			Email:       email,
			Name:        name,
			AvatarURL:   "https://avatar.vercel.sh/" + email,
		}, nil
	}

	return nil, fmt.Errorf("invalid mock token format")
}
