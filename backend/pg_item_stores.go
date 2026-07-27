package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
)

// ---- Categories ----

type PostgresCategoryStore struct {
	db *sql.DB
}

func NewPostgresCategoryStore(db *sql.DB) *PostgresCategoryStore {
	return &PostgresCategoryStore{db: db}
}

func (s *PostgresCategoryStore) Create(tenantID string, c *Category) (*Category, error) {
	c.ID = newID()
	c.UserID = tenantID
	_, err := s.db.Exec(
		`INSERT INTO categories (id, name, type, color, icon, user_id) VALUES ($1, $2, $3, $4, $5, $6)`,
		c.ID, c.Name, c.Type, c.Color, c.Icon, c.UserID,
	)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (s *PostgresCategoryStore) List(tenantID string) []*Category {
	rows, err := s.db.Query(`SELECT id, name, type, color, icon, user_id FROM categories WHERE user_id = $1 ORDER BY name`, tenantID)
	if err != nil {
		log.Printf("PostgresCategoryStore.List: query failed: %v", err)
		return nil
	}
	defer rows.Close()

	var out []*Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Type, &c.Color, &c.Icon, &c.UserID); err != nil {
			log.Printf("PostgresCategoryStore.List: scan failed: %v", err)
			continue
		}
		out = append(out, &c)
	}
	return out
}

func (s *PostgresCategoryStore) Get(tenantID, id string) (*Category, error) {
	var c Category
	err := s.db.QueryRow(`SELECT id, name, type, color, icon, user_id FROM categories WHERE id = $1 AND user_id = $2`, id, tenantID).
		Scan(&c.ID, &c.Name, &c.Type, &c.Color, &c.Icon, &c.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *PostgresCategoryStore) Update(tenantID, id string, updated *Category) (*Category, error) {
	updated.ID = id
	updated.UserID = tenantID
	res, err := s.db.Exec(
		`UPDATE categories SET name = $1, type = $2, color = $3, icon = $4 WHERE id = $5 AND user_id = $6`,
		updated.Name, updated.Type, updated.Color, updated.Icon, id, tenantID,
	)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return updated, nil
}

func (s *PostgresCategoryStore) Delete(tenantID, id string) error {
	res, err := s.db.Exec(`DELETE FROM categories WHERE id = $1 AND user_id = $2`, id, tenantID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresCategoryStore) IsEmptyFor(tenantID string) bool {
	var count int
	if err := s.db.QueryRow(`SELECT count(*) FROM categories WHERE user_id = $1`, tenantID).Scan(&count); err != nil {
		log.Printf("PostgresCategoryStore.IsEmptyFor: query failed: %v", err)
		return false // fail closed — don't re-seed defaults on top of a query error
	}
	return count == 0
}

// ---- Budgets ----

type PostgresBudgetStore struct {
	db *sql.DB
}

func NewPostgresBudgetStore(db *sql.DB) *PostgresBudgetStore {
	return &PostgresBudgetStore{db: db}
}

func (s *PostgresBudgetStore) Create(tenantID string, b *Budget) (*Budget, error) {
	b.ID = newID()
	b.UserID = tenantID
	historyJSON, err := json.Marshal(b.LimitHistory)
	if err != nil {
		return nil, err
	}
	_, err = s.db.Exec(
		`INSERT INTO budgets (id, category, monthly_limit, user_id, limit_history) VALUES ($1, $2, $3, $4, $5)`,
		b.ID, b.Category, b.MonthlyLimit, b.UserID, historyJSON,
	)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (s *PostgresBudgetStore) List(tenantID string) []*Budget {
	rows, err := s.db.Query(`SELECT id, category, monthly_limit, user_id, limit_history FROM budgets WHERE user_id = $1 ORDER BY category`, tenantID)
	if err != nil {
		log.Printf("PostgresBudgetStore.List: query failed: %v", err)
		return nil
	}
	defer rows.Close()

	var out []*Budget
	for rows.Next() {
		var b Budget
		var historyJSON []byte
		if err := rows.Scan(&b.ID, &b.Category, &b.MonthlyLimit, &b.UserID, &historyJSON); err != nil {
			log.Printf("PostgresBudgetStore.List: scan failed: %v", err)
			continue
		}
		if err := json.Unmarshal(historyJSON, &b.LimitHistory); err != nil {
			log.Printf("PostgresBudgetStore.List: limit_history unmarshal failed: %v", err)
		}
		out = append(out, &b)
	}
	return out
}

func (s *PostgresBudgetStore) Get(tenantID, id string) (*Budget, error) {
	var b Budget
	var historyJSON []byte
	err := s.db.QueryRow(`SELECT id, category, monthly_limit, user_id, limit_history FROM budgets WHERE id = $1 AND user_id = $2`, id, tenantID).
		Scan(&b.ID, &b.Category, &b.MonthlyLimit, &b.UserID, &historyJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(historyJSON, &b.LimitHistory); err != nil {
		return nil, err
	}
	return &b, nil
}

func (s *PostgresBudgetStore) Update(tenantID, id string, updated *Budget) (*Budget, error) {
	updated.ID = id
	updated.UserID = tenantID
	historyJSON, err := json.Marshal(updated.LimitHistory)
	if err != nil {
		return nil, err
	}
	res, err := s.db.Exec(
		`UPDATE budgets SET category = $1, monthly_limit = $2, limit_history = $3 WHERE id = $4 AND user_id = $5`,
		updated.Category, updated.MonthlyLimit, historyJSON, id, tenantID,
	)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return updated, nil
}

func (s *PostgresBudgetStore) Delete(tenantID, id string) error {
	res, err := s.db.Exec(`DELETE FROM budgets WHERE id = $1 AND user_id = $2`, id, tenantID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- Recurring rules ----

type PostgresRecurringStore struct {
	db *sql.DB
}

func NewPostgresRecurringStore(db *sql.DB) *PostgresRecurringStore {
	return &PostgresRecurringStore{db: db}
}

func (s *PostgresRecurringStore) Create(tenantID string, r *RecurringRule) (*RecurringRule, error) {
	r.ID = newID()
	r.UserID = tenantID
	_, err := s.db.Exec(
		`INSERT INTO recurring_rules (id, type, amount, category, description, frequency, start_date, next_due, active, user_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		r.ID, r.Type, r.Amount, r.Category, r.Description, r.Frequency, r.StartDate, r.NextDue, r.Active, r.UserID,
	)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (s *PostgresRecurringStore) List(tenantID string) []*RecurringRule {
	rows, err := s.db.Query(
		`SELECT id, type, amount, category, description, frequency, start_date, next_due, active, user_id
		 FROM recurring_rules WHERE user_id = $1 ORDER BY next_due`,
		tenantID,
	)
	if err != nil {
		log.Printf("PostgresRecurringStore.List: query failed: %v", err)
		return nil
	}
	defer rows.Close()
	return scanRecurringRows(rows)
}

// ListAll is unfiltered — see the interface doc in stores.go for why this
// exists and who's allowed to call it.
func (s *PostgresRecurringStore) ListAll() []*RecurringRule {
	rows, err := s.db.Query(
		`SELECT id, type, amount, category, description, frequency, start_date, next_due, active, user_id
		 FROM recurring_rules ORDER BY next_due`,
	)
	if err != nil {
		log.Printf("PostgresRecurringStore.ListAll: query failed: %v", err)
		return nil
	}
	defer rows.Close()
	return scanRecurringRows(rows)
}

func scanRecurringRows(rows *sql.Rows) []*RecurringRule {
	var out []*RecurringRule
	for rows.Next() {
		var r RecurringRule
		if err := rows.Scan(&r.ID, &r.Type, &r.Amount, &r.Category, &r.Description, &r.Frequency, &r.StartDate, &r.NextDue, &r.Active, &r.UserID); err != nil {
			log.Printf("PostgresRecurringStore: scan failed: %v", err)
			continue
		}
		out = append(out, &r)
	}
	return out
}

func (s *PostgresRecurringStore) Get(tenantID, id string) (*RecurringRule, error) {
	var r RecurringRule
	err := s.db.QueryRow(
		`SELECT id, type, amount, category, description, frequency, start_date, next_due, active, user_id
		 FROM recurring_rules WHERE id = $1 AND user_id = $2`,
		id, tenantID,
	).Scan(&r.ID, &r.Type, &r.Amount, &r.Category, &r.Description, &r.Frequency, &r.StartDate, &r.NextDue, &r.Active, &r.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *PostgresRecurringStore) Update(tenantID, id string, updated *RecurringRule) (*RecurringRule, error) {
	updated.ID = id
	updated.UserID = tenantID
	res, err := s.db.Exec(
		`UPDATE recurring_rules
		 SET type = $1, amount = $2, category = $3, description = $4, frequency = $5,
		     start_date = $6, next_due = $7, active = $8
		 WHERE id = $9 AND user_id = $10`,
		updated.Type, updated.Amount, updated.Category, updated.Description, updated.Frequency,
		updated.StartDate, updated.NextDue, updated.Active, id, tenantID,
	)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return updated, nil
}

func (s *PostgresRecurringStore) Delete(tenantID, id string) error {
	res, err := s.db.Exec(`DELETE FROM recurring_rules WHERE id = $1 AND user_id = $2`, id, tenantID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
