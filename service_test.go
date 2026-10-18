package service_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"sis/internal/password"
	"sis/internal/service"
	"sis/internal/store"
	"sis/internal/task"
)

// newService builds a service with the real in-memory store and the real analyzer.
func newService() *service.Service {
	return service.New(store.NewMemoryStore(), password.NewDefaultAnalyzer())
}

func mustCreate(t *testing.T, svc *service.Service, pw string) task.Task {
	t.Helper()
	tk, err := svc.Create(task.Input{Password: pw})
	if err != nil {
		t.Fatalf("Create(%q) error = %v", pw, err)
	}
	return tk
}

func mustProcess(t *testing.T, svc *service.Service, id string) task.Task {
	t.Helper()
	tk, err := svc.Process(id)
	if err != nil {
		t.Fatalf("Process(%q) error = %v", id, err)
	}
	return tk
}

// processorFunc lets a test use a plain function as a service.Processor.
type processorFunc func(task.Input) (task.Result, error)

func (f processorFunc) Process(in task.Input) (task.Result, error) { return f(in) }

// 1. Successful task creation.
func TestCreateSuccess(t *testing.T) {
	svc := newService()

	tk, err := svc.Create(task.Input{Password: "Tr0ub4dor&3"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if tk.ID == "" {
		t.Error("created task has no ID")
	}
	if tk.Status != task.StatusPending {
		t.Errorf("Status = %s, want Pending", tk.Status)
	}
	if tk.Result != nil {
		t.Errorf("Result = %+v, want nil before processing", tk.Result)
	}
	if tk.Input.Password != "Tr0ub4dor&3" {
		t.Errorf("Input was not preserved: %v", tk.Input.Password)
	}
}

// 2. Invalid task input.
func TestCreateInvalidInput(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{"empty password", "", task.ErrEmptyPassword},
		{"too long password", strings.Repeat("x", task.MaxPasswordLength+1), task.ErrPasswordTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newService()

			tk, err := svc.Create(task.Input{Password: tt.password})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if tk.ID != "" {
				t.Errorf("a task was returned for invalid input: %+v", tk)
			}
			if n := len(svc.List()); n != 0 {
				t.Errorf("invalid input was stored: %d tasks in the store", n)
			}
		})
	}
}

func TestCreateAssignsUniqueIDs(t *testing.T) {
	svc := newService()
	seen := make(map[string]bool)
	for range 50 {
		tk := mustCreate(t, svc, "secret")
		if seen[tk.ID] {
			t.Fatalf("duplicate ID %q", tk.ID)
		}
		seen[tk.ID] = true
	}
}

func TestRejectedInputDoesNotConsumeAnID(t *testing.T) {
	svc := newService()
	first := mustCreate(t, svc, "one")
	_, _ = svc.Create(task.Input{Password: ""})
	second := mustCreate(t, svc, "two")

	if first.ID != "task-1" || second.ID != "task-2" {
		t.Errorf("IDs = %q, %q; want task-1, task-2", first.ID, second.ID)
	}
}

// 3. Successful task retrieval.
func TestGetSuccess(t *testing.T) {
	svc := newService()
	created := mustCreate(t, svc, "secret")

	got, err := svc.Get(created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.ID != created.ID || got.Input.Password != "secret" || got.Status != task.StatusPending {
		t.Errorf("Get() = %+v, want the created task", got)
	}
}

// 4. Retrieval of a nonexistent task.
func TestGetNotFound(t *testing.T) {
	svc := newService()
	mustCreate(t, svc, "secret")

	_, err := svc.Get("task-999")
	if !errors.Is(err, task.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

// 5, 6, 7. Successful processing, correct result, correct state.
func TestProcessSuccess(t *testing.T) {
	tests := []struct {
		name           string
		password       string
		wantScore      int
		wantLevel      task.Level
		wantViolations []string
	}{
		{
			name:     "weak password",
			password: "abc", wantScore: 30, wantLevel: task.LevelWeak,
			wantViolations: []string{
				"at least 8 characters", "at least 12 characters", "an uppercase letter",
				"a digit", "a symbol or punctuation mark",
			},
		},
		{
			name:     "common password",
			password: "password", wantScore: 20, wantLevel: task.LevelWeak,
			wantViolations: []string{
				"at least 12 characters", "an uppercase letter", "a digit",
				"a symbol or punctuation mark", "not a commonly used password",
			},
		},
		{
			name:     "strong password",
			password: "Tr0ub4dor&3", wantScore: 85, wantLevel: task.LevelStrong,
			wantViolations: []string{"at least 12 characters"},
		},
		{
			name:     "very strong password",
			password: "correct-Horse-battery-9", wantScore: 100, wantLevel: task.LevelVeryStrong,
			wantViolations: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newService()
			created := mustCreate(t, svc, tt.password)

			done := mustProcess(t, svc, created.ID)

			// 7. State.
			if done.Status != task.StatusCompleted {
				t.Errorf("Status = %s, want Completed", done.Status)
			}
			// 6. Result.
			if done.Result == nil {
				t.Fatal("Result is nil after processing")
			}
			if done.Result.Score != tt.wantScore || done.Result.Level != tt.wantLevel {
				t.Errorf("Result = score %d / %s, want %d / %s",
					done.Result.Score, done.Result.Level, tt.wantScore, tt.wantLevel)
			}
			if !slices.Equal(done.Result.Violations, tt.wantViolations) {
				t.Errorf("Violations = %q, want %q", done.Result.Violations, tt.wantViolations)
			}

			// The result is really stored, not only returned.
			stored, err := svc.Get(created.ID)
			if err != nil {
				t.Fatalf("Get() error = %v", err)
			}
			if stored.Status != task.StatusCompleted || stored.Result == nil || stored.Result.Score != tt.wantScore {
				t.Errorf("stored task = %+v, want a completed task with score %d", stored, tt.wantScore)
			}
		})
	}
}

func TestProcessNotFound(t *testing.T) {
	svc := newService()
	_, err := svc.Process("task-999")
	if !errors.Is(err, task.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

// 8. Invalid operation / invalid state transition.
func TestProcessCompletedTaskAgain(t *testing.T) {
	svc := newService()
	created := mustCreate(t, svc, "Tr0ub4dor&3")
	first := mustProcess(t, svc, created.ID)

	_, err := svc.Process(created.ID)
	if !errors.Is(err, task.ErrInvalidTransition) {
		t.Fatalf("error = %v, want ErrInvalidTransition", err)
	}
	var te *task.TransitionError
	if !errors.As(err, &te) || te.From != task.StatusCompleted || te.To != task.StatusProcessing {
		t.Errorf("TransitionError = %+v, want Completed -> Processing", te)
	}

	// The completed task and its result must be untouched.
	stored, _ := svc.Get(created.ID)
	if stored.Status != task.StatusCompleted || stored.Result == nil || stored.Result.Score != first.Result.Score {
		t.Errorf("rejected operation changed the task: %+v", stored)
	}
}

// The stored state must follow the real lifecycle, including Processing.
func TestStateIsProcessingWhileProcessorRuns(t *testing.T) {
	var svc *service.Service
	var id string
	var observed task.Status

	proc := processorFunc(func(task.Input) (task.Result, error) {
		tk, err := svc.Get(id)
		if err != nil {
			t.Errorf("Get() during processing error = %v", err)
		}
		observed = tk.Status
		return task.Result{Score: 1}, nil
	})
	svc = service.New(store.NewMemoryStore(), proc)
	id = mustCreate(t, svc, "secret").ID

	mustProcess(t, svc, id)

	if observed != task.StatusProcessing {
		t.Errorf("status seen inside the processor = %s, want Processing", observed)
	}
}

func TestProcessorFailureMarksTaskFailed(t *testing.T) {
	errBoom := errors.New("boom")
	svc := service.New(store.NewMemoryStore(),
		processorFunc(func(task.Input) (task.Result, error) { return task.Result{}, errBoom }))
	created := mustCreate(t, svc, "secret")

	got, err := svc.Process(created.ID)
	if !errors.Is(err, errBoom) {
		t.Fatalf("error = %v, want it to wrap the processor error", err)
	}
	if got.Status != task.StatusFailed || got.FailReason != "boom" || got.Result != nil {
		t.Errorf("returned task = %+v, want Failed / reason boom / no result", got)
	}

	stored, _ := svc.Get(created.ID)
	if stored.Status != task.StatusFailed || stored.FailReason != "boom" || stored.Result != nil {
		t.Errorf("stored task = %+v, want Failed / reason boom / no result", stored)
	}

	// Failed is terminal.
	if _, err := svc.Process(created.ID); !errors.Is(err, task.ErrInvalidTransition) {
		t.Errorf("reprocessing a failed task: error = %v, want ErrInvalidTransition", err)
	}
}

func TestProcessWithoutRulesFailsTask(t *testing.T) {
	svc := service.New(store.NewMemoryStore(), password.NewAnalyzer())
	created := mustCreate(t, svc, "secret")

	_, err := svc.Process(created.ID)
	if !errors.Is(err, password.ErrNoRules) {
		t.Fatalf("error = %v, want ErrNoRules", err)
	}
	stored, _ := svc.Get(created.ID)
	if stored.Status != task.StatusFailed {
		t.Errorf("Status = %s, want Failed", stored.Status)
	}
}

// 9. Multiple independent tasks.
func TestMultipleIndependentTasks(t *testing.T) {
	svc := newService()
	weak := mustCreate(t, svc, "abc")
	strong := mustCreate(t, svc, "Tr0ub4dor&3")
	untouched := mustCreate(t, svc, "correct-Horse-battery-9")

	if weak.ID == strong.ID || strong.ID == untouched.ID || weak.ID == untouched.ID {
		t.Fatalf("IDs are not unique: %s, %s, %s", weak.ID, strong.ID, untouched.ID)
	}

	// Process only two of the three tasks.
	mustProcess(t, svc, weak.ID)
	mustProcess(t, svc, strong.ID)

	gotWeak, _ := svc.Get(weak.ID)
	gotStrong, _ := svc.Get(strong.ID)
	gotUntouched, _ := svc.Get(untouched.ID)

	if gotWeak.Status != task.StatusCompleted || gotWeak.Result == nil || gotWeak.Result.Score != 30 {
		t.Errorf("weak task = %+v, want completed with score 30", gotWeak)
	}
	if gotStrong.Status != task.StatusCompleted || gotStrong.Result == nil || gotStrong.Result.Score != 85 {
		t.Errorf("strong task = %+v, want completed with score 85", gotStrong)
	}
	if gotUntouched.Status != task.StatusPending || gotUntouched.Result != nil {
		t.Errorf("untouched task = %+v, want pending without a result", gotUntouched)
	}

	// Inputs were not mixed up.
	if gotWeak.Input.Password != "abc" || gotStrong.Input.Password != "Tr0ub4dor&3" ||
		gotUntouched.Input.Password != "correct-Horse-battery-9" {
		t.Error("task inputs were mixed up")
	}

	// List returns everything, in creation order.
	var ids []string
	for _, tk := range svc.List() {
		ids = append(ids, tk.ID)
	}
	if want := []string{weak.ID, strong.ID, untouched.ID}; !slices.Equal(ids, want) {
		t.Errorf("List() IDs = %v, want %v", ids, want)
	}
}

func TestReturnedTaskIsACopy(t *testing.T) {
	svc := newService()
	created := mustCreate(t, svc, "Tr0ub4dor&3")
	done := mustProcess(t, svc, created.ID)

	// Mutating what the service returned must not change what it stores.
	done.Status = task.StatusPending
	done.Result.Score = 0
	done.Result.Violations = append(done.Result.Violations, "tampered")

	stored, _ := svc.Get(created.ID)
	if stored.Status != task.StatusCompleted || stored.Result.Score != 85 || len(stored.Result.Violations) != 1 {
		t.Errorf("stored task was changed through a returned copy: %+v", stored)
	}
}
