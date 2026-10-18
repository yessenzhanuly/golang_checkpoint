// Command sis is a small demo of the SIS project (Checkpoint 1): it creates
// several password-analysis tasks, processes them and prints the results.
package main

import (
	"errors"
	"fmt"

	"sis/internal/password"
	"sis/internal/service"
	"sis/internal/store"
	"sis/internal/task"
)

func main() {
	svc := service.New(store.NewMemoryStore(), password.NewDefaultAnalyzer())

	inputs := []string{"abc", "password", "Tr0ub4dor&3", "correct-Horse-battery-9", ""}

	fmt.Println("== Creating and processing tasks ==")
	var firstID string
	for _, pw := range inputs {
		t, err := svc.Create(task.Input{Password: pw})
		if err != nil {
			fmt.Printf("rejected input: %v\n", err)
			continue
		}
		if firstID == "" {
			firstID = t.ID
		}

		done, err := svc.Process(t.ID)
		if err != nil {
			fmt.Printf("%s: processing failed: %v\n", t.ID, err)
			continue
		}
		printTask(done)
	}

	fmt.Println("\n== Error handling ==")
	_, err := svc.Process(firstID)
	fmt.Printf("process %s again: %v\n  invalid transition: %t\n",
		firstID, err, errors.Is(err, task.ErrInvalidTransition))

	_, err = svc.Get("task-999")
	fmt.Printf("get task-999: %v\n  not found: %t\n", err, errors.Is(err, task.ErrNotFound))

	fmt.Println("\n== All tasks ==")
	for _, t := range svc.List() {
		fmt.Println(t)
	}
}

func printTask(t task.Task) {
	fmt.Printf("%s [%s] input=%s\n", t.ID, t.Status, t.Input)
	if t.Result == nil {
		return
	}
	fmt.Printf("  score=%d level=%s\n", t.Result.Score, t.Result.Level)
	for _, v := range t.Result.Violations {
		fmt.Printf("  missing: %s\n", v)
	}
}
