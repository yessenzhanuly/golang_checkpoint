package password_test

import (
	"errors"
	"slices"
	"testing"

	"sis/internal/password"
	"sis/internal/task"
)

// Names of the default rules, as reported in Result.Violations.
const (
	vLen8    = "at least 8 characters"
	vLen12   = "at least 12 characters"
	vLower   = "a lowercase letter"
	vUpper   = "an uppercase letter"
	vDigit   = "a digit"
	vSymbol  = "a symbol or punctuation mark"
	vRepeat  = "no character repeated 3 or more times in a row"
	vCommon  = "not a commonly used password"
	custName = "custom requirement"
)

func TestAnalyzerProcess(t *testing.T) {
	tests := []struct {
		name           string
		password       string
		wantScore      int
		wantLevel      task.Level
		wantViolations []string
	}{
		{
			name:     "short lowercase only",
			password: "abc", wantScore: 30, wantLevel: task.LevelWeak,
			wantViolations: []string{vLen8, vLen12, vUpper, vDigit, vSymbol},
		},
		{
			name:     "common password is capped at weak",
			password: "password", wantScore: 20, wantLevel: task.LevelWeak,
			wantViolations: []string{vLen12, vUpper, vDigit, vSymbol, vCommon},
		},
		{
			name:     "common password check is case-insensitive",
			password: "PASSWORD", wantScore: 20, wantLevel: task.LevelWeak,
			wantViolations: []string{vLen12, vLower, vDigit, vSymbol, vCommon},
		},
		{
			name:     "medium",
			password: "Passw0rd", wantScore: 70, wantLevel: task.LevelMedium,
			wantViolations: []string{vLen12, vSymbol},
		},
		{
			name:     "strong",
			password: "Tr0ub4dor&3", wantScore: 85, wantLevel: task.LevelStrong,
			wantViolations: []string{vLen12},
		},
		{
			name:     "long but repetitive",
			password: "aaaBBB111!!!", wantScore: 90, wantLevel: task.LevelStrong,
			wantViolations: []string{vRepeat},
		},
		{
			name:     "very strong",
			password: "correct-Horse-battery-9", wantScore: 100, wantLevel: task.LevelVeryStrong,
			wantViolations: nil,
		},
	}

	a := password.NewDefaultAnalyzer()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := a.Process(task.Input{Password: tt.password})
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}
			if got.Score != tt.wantScore {
				t.Errorf("Score = %d, want %d", got.Score, tt.wantScore)
			}
			if got.Level != tt.wantLevel {
				t.Errorf("Level = %s, want %s", got.Level, tt.wantLevel)
			}
			if !slices.Equal(got.Violations, tt.wantViolations) {
				t.Errorf("Violations = %q, want %q", got.Violations, tt.wantViolations)
			}
		})
	}
}

func TestAnalyzerIsDeterministic(t *testing.T) {
	a := password.NewDefaultAnalyzer()
	in := task.Input{Password: "Tr0ub4dor&3"}

	first, _ := a.Process(in)
	second, _ := a.Process(in)

	if first.Score != second.Score || first.Level != second.Level || !slices.Equal(first.Violations, second.Violations) {
		t.Errorf("same input gave different results: %+v vs %+v", first, second)
	}
}

func TestAnalyzerWithoutRules(t *testing.T) {
	tests := []struct {
		name string
		a    *password.Analyzer
	}{
		{"no rules", password.NewAnalyzer()},
		{"rules without weight", password.NewAnalyzer(customRule{points: 0, ok: true})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.a.Process(task.Input{Password: "anything"})
			if !errors.Is(err, password.ErrNoRules) {
				t.Fatalf("error = %v, want ErrNoRules", err)
			}
		})
	}
}

// customRule shows that the Analyzer works with any Rule implementation.
type customRule struct {
	points   int
	blocking bool
	ok       bool
}

func (customRule) Name() string         { return custName }
func (r customRule) Points() int        { return r.points }
func (r customRule) Blocking() bool     { return r.blocking }
func (r customRule) Passes(string) bool { return r.ok }

func TestAnalyzerCustomRules(t *testing.T) {
	tests := []struct {
		name           string
		rules          []password.Rule
		wantScore      int
		wantLevel      task.Level
		wantViolations []string
	}{
		{"single passing rule", []password.Rule{customRule{points: 10, ok: true}}, 100, task.LevelVeryStrong, nil},
		{"single failing rule", []password.Rule{customRule{points: 10}}, 0, task.LevelWeak, []string{custName}},
		{
			"half of the points",
			[]password.Rule{customRule{points: 50, ok: true}, customRule{points: 50}},
			50, task.LevelMedium, []string{custName},
		},
		{
			"failed blocking rule caps the score",
			[]password.Rule{customRule{points: 50, ok: true}, customRule{points: 50, blocking: true}},
			20, task.LevelWeak, []string{custName},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := password.NewAnalyzer(tt.rules...).Process(task.Input{Password: "x"})
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}
			if got.Score != tt.wantScore || got.Level != tt.wantLevel || !slices.Equal(got.Violations, tt.wantViolations) {
				t.Errorf("got %+v, want score=%d level=%s violations=%q",
					got, tt.wantScore, tt.wantLevel, tt.wantViolations)
			}
		})
	}
}

func TestRules(t *testing.T) {
	tests := []struct {
		name string
		rule password.Rule
		pass []string
		fail []string
	}{
		{
			name: "min length 8 counts runes",
			rule: password.MinLength(8, 1),
			pass: []string{"12345678", "пароль12"}, // 8 runes, 14 bytes
			fail: []string{"", "1234567", "пароль1"},
		},
		{
			name: "lowercase letter",
			rule: password.RequireLower(1),
			pass: []string{"aBC", "ёж"},
			fail: []string{"", "ABC", "123"},
		},
		{
			name: "uppercase letter",
			rule: password.RequireUpper(1),
			pass: []string{"abC", "Ёж"},
			fail: []string{"", "abc", "123"},
		},
		{
			name: "digit",
			rule: password.RequireDigit(1),
			pass: []string{"a1", "007"},
			fail: []string{"", "abc", "!!!"},
		},
		{
			name: "symbol",
			rule: password.RequireSymbol(1),
			pass: []string{"a!", "a-b", "a+b", "a$"},
			fail: []string{"", "abc123", "ABC"},
		},
		{
			name: "no triple repeat",
			rule: password.NoTripleRepeat(1),
			pass: []string{"", "aa", "aabb", "abab", "aabaa"},
			fail: []string{"aaa", "xbbbx", "1111"},
		},
		{
			name: "not common",
			rule: password.NotCommon(1),
			pass: []string{"Tr0ub4dor&3", "correct-Horse-battery-9"},
			fail: []string{"password", "PassWord", "123456", "qwerty"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, in := range tt.pass {
				if !tt.rule.Passes(in) {
					t.Errorf("Passes(%q) = false, want true", in)
				}
			}
			for _, in := range tt.fail {
				if tt.rule.Passes(in) {
					t.Errorf("Passes(%q) = true, want false", in)
				}
			}
		})
	}
}

func TestDefaultRulesWeightsAddUpTo100(t *testing.T) {
	total := 0
	for _, r := range password.DefaultRules() {
		total += r.Points()
	}
	if total != 100 {
		t.Errorf("total points = %d, want 100", total)
	}
}
