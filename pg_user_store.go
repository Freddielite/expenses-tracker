package main

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

// PostgresUserStore is a Postgres-backed implementation of UserStore — a
// drop-in replacement for FileUserStore (users.go), chosen automatically
// when DATABASE_URL is set (see initUserStore in storage.go). Same
// Register/Authenticate/Get/HasAny surface and error semantics; password
// hashing (hashPassword/verifyPassword/pbkdf2) is shared with
// FileUserStore since it doesn't depend on how the record is stored.
type PostgresUserStore struct {
	db *sql.DB
}

func NewPostgresUserStore(db *sql.DB) *PostgresUserStore {
	return &PostgresUserStore{db: db}
}

func (s *PostgresUserStore) Register(email, password string) (*User, error) {
	email = strings.TrimSpace(email)
	if !emailFormat.MatchString(email) {
		return nil, ErrInvalidEmail
	}
	if len(password) < minPasswordLen {
		return nil, ErrPasswordTooShort
	}

	salt, hash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}
	u := &User{
		ID:           newID(),
		Email:        email,
		Salt:         salt,
		PasswordHash: hash,
		CreatedAt:    time.Now().UTC(),
	}

	// The unique index on lower(email) (see schema.go) is what actually
	// enforces case-insensitive uniqueness under concurrent registrations
	// — the pre-check below is just for a clean error message in the
	// common case; a race that slips past it still gets caught by the
	// constraint and reported the same way.
	var exists bool
	if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE lower(email) = lower($1))`, email).Scan(&exists); err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrEmailTaken
	}

	_, err = s.db.Exec(
		`INSERT INTO users (id, email, salt, password_hash, created_at) VALUES ($1, $2, $3, $4, $5)`,
		u.ID, u.Email, u.Salt, u.PasswordHash, u.CreatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrEmailTaken
		}
		return nil, err
	}
	return u, nil
}

func (s *PostgresUserStore) Authenticate(email, password string) (*User, error) {
	u, err := s.getByEmail(email)
	if err != nil {
		return nil, err
	}
	if !verifyPassword(u.Salt, u.PasswordHash, password) {
		return nil, ErrWrongPassword
	}
	return u, nil
}

func (s *PostgresUserStore) getByEmail(email string) (*User, error) {
	var u User
	err := s.db.QueryRow(
		`SELECT id, email, salt, password_hash, created_at FROM users WHERE lower(email) = lower($1)`,
		strings.TrimSpace(email),
	).Scan(&u.ID, &u.Email, &u.Salt, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *PostgresUserStore) Get(id string) (*User, error) {
	var u User
	err := s.db.QueryRow(
		`SELECT id, email, salt, password_hash, created_at FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.Salt, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *PostgresUserStore) HasAny() bool {
	var exists bool
	// EXISTS(SELECT 1 FROM users LIMIT 1) short-circuits on the first
	// row rather than counting the whole table — matters little at
	// household scale, but it's the same cost either way to just do it
	// right.
	if err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM users)`).Scan(&exists); err != nil {
		return false // fail closed — see PostgresCategoryStore.IsEmptyFor for the same reasoning
	}
	return exists
}

// isUniqueViolation checks for Postgres's unique_violation SQLSTATE
// (23505) without importing the pgconn error type directly, so this file
// doesn't need its own pgx import — errors.Is against a plain string
// match on the wrapped error text is good enough here since this only
// gates which of two already-close error messages the person sees.
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "23505")
}
