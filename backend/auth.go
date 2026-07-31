package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"time"
)

const (
	sessionTTL        = 30 * 24 * time.Hour // stay logged in for 30 days
	maxFailedAttempts = 5
	lockoutDuration   = 60 * time.Second

	// RoleOwner is the original single-user role — full read/write access.
	// RoleGuest is a second, more limited role (see guestAllowed in
	// middleware.go) meant for people the owner shares a tunnel link with.
	// RoleUser marks a session issued to a registered account (see
	// users.go) rather than the legacy owner/guest PIN. Until the data
	// stores themselves are scoped per-account — a follow-up piece of
	// work — a "user" session is treated the same as "owner" everywhere
	// that checks role, since guestAllowed only special-cases RoleGuest.
	RoleOwner = "owner"
	RoleGuest = "guest"
	RoleUser  = "user"
)

var ErrLockedOut = errors.New("too many failed attempts, try again shortly")
var ErrWrongPIN = errors.New("incorrect PIN")
var ErrPINAlreadySet = errors.New("a PIN is already set")
var ErrNoPINSet = errors.New("no PIN has been set yet")
var ErrNoGuestPINSet = errors.New("no guest PIN has been set yet")

// pinRecord is a single salted hash. The PIN itself is never stored.
type pinRecord struct {
	Salt string `json:"salt"`
	Hash string `json:"hash"`
}

// pinFile is what's actually persisted to pin.json. The owner fields keep
// their original top-level JSON keys so pre-guest-PIN files still load
// unmodified; the guest fields are new and simply absent (nil) on any file
// written before this feature existed.
type pinFile struct {
	Salt      string `json:"salt"`
	Hash      string `json:"hash"`
	GuestSalt string `json:"guest_salt,omitempty"`
	GuestHash string `json:"guest_hash,omitempty"`
}

type sessionRecord struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	// Role is omitted (empty string) in any sessions.json written before
	// guest PINs existed — normalizeRole treats "" as RoleOwner so those
	// sessions keep working exactly as before across the upgrade.
	Role string `json:"role,omitempty"`
	// UserID identifies which registered account (see users.go) this
	// session belongs to. Empty for the legacy owner/guest PIN sessions,
	// which predate per-account registration and aren't tied to a User
	// record at all.
	UserID string `json:"user_id,omitempty"`
}

type sessionInfo struct {
	ExpiresAt time.Time
	Role      string
	UserID    string
}

// AuthStore holds the owner/guest PIN hashes, active sessions, and
// failed-attempt tracking. Everything except the failed-attempt counter is
// persisted to disk so a server restart doesn't force everyone to re-enter
// their PIN.
type AuthStore struct {
	mu           sync.RWMutex
	pinFile      string
	sessionsFile string

	pin      *pinRecord // owner PIN
	guestPin *pinRecord // nil until the owner sets one up
	sessions map[string]sessionInfo

	failedAttempts int
	lockedUntil    time.Time
}

func normalizeRole(role string) string {
	if role == "" {
		return RoleOwner
	}
	return role
}

func NewAuthStore(pinFile, sessionsFile string) (*AuthStore, error) {
	a := &AuthStore{
		pinFile:      pinFile,
		sessionsFile: sessionsFile,
		sessions:     make(map[string]sessionInfo),
	}
	if err := a.loadPIN(); err != nil {
		return nil, err
	}
	if err := a.loadSessions(); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *AuthStore) loadPIN() error {
	data, err := os.ReadFile(a.pinFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil // no PIN set up yet
	}
	if err != nil {
		return err
	}
	var rec pinFile
	if err := json.Unmarshal(data, &rec); err != nil {
		return err
	}
	if rec.Hash != "" {
		a.pin = &pinRecord{Salt: rec.Salt, Hash: rec.Hash}
	}
	if rec.GuestHash != "" {
		a.guestPin = &pinRecord{Salt: rec.GuestSalt, Hash: rec.GuestHash}
	}
	return nil
}

func (a *AuthStore) loadSessions() error {
	data, err := os.ReadFile(a.sessionsFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	var list []sessionRecord
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}
	now := time.Now()
	for _, s := range list {
		if s.ExpiresAt.After(now) {
			a.sessions[s.Token] = sessionInfo{ExpiresAt: s.ExpiresAt, Role: normalizeRole(s.Role), UserID: s.UserID}
		}
	}
	return nil
}

func (a *AuthStore) persistSessions() error {
	list := make([]sessionRecord, 0, len(a.sessions))
	for token, info := range a.sessions {
		list = append(list, sessionRecord{Token: token, ExpiresAt: info.ExpiresAt, Role: info.Role, UserID: info.UserID})
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.sessionsFile, data, 0644)
}

// IsPINSet reports whether the owner PIN has been configured yet.
func (a *AuthStore) IsPINSet() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.pin != nil
}

// IsGuestPINSet reports whether a guest PIN has been configured yet.
func (a *AuthStore) IsGuestPINSet() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.guestPin != nil
}

// SetupPIN configures the owner PIN for the first time. Refuses to
// overwrite an existing one — use ChangePIN for that, which requires the
// current PIN.
func (a *AuthStore) SetupPIN(pin string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.pin != nil {
		return "", ErrPINAlreadySet
	}

	rec, err := newPINRecord(pin)
	if err != nil {
		return "", err
	}
	a.pin = rec
	if err := a.persistPINLocked(); err != nil {
		return "", err
	}
	return a.createSessionLocked(RoleOwner, "")
}

// ChangePIN replaces the owner PIN, requiring the current one to succeed.
func (a *AuthStore) ChangePIN(currentPIN, newPIN string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.pin == nil {
		return ErrNoPINSet
	}
	if !verifyPIN(a.pin, currentPIN) {
		return ErrWrongPIN
	}
	rec, err := newPINRecord(newPIN)
	if err != nil {
		return err
	}
	a.pin = rec
	return a.persistPINLocked()
}

// SetGuestPIN creates or replaces the guest PIN. Requires the current owner
// PIN, same as ChangePIN — only the owner can grant or change guest access.
// Unlike SetupPIN for the owner PIN, this is fine to call again later to
// rotate the guest PIN; there's no "already set" restriction.
func (a *AuthStore) SetGuestPIN(ownerPIN, guestPIN string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.pin == nil {
		return ErrNoPINSet
	}
	if !verifyPIN(a.pin, ownerPIN) {
		return ErrWrongPIN
	}
	rec, err := newPINRecord(guestPIN)
	if err != nil {
		return err
	}
	a.guestPin = rec
	return a.persistPINLocked()
}

// RemoveGuestPIN turns off guest access entirely and kicks out any guest
// sessions that are currently active — otherwise someone already logged in
// as a guest would keep their access until their session naturally expired,
// up to 30 days later, which defeats the point of revoking it.
func (a *AuthStore) RemoveGuestPIN(ownerPIN string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.pin == nil {
		return ErrNoPINSet
	}
	if !verifyPIN(a.pin, ownerPIN) {
		return ErrWrongPIN
	}
	if a.guestPin == nil {
		return ErrNoGuestPINSet
	}
	a.guestPin = nil
	for token, info := range a.sessions {
		if info.Role == RoleGuest {
			delete(a.sessions, token)
		}
	}
	if err := a.persistPINLocked(); err != nil {
		return err
	}
	return a.persistSessions()
}

// Login checks the PIN against both the owner and guest PIN and, on
// success, issues a new session token tagged with whichever one matched.
// The owner PIN is checked first so that if the two are ever accidentally
// set to the same value, the session comes back as the more-privileged
// role rather than silently downgrading the owner to guest permissions.
// Tracks failed attempts globally (this is a single-user local app) and
// locks out further attempts for a short period after too many failures.
func (a *AuthStore) Login(pin string) (string, string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.pin == nil {
		return "", "", ErrNoPINSet
	}
	if time.Now().Before(a.lockedUntil) {
		return "", "", ErrLockedOut
	}

	role := ""
	switch {
	case verifyPIN(a.pin, pin):
		role = RoleOwner
	case a.guestPin != nil && verifyPIN(a.guestPin, pin):
		role = RoleGuest
	}

	if role == "" {
		a.failedAttempts++
		if a.failedAttempts >= maxFailedAttempts {
			a.lockedUntil = time.Now().Add(lockoutDuration)
			a.failedAttempts = 0
		}
		return "", "", ErrWrongPIN
	}

	a.failedAttempts = 0
	token, err := a.createSessionLocked(role, "")
	return token, role, err
}

// ValidateSession reports whether a token corresponds to an active
// session, and if so, which role and UserID (empty for legacy owner/guest
// PIN sessions) it was issued under.
func (a *AuthStore) ValidateSession(token string) (role string, userID string, ok bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	info, found := a.sessions[token]
	if !found || !time.Now().Before(info.ExpiresAt) {
		return "", "", false
	}
	return info.Role, info.UserID, true
}

// Logout invalidates a single session token.
func (a *AuthStore) Logout(token string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.sessions, token)
	return a.persistSessions()
}

// PruneExpiredSessions removes any sessions past their expiry from memory
// and disk. Sessions are only filtered out at startup otherwise, so a
// long-running process would keep accumulating dead entries in
// sessions.json between restarts without this.
func (a *AuthStore) PruneExpiredSessions() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	changed := false
	for token, info := range a.sessions {
		if now.After(info.ExpiresAt) {
			delete(a.sessions, token)
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return a.persistSessions()
}

func (a *AuthStore) createSessionLocked(role, userID string) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	a.sessions[token] = sessionInfo{ExpiresAt: time.Now().Add(sessionTTL), Role: role, UserID: userID}
	if err := a.persistSessions(); err != nil {
		return "", err
	}
	return token, nil
}

// CreateUserSession issues a session tied to a specific registered
// account (see users.go), used right after a successful registration or
// account login. Exported (unlike createSessionLocked) because it needs
// to take the lock itself — callers are handlers, not other AuthStore
// methods that already hold it.
func (a *AuthStore) CreateUserSession(userID string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.createSessionLocked(RoleUser, userID)
}

func (a *AuthStore) persistPINLocked() error {
	rec := pinFile{}
	if a.pin != nil {
		rec.Salt, rec.Hash = a.pin.Salt, a.pin.Hash
	}
	if a.guestPin != nil {
		rec.GuestSalt, rec.GuestHash = a.guestPin.Salt, a.guestPin.Hash
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.pinFile, data, 0644)
}

func newPINRecord(pin string) (*pinRecord, error) {
	saltBytes := make([]byte, 16)
	if _, err := rand.Read(saltBytes); err != nil {
		return nil, err
	}
	salt := hex.EncodeToString(saltBytes)
	return &pinRecord{Salt: salt, Hash: hashPIN(pin, salt)}, nil
}

func hashPIN(pin, salt string) string {
	sum := sha256.Sum256([]byte(salt + pin))
	return hex.EncodeToString(sum[:])
}

// verifyPIN uses a constant-time comparison so response timing can't leak
// information about how much of the PIN was correct.
func verifyPIN(rec *pinRecord, pin string) bool {
	candidate := hashPIN(pin, rec.Salt)
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(rec.Hash)) == 1
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
