package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
)

type API struct {
	store      TransactionStore
	categories CategoryStore
	budgets    BudgetStore
	recurring  RecurringStore
	// goals is kept as the concrete JSON-backed store rather than an
	// interface, same deliberate choice as auth — small, not part of the
	// Postgres swap yet (see storage.go / main.go).
	goals *FileStore[*Goal]
	auth  *AuthStore
	users UserStore
}

func NewAPI(store TransactionStore, categories CategoryStore, budgets BudgetStore, recurring RecurringStore, goals *FileStore[*Goal], auth *AuthStore, users UserStore) *API {
	return &API{store: store, categories: categories, budgets: budgets, recurring: recurring, goals: goals, auth: auth, users: users}
}

// writeJSON is a small helper to keep handlers tidy.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload != nil {
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			log.Printf("error encoding response: %v", err)
		}
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// ---- Transactions ----

// HandleTransactions handles GET (list) and POST (create) on /api/transactions
func (a *API) HandleTransactions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list := a.store.List(tenantFrom(r))
		writeJSON(w, http.StatusOK, list)

	case http.MethodPost:
		var t Transaction
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if err := validateTransaction(&t); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := a.store.Create(tenantFrom(r), &t); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save transaction")
			return
		}
		writeJSON(w, http.StatusCreated, t)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// HandleTransactionByID handles GET, PUT, PATCH, DELETE on /api/transactions/{id}
func (a *API) HandleTransactionByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/transactions/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing transaction id")
		return
	}

	switch r.Method {
	case http.MethodGet:
		t, err := a.store.Get(tenantFrom(r), id)
		if err != nil {
			handleStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, t)

	case http.MethodPut:
		var t Transaction
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if err := validateTransaction(&t); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		updated, err := a.store.Update(tenantFrom(r), id, &t)
		if err != nil {
			handleStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, updated)

	case http.MethodPatch:
		existing, err := a.store.Get(tenantFrom(r), id)
		if err != nil {
			handleStoreErr(w, err)
			return
		}
		var patch TransactionPatch
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		merged := patch.Apply(existing)
		if err := validateTransaction(merged); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		updated, err := a.store.Update(tenantFrom(r), id, merged)
		if err != nil {
			handleStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, updated)

	case http.MethodDelete:
		if err := a.store.Delete(tenantFrom(r), id); err != nil {
			handleStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusNoContent, nil)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// HandleReports handles GET /api/reports
func (a *API) HandleReports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	report := BuildReport(a.store.List(tenantFrom(r)))
	writeJSON(w, http.StatusOK, report)
}

func handleStoreErr(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "transaction not found")
		return
	}
	writeError(w, http.StatusInternalServerError, "internal error")
}

func validateTransaction(t *Transaction) error {
	if t.Type != TypeIncome && t.Type != TypeExpense {
		return errors.New("type must be 'income' or 'expense'")
	}
	if t.Amount <= 0 {
		return errors.New("amount must be greater than 0")
	}
	if strings.TrimSpace(t.Category) == "" {
		return errors.New("category is required")
	}
	if strings.TrimSpace(t.Date) == "" {
		return errors.New("date is required")
	}
	return nil
}
