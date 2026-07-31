package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	// pbkdf2Iterations follows the OWASP-recommended minimum for
	// PBKDF2-HMAC-SHA256 as of this writing. Revisit upward over time —
	// this number is meant to keep getting more expensive as hardware
	// does.
	pbkdf2Iterations = 210_000
	pbkdf2KeyLen     = 32
	minPasswordLen   = 8
)

var (
	ErrEmailTaken       = errors.New("an account with that email already exists")
	ErrInvalidEmail     = errors.New("invalid email address")
	ErrPasswordTooShort = errors.New("password must be at least 8 characters")
	ErrUserNotFound     = errors.New("no account with that email")
	ErrWrongPassword    = errors.New("incorrect password")
)

var emailFormat = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// User is one registered account. The password itself is never stored —
// only a salted PBKDF2 hash of it (see hashPassword below).
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Salt         string    `json:"salt"`
	PasswordHash string    `json:"password_hash"`
	CreatedAt    time.Time `json:"created_at"`
}

// UserStore is implemented by both FileUserStore (below, the original
// local-JSON-file backing) and PostgresUserStore (pg_user_store.go). Which
// one gets used is decided once at startup by initUserStore in storage.go,
// the same DATABASE_URL-driven choice already made for every other store
// (transactions, categories, budgets, recurring) — see the doc comment on
// initStores. Before this existed, accounts were always file-backed
// regardless of DATABASE_URL, which meant registered accounts vanished on
// anything that wiped the local working directory (e.g. re-extracting a
// fresh copy of the project) even when Postgres was configured and every
// other kind of data survived that just fine.
type UserStore interface {
	Register(email, password string) (*User, error)
	Authenticate(email, password string) (*User, error)
	Get(id string) (*User, error)
	HasAny() bool
}

// FileUserStore is a thread-safe, file-backed collection of registered
// accounts, indexed by both ID and lowercased email — emails must be
// unique case-insensitively ("A@b.com" and "a@b.com" are the same
// account), but the original casing is kept for display. Used when no
// DATABASE_URL is configured; see UserStore's doc comment above.
type FileUserStore struct {
	mu       sync.RWMutex
	filePath string
	byID     map[string]*User
	byEmail  map[string]*User
}

func NewFileUserStore(filePath string) (*FileUserStore, error) {
	s := &FileUserStore{
		filePath: filePath,
		byID:     make(map[string]*User),
		byEmail:  make(map[string]*User),
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *FileUserStore) load() error {
	data, err := os.ReadFile(s.filePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil // fresh start, no accounts yet
	}
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	var list []*User
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}
	for _, u := range list {
		s.byID[u.ID] = u
		s.byEmail[strings.ToLower(u.Email)] = u
	}
	return nil
}

// persist must be called with s.mu already held.
func (s *FileUserStore) persist() error {
	list := make([]*User, 0, len(s.byID))
	for _, u := range s.byID {
		list = append(list, u)
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

// Register creates a new account. Returns ErrInvalidEmail /
// ErrPasswordTooShort for malformed input, or ErrEmailTaken if the
// (case-insensitive) email is already registered.
func (s *FileUserStore) Register(email, password string) (*User, error) {
	email = strings.TrimSpace(email)
	if !emailFormat.MatchString(email) {
		return nil, ErrInvalidEmail
	}
	if len(password) < minPasswordLen {
		return nil, ErrPasswordTooShort
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := strings.ToLower(email)
	if _, exists := s.byEmail[key]; exists {
		return nil, ErrEmailTaken
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
	s.byID[u.ID] = u
	s.byEmail[key] = u
	if err := s.persist(); err != nil {
		// Roll back the in-memory write so a failed persist doesn't
		// leave the store claiming an account exists that isn't
		// actually saved to disk.
		delete(s.byID, u.ID)
		delete(s.byEmail, key)
		return nil, err
	}
	return u, nil
}

// Authenticate checks a password against a stored account. Deliberately
// returns distinct errors (ErrUserNotFound vs. ErrWrongPassword)
// internally, but callers should generally present both to the user as
// one generic "incorrect email or password" — otherwise a login form
// doubles as a way to check which addresses are registered.
func (s *FileUserStore) Authenticate(email, password string) (*User, error) {
	s.mu.RLock()
	u, ok := s.byEmail[strings.ToLower(strings.TrimSpace(email))]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrUserNotFound
	}
	if !verifyPassword(u.Salt, u.PasswordHash, password) {
		return nil, ErrWrongPassword
	}
	return u, nil
}

// HasAny reports whether at least one account has been registered — used
// by withAuth to decide whether the API should still run in its original
// wide-open "no auth configured yet" mode (see the comment on that
// function for why that mode exists at all).
func (s *FileUserStore) HasAny() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.byID) > 0
}

// Get looks up an account by ID (e.g. from a session's UserID).
func (s *FileUserStore) Get(id string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.byID[id]
	if !ok {
		return nil, ErrUserNotFound
	}
	return u, nil
}

// ---- Password hashing (PBKDF2-HMAC-SHA256, standard library only) ----
//
// The owner/guest PIN hashing in auth.go (single salted SHA-256 round) is
// fine for a 4-8 digit PIN because AuthStore's lockout is what actually
// protects it, not the hash's cost. A password deserves a slow hash on
// its own merits, since someone who gets hold of users.json shouldn't be
// able to brute-force it offline in seconds. golang.org/x/crypto ships
// bcrypt/scrypt/argon2, but this project doesn't otherwise depend on
// anything outside the standard library plus pgx — PBKDF2 needs nothing
// beyond crypto/hmac and crypto/sha256, and is still an accepted choice
// (NIST SP 800-63B) at a high iteration count.

func hashPassword(password string) (salt, hash string, err error) {
	saltBytes := make([]byte, 16)
	if _, err := rand.Read(saltBytes); err != nil {
		return "", "", err
	}
	salt = hex.EncodeToString(saltBytes)
	return salt, pbkdf2(password, salt), nil
}

func verifyPassword(salt, hash, candidate string) bool {
	computed := pbkdf2(candidate, salt)
	return subtle.ConstantTimeCompare([]byte(computed), []byte(hash)) == 1
}

// pbkdf2 implements PBKDF2-HMAC-SHA256 (RFC 8018) directly against
// crypto/hmac and crypto/sha256. pbkdf2KeyLen (32) equals the SHA-256
// output size, so this only ever needs a single block.
func pbkdf2(password, salt string) string {
	saltBytes, _ := hex.DecodeString(salt)
	derived := pbkdf2Block(password, saltBytes, 1)
	return hex.EncodeToString(derived[:pbkdf2KeyLen])
}

func pbkdf2Block(password string, salt []byte, blockNum int) []byte {
	mac := hmac.New(sha256.New, []byte(password))
	blockIndex := []byte{
		byte(blockNum >> 24),
		byte(blockNum >> 16),
		byte(blockNum >> 8),
		byte(blockNum),
	}
	mac.Write(salt)
	mac.Write(blockIndex)
	u := mac.Sum(nil)

	result := make([]byte, len(u))
	copy(result, u)

	for i := 1; i < pbkdf2Iterations; i++ {
		mac.Reset()
		mac.Write(u)
		u = mac.Sum(nil)
		for j := range result {
			result[j] ^= u[j]
		}
	}
	return result
}
