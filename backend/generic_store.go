package main

import (
	"encoding/json"
	"errors"
	"os"
	"sync"
)

// Identifiable is implemented by pointer types that have a mutable string ID.
type Identifiable interface {
	GetID() string
	SetID(string)
}

// Owned extends Identifiable with a mutable tenant (see tenant.go).
// Category, Budget, RecurringRule, and Goal all satisfy this, which lets
// a single generic FileStore handle CRUD for all four instead of
// copy-pasting store.go four times.
type Owned interface {
	Identifiable
	GetUserID() string
	SetUserID(string)
}

// FileStore is a generic version of the Store type in store.go, holding
// every tenant's items in one file and filtering per call. T is
// constrained to pointer types (e.g. *Category) so mutations made inside
// Create/Update are visible to the caller and persist correctly in the map.
type FileStore[T Owned] struct {
	mu       sync.RWMutex
	filePath string
	items    map[string]T
}

func NewFileStore[T Owned](filePath string) (*FileStore[T], error) {
	s := &FileStore[T]{
		filePath: filePath,
		items:    make(map[string]T),
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *FileStore[T]) load() error {
	data, err := os.ReadFile(s.filePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	var list []T
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}
	for _, item := range list {
		s.items[item.GetID()] = item
	}
	return nil
}

func (s *FileStore[T]) persist() error {
	list := make([]T, 0, len(s.items))
	for _, item := range s.items {
		list = append(list, item)
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

func (s *FileStore[T]) Create(tenantID string, item T) (T, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item.SetID(newID())
	item.SetUserID(tenantID)
	s.items[item.GetID()] = item
	if err := s.persist(); err != nil {
		var zero T
		return zero, err
	}
	return item, nil
}

func (s *FileStore[T]) List(tenantID string) []T {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]T, 0, len(s.items))
	for _, item := range s.items {
		if item.GetUserID() != tenantID {
			continue
		}
		list = append(list, item)
	}
	return list
}

// ListAll is unfiltered, for internal background jobs (e.g. the recurring-
// transaction generator) that have to sweep every tenant's items, not just
// one request's. Never call this from a handler.
func (s *FileStore[T]) ListAll() []T {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]T, 0, len(s.items))
	for _, item := range s.items {
		list = append(list, item)
	}
	return list
}

func (s *FileStore[T]) Get(tenantID, id string) (T, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, ok := s.items[id]
	if !ok || item.GetUserID() != tenantID {
		var zero T
		return zero, ErrNotFound
	}
	return item, nil
}

func (s *FileStore[T]) Update(tenantID, id string, updated T) (T, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.items[id]
	if !ok || existing.GetUserID() != tenantID {
		var zero T
		return zero, ErrNotFound
	}
	updated.SetID(id)
	updated.SetUserID(tenantID)
	s.items[id] = updated
	if err := s.persist(); err != nil {
		var zero T
		return zero, err
	}
	return updated, nil
}

func (s *FileStore[T]) Delete(tenantID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.items[id]
	if !ok || existing.GetUserID() != tenantID {
		return ErrNotFound
	}
	delete(s.items, id)
	return s.persist()
}

// IsEmptyFor is used to decide whether to seed default data for a tenant
// (the legacy shared household at startup, or a freshly registered
// account — see main.go and HandleAuthRegister).
func (s *FileStore[T]) IsEmptyFor(tenantID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.items {
		if item.GetUserID() == tenantID {
			return false
		}
	}
	return true
}
