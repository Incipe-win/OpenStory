package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Common errors.
var (
	ErrUserNotFound     = errors.New("user not found")
	ErrEmailExists      = errors.New("email already exists")
	ErrUsernameExists   = errors.New("username already exists")
	ErrTokenNotFound    = errors.New("refresh token not found")
	ErrTokenExpired     = errors.New("refresh token expired")
	ErrTokenRevoked     = errors.New("refresh token revoked")
)

// Repository defines user and token database operations.
type Repository interface {
	CreateUser(ctx context.Context, user *User) error
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*User, error)
	CreateRefreshToken(ctx context.Context, token *RefreshToken) error
	GetRefreshTokenByHash(ctx context.Context, hash string) (*RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, id uuid.UUID) error
}

// PgRepository implements Repository using pgxpool.
type PgRepository struct {
	pool *pgxpool.Pool
}

// NewPgRepository creates a new PostgreSQL-backed auth repository.
func NewPgRepository(pool *pgxpool.Pool) *PgRepository {
	return &PgRepository{pool: pool}
}

func (r *PgRepository) CreateUser(ctx context.Context, user *User) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO users (email, username, password_hash, display_name, role)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, status, created_at, updated_at`,
		user.Email, user.Username, user.PasswordHash, user.DisplayName, user.Role,
	).Scan(&user.ID, &user.Status, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		errMsg := err.Error()
		if contains(errMsg, "uq_users_email") {
			return ErrEmailExists
		}
		if contains(errMsg, "uq_users_username") {
			return ErrUsernameExists
		}
		return fmt.Errorf("inserting user: %w", err)
	}
	return nil
}

func (r *PgRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	u := &User{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, email, username, password_hash, display_name, avatar_url, role, status,
		        email_verified_at, created_at, updated_at
		 FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Email, &u.Username, &u.PasswordHash, &u.DisplayName, &u.AvatarURL,
		&u.Role, &u.Status, &u.EmailVerifiedAt, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying user by email: %w", err)
	}
	return u, nil
}

func (r *PgRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	u := &User{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, email, username, password_hash, display_name, avatar_url, role, status,
		        email_verified_at, created_at, updated_at
		 FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.Username, &u.PasswordHash, &u.DisplayName, &u.AvatarURL,
		&u.Role, &u.Status, &u.EmailVerifiedAt, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying user by id: %w", err)
	}
	return u, nil
}

func (r *PgRepository) CreateRefreshToken(ctx context.Context, token *RefreshToken) error {
	err := r.pool.QueryRow(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, device_info, ip_address, expires_at)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at`,
		token.UserID, token.TokenHash, token.DeviceInfo, token.IPAddress, token.ExpiresAt,
	).Scan(&token.ID, &token.CreatedAt)
	if err != nil {
		return fmt.Errorf("inserting refresh token: %w", err)
	}
	return nil
}

func (r *PgRepository) GetRefreshTokenByHash(ctx context.Context, hash string) (*RefreshToken, error) {
	t := &RefreshToken{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, token_hash, device_info, ip_address, expires_at, revoked_at, created_at
		 FROM refresh_tokens WHERE token_hash = $1`, hash,
	).Scan(&t.ID, &t.UserID, &t.TokenHash, &t.DeviceInfo, &t.IPAddress,
		&t.ExpiresAt, &t.RevokedAt, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTokenNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying refresh token: %w", err)
	}
	if t.RevokedAt != nil {
		return nil, ErrTokenRevoked
	}
	if t.ExpiresAt.Before(time.Now()) {
		return nil, ErrTokenExpired
	}
	return t, nil
}

func (r *PgRepository) RevokeRefreshToken(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("revoking refresh token: %w", err)
	}
	return nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
