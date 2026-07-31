package main

import (
	"net/http"
	"strings"
)

// withCORS allows the Vite dev server (localhost:5173) to call this API
// during development. In production you'd tighten this to your real domain.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// withLogging prints a line per request, handy while learning what's happening.
func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logRequest(r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

// publicPaths don't require a session token — the auth endpoints themselves
// (you can't present a token before you've logged in) and the health check.
var publicPaths = map[string]bool{
	"/api/auth/status":        true,
	"/api/auth/setup":         true,
	"/api/auth/login":         true,
	"/api/auth/register":      true,
	"/api/auth/account-login": true,
	"/health":                 true,
}

// withAuth rejects any request to a non-public path that doesn't carry a
// valid session token. Only fully open (no session required at all) when
// NEITHER an owner PIN NOR any registered account exists yet — that's
// the original single-local-user "nothing to protect until you set
// something up" state. The moment either exists, every non-public route
// requires a valid session; this matters more than it used to now that
// registered accounts (users.go) exist, since leaving the API open by
// default would mean every account's private data is reachable by
// anyone with no token at all.
//
// Once a session is confirmed valid, a guest-role session is further
// checked against guestAllowed. Deliberately distinct status codes for the
// two failure modes: 401 means "you're not logged in" (frontend bounces to
// the lock screen), 403 means "you're logged in as a guest and this action
// just isn't available to you" (frontend can show that message instead of
// pretending the session expired).
func withAuth(auth *AuthStore, users UserStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only /api/* routes carry actual data — the frontend files
			// (index.html, JS, CSS bundles) served from "/" contain no
			// data of their own and must load *before* a PIN can be
			// entered, so they're never gated here. Auth for the data
			// still happens per-request, same as before.
			if !strings.HasPrefix(r.URL.Path, "/api/") || publicPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}
			if !auth.IsPINSet() && !users.HasAny() {
				// Nothing has been set up yet at all — same
				// deliberately open first-run state as before,
				// resolving to the shared legacy tenant.
				next.ServeHTTP(w, withTenant(r, LegacyTenant))
				return
			}

			token := bearerToken(r)
			if token == "" {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			role, userID, ok := auth.ValidateSession(token)
			if !ok {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			if role == RoleGuest && !guestAllowed(r.Method, r.URL.Path) {
				writeError(w, http.StatusForbidden, "guests don't have permission to do this")
				return
			}
			// Only a registered-account (RoleUser) session resolves to
			// its own private tenant; owner/guest PIN sessions share
			// the one legacy tenant, as they always have.
			tenant := LegacyTenant
			if role == RoleUser {
				tenant = userID
			}
			next.ServeHTTP(w, withTenant(r, tenant))
		})
	}
}

// guestAllowed is the allowlist for the guest role: view the ledger,
// reports, budgets/budget status, recurring rules, and goals, but no
// creating/editing/deleting anything, no contributing to goals, no
// import, and no export. Export is blocked even though it's read-only in
// spirit, since it lets a guest walk away with a full copy of the data.
// Deny-by-default — a new route only becomes guest-visible by being
// explicitly allowed here, not by omission from a blocklist.
func guestAllowed(method, path string) bool {
	if strings.HasPrefix(path, "/api/export/") || strings.HasPrefix(path, "/api/import/") {
		return false
	}
	if strings.HasPrefix(path, "/api/auth/") {
		// Guests can end their own session, and can poll the lightweight
		// session-check endpoint (that's precisely what lets a guest's
		// open tab notice the owner revoked access — see
		// HandleAuthSession). Everything else under auth (changing the
		// owner PIN, setting/rotating/removing the guest PIN) is
		// owner-only.
		if path == "/api/auth/logout" && method == http.MethodPost {
			return true
		}
		return path == "/api/auth/session" && method == http.MethodGet
	}
	// Every remaining data endpoint (transactions, reports, categories,
	// budgets, recurring, goals) is read-only for guests.
	return method == http.MethodGet
}

func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimPrefix(header, prefix)
}
