// Package task contains the domain model of the SIS project: what a task is,
// what its input and result look like, and how its state may change.
// It has no dependencies on the rest of the application.
package task

import (
	"fmt"
	"slices"
	"unicode/utf8"
)

// MaxPasswordLength is the maximum number of characters (runes) accepted as
// input. It protects the application from absurdly large payloads.
const MaxPasswordLength = 128

// Input is the domain-specific payload of a task: a password to be analyzed.
type Input struct {
	Password string
}

// Validate checks the input and returns a sentinel error when it is invalid.
func (i Input) Validate() error {
	if i.Password == "" {
		return ErrEmptyPassword
	}
	if utf8.RuneCountInString(i.Password) > MaxPasswordLength {
		return ErrPasswordTooLong
	}
	return nil
}

// String implements fmt.Stringer and never reveals the password, so logging a
// Task or an Input with %v, %+v or %s cannot leak it by accident.
func (i Input) String() string { return "[redacted]" }

// GoString makes %#v redact the password as well.
func (i Input) GoString() string { return i.String() }

// Level is the human-readable strength category of a password.
type Level int

const (
	LevelWeak Level = iota
	LevelMedium
	LevelStrong
	LevelVeryStrong
)

func (l Level) String() string {
	switch l {
	case LevelWeak:
		return "weak"
	case LevelMedium:
		return "medium"
	case LevelStrong:
		return "strong"
	case LevelVeryStrong:
		return "very strong"
	default:
		return fmt.Sprintf("Level(%d)", int(l))
	}
}

// Result is what processing a task produces.
type Result struct {
	Score      int      // 0..100
	Level      Level    // category derived from Score
	Violations []string // unmet requirements, in rule order; empty if none
}

// Task is a unit of work: analyze the strength of one password.
//
// Receivers: methods that change the task use a pointer receiver, so the
// change is visible to the caller. Read-only methods use a value receiver
// because the struct is small and copying it is cheap and safe.
type Task struct {
	ID         string
	Input      Input
	Status     Status
	Result     *Result // nil until the task has completed successfully
	FailReason string  // set only when Status == StatusFailed
}

// New creates a task in the Pending state.
func New(id string, in Input) Task {
	return Task{ID: id, Input: in, Status: StatusPending}
}

// Start moves the task from Pending to Processing.
func (t *Task) Start() error {
	return t.transition(StatusProcessing)
}

// Complete moves the task from Processing to Completed and stores the result.
func (t *Task) Complete(r Result) error {
	if err := t.transition(StatusCompleted); err != nil {
		return err
	}
	r.Violations = slices.Clone(r.Violations) // do not share the caller's slice
	t.Result = &r
	return nil
}

// Fail moves the task from Processing to Failed and records the reason.
func (t *Task) Fail(reason string) error {
	if err := t.transition(StatusFailed); err != nil {
		return err
	}
	t.FailReason = reason
	return nil
}

// transition changes the status only if the move is allowed.
func (t *Task) transition(next Status) error {
	if !t.Status.CanTransitionTo(next) {
		return &TransitionError{From: t.Status, To: next}
	}
	t.Status = next
	return nil
}

// Clone returns a deep copy of the task. A plain assignment would copy the
// Result pointer, and both copies would then share (and could corrupt) the
// same Result; Clone prevents that.
func (t Task) Clone() Task {
	if t.Result != nil {
		r := *t.Result
		r.Violations = slices.Clone(r.Violations)
		t.Result = &r
	}
	return t
}

// String returns a one-line description that never contains the password.
func (t Task) String() string {
	s := fmt.Sprintf("%s [%s] input=%s", t.ID, t.Status, t.Input)
	if t.Result != nil {
		s += fmt.Sprintf(" score=%d level=%s", t.Result.Score, t.Result.Level)
	}
	if t.Status == StatusFailed {
		s += fmt.Sprintf(" reason=%q", t.FailReason)
	}
	return s
}
