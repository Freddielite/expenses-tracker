package main

import (
	"context"
	"net/http"
)

type contextKey string

const tenantContextKey contextKey = "tenant"

// LegacyTenant is the shared "tenant" every record predating per-account
// registration belongs to — an empty UserID. It's also what any
// owner/guest PIN session (RoleOwner/RoleGuest) still resolves to today:
// the original single-shared-household PIN system keeps working exactly
// as it always has, as one shared tenant, entirely separate from each
// newly registered account's own private data. Only a RoleUser session
// (see users.go, CreateUserSession) resolves to a real per-account
// tenant — see resolveTenant in middleware.go.
const LegacyTenant = ""

// withTenant attaches the resolved tenant ID to the request context so
// handlers can scope every store call to it without re-deriving it from
// the bearer token themselves.
func withTenant(r *http.Request, tenantID string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), tenantContextKey, tenantID))
}

// tenantFrom returns the tenant ID withAuth attached to this request.
// Falls back to LegacyTenant if none was attached — the case for public
// routes and for the wide-open "no auth configured yet" mode, both of
// which never go through session validation at all.
func tenantFrom(r *http.Request) string {
	if v, ok := r.Context().Value(tenantContextKey).(string); ok {
		return v
	}
	return LegacyTenant
}
