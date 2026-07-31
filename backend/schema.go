package main

import "database/sql"

// schema is intentionally plain CREATE TABLE IF NOT EXISTS statements, run
// once at startup — no migration framework, consistent with the rest of
// this project's "standard library first" approach. Fine for a
// single-developer app; if this ever needs real schema migrations (adding
// columns to existing tables, etc.) reach for something like golang-migrate
// at that point instead of growing this by hand.
//
// user_id is TEXT NOT NULL DEFAULT ” (a loose reference to users.id
// below, not a real foreign key — deliberately, since accounts can
// predate the users table on an upgraded install, same reasoning as the
// ALTER TABLE backfills below) so every row predating per-account
// registration keeps working as the shared legacy tenant ("") with zero
// backfill needed. See tenant.go.
//
// CREATE TABLE IF NOT EXISTS is a no-op against a table that already
// exists — it does NOT add columns to it. Anyone running this against a
// database from before per-account registration existed has tables
// without user_id at all, so each one gets an explicit
// ALTER TABLE ... ADD COLUMN IF NOT EXISTS right after its CREATE TABLE.
// Safe to run every time: a no-op once the column is already there,
// whether that's because the table was just created fresh above or
// because a previous run already added it.
const schema = `
CREATE TABLE IF NOT EXISTS transactions (
	id          TEXT PRIMARY KEY,
	type        TEXT NOT NULL CHECK (type IN ('income', 'expense')),
	amount      DOUBLE PRECISION NOT NULL CHECK (amount > 0),
	category    TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	date        TEXT NOT NULL,
	created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
	user_id     TEXT NOT NULL DEFAULT ''
);
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS user_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_transactions_date ON transactions (date DESC);
CREATE INDEX IF NOT EXISTS idx_transactions_user_id ON transactions (user_id);

CREATE TABLE IF NOT EXISTS categories (
	id      TEXT PRIMARY KEY,
	name    TEXT NOT NULL,
	type    TEXT NOT NULL CHECK (type IN ('income', 'expense')),
	color   TEXT NOT NULL DEFAULT '',
	icon    TEXT NOT NULL DEFAULT '',
	user_id TEXT NOT NULL DEFAULT ''
);
ALTER TABLE categories ADD COLUMN IF NOT EXISTS user_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_categories_user_id ON categories (user_id);

CREATE TABLE IF NOT EXISTS budgets (
	id            TEXT PRIMARY KEY,
	category      TEXT NOT NULL,
	monthly_limit DOUBLE PRECISION NOT NULL,
	user_id       TEXT NOT NULL DEFAULT '',
	limit_history JSONB NOT NULL DEFAULT '[]'::jsonb
);
ALTER TABLE budgets ADD COLUMN IF NOT EXISTS user_id TEXT NOT NULL DEFAULT '';
ALTER TABLE budgets ADD COLUMN IF NOT EXISTS limit_history JSONB NOT NULL DEFAULT '[]'::jsonb;
CREATE INDEX IF NOT EXISTS idx_budgets_user_id ON budgets (user_id);

CREATE TABLE IF NOT EXISTS recurring_rules (
	id          TEXT PRIMARY KEY,
	type        TEXT NOT NULL CHECK (type IN ('income', 'expense')),
	amount      DOUBLE PRECISION NOT NULL,
	category    TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	frequency   TEXT NOT NULL,
	start_date  TEXT NOT NULL,
	next_due    TEXT NOT NULL,
	active      BOOLEAN NOT NULL DEFAULT true,
	user_id     TEXT NOT NULL DEFAULT ''
);
ALTER TABLE recurring_rules ADD COLUMN IF NOT EXISTS user_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_recurring_rules_user_id ON recurring_rules (user_id);

CREATE TABLE IF NOT EXISTS users (
	id            TEXT PRIMARY KEY,
	email         TEXT NOT NULL,
	salt          TEXT NOT NULL,
	password_hash TEXT NOT NULL,
	created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_lower ON users (lower(email));
`

func runMigrations(db *sql.DB) error {
	_, err := db.Exec(schema)
	return err
}
