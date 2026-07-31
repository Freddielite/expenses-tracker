package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
)

// initStores wires up every store that needs to survive more than a
// local dev session: transactions/categories/budgets/recurring, and
// registered accounts (users.go). If DATABASE_URL is set, all five
// connect to the same Postgres pool, get their schema migrated, and use
// the Postgres-backed implementations (pg_store.go / pg_item_stores.go /
// pg_user_store.go). Otherwise everything falls back to the original
// JSON-file stores (store.go / generic_store.go / users.go) — same
// behavior as before Postgres support existed, so nothing changes for
// anyone not setting the env var.
//
// Accounts were file-backed unconditionally until this function grew to
// cover them too — meaning registered accounts vanished on anything that
// wiped the local working directory (re-cloning the project, deploying a
// fresh copy, etc.) even with DATABASE_URL configured and every other
// kind of data surviving that exact same operation just fine. Routing
// users through the same DATABASE_URL choice as everything else closes
// that gap.
//
// auth.go (the owner/guest PIN and session tokens) and goals.json are
// deliberately NOT part of this — see their own doc comments for why
// that's a conscious choice rather than an oversight.
//
// The returned cleanup func should be deferred by the caller; it closes
// the DB connection pool if one was opened (a no-op in JSON mode).
func initStores() (txStore TransactionStore, categories CategoryStore, budgets BudgetStore, recurring RecurringStore, users UserStore, cleanup func(), err error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return initJSONStores()
	}
	return initPostgresStores(dsn)
}

func initJSONStores() (TransactionStore, CategoryStore, BudgetStore, RecurringStore, UserStore, func(), error) {
	log.Println("storage: DATABASE_URL not set, using local JSON files")

	store, err := NewStore("data.json")
	if err != nil {
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to initialize transaction store: %w", err)
	}
	categories, err := NewFileStore[*Category]("categories.json")
	if err != nil {
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to initialize category store: %w", err)
	}
	budgets, err := NewFileStore[*Budget]("budgets.json")
	if err != nil {
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to initialize budget store: %w", err)
	}
	recurring, err := NewFileStore[*RecurringRule]("recurring.json")
	if err != nil {
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to initialize recurring rule store: %w", err)
	}
	users, err := NewFileUserStore("users.json")
	if err != nil {
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to initialize user store: %w", err)
	}

	return store, categories, budgets, recurring, users, func() {}, nil
}

func initPostgresStores(dsn string) (TransactionStore, CategoryStore, BudgetStore, RecurringStore, UserStore, func(), error) {
	log.Println("storage: DATABASE_URL is set, using Postgres")

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to open Postgres connection: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to reach Postgres (check DATABASE_URL and that the server is running): %w", err)
	}
	if err := runMigrations(db); err != nil {
		db.Close()
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to run schema migration: %w", err)
	}

	cleanup := func() {
		if err := db.Close(); err != nil {
			log.Printf("storage: error closing Postgres connection: %v", err)
		}
	}

	return NewPostgresStore(db),
		NewPostgresCategoryStore(db),
		NewPostgresBudgetStore(db),
		NewPostgresRecurringStore(db),
		NewPostgresUserStore(db),
		cleanup,
		nil
}
