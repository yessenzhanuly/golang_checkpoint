package task

import (
	"errors"
	"fmt"
)

// Sentinel errors of the domain. Callers compare them with errors.Is, so the
// functions that return them may wrap them with extra context (fmt.Errorf %w).
var (
	ErrEmptyPassword     = errors.New("password must not be empty")
	ErrPasswordTooLong   = fmt.Errorf("password must not be longer than %d characters", MaxPasswordLength)
	ErrNotFound          = errors.New("task not found")
	ErrAlreadyExists     = errors.New("task already exists")
	ErrInvalidTransition = errors.New("invalid state transition")
)

// TransitionError describes a forbidden state change. It carries the states
// involved (use errors.As to read them) and unwraps to ErrInvalidTransition
// (use errors.Is to detect the whole class of errors).
type TransitionError struct {
	From, To Status
}

func (e *TransitionError) Error() string {
	return fmt.Sprintf("%v: %s -> %s", ErrInvalidTransition, e.From, e.To)
}

func (e *TransitionError) Unwrap() error { return ErrInvalidTransition }
