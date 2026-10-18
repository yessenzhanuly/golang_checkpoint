// Package password implements the processing logic of the SIS project:
// rating the strength of a password against a set of rules.
package password

import (
	"errors"
	"slices"

	"sis/internal/task"
)

const (
	maxScore        = 100
	blockedScoreCap = 20 // highest score possible when a blocking rule fails
)

// ErrNoRules is returned when the analyzer has nothing to score with.
var ErrNoRules = errors.New("analyzer has no scoring rules")

// Analyzer rates passwords using a list of rules.
// It is stateless after construction, so one Analyzer can be reused for
// any number of tasks.
type Analyzer struct {
	rules []Rule
}

// NewAnalyzer creates an Analyzer with the given rules.
func NewAnalyzer(rules ...Rule) *Analyzer {
	return &Analyzer{rules: slices.Clone(rules)}
}

// NewDefaultAnalyzer creates an Analyzer with DefaultRules.
func NewDefaultAnalyzer() *Analyzer {
	return NewAnalyzer(DefaultRules()...)
}

// Process scores the password in the input. The score is the share of rule
// points earned, scaled to 0..100. If a blocking rule is violated, the score
// is capped at blockedScoreCap. The result is deterministic.
func (a *Analyzer) Process(in task.Input) (task.Result, error) {
	var total, earned int
	blocked := false
	violations := make([]string, 0, len(a.rules))

	for _, r := range a.rules {
		total += r.Points()
		if r.Passes(in.Password) {
			earned += r.Points()
			continue
		}
		violations = append(violations, r.Name())
		if r.Blocking() {
			blocked = true
		}
	}
	if total <= 0 {
		return task.Result{}, ErrNoRules
	}

	score := earned * maxScore / total
	if blocked {
		score = min(score, blockedScoreCap)
	}
	return task.Result{Score: score, Level: levelFor(score), Violations: violations}, nil
}

// levelFor maps a score to a strength category.
func levelFor(score int) task.Level {
	switch {
	case score < 40:
		return task.LevelWeak
	case score < 75:
		return task.LevelMedium
	case score < 95:
		return task.LevelStrong
	default:
		return task.LevelVeryStrong
	}
}
