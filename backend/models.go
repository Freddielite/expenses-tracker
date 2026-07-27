package main

import "time"

// TransactionType distinguishes income from expense entries.
type TransactionType string

const (
	TypeIncome  TransactionType = "income"
	TypeExpense TransactionType = "expense"
)

// Transaction represents a single income or expense record.
type Transaction struct {
	ID          string          `json:"id"`
	Type        TransactionType `json:"type"`
	Amount      float64         `json:"amount"`
	Category    string          `json:"category"`
	Description string          `json:"description"`
	Date        string          `json:"date"` // ISO date string, e.g. "2026-07-14"
	CreatedAt   time.Time       `json:"created_at"`
	// UserID is the owning tenant — empty for the shared legacy
	// owner/guest PIN household, or a registered account's ID (see
	// users.go / tenant.go). Store.Create always overwrites this from
	// the authenticated session's resolved tenant (see tenant.go),
	// exactly like it already does for ID/CreatedAt — a client can't
	// set it by including it in a request body.
	UserID string `json:"user_id,omitempty"`
}

// TransactionPatch is the payload for PATCH /api/transactions/{id}.
// Every field is a pointer so a field left out of the JSON body is left
// nil (untouched) and stays distinguishable from one explicitly set to
// its zero value (e.g. an empty description).
type TransactionPatch struct {
	Type        *TransactionType `json:"type"`
	Amount      *float64         `json:"amount"`
	Category    *string          `json:"category"`
	Description *string          `json:"description"`
	Date        *string          `json:"date"`
}

// Apply merges the non-nil fields of the patch onto a copy of t and
// returns the result — t itself is left untouched.
func (p *TransactionPatch) Apply(t *Transaction) *Transaction {
	merged := *t
	if p.Type != nil {
		merged.Type = *p.Type
	}
	if p.Amount != nil {
		merged.Amount = *p.Amount
	}
	if p.Category != nil {
		merged.Category = *p.Category
	}
	if p.Description != nil {
		merged.Description = *p.Description
	}
	if p.Date != nil {
		merged.Date = *p.Date
	}
	return &merged
}

// CategorySummary aggregates totals for a single category.
type CategorySummary struct {
	Category string  `json:"category"`
	Total    float64 `json:"total"`
	Count    int     `json:"count"`
}

// MonthlySummary aggregates totals for a given month.
type MonthlySummary struct {
	Month        string  `json:"month"` // "2026-07"
	TotalIncome  float64 `json:"total_income"`
	TotalExpense float64 `json:"total_expense"`
	Net          float64 `json:"net"`
}

// ReportResponse is the full payload returned by the reports endpoint.
type ReportResponse struct {
	TotalIncome      float64           `json:"total_income"`
	TotalExpense     float64           `json:"total_expense"`
	Net              float64           `json:"net"`
	ByCategory       []CategorySummary `json:"by_category"`
	ByMonth          []MonthlySummary  `json:"by_month"`
	TransactionCount int               `json:"transaction_count"`
}

// Category is a user-managed income/expense category. Transactions
// reference categories by name (a plain string), not by ID — that keeps
// historical entries intact even if a category is later renamed or removed.
type Category struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Type  TransactionType `json:"type"`
	Color string          `json:"color"`
	// Icon is a key the frontend maps to an actual icon component
	// (e.g. "utensils", "car", "wallet") — the backend just stores the key.
	Icon string `json:"icon"`
	// UserID is the owning tenant — see the identical field on
	// Transaction for what it means and why FileStore always
	// overwrites it rather than trusting the request body.
	UserID string `json:"user_id,omitempty"`
}

func (c *Category) GetID() string       { return c.ID }
func (c *Category) SetID(id string)     { c.ID = id }
func (c *Category) GetUserID() string   { return c.UserID }
func (c *Category) SetUserID(id string) { c.UserID = id }

// Budget sets a monthly spending limit for one category.
type Budget struct {
	ID           string  `json:"id"`
	Category     string  `json:"category"`
	MonthlyLimit float64 `json:"monthly_limit"`
	UserID       string  `json:"user_id,omitempty"`

	// LimitHistory is an effective-dated log of past MonthlyLimit values,
	// oldest first, so a budget's spend history can be judged against the
	// limit that was actually in effect at the time rather than always
	// today's number. A blank Month means "since the beginning" — see
	// recordLimitChange in handlers_budgets.go. Nil/empty for any budget
	// that predates this feature and has never been edited since.
	LimitHistory []BudgetLimitChange `json:"limit_history,omitempty"`
}

// BudgetLimitChange is one entry in a budget's effective-dated limit
// history: "starting in Month, the limit became Limit."
type BudgetLimitChange struct {
	Month string  `json:"month"` // "2026-07", or "" for the since-the-beginning baseline
	Limit float64 `json:"limit"`
}

func (b *Budget) GetID() string       { return b.ID }
func (b *Budget) SetID(id string)     { b.ID = id }
func (b *Budget) GetUserID() string   { return b.UserID }
func (b *Budget) SetUserID(id string) { b.UserID = id }

// BudgetStatus is the computed view of a budget against actual spend for
// a given calendar month (the current month, by default).
type BudgetStatus struct {
	ID           string  `json:"id"`
	Category     string  `json:"category"`
	MonthlyLimit float64 `json:"monthly_limit"`
	Spent        float64 `json:"spent"`
	Remaining    float64 `json:"remaining"`
	PercentUsed  float64 `json:"percent_used"`
}

// BudgetMonthStatus is one month's worth of spend-vs-limit for a single
// budget, used by the history endpoint. Limit is whatever was actually in
// effect during that month per the budget's LimitHistory (see
// limitAtMonth), which can differ from the budget's current MonthlyLimit
// if it's been changed since.
type BudgetMonthStatus struct {
	Month       string  `json:"month"` // "2026-07"
	Limit       float64 `json:"limit"`
	Spent       float64 `json:"spent"`
	PercentUsed float64 `json:"percent_used"`
}

// BudgetHistory is a budget's spend trend across several recent months.
// MonthlyLimit is the budget's *current* limit; each entry in Months
// carries the limit that actually applied to that specific month.
type BudgetHistory struct {
	ID           string              `json:"id"`
	Category     string              `json:"category"`
	MonthlyLimit float64             `json:"monthly_limit"`
	Months       []BudgetMonthStatus `json:"months"` // oldest first
}

// RecurringRule describes a transaction that should repeat automatically.
// NextDue advances each time GenerateDueTransactions creates an occurrence.
type RecurringRule struct {
	ID          string          `json:"id"`
	Type        TransactionType `json:"type"`
	Amount      float64         `json:"amount"`
	Category    string          `json:"category"`
	Description string          `json:"description"`
	Frequency   string          `json:"frequency"` // "weekly" or "monthly"
	StartDate   string          `json:"start_date"`
	NextDue     string          `json:"next_due"`
	Active      bool            `json:"active"`
	UserID      string          `json:"user_id,omitempty"`
}

func (r *RecurringRule) GetID() string       { return r.ID }
func (r *RecurringRule) SetID(id string)     { r.ID = id }
func (r *RecurringRule) GetUserID() string   { return r.UserID }
func (r *RecurringRule) SetUserID(id string) { r.UserID = id }

// Goal is a savings target the user is accumulating money toward (e.g.
// "Emergency fund — ₦500,000"). This is deliberately separate from Budget:
// a Budget caps spending in a category, a Goal tracks progress toward
// saving something. SavedAmount is updated via contributions
// (POST /api/goals/{id}/contribute) rather than derived from transactions,
// so it doesn't double-count money that's already reflected in the ledger.
type Goal struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	TargetAmount float64 `json:"target_amount"`
	SavedAmount  float64 `json:"saved_amount"`
	TargetDate   string  `json:"target_date,omitempty"` // optional ISO date
	UserID       string  `json:"user_id,omitempty"`
}

func (g *Goal) GetID() string       { return g.ID }
func (g *Goal) SetID(id string)     { g.ID = id }
func (g *Goal) GetUserID() string   { return g.UserID }
func (g *Goal) SetUserID(id string) { g.UserID = id }

// GoalContribution is the payload for POST /api/goals/{id}/contribute.
// Amount can be negative to correct an over-contribution, but a goal's
// saved_amount is never allowed to drop below 0.
type GoalContribution struct {
	Amount float64 `json:"amount"`
}

// ImportResult summarizes the outcome of a transaction import (from a CSV,
// Excel, or PDF file).
type ImportResult struct {
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors"`
	// Note carries a format-specific caveat — e.g. PDF import is a
	// best-effort text extraction, so the frontend surfaces this to the
	// user as a reason to double check the results. Empty for formats
	// (CSV, XLSX) that are parsed exactly rather than heuristically.
	Note string `json:"note,omitempty"`
}
