package task_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"sis/internal/task"
)

func TestInputValidate(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{"normal password", "Tr0ub4dor&3", nil},
		{"unicode password", "пароль", nil},
		{"single space is allowed", " ", nil},
		{"max length is allowed", strings.Repeat("a", task.MaxPasswordLength), nil},
		{"empty", "", task.ErrEmptyPassword},
		{"too long", strings.Repeat("a", task.MaxPasswordLength+1), task.ErrPasswordTooLong},
		{"length counts runes, not bytes", strings.Repeat("я", task.MaxPasswordLength), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := task.Input{Password: tt.password}.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestStatusCanTransitionTo(t *testing.T) {
	const (
		pending    = task.StatusPending
		processing = task.StatusProcessing
		completed  = task.StatusCompleted
		failed     = task.StatusFailed
	)
	tests := []struct {
		from, to task.Status
		want     bool
	}{
		{pending, processing, true},
		{processing, completed, true},
		{processing, failed, true},

		{pending, pending, false},
		{pending, completed, false},
		{pending, failed, false},
		{processing, pending, false},
		{processing, processing, false},
		{completed, pending, false},
		{completed, processing, false},
		{completed, failed, false},
		{failed, pending, false},
		{failed, processing, false},
		{failed, completed, false},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s to %s", tt.from, tt.to), func(t *testing.T) {
			if got := tt.from.CanTransitionTo(tt.to); got != tt.want {
				t.Errorf("CanTransitionTo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStatusIsTerminal(t *testing.T) {
	tests := map[task.Status]bool{
		task.StatusPending:    false,
		task.StatusProcessing: false,
		task.StatusCompleted:  true,
		task.StatusFailed:     true,
	}
	for s, want := range tests {
		if got := s.IsTerminal(); got != want {
			t.Errorf("%s.IsTerminal() = %v, want %v", s, got, want)
		}
	}
}

func TestNewTaskIsPending(t *testing.T) {
	tk := task.New("task-1", task.Input{Password: "secret"})
	if tk.Status != task.StatusPending {
		t.Errorf("Status = %s, want Pending", tk.Status)
	}
	if tk.Result != nil {
		t.Errorf("Result = %+v, want nil", tk.Result)
	}
	if tk.ID != "task-1" || tk.Input.Password != "secret" {
		t.Errorf("unexpected task contents: %+v", tk)
	}
}

func TestTaskHappyPath(t *testing.T) {
	tk := task.New("task-1", task.Input{Password: "secret"})

	if err := tk.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if tk.Status != task.StatusProcessing {
		t.Fatalf("Status after Start = %s, want Processing", tk.Status)
	}

	res := task.Result{Score: 42, Level: task.LevelMedium, Violations: []string{"a digit"}}
	if err := tk.Complete(res); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if tk.Status != task.StatusCompleted {
		t.Fatalf("Status after Complete = %s, want Completed", tk.Status)
	}
	if tk.Result == nil || tk.Result.Score != 42 || tk.Result.Level != task.LevelMedium {
		t.Fatalf("Result = %+v, want score 42 / medium", tk.Result)
	}

	// The task must not share the caller's slice.
	res.Violations[0] = "changed"
	if tk.Result.Violations[0] != "a digit" {
		t.Errorf("Result shares its Violations slice with the caller")
	}
}

func TestTaskFail(t *testing.T) {
	tk := task.New("task-1", task.Input{Password: "secret"})
	if err := tk.Start(); err != nil {
		t.Fatal(err)
	}
	if err := tk.Fail("boom"); err != nil {
		t.Fatalf("Fail() error = %v", err)
	}
	if tk.Status != task.StatusFailed || tk.FailReason != "boom" || tk.Result != nil {
		t.Errorf("unexpected failed task: %+v", tk)
	}
}

func TestTaskInvalidOperations(t *testing.T) {
	tests := []struct {
		name string
		prep func(*task.Task) // brings the task into the starting state
		op   func(*task.Task) error
		from task.Status
		to   task.Status
	}{
		{
			name: "complete a pending task",
			prep: func(*task.Task) {},
			op:   func(tk *task.Task) error { return tk.Complete(task.Result{}) },
			from: task.StatusPending, to: task.StatusCompleted,
		},
		{
			name: "fail a pending task",
			prep: func(*task.Task) {},
			op:   func(tk *task.Task) error { return tk.Fail("x") },
			from: task.StatusPending, to: task.StatusFailed,
		},
		{
			name: "start a task twice",
			prep: func(tk *task.Task) { _ = tk.Start() },
			op:   func(tk *task.Task) error { return tk.Start() },
			from: task.StatusProcessing, to: task.StatusProcessing,
		},
		{
			name: "start a completed task",
			prep: func(tk *task.Task) { _ = tk.Start(); _ = tk.Complete(task.Result{}) },
			op:   func(tk *task.Task) error { return tk.Start() },
			from: task.StatusCompleted, to: task.StatusProcessing,
		},
		{
			name: "complete a failed task",
			prep: func(tk *task.Task) { _ = tk.Start(); _ = tk.Fail("x") },
			op:   func(tk *task.Task) error { return tk.Complete(task.Result{Score: 1}) },
			from: task.StatusFailed, to: task.StatusCompleted,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tk := task.New("task-1", task.Input{Password: "secret"})
			tt.prep(&tk)
			before := tk.Clone()

			err := tt.op(&tk)
			if !errors.Is(err, task.ErrInvalidTransition) {
				t.Fatalf("error = %v, want ErrInvalidTransition", err)
			}
			var te *task.TransitionError
			if !errors.As(err, &te) {
				t.Fatalf("error %v is not a *TransitionError", err)
			}
			if te.From != tt.from || te.To != tt.to {
				t.Errorf("TransitionError = %s -> %s, want %s -> %s", te.From, te.To, tt.from, tt.to)
			}
			// A rejected operation must leave the task untouched.
			if tk.Status != before.Status || tk.Result != nil != (before.Result != nil) {
				t.Errorf("task changed by a rejected operation: before %+v, after %+v", before, tk)
			}
		})
	}
}

func TestTaskCloneIsDeep(t *testing.T) {
	tk := task.New("task-1", task.Input{Password: "secret"})
	_ = tk.Start()
	_ = tk.Complete(task.Result{Score: 10, Violations: []string{"a", "b"}})

	c := tk.Clone()
	c.Result.Score = 99
	c.Result.Violations[0] = "changed"

	if tk.Result.Score != 10 || !slices.Equal(tk.Result.Violations, []string{"a", "b"}) {
		t.Errorf("modifying the clone changed the original: %+v", tk.Result)
	}
}

func TestPasswordIsNeverPrinted(t *testing.T) {
	const secret = "S3cret-Pa$$word"
	tk := task.New("task-1", task.Input{Password: secret})

	outputs := []string{
		fmt.Sprintf("%v", tk),
		fmt.Sprintf("%+v", tk),
		fmt.Sprintf("%s", tk),
		fmt.Sprintf("%#v", tk.Input),
		fmt.Sprintf("%v", tk.Input),
		tk.String(),
	}
	for _, out := range outputs {
		if strings.Contains(out, secret) {
			t.Errorf("output leaks the password: %q", out)
		}
	}
}

func TestLevelString(t *testing.T) {
	tests := map[task.Level]string{
		task.LevelWeak:       "weak",
		task.LevelMedium:     "medium",
		task.LevelStrong:     "strong",
		task.LevelVeryStrong: "very strong",
	}
	for l, want := range tests {
		if got := l.String(); got != want {
			t.Errorf("Level(%d).String() = %q, want %q", int(l), got, want)
		}
	}
}
