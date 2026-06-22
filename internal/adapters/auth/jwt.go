package auth

import (
	"fmt"
	"time"

	"scribe/backend/internal/domain/user"
	"scribe/backend/internal/ports"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JWTTokenService struct {
	secretKey      []byte
	accessTokenTTL time.Duration
}

func NewJWTTokenService(secret string, expiry time.Duration) *JWTTokenService {
	if secret == "" {
		secret = "scribes-app-super-secret-development-key-change-in-production"
	}
	if expiry == 0 {
		expiry = 15 * time.Minute
	}
	return &JWTTokenService{
		secretKey:      []byte(secret),
		accessTokenTTL: expiry,
	}
}

var _ ports.TokenService = (*JWTTokenService)(nil)

func (s *JWTTokenService) GenerateAccessToken(u *user.User) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":          u.ID,
		"user_id":      u.ID,
		"firebase_uid": u.FirebaseUID,
		"email":        u.Email,
		"name":         u.Name,
		"iat":          now.Unix(),
		"exp":          now.Add(s.accessTokenTTL).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secretKey)
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}
	return signed, nil
}

func (s *JWTTokenService) GenerateRefreshToken(_ *user.User) (string, error) {
	return uuid.NewString() + "-" + uuid.NewString(), nil
}

func (s *JWTTokenService) ValidateAccessToken(tokenStr string) (*user.User, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secretKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid jwt token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("failed to read token claims")
	}

	expVal, ok := claims["exp"].(float64)
	if !ok {
		return nil, fmt.Errorf("expiration claim missing")
	}
	if time.Now().Unix() > int64(expVal) {
		return nil, fmt.Errorf("token has expired")
	}

	userID, _ := claims["user_id"].(string)
	firebaseUID, _ := claims["firebase_uid"].(string)
	email, _ := claims["email"].(string)
	name, _ := claims["name"].(string)

	if userID == "" {
		return nil, fmt.Errorf("token missing user identifier")
	}

	return &user.User{
		ID:          userID,
		FirebaseUID: firebaseUID,
		Email:       email,
		Name:        name,
	}, nil
}
