package store_test

import (
	"errors"
	"slices"
	"testing"

	"sis/internal/store"
	"sis/internal/task"
)

func newTask(id, pw string) task.Task {
	return task.New(id, task.Input{Password: pw})
}

func TestAddAndGet(t *testing.T) {
	s := store.NewMemoryStore()
	if err := s.Add(newTask("task-1", "secret")); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	got, err := s.Get("task-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.ID != "task-1" || got.Input.Password != "secret" || got.Status != task.StatusPending {
		t.Errorf("Get() = %+v, unexpected contents", got)
	}
}

func TestGetNotFound(t *testing.T) {
	s := store.NewMemoryStore()
	_, err := s.Get("missing")
	if !errors.Is(err, task.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestAddDuplicate(t *testing.T) {
	s := store.NewMemoryStore()
	_ = s.Add(newTask("task-1", "first"))

	err := s.Add(newTask("task-1", "second"))
	if !errors.Is(err, task.ErrAlreadyExists) {
		t.Fatalf("error = %v, want ErrAlreadyExists", err)
	}
	got, _ := s.Get("task-1")
	if got.Input.Password != "first" {
		t.Errorf("duplicate Add overwrote the stored task: %+v", got)
	}
}

func TestUpdate(t *testing.T) {
	s := store.NewMemoryStore()
	_ = s.Add(newTask("task-1", "secret"))

	tk, _ := s.Get("task-1")
	_ = tk.Start()
	if err := s.Update(tk); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, _ := s.Get("task-1")
	if got.Status != task.StatusProcessing {
		t.Errorf("Status = %s, want Processing", got.Status)
	}
}

func TestUpdateNotFound(t *testing.T) {
	s := store.NewMemoryStore()
	err := s.Update(newTask("missing", "x"))
	if !errors.Is(err, task.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestStoredDataIsIsolatedFromCallers(t *testing.T) {
	s := store.NewMemoryStore()

	tk := newTask("task-1", "secret")
	_ = tk.Start()
	_ = tk.Complete(task.Result{Score: 10, Violations: []string{"a digit"}})
	_ = s.Add(tk)

	// Changing the value passed to Add must not change the stored task.
	tk.Result.Score = 99

	// Changing a value returned by Get must not change the stored task.
	got, _ := s.Get("task-1")
	got.Result.Score = 55
	got.Result.Violations[0] = "changed"
	got.Status = task.StatusFailed

	again, _ := s.Get("task-1")
	if again.Result.Score != 10 || again.Result.Violations[0] != "a digit" || again.Status != task.StatusCompleted {
		t.Errorf("stored task was modified from outside: %+v", again)
	}
}

func TestListKeepsInsertionOrder(t *testing.T) {
	s := store.NewMemoryStore()
	if got := s.List(); len(got) != 0 {
		t.Fatalf("List() of empty store = %v, want empty", got)
	}

	ids := []string{"task-3", "task-1", "task-2"}
	for _, id := range ids {
		_ = s.Add(newTask(id, "pw-"+id))
	}

	var got []string
	for _, tk := range s.List() {
		got = append(got, tk.ID)
	}
	if !slices.Equal(got, ids) {
		t.Errorf("List() IDs = %v, want %v", got, ids)
	}
}
