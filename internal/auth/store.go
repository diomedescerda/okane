package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kenma/okane/internal/domain"
)

var (
	ErrEmailTaken         = errors.New("email is already registered")
	ErrNotFound           = errors.New("not found")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidSession     = errors.New("invalid or expired session")
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) CreateUser(ctx context.Context, email, passwordHash, name string) (domain.User, error) {
	const q = `
		INSERT INTO users (email, password_hash, name)
		VALUES ($1, $2, $3)
		RETURNING id, email, password_hash, name, created_at, updated_at`

	var u domain.User
	err := s.pool.QueryRow(ctx, q, email, passwordHash, name).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.User{}, ErrEmailTaken
		}
		return domain.User{}, fmt.Errorf("insert user: %w", err)
	}
	return u, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	const q = `
		SELECT id, email, password_hash, name, created_at, updated_at
		FROM users WHERE lower(email) = lower($1)`

	var u domain.User
	err := s.pool.QueryRow(ctx, q, email).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("select user: %w", err)
	}
	return u, nil
}

func (s *Store) CreateSession(ctx context.Context, userID uuid.UUID, tokenHash []byte, userAgent, ip string, expiresAt time.Time) (domain.Session, error) {
	const q = `
		INSERT INTO sessions (user_id, token_hash, user_agent, ip, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, user_agent, ip, expires_at, created_at, last_used_at`

	var sess domain.Session
	err := s.pool.QueryRow(ctx, q, userID, tokenHash, userAgent, ip, expiresAt).Scan(
		&sess.ID, &sess.UserID, &sess.UserAgent, &sess.IP, &sess.ExpiresAt, &sess.CreatedAt, &sess.LastUsedAt,
	)
	if err != nil {
		return domain.Session{}, fmt.Errorf("insert session: %w", err)
	}
	return sess, nil
}

// SessionUser returns the user for a session token hash, or ErrInvalidSession.
func (s *Store) SessionUser(ctx context.Context, tokenHash []byte) (domain.User, domain.Session, error) {
	const q = `
		SELECT u.id, u.email, u.password_hash, u.name, u.created_at, u.updated_at,
		       s.id, s.user_id, s.user_agent, s.ip, s.expires_at, s.created_at, s.last_used_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > now()`

	var u domain.User
	var sess domain.Session
	err := s.pool.QueryRow(ctx, q, tokenHash).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.CreatedAt, &u.UpdatedAt,
		&sess.ID, &sess.UserID, &sess.UserAgent, &sess.IP, &sess.ExpiresAt, &sess.CreatedAt, &sess.LastUsedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.Session{}, ErrInvalidSession
	}
	if err != nil {
		return domain.User{}, domain.Session{}, fmt.Errorf("select session: %w", err)
	}
	return u, sess, nil
}

func (s *Store) TouchSession(ctx context.Context, sessionID uuid.UUID, at time.Time) error {
	const q = `UPDATE sessions SET last_used_at = $2 WHERE id = $1`
	_, err := s.pool.Exec(ctx, q, sessionID, at)
	return err
}

func (s *Store) DeleteSessionByHash(ctx context.Context, tokenHash []byte) error {
	const q = `DELETE FROM sessions WHERE token_hash = $1`
	_, err := s.pool.Exec(ctx, q, tokenHash)
	return err
}

// DeleteExpiredSessions removes sessions that expired before the given time.
func (s *Store) DeleteExpiredSessions(ctx context.Context, before time.Time) (int64, error) {
	const q = `DELETE FROM sessions WHERE expires_at < $1`
	tag, err := s.pool.Exec(ctx, q, before)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// normalizeEmail trims and lowercases an address.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
