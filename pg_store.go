package main

import (
	"database/sql"
	"errors"
	"log"
	"time"
)

// PostgresStore is a Postgres-backed implementation of TransactionStore —
// a drop-in replacement for the JSON-backed Store in store.go. Same
// Create/List/Get/Update/Delete surface, same ErrNotFound semantics, same
// tenant scoping (see tenant.go and the interface doc in stores.go).
type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) Create(tenantID string, t *Transaction) error {
	t.ID = newID()
	t.CreatedAt = time.Now().UTC()
	t.UserID = tenantID
	_, err := s.db.Exec(
		`INSERT INTO transactions (id, type, amount, category, description, date, created_at, user_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		t.ID, t.Type, t.Amount, t.Category, t.Description, t.Date, t.CreatedAt, t.UserID,
	)
	return err
}

func (s *PostgresStore) List(tenantID string) []*Transaction {
	rows, err := s.db.Query(
		`SELECT id, type, amount, category, description, date, created_at, user_id
		 FROM transactions WHERE user_id = $1 ORDER BY date DESC`,
		tenantID,
	)
	if err != nil {
		log.Printf("PostgresStore.List: query failed: %v", err)
		return nil
	}
	defer rows.Close()

	var out []*Transaction
	for rows.Next() {
		var t Transaction
		if err := rows.Scan(&t.ID, &t.Type, &t.Amount, &t.Category, &t.Description, &t.Date, &t.CreatedAt, &t.UserID); err != nil {
			log.Printf("PostgresStore.List: scan failed: %v", err)
			continue
		}
		out = append(out, &t)
	}
	if err := rows.Err(); err != nil {
		log.Printf("PostgresStore.List: row iteration failed: %v", err)
	}
	return out
}

func (s *PostgresStore) Get(tenantID, id string) (*Transaction, error) {
	var t Transaction
	err := s.db.QueryRow(
		`SELECT id, type, amount, category, description, date, created_at, user_id
		 FROM transactions WHERE id = $1 AND user_id = $2`,
		id, tenantID,
	).Scan(&t.ID, &t.Type, &t.Amount, &t.Category, &t.Description, &t.Date, &t.CreatedAt, &t.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *PostgresStore) Update(tenantID, id string, updated *Transaction) (*Transaction, error) {
	// Fetch first so we preserve ID/CreatedAt exactly like the JSON-backed
	// Store does, and so a missing (or another tenant's) id reliably
	// surfaces as ErrNotFound.
	existing, err := s.Get(tenantID, id)
	if err != nil {
		return nil, err
	}
	updated.ID = existing.ID
	updated.CreatedAt = existing.CreatedAt
	updated.UserID = existing.UserID

	_, err = s.db.Exec(
		`UPDATE transactions SET type = $1, amount = $2, category = $3, description = $4, date = $5
		 WHERE id = $6 AND user_id = $7`,
		updated.Type, updated.Amount, updated.Category, updated.Description, updated.Date, id, tenantID,
	)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *PostgresStore) Delete(tenantID, id string) error {
	res, err := s.db.Exec(`DELETE FROM transactions WHERE id = $1 AND user_id = $2`, id, tenantID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
