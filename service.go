// Package service contains the application logic: it creates tasks, runs them
// through the processor and keeps their lifecycle consistent.
package service

import (
	"fmt"

	"sis/internal/task"
)

// Store is the storage the service needs. It is declared here, next to its
// only user ("accept interfaces"), so the service does not depend on a
// concrete storage and tests can plug in any implementation.
type Store interface {
	Add(t task.Task) error
	Get(id string) (task.Task, error)
	Update(t task.Task) error
	List() []task.Task
}

// Processor turns a task input into a result.
type Processor interface {
	Process(in task.Input) (task.Result, error)
}

// Service coordinates storage and processing.
type Service struct {
	store     Store
	processor Processor
	nextID    int
}

// New creates a service from its dependencies.
func New(s Store, p Processor) *Service {
	return &Service{store: s, processor: p}
}

// Create validates the input, assigns a unique ID and stores a new Pending
// task. Invalid input is rejected and nothing is stored.
func (s *Service) Create(in task.Input) (task.Task, error) {
	if err := in.Validate(); err != nil {
		return task.Task{}, fmt.Errorf("create task: %w", err)
	}

	s.nextID++
	t := task.New(fmt.Sprintf("task-%d", s.nextID), in)
	if err := s.store.Add(t); err != nil {
		return task.Task{}, fmt.Errorf("create task: %w", err)
	}
	return t, nil
}

// Get returns the task with the given ID.
func (s *Service) Get(id string) (task.Task, error) {
	t, err := s.store.Get(id)
	if err != nil {
		return task.Task{}, fmt.Errorf("get task: %w", err)
	}
	return t, nil
}

// List returns all tasks in creation order.
func (s *Service) List() []task.Task {
	return s.store.List()
}

// Process runs the task through its lifecycle:
//
//	Pending -> Processing -> Completed (result stored)
//	                      \-> Failed    (reason stored, processor error returned)
//
// The Processing state is saved before the processor runs, so the stored state
// always matches what is really happening. Processing a task that is not
// Pending is an invalid operation and returns task.ErrInvalidTransition.
func (s *Service) Process(id string) (task.Task, error) {
	wrap := func(err error) error { return fmt.Errorf("process task %q: %w", id, err) }

	t, err := s.store.Get(id)
	if err != nil {
		return task.Task{}, wrap(err)
	}

	if err := t.Start(); err != nil {
		return t, wrap(err)
	}
	if err := s.store.Update(t); err != nil {
		return t, wrap(err)
	}

	result, procErr := s.processor.Process(t.Input)
	if procErr != nil {
		if err := t.Fail(procErr.Error()); err != nil {
			return t, wrap(err)
		}
		if err := s.store.Update(t); err != nil {
			return t, wrap(err)
		}
		return t, wrap(procErr)
	}

	if err := t.Complete(result); err != nil {
		return t, wrap(err)
	}
	if err := s.store.Update(t); err != nil {
		return t, wrap(err)
	}
	return t, nil
}
