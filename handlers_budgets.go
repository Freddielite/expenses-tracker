package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (a *API) HandleBudgets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, a.budgets.List(tenantFrom(r)))

	case http.MethodPost:
		var b Budget
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if err := validateBudget(&b); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		// A brand-new budget's limit has been true since its creation
		// month — seed history with that one entry rather than leaving it
		// empty, so an edit later this same month collapses into it
		// instead of creating a spurious two-entry history for a limit
		// that never actually changed mid-month.
		b.LimitHistory = []BudgetLimitChange{{Month: currentMonthStr(), Limit: b.MonthlyLimit}}
		created, err := a.budgets.Create(tenantFrom(r), &b)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save budget")
			return
		}
		writeJSON(w, http.StatusCreated, created)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (a *API) HandleBudgetByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/budgets/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing budget id")
		return
	}

	switch r.Method {
	case http.MethodPut:
		var b Budget
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if err := validateBudget(&b); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		// PUT is a full replacement, so a client-supplied (or absent)
		// limit_history would otherwise silently wipe or fake it. Fetch
		// what's actually on record and derive the new history from that
		// instead of trusting the body.
		existing, err := a.budgets.Get(tenantFrom(r), id)
		if err != nil {
			handleGenericStoreErr(w, err)
			return
		}
		b.LimitHistory = recordLimitChange(existing.LimitHistory, existing.MonthlyLimit, b.MonthlyLimit)
		updated, err := a.budgets.Update(tenantFrom(r), id, &b)
		if err != nil {
			handleGenericStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, updated)

	case http.MethodDelete:
		if err := a.budgets.Delete(tenantFrom(r), id); err != nil {
			handleGenericStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusNoContent, nil)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// currentMonthStr is today's month as "2006-01".
func currentMonthStr() string {
	return time.Now().Format("2006-01")
}

// recordLimitChange returns the effective-dated history a budget should
// carry after its limit changes from oldLimit to newLimit, so past months
// keep comparing against whatever limit was actually in effect at the
// time instead of only ever seeing today's number.
//
// history is expected sorted oldest-first by Month, which recordLimitChange
// preserves: it only ever reads/updates the last entry or appends a new
// one after it, never inserts out of order.
func recordLimitChange(history []BudgetLimitChange, oldLimit, newLimit float64) []BudgetLimitChange {
	if newLimit == oldLimit {
		return history
	}
	if len(history) == 0 {
		// A legacy budget (predates this feature) or one whose history
		// was otherwise never seeded — record what its limit always was
		// up to now with the blank "since the beginning" month, so months
		// before this edit still resolve to the old limit rather than
		// silently inheriting the new one.
		history = append(history, BudgetLimitChange{Month: "", Limit: oldLimit})
	}
	month := currentMonthStr()
	if last := &history[len(history)-1]; last.Month == month {
		// Second edit within the same calendar month — collapse into the
		// existing entry rather than recording two changes for one month.
		last.Limit = newLimit
		return history
	}
	return append(history, BudgetLimitChange{Month: month, Limit: newLimit})
}

// limitAtMonth returns the limit that was in effect for a budget during
// the given "2006-01" month, per its effective-dated LimitHistory. Falls
// back to the budget's current MonthlyLimit when there's no applicable
// entry — covers months before any recorded change, and budgets with no
// history at all (nil LimitHistory), so a budget that's never been
// edited under this feature behaves exactly as it did before.
func limitAtMonth(b *Budget, month string) float64 {
	limit := b.MonthlyLimit
	for _, h := range b.LimitHistory {
		if h.Month > month {
			break
		}
		limit = h.Limit
	}
	return limit
}

// spendByCategoryForMonth sums expense transactions in a given "2006-01"
// month, keyed by category. Shared by the status and history endpoints so
// they can't drift out of sync on what counts as "spend."
func spendByCategoryForMonth(transactions []*Transaction, month string) map[string]float64 {
	spend := make(map[string]float64)
	for _, t := range transactions {
		if t.Type != TypeExpense {
			continue
		}
		if len(t.Date) < 7 || t.Date[:7] != month {
			continue
		}
		spend[t.Category] += t.Amount
	}
	return spend
}

// lastNMonths returns the N calendar months up to and including the given
// month, oldest first, as "2006-01" strings.
func lastNMonths(month string, n int) []string {
	anchor, err := time.Parse("2006-01", month)
	if err != nil {
		anchor = time.Now()
	}
	months := make([]string, n)
	for i := 0; i < n; i++ {
		months[n-1-i] = anchor.AddDate(0, -i, 0).Format("2006-01")
	}
	return months
}

// HandleBudgetStatus computes spend-vs-limit for every configured budget,
// for a single month — the current calendar month by default, or an
// arbitrary one via ?month=2026-05.
func (a *API) HandleBudgetStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	month := r.URL.Query().Get("month")
	if month == "" {
		month = time.Now().Format("2006-01")
	} else if _, err := time.Parse("2006-01", month); err != nil {
		writeError(w, http.StatusBadRequest, "month must be in YYYY-MM format")
		return
	}

	spendByCategory := spendByCategoryForMonth(a.store.List(tenantFrom(r)), month)

	budgetList := a.budgets.List(tenantFrom(r))
	statuses := make([]BudgetStatus, 0, len(budgetList))
	for _, b := range budgetList {
		limit := limitAtMonth(b, month)
		spent := spendByCategory[b.Category]
		remaining := limit - spent
		percent := 0.0
		if limit > 0 {
			percent = (spent / limit) * 100
		}
		statuses = append(statuses, BudgetStatus{
			ID:           b.ID,
			Category:     b.Category,
			MonthlyLimit: limit,
			Spent:        spent,
			Remaining:    remaining,
			PercentUsed:  percent,
		})
	}

	writeJSON(w, http.StatusOK, statuses)
}

// HandleBudgetHistory returns each budget's spend trend across the last
// N calendar months (default 6, clamped to 1-24 via ?months=). Each
// month is compared against whatever limit was actually in effect then
// (via limitAtMonth), not just today's — see BudgetLimitChange.
func (a *API) HandleBudgetHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	months := 6
	if raw := r.URL.Query().Get("months"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			writeError(w, http.StatusBadRequest, "months must be a positive integer")
			return
		}
		if parsed > 24 {
			parsed = 24
		}
		months = parsed
	}

	monthList := lastNMonths(time.Now().Format("2006-01"), months)

	// Compute spend-by-category once per month, not once per budget per
	// month — a full transaction scan per budget would be O(budgets ×
	// months × transactions) for no reason.
	spendPerMonth := make(map[string]map[string]float64, len(monthList))
	transactions := a.store.List(tenantFrom(r))
	for _, m := range monthList {
		spendPerMonth[m] = spendByCategoryForMonth(transactions, m)
	}

	budgetHistoryList := a.budgets.List(tenantFrom(r))
	history := make([]BudgetHistory, 0, len(budgetHistoryList))
	for _, b := range budgetHistoryList {
		entry := BudgetHistory{
			ID:           b.ID,
			Category:     b.Category,
			MonthlyLimit: b.MonthlyLimit,
			Months:       make([]BudgetMonthStatus, 0, len(monthList)),
		}
		for _, m := range monthList {
			limit := limitAtMonth(b, m)
			spent := spendPerMonth[m][b.Category]
			percent := 0.0
			if limit > 0 {
				percent = (spent / limit) * 100
			}
			entry.Months = append(entry.Months, BudgetMonthStatus{
				Month:       m,
				Limit:       limit,
				Spent:       spent,
				PercentUsed: percent,
			})
		}
		history = append(history, entry)
	}

	writeJSON(w, http.StatusOK, history)
}

func validateBudget(b *Budget) error {
	if strings.TrimSpace(b.Category) == "" {
		return errors.New("category is required")
	}
	if b.MonthlyLimit <= 0 {
		return errors.New("monthly_limit must be greater than 0")
	}
	return nil
}
