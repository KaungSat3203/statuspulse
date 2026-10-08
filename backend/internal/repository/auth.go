package repository

import (
	"context"
	"errors"
	"time"

	"github.com/KaungSat3203/statuspulse/backend/internal/auth"
	"github.com/KaungSat3203/statuspulse/backend/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthRepository struct{ db *pgxpool.Pool }

func NewAuthRepository(db *pgxpool.Pool) *AuthRepository { return &AuthRepository{db: db} }

func (r *AuthRepository) CreateUser(ctx context.Context, email, hash string) (*models.User, error) {
	user := &models.User{}
	err := r.db.QueryRow(ctx, `INSERT INTO users(email, password_hash) VALUES($1,$2)
        RETURNING id,email,password_hash,created_at`, email, hash).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return nil, auth.ErrDuplicateEmail
	}
	return user, err
}

func (r *AuthRepository) FindUser(ctx context.Context, email string) (*models.User, error) {
	user := &models.User{}
	err := r.db.QueryRow(ctx, `SELECT id,email,password_hash,created_at FROM users WHERE lower(email)=$1`, email).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, auth.ErrUnauthenticated
	}
	return user, err
}

func (r *AuthRepository) CreateSession(ctx context.Context, hash []byte, userID string, expires time.Time) error {
	_, err := r.db.Exec(ctx, `INSERT INTO sessions(token_hash,user_id,expires_at) VALUES($1,$2,$3)`, hash, userID, expires)
	return err
}

func (r *AuthRepository) FindSession(ctx context.Context, hash []byte, now time.Time) (*models.Session, error) {
	session := &models.Session{}
	err := r.db.QueryRow(ctx, `SELECT u.id,u.email,s.expires_at FROM sessions s
        JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>$2`, hash, now).
		Scan(&session.User.ID, &session.User.Email, &session.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, auth.ErrUnauthenticated
	}
	return session, err
}

func (r *AuthRepository) RevokeSession(ctx context.Context, hash []byte, now time.Time) error {
	_, err := r.db.Exec(ctx, `UPDATE sessions SET revoked_at=COALESCE(revoked_at,$2) WHERE token_hash=$1`, hash, now)
	return err
}

func (r *AuthRepository) TakeRateLimit(ctx context.Context, hash []byte, limit int, window time.Duration) (bool, error) {
	var allowed bool
	err := r.db.QueryRow(ctx, `INSERT INTO auth_rate_limits(key_hash,attempts,expires_at)
        VALUES($1,1,clock_timestamp()+$3*interval '1 second')
        ON CONFLICT(key_hash) DO UPDATE SET
            attempts=CASE WHEN auth_rate_limits.expires_at<=clock_timestamp() THEN 1
                ELSE LEAST(auth_rate_limits.attempts+1,$2+1) END,
            expires_at=CASE WHEN auth_rate_limits.expires_at<=clock_timestamp()
                THEN clock_timestamp()+$3*interval '1 second' ELSE auth_rate_limits.expires_at END
        RETURNING attempts<=$2`, hash, limit, window.Seconds()).Scan(&allowed)
	return allowed, err
}
