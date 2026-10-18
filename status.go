package task

import (
	"fmt"
	"slices"
)

// Status is the position of a task in its processing lifecycle.
// The zero value is StatusPending, so a freshly created Task is valid.
type Status int

const (
	StatusPending    Status = iota // created, not processed yet
	StatusProcessing               // processing is in progress
	StatusCompleted                // processed successfully, Result is set
	StatusFailed                   // processing failed, FailReason is set
)

func (s Status) String() string {
	switch s {
	case StatusPending:
		return "Pending"
	case StatusProcessing:
		return "Processing"
	case StatusCompleted:
		return "Completed"
	case StatusFailed:
		return "Failed"
	default:
		return fmt.Sprintf("Status(%d)", int(s))
	}
}

// transitions lists the allowed next states for every state.
// Completed and Failed are terminal: they have no entry.
var transitions = map[Status][]Status{
	StatusPending:    {StatusProcessing},
	StatusProcessing: {StatusCompleted, StatusFailed},
}

// CanTransitionTo reports whether a task may move from s to next.
func (s Status) CanTransitionTo(next Status) bool {
	return slices.Contains(transitions[s], next)
}

// IsTerminal reports whether no further transitions are possible from s.
func (s Status) IsTerminal() bool {
	return len(transitions[s]) == 0
}
