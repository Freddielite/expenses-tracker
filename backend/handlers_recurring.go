package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

func (a *API) HandleRecurring(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, a.recurring.List(tenantFrom(r)))

	case http.MethodPost:
		var rule RecurringRule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if err := validateRecurringRule(&rule); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if rule.NextDue == "" {
			rule.NextDue = rule.StartDate
		}
		rule.Active = true

		created, err := a.recurring.Create(tenantFrom(r), &rule)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save recurring rule")
			return
		}
		// Pick up anything already due immediately (e.g. a start date in the past).
		GenerateDueTransactions(a.store, a.recurring)
		writeJSON(w, http.StatusCreated, created)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (a *API) HandleRecurringByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/recurring/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing recurring rule id")
		return
	}

	switch r.Method {
	case http.MethodPut:
		var rule RecurringRule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if err := validateRecurringRule(&rule); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		updated, err := a.recurring.Update(tenantFrom(r), id, &rule)
		if err != nil {
			handleGenericStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, updated)

	case http.MethodDelete:
		if err := a.recurring.Delete(tenantFrom(r), id); err != nil {
			handleGenericStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusNoContent, nil)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func validateRecurringRule(r *RecurringRule) error {
	if r.Type != TypeIncome && r.Type != TypeExpense {
		return errors.New("type must be 'income' or 'expense'")
	}
	if r.Amount <= 0 {
		return errors.New("amount must be greater than 0")
	}
	if strings.TrimSpace(r.Category) == "" {
		return errors.New("category is required")
	}
	if strings.TrimSpace(r.StartDate) == "" {
		return errors.New("start_date is required")
	}
	if r.Frequency != "weekly" && r.Frequency != "monthly" {
		return errors.New("frequency must be 'weekly' or 'monthly'")
	}
	return nil
}
