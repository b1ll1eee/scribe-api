package user

import (
	"errors"
	"strings"
	"time"
)

type User struct {
	ID          string    `json:"id"`
	FirebaseUID string    `json:"firebase_uid"`
	Email       string    `json:"email"`
	Name        string    `json:"name"`
	AvatarURL   string    `json:"avatar_url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type RefreshToken struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Token     string    `json:"token"`
	ExpiredAt time.Time `json:"expired_at"`
	CreatedAt time.Time `json:"created_at"`
}

func (u *User) Validate() error {
	if strings.TrimSpace(u.FirebaseUID) == "" {
		return errors.New("firebase uid cannot be empty")
	}
	if strings.TrimSpace(u.Email) == "" {
		return errors.New("email cannot be empty")
	}
	if !strings.Contains(u.Email, "@") {
		return errors.New("invalid email address")
	}
	return nil
}
