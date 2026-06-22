package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"scribe/backend/internal/domain/user"
	"scribe/backend/internal/ports"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

var _ ports.UserRepository = (*PostgresRepository)(nil)
var _ ports.RefreshTokenRepository = (*PostgresRepository)(nil)

func (r *PostgresRepository) Create(ctx context.Context, u *user.User) error {
	query := `
		INSERT INTO users (firebase_uid, email, name, avatar_url, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`
	now := time.Now()
	err := r.db.QueryRowContext(ctx, query, u.FirebaseUID, u.Email, u.Name, u.AvatarURL, now, now).
		Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id string) (*user.User, error) {
	query := `
		SELECT id, firebase_uid, email, name, avatar_url, created_at, updated_at
		FROM users WHERE id = $1
	`
	var u user.User
	err := r.db.QueryRowContext(ctx, query, id).
		Scan(&u.ID, &u.FirebaseUID, &u.Email, &u.Name, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("user not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &u, nil
}

func (r *PostgresRepository) GetByFirebaseUID(ctx context.Context, uid string) (*user.User, error) {
	query := `
		SELECT id, firebase_uid, email, name, avatar_url, created_at, updated_at
		FROM users WHERE firebase_uid = $1
	`
	var u user.User
	err := r.db.QueryRowContext(ctx, query, uid).
		Scan(&u.ID, &u.FirebaseUID, &u.Email, &u.Name, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by firebase uid: %w", err)
	}
	return &u, nil
}

func (r *PostgresRepository) Update(ctx context.Context, u *user.User) error {
	query := `UPDATE users SET name = $1, avatar_url = $2, updated_at = $3 WHERE id = $4`
	now := time.Now()
	_, err := r.db.ExecContext(ctx, query, u.Name, u.AvatarURL, now, u.ID)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	u.UpdatedAt = now
	return nil
}

func (r *PostgresRepository) Save(ctx context.Context, token *user.RefreshToken) error {
	query := `
		INSERT INTO refresh_tokens (user_id, token, expired_at, created_at)
		VALUES ($1, $2, $3, $4) RETURNING id, created_at
	`
	now := time.Now()
	err := r.db.QueryRowContext(ctx, query, token.UserID, token.Token, token.ExpiredAt, now).
		Scan(&token.ID, &token.CreatedAt)
	if err != nil {
		return fmt.Errorf("save refresh token: %w", err)
	}
	return nil
}

func (r *PostgresRepository) GetByToken(ctx context.Context, token string) (*user.RefreshToken, error) {
	query := `SELECT id, user_id, token, expired_at, created_at FROM refresh_tokens WHERE token = $1`
	var t user.RefreshToken
	err := r.db.QueryRowContext(ctx, query, token).
		Scan(&t.ID, &t.UserID, &t.Token, &t.ExpiredAt, &t.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("refresh token not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get refresh token: %w", err)
	}
	return &t, nil
}

func (r *PostgresRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete refresh token: %w", err)
	}
	return nil
}

func (r *PostgresRepository) DeleteByUserID(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("delete refresh tokens by user: %w", err)
	}
	return nil
}
