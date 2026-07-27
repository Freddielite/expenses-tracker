package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"
)

var pinFormat = regexp.MustCompile(`^\d{4,8}$`)

type pinRequest struct {
	PIN string `json:"pin"`
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type accountLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// HandleAuthRegister creates a new account and immediately signs it in —
// same "success returns a live session" pattern as HandleAuthSetup for
// the owner PIN — so the frontend can go straight from the registration
// form into the app without a separate login round trip.
//
// The account gets its own private tenant (see tenant.go): completely
// separate from the shared legacy owner/guest PIN household and from
// every other registered account. A fresh account starts with zero
// transactions/categories/etc — except categories, which are seeded with
// the same defaults a brand-new PIN household gets (see defaultCategories
// in seed.go), so there's something to pick from immediately.
func (a *API) HandleAuthRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	user, err := a.users.Register(req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidEmail):
			writeError(w, http.StatusBadRequest, "please enter a valid email address")
		case errors.Is(err, ErrPasswordTooShort):
			writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		case errors.Is(err, ErrEmailTaken):
			writeError(w, http.StatusConflict, "an account with that email already exists")
		default:
			writeError(w, http.StatusInternalServerError, "failed to create account")
		}
		return
	}
	token, err := a.auth.CreateUserSession(user.ID)
	if err != nil {
		// The account was created and persisted successfully; only
		// session creation failed. Say so explicitly rather than
		// implying registration itself failed — the person shouldn't
		// try to register the same email again.
		writeError(w, http.StatusInternalServerError, "account created, but failed to start a session — try logging in")
		return
	}

	for _, c := range defaultCategories() {
		if _, err := a.categories.Create(user.ID, c); err != nil {
			// Not fatal to registration — the account and session are
			// both already good. Worst case the person starts with an
			// empty category list and adds their own.
			log.Printf("register: failed to seed default categories for user %s: %v", user.ID, err)
			break
		}
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"token": token,
		"role":  RoleUser,
		"email": user.Email,
	})
}

// HandleAuthAccountLogin signs an existing registered account back in —
// the counterpart to HandleAuthRegister for returning users. Distinct
// from HandleAuthLogin, which is the original numeric owner/guest PIN
// login and unrelated to registered accounts entirely.
func (a *API) HandleAuthAccountLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req accountLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	user, err := a.users.Authenticate(req.Email, req.Password)
	if err != nil {
		// Deliberately the same generic message for "no such account"
		// and "wrong password" — see the doc comment on
		// UserStore.Authenticate for why: distinguishing them lets a
		// login form double as a way to check which emails are
		// registered.
		writeError(w, http.StatusUnauthorized, "incorrect email or password")
		return
	}
	token, err := a.auth.CreateUserSession(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start a session")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"token": token,
		"role":  RoleUser,
		"email": user.Email,
	})
}

type changePINRequest struct {
	CurrentPIN string `json:"current_pin"`
	NewPIN     string `json:"new_pin"`
}

// guestPINRequest is shared by set and remove — remove just ignores GuestPIN.
type guestPINRequest struct {
	CurrentPIN string `json:"current_pin"`
	GuestPIN   string `json:"guest_pin"`
}

// HandleAuthStatus is always public (no session required) so the frontend
// can decide whether to show the "set up a PIN" screen or the "enter PIN"
// screen before it has any session token at all. guest_pin_set is included
// so the owner's settings UI can show current guest-access state without
// an authenticated round trip, and because "is guest access on" isn't
// sensitive on its own — it doesn't reveal the guest PIN itself.
//
// accounts_exist reports whether any registered account (see users.go)
// exists yet, independent of pin_set. Without this, a household that only
// ever uses registered accounts and never sets up the legacy owner PIN
// would see pin_set: false forever and get stuck on the "Set up a PIN"
// screen — including right after pressing "lock" — with no way back to
// a login screen for the account they already have.
func (a *API) HandleAuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{
		"pin_set":        a.auth.IsPINSet(),
		"guest_pin_set":  a.auth.IsGuestPINSet(),
		"accounts_exist": a.users.HasAny(),
	})
}

func (a *API) HandleAuthSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req pinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if !pinFormat.MatchString(req.PIN) {
		writeError(w, http.StatusBadRequest, "PIN must be 4-8 digits")
		return
	}
	token, err := a.auth.SetupPIN(req.PIN)
	if err != nil {
		if errors.Is(err, ErrPINAlreadySet) {
			writeError(w, http.StatusConflict, "a PIN is already set")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to set up PIN")
		return
	}
	// The owner is the only role SetupPIN can ever issue a token for.
	writeJSON(w, http.StatusCreated, map[string]string{"token": token, "role": RoleOwner})
}

func (a *API) HandleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req pinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	token, role, err := a.auth.Login(req.PIN)
	if err != nil {
		switch {
		case errors.Is(err, ErrLockedOut):
			writeError(w, http.StatusTooManyRequests, err.Error())
		case errors.Is(err, ErrWrongPIN):
			writeError(w, http.StatusUnauthorized, "incorrect PIN")
		case errors.Is(err, ErrNoPINSet):
			writeError(w, http.StatusBadRequest, "no PIN has been set up yet")
		default:
			writeError(w, http.StatusInternalServerError, "login failed")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token, "role": role})
}

func (a *API) HandleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	token := bearerToken(r)
	if token != "" {
		_ = a.auth.Logout(token)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

// HandleAuthEnablePIN sets up the household PIN for the first time from
// inside the already-unlocked app — the counterpart to HandleAuthSetup for
// someone who's authenticated via a registered account (see users.go) and
// never went through the pre-login "Set up a PIN" screen. That screen only
// ever renders before any account exists (see the accounts_exist doc on
// HandleAuthStatus), so once someone's registered, it's the only path left
// for a household that wants both a PIN *and* accounts.
//
// Unlike HandleAuthSetup, this is an authenticated route (not in
// publicPaths) and doesn't touch the caller's own session — it reuses
// AuthStore.SetupPIN but discards the owner-role token it returns, since
// the caller is already logged in as themselves and swapping their
// session out from under them would be surprising. This only creates the
// PIN; logging in with it afterward is a separate, ordinary PIN login.
func (a *API) HandleAuthEnablePIN(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req pinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if !pinFormat.MatchString(req.PIN) {
		writeError(w, http.StatusBadRequest, "PIN must be 4-8 digits")
		return
	}
	if _, err := a.auth.SetupPIN(req.PIN); err != nil {
		if errors.Is(err, ErrPINAlreadySet) {
			writeError(w, http.StatusConflict, "a PIN is already set")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to set up PIN")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "PIN set up"})
}

func (a *API) HandleAuthChangePIN(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req changePINRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if !pinFormat.MatchString(req.NewPIN) {
		writeError(w, http.StatusBadRequest, "new PIN must be 4-8 digits")
		return
	}
	if err := a.auth.ChangePIN(req.CurrentPIN, req.NewPIN); err != nil {
		switch {
		case errors.Is(err, ErrWrongPIN):
			writeError(w, http.StatusUnauthorized, "current PIN is incorrect")
		case errors.Is(err, ErrNoPINSet):
			writeError(w, http.StatusBadRequest, "no PIN has been set up yet")
		default:
			writeError(w, http.StatusInternalServerError, "failed to change PIN")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "PIN updated"})
}

// HandleAuthGuestPIN sets up or rotates the guest PIN (POST) or turns guest
// access off entirely (DELETE). Both require the current owner PIN — the
// middleware already keeps guests from reaching this route at all (see
// guestAllowed), but the current-PIN check here is what stops someone who's
// merely holding a live owner *session* (e.g. an unattended unlocked
// browser tab) from silently granting themselves guest access without ever
// having proven they know the PIN.
// HandleAuthSession is a deliberately cheap, authenticated endpoint the
// frontend polls frequently (every few seconds) purely to notice a revoked
// session fast — most importantly, a guest whose access the owner just
// turned off. Reaching this handler at all means withAuth already
// validated the bearer token, so there's nothing left to do but say so;
// it does no data lookups of its own. If the token were no longer valid
// (guest access removed, logged out elsewhere, expired), withAuth would
// have already responded 401 before this handler ever ran.
func (a *API) HandleAuthSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) HandleAuthGuestPIN(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req guestPINRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if !pinFormat.MatchString(req.GuestPIN) {
			writeError(w, http.StatusBadRequest, "guest PIN must be 4-8 digits")
			return
		}
		if err := a.auth.SetGuestPIN(req.CurrentPIN, req.GuestPIN); err != nil {
			switch {
			case errors.Is(err, ErrWrongPIN):
				writeError(w, http.StatusUnauthorized, "current PIN is incorrect")
			case errors.Is(err, ErrNoPINSet):
				writeError(w, http.StatusBadRequest, "no owner PIN has been set up yet")
			default:
				writeError(w, http.StatusInternalServerError, "failed to set guest PIN")
			}
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "guest PIN set"})
	case http.MethodDelete:
		var req guestPINRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if err := a.auth.RemoveGuestPIN(req.CurrentPIN); err != nil {
			switch {
			case errors.Is(err, ErrWrongPIN):
				writeError(w, http.StatusUnauthorized, "current PIN is incorrect")
			case errors.Is(err, ErrNoPINSet):
				writeError(w, http.StatusBadRequest, "no owner PIN has been set up yet")
			case errors.Is(err, ErrNoGuestPINSet):
				writeError(w, http.StatusBadRequest, "no guest PIN has been set up yet")
			default:
				writeError(w, http.StatusInternalServerError, "failed to remove guest PIN")
			}
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "guest PIN removed"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
