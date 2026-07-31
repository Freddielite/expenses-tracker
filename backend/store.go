package main

import (
	"encoding/json"
	"errors"
	"os"
	"sort"
	"sync"
	"time"
)

// Store is a thread-safe, file-backed collection of transactions across
// every tenant (see tenant.go) — every mutation is written straight to
// disk so the app survives restarts without needing a real database.
type Store struct {
	mu           sync.RWMutex
	filePath     string
	transactions map[string]*Transaction
}

var ErrNotFound = errors.New("transaction not found")

func NewStore(filePath string) (*Store, error) {
	s := &Store{
		filePath:     filePath,
		transactions: make(map[string]*Transaction),
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.filePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil // fresh start, nothing to load yet
	}
	if err != nil {
		return err
	}
	var list []*Transaction
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}
	for _, t := range list {
		s.transactions[t.ID] = t
	}
	return nil
}

// persist must be called with s.mu already held (read or write lock is fine
// for reading the map, but we always call it after mutations under write lock).
func (s *Store) persist() error {
	list := make([]*Transaction, 0, len(s.transactions))
	for _, t := range s.transactions {
		list = append(list, t)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Date > list[j].Date })

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

func (s *Store) Create(tenantID string, t *Transaction) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t.ID = newID()
	t.CreatedAt = time.Now().UTC()
	t.UserID = tenantID
	s.transactions[t.ID] = t
	return s.persist()
}

func (s *Store) List(tenantID string) []*Transaction {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*Transaction, 0, len(s.transactions))
	for _, t := range s.transactions {
		if t.UserID != tenantID {
			continue
		}
		list = append(list, t)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Date > list[j].Date })
	return list
}

func (s *Store) Get(tenantID, id string) (*Transaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.transactions[id]
	if !ok || t.UserID != tenantID {
		return nil, ErrNotFound
	}
	return t, nil
}

func (s *Store) Update(tenantID, id string, updated *Transaction) (*Transaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.transactions[id]
	if !ok || existing.UserID != tenantID {
		return nil, ErrNotFound
	}

	updated.ID = existing.ID
	updated.CreatedAt = existing.CreatedAt
	updated.UserID = existing.UserID
	s.transactions[id] = updated

	if err := s.persist(); err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *Store) Delete(tenantID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.transactions[id]
	if !ok || existing.UserID != tenantID {
		return ErrNotFound
	}
	delete(s.transactions, id)
	return s.persist()
}
