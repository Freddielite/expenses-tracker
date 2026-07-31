package main

// These interfaces describe the CRUD surface each concrete store already
// exposes (Store in store.go, FileStore[T] in generic_store.go). Handlers,
// recurring.go, and main.go depend on these interfaces rather than the
// concrete JSON-backed types, so a Postgres-backed implementation
// (pg_store.go) is a drop-in swap — see storage.go for how the choice is
// made at startup.
//
// Every method takes a tenantID first (see tenant.go) — the shared legacy
// PIN household ("") or a registered account's own ID — so each
// implementation is responsible for only ever returning or mutating that
// tenant's own records. Create always sets the created item's owning
// tenant from this parameter, never from caller-supplied data; Get/Update/
// Delete treat another tenant's item as if it doesn't exist (ErrNotFound),
// not as a permission error, so a guessed ID can't be used to even confirm
// another tenant has a record with that ID.

type TransactionStore interface {
	Create(tenantID string, t *Transaction) error
	List(tenantID string) []*Transaction
	Get(tenantID, id string) (*Transaction, error)
	Update(tenantID, id string, updated *Transaction) (*Transaction, error)
	Delete(tenantID, id string) error
}

type CategoryStore interface {
	Create(tenantID string, item *Category) (*Category, error)
	List(tenantID string) []*Category
	Get(tenantID, id string) (*Category, error)
	Update(tenantID, id string, updated *Category) (*Category, error)
	Delete(tenantID, id string) error
	IsEmptyFor(tenantID string) bool
}

type BudgetStore interface {
	Create(tenantID string, item *Budget) (*Budget, error)
	List(tenantID string) []*Budget
	Get(tenantID, id string) (*Budget, error)
	Update(tenantID, id string, updated *Budget) (*Budget, error)
	Delete(tenantID, id string) error
}

type RecurringStore interface {
	Create(tenantID string, item *RecurringRule) (*RecurringRule, error)
	List(tenantID string) []*RecurringRule
	Get(tenantID, id string) (*RecurringRule, error)
	Update(tenantID, id string, updated *RecurringRule) (*RecurringRule, error)
	Delete(tenantID, id string) error
	// ListAll is unfiltered — for the background recurring-transaction
	// generator only (recurring.go), which has to sweep every tenant's
	// due rules, not just one request's. Never call this from a handler.
	ListAll() []*RecurringRule
}
