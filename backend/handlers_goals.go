package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// HandleGoals handles GET (list) and POST (create) on /api/goals.
func (a *API) HandleGoals(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, a.goals.List(tenantFrom(r)))

	case http.MethodPost:
		var g Goal
		if err := json.NewDecoder(r.Body).Decode(&g); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if err := validateGoal(&g); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		g.SavedAmount = 0 // a new goal always starts at zero saved, regardless of what's in the body
		created, err := a.goals.Create(tenantFrom(r), &g)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save goal")
			return
		}
		writeJSON(w, http.StatusCreated, created)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// HandleGoalByID handles PUT/DELETE on /api/goals/{id} and POST on
// /api/goals/{id}/contribute. The latter is routed here too and split
// manually — Go 1.22's ServeMux doesn't support wildcard sub-paths without
// registering a second, more specific pattern, and this keeps route
// registration in main.go to one line per resource like everything else.
func (a *API) HandleGoalByID(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/goals/")
	if rest == "" {
		writeError(w, http.StatusBadRequest, "missing goal id")
		return
	}

	if id, ok := strings.CutSuffix(rest, "/contribute"); ok {
		a.handleGoalContribute(w, r, id)
		return
	}
	id := rest

	switch r.Method {
	case http.MethodPut:
		var g Goal
		if err := json.NewDecoder(r.Body).Decode(&g); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if err := validateGoal(&g); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		// Editing a goal's name/target shouldn't reset progress already
		// made — carry the existing saved_amount forward unless the
		// request body explicitly sets one.
		existing, err := a.goals.Get(tenantFrom(r), id)
		if err != nil {
			handleGenericStoreErr(w, err)
			return
		}
		if g.SavedAmount == 0 {
			g.SavedAmount = existing.SavedAmount
		}
		updated, err := a.goals.Update(tenantFrom(r), id, &g)
		if err != nil {
			handleGenericStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, updated)

	case http.MethodDelete:
		if err := a.goals.Delete(tenantFrom(r), id); err != nil {
			handleGenericStoreErr(w, err)
			return
		}
		writeJSON(w, http.StatusNoContent, nil)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (a *API) handleGoalContribute(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	existing, err := a.goals.Get(tenantFrom(r), id)
	if err != nil {
		handleGenericStoreErr(w, err)
		return
	}

	var contribution GoalContribution
	if err := json.NewDecoder(r.Body).Decode(&contribution); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if contribution.Amount == 0 {
		writeError(w, http.StatusBadRequest, "amount must be non-zero")
		return
	}

	newSaved := existing.SavedAmount + contribution.Amount
	if newSaved < 0 {
		writeError(w, http.StatusBadRequest, "that would make saved_amount negative")
		return
	}
	existing.SavedAmount = newSaved

	updated, err := a.goals.Update(tenantFrom(r), id, existing)
	if err != nil {
		handleGenericStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func validateGoal(g *Goal) error {
	if strings.TrimSpace(g.Name) == "" {
		return errors.New("name is required")
	}
	if g.TargetAmount <= 0 {
		return errors.New("target_amount must be greater than 0")
	}
	return nil
}
