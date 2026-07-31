package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

func (a *API) HandleCategories(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, a.categories.List(tenantFrom(r)))

	case http.MethodPost:
		var c Category
		if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if err := validateCategory(&c); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		created, err := a.categories.Create(tenantFrom(r), &c)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save category")
			return
		}
		writeJSON(w, http.StatusCreated, created)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (a *API) HandleCategoryByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/categories/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing category id")
		return
	}

	switch r.Method {
	case http.MethodPut:
		var c Category
		if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if err := validateCategory(&c); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		updated, err := a.categories.Update(tenantFrom(r), id, &c)
		if err != nil {
			handleGenericStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, updated)

	case http.MethodDelete:
		if err := a.categories.Delete(tenantFrom(r), id); err != nil {
			handleGenericStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusNoContent, nil)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func validateCategory(c *Category) error {
	if strings.TrimSpace(c.Name) == "" {
		return errors.New("name is required")
	}
	if c.Type != TypeIncome && c.Type != TypeExpense {
		return errors.New("type must be 'income' or 'expense'")
	}
	if strings.TrimSpace(c.Color) == "" {
		c.Color = "#6b6f76"
	}
	if strings.TrimSpace(c.Icon) == "" {
		c.Icon = "more-horizontal"
	}
	return nil
}

func handleGenericStoreErr(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeError(w, http.StatusInternalServerError, "internal error")
}
