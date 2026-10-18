// Package store keeps tasks in memory.
package store

import (
	"fmt"

	"sis/internal/task"
)

// MemoryStore is an in-memory task storage.
//
// It keeps tasks BY VALUE in a map (fast lookup by ID) and remembers the
// insertion order in a slice (a map has no order, but List should be stable).
// Tasks are cloned on the way in and on the way out, so callers can never
// modify stored data by accident - every change must go through Update.
//
// MemoryStore is not safe for concurrent use; concurrency is out of scope
// for this checkpoint.
type MemoryStore struct {
	tasks map[string]task.Task
	order []string
}

// NewMemoryStore creates an empty store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{tasks: make(map[string]task.Task)}
}

// Add stores a new task. It fails if a task with the same ID already exists.
func (s *MemoryStore) Add(t task.Task) error {
	if _, exists := s.tasks[t.ID]; exists {
		return fmt.Errorf("%w: %q", task.ErrAlreadyExists, t.ID)
	}
	s.tasks[t.ID] = t.Clone()
	s.order = append(s.order, t.ID)
	return nil
}

// Get returns a copy of the task with the given ID.
func (s *MemoryStore) Get(id string) (task.Task, error) {
	t, ok := s.tasks[id]
	if !ok {
		return task.Task{}, fmt.Errorf("%w: %q", task.ErrNotFound, id)
	}
	return t.Clone(), nil
}

// Update replaces a stored task with the given one. The task must exist.
func (s *MemoryStore) Update(t task.Task) error {
	if _, ok := s.tasks[t.ID]; !ok {
		return fmt.Errorf("%w: %q", task.ErrNotFound, t.ID)
	}
	s.tasks[t.ID] = t.Clone()
	return nil
}

// List returns copies of all tasks in the order they were added.
func (s *MemoryStore) List() []task.Task {
	out := make([]task.Task, 0, len(s.order))
	for _, id := range s.order {
		out = append(out, s.tasks[id].Clone())
	}
	return out
}
