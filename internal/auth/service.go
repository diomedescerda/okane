package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kenma/okane/internal/domain"
)

const minPasswordLen = 8

// dummyHash is verified against when the user does not exist, so that login
// timing does not reveal whether an email is registered.
var dummyHash = func() string {
	h, err := HashPassword("okane-dummy-password")
	if err != nil {
		panic(err)
	}
	return h
}()

type Service struct {
	pool *pgxpool.Pool
	ttl  time.Duration
}

func NewService(pool *pgxpool.Pool, ttl time.Duration) *Service {
	return &Service{pool: pool, ttl: ttl}
}

func (s *Service) Store() *Store { return NewStore(s.pool) }

func (s *Service) TTL() time.Duration { return s.ttl }

// Register creates a user and a session, returning the session token.
func (s *Service) Register(ctx context.Context, email, password, name string) (domain.User, string, error) {
	email = normalizeEmail(email)
	name = strings.TrimSpace(name)

	if _, err := mail.ParseAddress(email); err != nil || !strings.Contains(email, "@") {
		return domain.User{}, "", fmt.Errorf("enter a valid email address")
	}
	if len(password) < minPasswordLen {
		return domain.User{}, "", fmt.Errorf("password must be at least %d characters", minPasswordLen)
	}

	hash, err := HashPassword(password)
	if err != nil {
		return domain.User{}, "", err
	}

	store := s.Store()
	user, err := store.CreateUser(ctx, email, hash, name)
	if err != nil {
		return domain.User{}, "", err
	}

	token, err := s.createSession(ctx, store, user.ID, "", "")
	if err != nil {
		return domain.User{}, "", err
	}
	return user, token, nil
}

// Login verifies credentials and returns a fresh session token.
func (s *Service) Login(ctx context.Context, email, password, userAgent, ip string) (domain.User, string, error) {
	store := s.Store()
	user, err := store.GetUserByEmail(ctx, normalizeEmail(email))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			VerifyPassword(dummyHash, password) // equalize timing
			return domain.User{}, "", ErrInvalidCredentials
		}
		return domain.User{}, "", err
	}
	if !VerifyPassword(user.PasswordHash, password) {
		return domain.User{}, "", ErrInvalidCredentials
	}

	token, err := s.createSession(ctx, store, user.ID, userAgent, ip)
	if err != nil {
		return domain.User{}, "", err
	}
	return user, token, nil
}

// Authenticate resolves a raw session token to its user.
func (s *Service) Authenticate(ctx context.Context, token string) (domain.User, error) {
	if token == "" {
		return domain.User{}, ErrInvalidSession
	}
	store := s.Store()
	hash := HashToken(token)
	user, sess, err := store.SessionUser(ctx, hash)
	if err != nil {
		return domain.User{}, err
	}
	// Refresh last_used_at at most once every 5 minutes.
	if time.Since(sess.LastUsedAt) > 5*time.Minute {
		if err := store.TouchSession(ctx, sess.ID, time.Now()); err != nil {
			return domain.User{}, err
		}
	}
	return user, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.Store().DeleteSessionByHash(ctx, HashToken(token))
}

func (s *Service) createSession(ctx context.Context, store *Store, userID uuid.UUID, userAgent, ip string) (string, error) {
	token, err := NewToken()
	if err != nil {
		return "", err
	}
	_, err = store.CreateSession(ctx, userID, HashToken(token), userAgent, ip, time.Now().Add(s.ttl))
	if err != nil {
		return "", err
	}
	return token, nil
}

// NewToken returns a 256-bit random, URL-safe session token.
func NewToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashToken returns the SHA-256 digest stored in the sessions table.
func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
